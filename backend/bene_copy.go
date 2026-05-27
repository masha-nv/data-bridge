package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	beneCopyExecutionTimeout = 12 * time.Hour
)

type beneCopyRequest struct {
	SourceEnvironment string `json:"sourceEnvironment"`
	TargetEnvironment string `json:"targetEnvironment"`
	BeneLinkPartKey   string `json:"beneLinkPartKey,omitempty"`
	BeneLinkKey       string `json:"beneLinkKey"`
	Engine            string `json:"engine,omitempty"`
}

type beneCopyDatabaseResult struct {
	DatabaseName string `json:"databaseName"`
	TableCount   int    `json:"tableCount,omitempty"`
	MatchedTables int   `json:"matchedTables,omitempty"`
	DiscoveredRows int  `json:"discoveredRows,omitempty"`
	CopiedRows   int    `json:"copiedRows"`
	SkippedRows  int    `json:"skippedRows"`
	Success      bool   `json:"success"`
	Message      string `json:"message,omitempty"`
}

type beneCopyResponse struct {
	Success           bool                     `json:"success"`
	Message           string                   `json:"message"`
	SourceEnvironment string                   `json:"sourceEnvironment"`
	TargetEnvironment string                   `json:"targetEnvironment"`
	BeneLinkKey       string                   `json:"beneLinkKey"`
	Results           []beneCopyDatabaseResult `json:"results,omitempty"`
}

type beneCopyTraversalItem struct {
	TableName           string
	TableHasBeneLinkKey bool
	ParentTableName     string
	ParentColumns       []beneCopyColumnMapping
	Depth               int
}

type beneCopyTraversalPlan struct {
	DatabaseName string
	Items        []beneCopyTraversalItem
}

func beneCopyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req beneCopyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	req.SourceEnvironment = strings.TrimSpace(req.SourceEnvironment)
	req.TargetEnvironment = strings.TrimSpace(req.TargetEnvironment)
	req.BeneLinkPartKey = strings.TrimSpace(req.BeneLinkPartKey)
	req.BeneLinkKey = strings.TrimSpace(req.BeneLinkKey)
	req.Engine = beneCopyEngineMarxMovedata

	if req.SourceEnvironment == "" || req.TargetEnvironment == "" || req.BeneLinkPartKey == "" || req.BeneLinkKey == "" {
		http.Error(w, "sourceEnvironment, targetEnvironment, beneLinkPartKey, and beneLinkKey are required", http.StatusBadRequest)
		return
	}

	if _, err := strconv.ParseInt(req.BeneLinkPartKey, 10, 16); err != nil {
		http.Error(w, fmt.Sprintf("beneLinkPartKey must be numeric: %v", err), http.StatusBadRequest)
		return
	}
	if _, err := strconv.ParseInt(req.BeneLinkKey, 10, 32); err != nil {
		http.Error(w, fmt.Sprintf("beneLinkKey must be numeric: %v", err), http.StatusBadRequest)
		return
	}

	if req.SourceEnvironment == req.TargetEnvironment {
		http.Error(w, "sourceEnvironment and targetEnvironment must be different", http.StatusBadRequest)
		return
	}

	if strings.EqualFold(req.TargetEnvironment, "Prod2") {
		http.Error(w, "targetEnvironment cannot be Prod2", http.StatusBadRequest)
		return
	}

	if err := validateBeneCopyTargetEnvironment(req.TargetEnvironment); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validateMovedataPWAEnvironments(req.SourceEnvironment, req.TargetEnvironment); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	authContext, err := buildBeneCopyJobAuthContext()
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	job := enqueueBeneCopyJob(req, authContext)

	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, buildBeneCopyJobSubmissionResponse(job))
}

func validateMovedataPWAEnvironments(sourceEnvironment, targetEnvironment string) error {
	for _, environment := range []string{sourceEnvironment, targetEnvironment} {
		definition, err := getDatabaseDefinition(environment, databaseNamePWA)
		if err != nil {
			return err
		}

		if getAppMode() == appModeReal {
			if strings.TrimSpace(definition.DBDriver) == "" || strings.TrimSpace(definition.DBHost) == "" || strings.TrimSpace(definition.DBName) == "" {
				return fmt.Errorf("PWA database is not fully configured for environment %s", environment)
			}
		}
	}

	return nil
}

func validateBeneCopyTargetEnvironment(targetEnvironment string) error {
	targetDefinition, err := getDatabaseDefinition(targetEnvironment, databaseNameMARx)
	if err != nil {
		return err
	}

	if isReadOnlyMARxEndpoint(targetDefinition.DBHost) {
		return fmt.Errorf(
			"target MARx database %s is configured with a read-only endpoint (%s); use the writer endpoint for bene copy",
			targetEnvironment,
			strings.TrimSpace(targetDefinition.DBHost),
		)
	}

	return nil
}

func buildBeneCopyJobAuthContext() (beneCopyJobAuthContext, error) {
	if getAppMode() != appModeReal {
		return beneCopyJobAuthContext{}, nil
	}

	session, ok := getActiveRealSession()
	if !ok {
		return beneCopyJobAuthContext{}, fmt.Errorf("no active real database session; log in first")
	}

	return beneCopyJobAuthContext{
		UserID:      session.UserID,
		Password:    session.Password,
		DisplayName: session.DisplayName,
	}, nil
}

type beneCopyExecutionResult struct {
	Success     bool
	CopiedRows  int
	SkippedRows int
	Message     string
}

type beneCopyTableError struct {
	TableName string
	Err       error
}

func (err *beneCopyTableError) Error() string {
	if strings.TrimSpace(err.TableName) == "" {
		return err.Err.Error()
	}

	return fmt.Sprintf("table %s: %v", err.TableName, err.Err)
}

func (err *beneCopyTableError) Unwrap() error {
	return err.Err
}

type beneCopyFilterBuilder func(alias string, nextAliasIndex *int) (string, []any, error)

type beneCopyProgress struct {
	CurrentTable string
	CopiedRows   int
	SkippedRows  int
}

type beneCopyProgressFunc func(progress beneCopyProgress)

func buildBeneCopyTraversalPlan(databaseName string, config beneCopyTableConfig) beneCopyTraversalPlan {
	items := make([]beneCopyTraversalItem, 0)
	for _, node := range config.Tables {
		appendBeneCopyTraversalItem(&items, node, 0)
	}

	return beneCopyTraversalPlan{
		DatabaseName: databaseName,
		Items:        items,
	}
}

func appendBeneCopyTraversalItem(items *[]beneCopyTraversalItem, node beneCopyTableNode, depth int) {
	definition := node.Table
	item := beneCopyTraversalItem{
		TableName:           definition.TableName,
		TableHasBeneLinkKey: definition.TableHasBeneLinkKey,
		Depth:               depth,
	}
	if definition.ParentTable != nil {
		item.ParentTableName = definition.ParentTable.TableName
		item.ParentColumns = append(item.ParentColumns, definition.ParentTable.Columns...)
	}

	*items = append(*items, item)

	for _, child := range definition.Tables {
		appendBeneCopyTraversalItem(items, child, depth+1)
	}
}

func buildBeneCopyFilterBuilder(definition beneCopyTableDefinition, beneLinkPartKey int64, beneLinkKey int64, parentFilterBuilder beneCopyFilterBuilder) beneCopyFilterBuilder {
	if definition.ParentTable != nil && parentFilterBuilder != nil {
		parentTable := *definition.ParentTable
		return func(alias string, nextAliasIndex *int) (string, []any, error) {
			parentAlias := fmt.Sprintf("t%d", *nextAliasIndex)
			*nextAliasIndex++

			joinConditions := make([]string, 0, len(parentTable.Columns))
			for _, column := range parentTable.Columns {
				joinConditions = append(joinConditions, fmt.Sprintf("%s.%s = %s.%s", parentAlias, column.NameParent, alias, column.NameChild))
			}

			parentFilter, parentArgs, err := parentFilterBuilder(parentAlias, nextAliasIndex)
			if err != nil {
				return "", nil, err
			}

			conditions := joinConditions
			if strings.TrimSpace(parentFilter) != "" {
				conditions = append(conditions, parentFilter)
			}

			return fmt.Sprintf("EXISTS (SELECT 1 FROM %s %s WHERE %s)", parentTable.TableName, parentAlias, strings.Join(conditions, " AND ")), parentArgs, nil
		}
	}

	if definition.RootFilterColumns != nil {
		rootFilterColumns := *definition.RootFilterColumns
		return func(alias string, nextAliasIndex *int) (string, []any, error) {
			conditions := make([]string, 0, 2)
			args := make([]any, 0, 2)

			if strings.TrimSpace(rootFilterColumns.BeneLinkPartKey) != "" {
				conditions = append(conditions, fmt.Sprintf("%s.%s = ?", alias, rootFilterColumns.BeneLinkPartKey))
				args = append(args, beneLinkPartKey)
			}

			if strings.TrimSpace(rootFilterColumns.BeneLinkKey) != "" {
				conditions = append(conditions, fmt.Sprintf("%s.%s = ?", alias, rootFilterColumns.BeneLinkKey))
				args = append(args, beneLinkKey)
			}

			if len(conditions) == 0 {
				return "", nil, fmt.Errorf("table %s defines RootFilterColumns without any column names", definition.TableName)
			}

			return strings.Join(conditions, " AND "), args, nil
		}
	}

	if definition.TableHasBeneLinkKey {
		return func(alias string, nextAliasIndex *int) (string, []any, error) {
			return fmt.Sprintf("%s.BENE_LINK_KEY = ?", alias), []any{beneLinkKey}, nil
		}
	}

	return nil
}

func copyBeneCopyMARxRows(ctx context.Context, jobID, sourceEnvironment, targetEnvironment, beneLinkKey string, config beneCopyTableConfig, progress beneCopyProgressFunc, authContext beneCopyJobAuthContext) (beneCopyExecutionResult, error) {
	parsedBeneLinkKey, err := strconv.ParseInt(strings.TrimSpace(beneLinkKey), 10, 64)
	if err != nil {
		return beneCopyExecutionResult{}, fmt.Errorf("beneLinkKey must be numeric: %w", err)
	}

	targetDefinition, err := getDatabaseDefinition(targetEnvironment, databaseNameMARx)
	if err != nil {
		return beneCopyExecutionResult{}, err
	}

	if isReadOnlyMARxEndpoint(targetDefinition.DBHost) {
		return beneCopyExecutionResult{}, fmt.Errorf(
			"target MARx database %s is configured with a read-only endpoint (%s); use the writer endpoint for bene copy",
			targetEnvironment,
			strings.TrimSpace(targetDefinition.DBHost),
		)
	}

	sourceDB, err := openDatabaseWithCredentials(sourceEnvironment, databaseNameMARx, authContext.UserID, authContext.Password)
	if err != nil {
		return beneCopyExecutionResult{}, err
	}
	defer sourceDB.Close()

	targetDB, err := openDatabaseWithCredentials(targetEnvironment, databaseNameMARx, authContext.UserID, authContext.Password)
	if err != nil {
		return beneCopyExecutionResult{}, err
	}
	defer targetDB.Close()

	tx, err := targetDB.BeginTx(ctx, nil)
	if err != nil {
		return beneCopyExecutionResult{}, err
	}

	result := beneCopyExecutionResult{}
	for _, node := range config.Tables {
		if err := copyBeneCopyRowsForNode(ctx, jobID, tx, sourceDB, sourceEnvironment, targetEnvironment, node, parsedBeneLinkKey, nil, &result, progress); err != nil {
			tx.Rollback()
			return beneCopyExecutionResult{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return beneCopyExecutionResult{}, err
	}

	result.Success = true
	result.Message = fmt.Sprintf("MARx copy complete; copied %d rows and skipped %d duplicates", result.CopiedRows, result.SkippedRows)
	return result, nil
}

func copyBeneCopyRowsForNode(ctx context.Context, jobID string, tx *sql.Tx, sourceDB *sql.DB, sourceEnvironment, targetEnvironment string, node beneCopyTableNode, beneLinkKey int64, parentFilterBuilder beneCopyFilterBuilder, result *beneCopyExecutionResult, progress beneCopyProgressFunc) error {
	definition := node.Table
		currentFilterBuilder := buildBeneCopyFilterBuilder(definition, 0, beneLinkKey, parentFilterBuilder)
	if currentFilterBuilder != nil {
		tableStart := time.Now()
		persistBeneCopyTableStarted(jobID, definition.TableName, tableStart)
		if progress != nil {
			progress(beneCopyProgress{
				CurrentTable: definition.TableName,
				CopiedRows:   result.CopiedRows,
				SkippedRows:  result.SkippedRows,
			})
		}

		log.Printf(
			"bene copy table started: table=%s source=%s target=%s beneLinkKey=%d totalCopied=%d totalSkipped=%d",
			definition.TableName,
			sourceEnvironment,
			targetEnvironment,
			beneLinkKey,
			result.CopiedRows,
			result.SkippedRows,
		)

		copiedRows, skippedRows, err := copyRowsWithFilter(ctx, tx, sourceDB, sourceEnvironment, targetEnvironment, definition.TableName, currentFilterBuilder)
		if err != nil {
			persistBeneCopyTableFailed(jobID, definition.TableName, tableStart, time.Now(), copiedRows, skippedRows, err)
			log.Printf(
				"bene copy table failed: table=%s source=%s target=%s beneLinkKey=%d duration=%s copied=%d skipped=%d err=%v",
				definition.TableName,
				sourceEnvironment,
				targetEnvironment,
				beneLinkKey,
				time.Since(tableStart).Round(time.Millisecond),
				copiedRows,
				skippedRows,
				err,
			)
			return &beneCopyTableError{TableName: definition.TableName, Err: err}
		}
		result.CopiedRows += copiedRows
		result.SkippedRows += skippedRows
		persistBeneCopyTableCompleted(jobID, definition.TableName, tableStart, time.Now(), copiedRows, skippedRows)

		log.Printf(
			"bene copy table completed: table=%s source=%s target=%s beneLinkKey=%d duration=%s copied=%d skipped=%d totalCopied=%d totalSkipped=%d",
			definition.TableName,
			sourceEnvironment,
			targetEnvironment,
			beneLinkKey,
			time.Since(tableStart).Round(time.Millisecond),
			copiedRows,
			skippedRows,
			result.CopiedRows,
			result.SkippedRows,
		)

		if progress != nil {
			progress(beneCopyProgress{
				CurrentTable: definition.TableName,
				CopiedRows:   result.CopiedRows,
				SkippedRows:  result.SkippedRows,
			})
		}
	}

	for _, child := range definition.Tables {
		if err := copyBeneCopyRowsForNode(ctx, jobID, tx, sourceDB, sourceEnvironment, targetEnvironment, child, beneLinkKey, currentFilterBuilder, result, progress); err != nil {
			return err
		}
	}

	return nil
}

func copyRowsWithFilter(ctx context.Context, tx *sql.Tx, sourceDB *sql.DB, sourceEnvironment, targetEnvironment, tableName string, filterBuilder beneCopyFilterBuilder) (int, int, error) {
	aliasIndex := 1
	filter, args, err := filterBuilder("t0", &aliasIndex)
	if err != nil {
		return 0, 0, err
	}

	selectQuery := fmt.Sprintf("SELECT t0.* FROM %s t0 WHERE %s", tableName, filter)
	reboundSelectQuery, err := rebindQueryForDatabase(sourceEnvironment, databaseNameMARx, selectQuery)
	if err != nil {
		return 0, 0, err
	}

	rows, err := sourceDB.QueryContext(ctx, reboundSelectQuery, args...)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return 0, 0, err
	}

	insertQuery, err := buildInsertQueryForTable(targetEnvironment, tableName, columns)
	if err != nil {
		return 0, 0, err
	}

	copiedRows := 0
	skippedRows := 0
	for rows.Next() {
		values := make([]any, len(columns))
		valuePointers := make([]any, len(columns))
		for index := range values {
			valuePointers[index] = &values[index]
		}

		if err := rows.Scan(valuePointers...); err != nil {
			return copiedRows, skippedRows, err
		}

		if _, err := tx.ExecContext(ctx, insertQuery, values...); err != nil {
			if isDuplicateInsertError(err) {
				skippedRows++
				continue
			}
			return copiedRows, skippedRows, err
		}

		copiedRows++
	}

	if err := rows.Err(); err != nil {
		return copiedRows, skippedRows, err
	}

	return copiedRows, skippedRows, nil
}

func buildInsertQueryForTable(environment, tableName string, columns []string) (string, error) {
	placeholders := make([]string, len(columns))
	for index := range columns {
		placeholders[index] = "?"
	}

	insertQuery := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	return rebindQueryForDatabase(environment, databaseNameMARx, insertQuery)
}

func isDuplicateInsertError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	duplicateIndicators := []string{
		"unique constraint failed",
		"unique constraint violation",
		"duplicate key value violates unique constraint",
		"constraint failed",
		"duplicate entry",
	}

	for _, indicator := range duplicateIndicators {
		if strings.Contains(message, indicator) {
			return true
		}
	}

	return false
}

func isReadOnlyMARxEndpoint(host string) bool {
	normalizedHost := strings.ToLower(strings.TrimSpace(host))
	return strings.Contains(normalizedHost, ".cluster-ro-")
}

func beneCopyTableNameFromError(err error) string {
	var tableErr *beneCopyTableError
	if errors.As(err, &tableErr) {
		return strings.TrimSpace(tableErr.TableName)
	}

	return ""
}