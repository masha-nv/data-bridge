package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type movedataStyleDatabasesMetadata []*movedataStyleDatabaseMetadata

type movedataStyleDatabaseMetadata struct {
	DatabaseType  string
	Db            *movedataStyleDB
	TableMetadata *map[string]any
	Execution     *movedataStyleExecutionContext
}

type movedataStyleDB struct {
	SourceDBReader      *sql.DB
	SourceDBWriter      *sql.DB
	DestinationDBWriter *sql.DB
}

type movedataStyleTxDB struct {
	databaseType        string
	sourceDBReader      *sql.DB
	sourceTxWriter      *sql.Tx
	destinationTxWriter *sql.Tx
}

type movedataStyleTask struct {
	Db                  *movedataStyleTxDB
	BeneLinkPartitionKey int16
	BeneLinkKey         int32
	MoveData            bool
	Context             *context.Context
	Execution           *movedataStyleExecutionContext
	SelectedRows        sync.Map
	CopiedRows          sync.Map
	DeletedRows         sync.Map
}

type movedataStyleExecutionContext struct {
	BaseContext        context.Context
	JobID              string
	SourceEnvironment  string
	TargetEnvironment  string
	Progress           beneCopyProgressFunc
	mu                 sync.Mutex
	CopiedRows         int
	SkippedRows        int
	CurrentTable       string
}

func newMovedataStyleDatabaseMetadata(databaseType string, dbReaderSource *sql.DB, dbWriterSource *sql.DB, dbWriterDestination *sql.DB, tableMetadataFile string) (*movedataStyleDatabaseMetadata, error) {
	tableMetadata, err := buildMovedataStyleTableMetadata(tableMetadataFile)
	if err != nil {
		return nil, fmt.Errorf("error buildTableMetadata %w", err)
	}

	db := &movedataStyleDB{
		SourceDBReader:      dbReaderSource,
		SourceDBWriter:      dbWriterSource,
		DestinationDBWriter: dbWriterDestination,
	}

	return &movedataStyleDatabaseMetadata{
		DatabaseType:  databaseType,
		Db:            db,
		TableMetadata: tableMetadata,
	}, nil
}

func buildMovedataStyleTableMetadata(tableMetadataFile string) (*map[string]any, error) {
	configPath, found, err := resolveBeneCopyConfigPath(tableMetadataFile)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("bene copy table config %s was not found", tableMetadataFile)
	}

	dataBytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error Reading File in TableMetadata %w", err)
	}

	var tables map[string]any
	if err := json.Unmarshal(dataBytes, &tables); err != nil {
		return nil, fmt.Errorf("error Unmarshalling File in TableMetadata %w", err)
	}

	return &tables, nil
}

func (databasesMetadata *movedataStyleDatabasesMetadata) Copy(beneLinkPartKey int16, beneLinkKey int32) error {
	return databasesMetadata.doRun(beneLinkPartKey, beneLinkKey)
}

func (databasesMetadata *movedataStyleDatabasesMetadata) doRun(beneLinkPartKey int16, beneLinkKey int32) error {
	baseContext := context.Background()
	if execution := databasesMetadata.executionContext(); execution != nil && execution.BaseContext != nil {
		baseContext = execution.BaseContext
	}

	eg, ctx := errgroup.WithContext(baseContext)

	dbTypeTransactions, err := databasesMetadata.createDatabaseTransactions()
	if err != nil {
		return err
	}

	for _, databaseMetadata := range *databasesMetadata {
		databaseMetadata := databaseMetadata
		eg.Go(func() error {
			if databaseMetadata == nil {
				return fmt.Errorf("movedata-style database metadata is nil")
			}

			txDB, ok := dbTypeTransactions[databaseMetadata.DatabaseType]
			if !ok {
				return fmt.Errorf("no movedata-style transaction found for database type %s", databaseMetadata.DatabaseType)
			}

			task := databaseMetadata.createTask(false, beneLinkPartKey, beneLinkKey, &ctx, txDB)
			return task.run(databaseMetadata.TableMetadata)
		})
	}

	err = eg.Wait()

	for _, txDB := range dbTypeTransactions {
		stopMovedataStyleTransaction(txDB.sourceTxWriter, err)
		stopMovedataStyleTransaction(txDB.destinationTxWriter, err)
	}

	return err
}

func (databasesMetadata *movedataStyleDatabasesMetadata) createDatabaseTransactions() (map[string]*movedataStyleTxDB, error) {
	dbTypeTransactions := make(map[string]*movedataStyleTxDB)

	for _, databaseMetadata := range *databasesMetadata {
		if databaseMetadata == nil {
			return nil, fmt.Errorf("movedata-style database metadata is nil")
		}
		if databaseMetadata.Db == nil {
			return nil, fmt.Errorf("movedata-style db is nil for database type %s", databaseMetadata.DatabaseType)
		}

		sourceTxWriter, err := startMovedataStyleTransaction(databaseMetadata.Db.SourceDBWriter)
		if err != nil {
			return nil, err
		}

		destinationTxWriter, err := startMovedataStyleTransaction(databaseMetadata.Db.DestinationDBWriter)
		if err != nil {
			stopMovedataStyleTransaction(sourceTxWriter, err)
			return nil, err
		}

		dbTypeTransactions[databaseMetadata.DatabaseType] = &movedataStyleTxDB{
			databaseType:        databaseMetadata.DatabaseType,
			sourceDBReader:      databaseMetadata.Db.SourceDBReader,
			sourceTxWriter:      sourceTxWriter,
			destinationTxWriter: destinationTxWriter,
		}
	}

	return dbTypeTransactions, nil
}

func (databaseMetadata *movedataStyleDatabaseMetadata) createTask(moveData bool, beneLinkPartKey int16, beneLinkKey int32, ctx *context.Context, db *movedataStyleTxDB) *movedataStyleTask {
	return &movedataStyleTask{
		Db:                   db,
		BeneLinkPartitionKey: beneLinkPartKey,
		BeneLinkKey:          beneLinkKey,
		MoveData:             moveData,
		Context:              ctx,
		Execution:            databaseMetadata.Execution,
	}
}

func (databasesMetadata *movedataStyleDatabasesMetadata) attachExecutionContext(execution *movedataStyleExecutionContext) {
	for _, databaseMetadata := range *databasesMetadata {
		if databaseMetadata == nil {
			continue
		}
		databaseMetadata.Execution = execution
	}
}

func (databasesMetadata *movedataStyleDatabasesMetadata) executionContext() *movedataStyleExecutionContext {
	for _, databaseMetadata := range *databasesMetadata {
		if databaseMetadata != nil && databaseMetadata.Execution != nil {
			return databaseMetadata.Execution
		}
	}

	return nil
}

func (task *movedataStyleTask) run(tables *map[string]any) error {
	if tables == nil {
		return fmt.Errorf("movedata-style table metadata is nil")
	}

	err := task.processTables(*tables, nil)
	if err != nil {
		return fmt.Errorf("error in run.processTables - %w", err)
	}

	if !task.MoveData {
		return nil
	}

	validationErr := error(nil)
	task.SelectedRows.Range(func(tableName, rowsSelected any) bool {
		rowsCopied, okCopy := task.CopiedRows.Load(tableName)
		rowsDeleted, okDelete := task.DeletedRows.Load(tableName)
		if !okCopy || !okDelete {
			validationErr = fmt.Errorf("movedata-style row counts missing for table %v", tableName)
			return false
		}

		selectedCount, okSelected := rowsSelected.(int)
		copiedCount, okCopied := rowsCopied.(int64)
		deletedCount, okDeleted := rowsDeleted.(int64)
		if !okSelected || !okCopied || !okDeleted {
			validationErr = fmt.Errorf("movedata-style row count types are invalid for table %v", tableName)
			return false
		}

		if int64(selectedCount) != copiedCount || int64(selectedCount) != deletedCount {
			validationErr = errors.New(fmt.Sprintf(
				"no of rows selected - %d or no of rows copied - %d or no of rows deleted - %d, are not equal for Table - %v",
				selectedCount,
				copiedCount,
				deletedCount,
				tableName,
			))
			return false
		}

		return true
	})

	return validationErr
}

func (task *movedataStyleTask) processTable(tableObj any, ctx context.Context, parentFilterBuilder beneCopyFilterBuilder) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	tableWrapper, ok := tableObj.(map[string]any)
	if !ok {
		return fmt.Errorf("movedata-style table object has unexpected type %T", tableObj)
	}

	table, ok := tableWrapper["Table"].(map[string]any)
	if !ok {
		return fmt.Errorf("movedata-style table entry is missing Table metadata")
	}

	definition, err := movedataStyleTableDefinitionFromMap(table)
	if err != nil {
		return err
	}

	currentFilterBuilder := buildBeneCopyFilterBuilder(definition, int64(task.BeneLinkKey), parentFilterBuilder)
	if currentFilterBuilder != nil {
		err = task.doCopy(table, currentFilterBuilder)
		if err != nil {
			return fmt.Errorf("error in doCopy function - %w", err)
		}
	}

	err = task.processTables(table, currentFilterBuilder)
	if err != nil {
		return fmt.Errorf("error in processTables function - %w", err)
	}

	if task.MoveData {
		err = task.doDelete(table)
		if err != nil {
			return fmt.Errorf("error in doDelete function - %w", err)
		}
	}

	return nil
}

func (task *movedataStyleTask) processTables(tablesObj map[string]any, parentFilterBuilder beneCopyFilterBuilder) error {
	eg, ctx := errgroup.WithContext(*task.Context)

	rawTables, ok := tablesObj["Tables"]
	if !ok {
		return nil
	}

	tables, ok := rawTables.([]any)
	if !ok {
		return fmt.Errorf("movedata-style Tables metadata has unexpected type %T", rawTables)
	}

	for _, tableObj := range tables {
		tableObj := tableObj
		eg.Go(func() error {
			return task.processTable(tableObj, ctx, parentFilterBuilder)
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

func (task *movedataStyleTask) doCopy(table map[string]any, filterBuilder beneCopyFilterBuilder) error {
	tableName, _ := table["TableName"].(string)
	if task.Context == nil || *task.Context == nil {
		return fmt.Errorf("movedata-style context is nil for table %s", tableName)
	}
	if task.Db == nil {
		return fmt.Errorf("movedata-style db is nil for table %s", tableName)
	}

	tableStartedAt := time.Now()
	if execution := task.Execution; execution != nil && strings.TrimSpace(execution.JobID) != "" {
		persistBeneCopyTableStarted(execution.JobID, tableName, tableStartedAt)
	}

	selectedRows, copiedRows, err := movedataStyleCopyRowsWithFilter(*task.Context, task.Db.destinationTxWriter, task.Db.sourceDBReader, task.Db.databaseType, tableName, filterBuilder)
	if err != nil {
		if execution := task.Execution; execution != nil && strings.TrimSpace(execution.JobID) != "" {
			persistBeneCopyTableFailed(execution.JobID, tableName, tableStartedAt, time.Now(), int(copiedRows), max(0, selectedRows-int(copiedRows)), err)
		}
		return fmt.Errorf("copy rows for table %s: %w", tableName, err)
	}

	task.SelectedRows.Store(tableName, selectedRows)
	task.CopiedRows.Store(tableName, copiedRows)
	skippedRows := max(0, selectedRows-int(copiedRows))
	if execution := task.Execution; execution != nil {
		execution.mu.Lock()
		execution.CurrentTable = tableName
		execution.CopiedRows += int(copiedRows)
		execution.SkippedRows += skippedRows
		copiedTotal := execution.CopiedRows
		skippedTotal := execution.SkippedRows
		progress := execution.Progress
		jobID := execution.JobID
		execution.mu.Unlock()

		if strings.TrimSpace(jobID) != "" {
			persistBeneCopyTableCompleted(jobID, tableName, tableStartedAt, time.Now(), int(copiedRows), skippedRows)
		}

		if progress != nil {
			progress(beneCopyProgress{
				CurrentTable: tableName,
				CopiedRows:   copiedTotal,
				SkippedRows:  skippedTotal,
			})
		}
	}
	return nil
}

func (task *movedataStyleTask) doDelete(table map[string]any) error {
	tableName, _ := table["TableName"].(string)
	log.Printf("movedata-style doDelete placeholder reached for table=%s beneLinkKey=%d", tableName, task.BeneLinkKey)
	return nil
}

func movedataStyleTableDefinitionFromMap(table map[string]any) (beneCopyTableDefinition, error) {
	dataBytes, err := json.Marshal(table)
	if err != nil {
		return beneCopyTableDefinition{}, fmt.Errorf("marshal movedata-style table metadata: %w", err)
	}

	var definition beneCopyTableDefinition
	if err := json.Unmarshal(dataBytes, &definition); err != nil {
		return beneCopyTableDefinition{}, fmt.Errorf("parse movedata-style table metadata: %w", err)
	}

	return definition, nil
}

func movedataStyleCopyRowsWithFilter(ctx context.Context, tx *sql.Tx, sourceDB *sql.DB, databaseType, tableName string, filterBuilder beneCopyFilterBuilder) (int, int64, error) {
	if tx == nil {
		return 0, 0, fmt.Errorf("destination transaction is nil")
	}
	if sourceDB == nil {
		return 0, 0, fmt.Errorf("source database is nil")
	}

	aliasIndex := 1
	filter, args, err := filterBuilder("t0", &aliasIndex)
	if err != nil {
		return 0, 0, err
	}

	selectQuery := fmt.Sprintf("SELECT t0.* FROM %s t0 WHERE %s", tableName, filter)
	reboundSelectQuery, err := rebindMovedataStyleQuery(databaseType, selectQuery)
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

	insertQuery, err := buildMovedataStyleInsertQuery(databaseType, tableName, columns)
	if err != nil {
		return 0, 0, err
	}

	selectedRows := 0
	copiedRows := int64(0)
	for rows.Next() {
		selectedRows++

		values := make([]any, len(columns))
		valuePointers := make([]any, len(columns))
		for index := range values {
			valuePointers[index] = &values[index]
		}

		if err := rows.Scan(valuePointers...); err != nil {
			return selectedRows - 1, copiedRows, err
		}

		if _, err := tx.ExecContext(ctx, insertQuery, values...); err != nil {
			if isDuplicateInsertError(err) {
				continue
			}
			return selectedRows, copiedRows, err
		}

		copiedRows++
	}

	if err := rows.Err(); err != nil {
		return selectedRows, copiedRows, err
	}

	return selectedRows, copiedRows, nil
}

func buildMovedataStyleInsertQuery(databaseType, tableName string, columns []string) (string, error) {
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

	return rebindMovedataStyleQuery(databaseType, insertQuery)
}

func rebindMovedataStyleQuery(databaseType, query string) (string, error) {
	driver, err := movedataStyleDriverName(databaseType)
	if err != nil {
		return "", err
	}

	if driver != "postgres" {
		return query, nil
	}

	var builder strings.Builder
	parameterIndex := 1
	for _, character := range query {
		if character == '?' {
			builder.WriteString("$")
			builder.WriteString(strconv.Itoa(parameterIndex))
			parameterIndex++
			continue
		}

		builder.WriteRune(character)
	}

	return builder.String(), nil
}

func movedataStyleDriverName(databaseType string) (string, error) {
	if getAppMode() != appModeReal {
		return sqliteDriverName, nil
	}

	switch strings.ToLower(strings.TrimSpace(databaseType)) {
	case strings.ToLower(databaseNameMARx), "postgres":
		return "postgres", nil
	case strings.ToLower(databaseNamePWA), "mysql", "pwa":
		return "mysql", nil
	default:
		return "", fmt.Errorf("unsupported movedata-style database type %s", databaseType)
	}
}

func copyBeneCopyMovedataRows(ctx context.Context, jobID, sourceEnvironment, targetEnvironment, beneLinkPartKey, beneLinkKey string, progress beneCopyProgressFunc, authContext beneCopyJobAuthContext) (beneCopyExecutionResult, error) {
	parsedBeneLinkPartKey, err := strconv.ParseInt(strings.TrimSpace(beneLinkPartKey), 10, 16)
	if err != nil {
		return beneCopyExecutionResult{}, fmt.Errorf("beneLinkPartKey must be numeric: %w", err)
	}
	parsedBeneLinkKey, err := strconv.ParseInt(strings.TrimSpace(beneLinkKey), 10, 32)
	if err != nil {
		return beneCopyExecutionResult{}, fmt.Errorf("beneLinkKey must be numeric: %w", err)
	}

	marxMetadata, marxClose, err := openMovedataStyleMetadata(databaseNameMARx, sourceEnvironment, targetEnvironment, defaultBeneCopyMARxConfigFileName, authContext)
	if err != nil {
		return beneCopyExecutionResult{}, err
	}
	defer marxClose()

	pwaMetadata, pwaClose, err := openMovedataStyleMetadata(databaseNamePWA, sourceEnvironment, targetEnvironment, defaultBeneCopyPWAConfigFileName, authContext)
	if err != nil {
		return beneCopyExecutionResult{}, err
	}
	defer pwaClose()

	databasesMetadata := movedataStyleDatabasesMetadata{marxMetadata, pwaMetadata}
	execution := &movedataStyleExecutionContext{
		BaseContext:       ctx,
		JobID:             jobID,
		SourceEnvironment: sourceEnvironment,
		TargetEnvironment: targetEnvironment,
		Progress:          progress,
	}
	databasesMetadata.attachExecutionContext(execution)

	if err := databasesMetadata.Copy(int16(parsedBeneLinkPartKey), int32(parsedBeneLinkKey)); err != nil {
		return beneCopyExecutionResult{}, err
	}

	execution.mu.Lock()
	defer execution.mu.Unlock()

	return beneCopyExecutionResult{
		Success:     true,
		CopiedRows:  execution.CopiedRows,
		SkippedRows: execution.SkippedRows,
		Message:     fmt.Sprintf("Movedata bene copy complete; copied %d rows and skipped %d duplicates", execution.CopiedRows, execution.SkippedRows),
	}, nil
}

func openMovedataStyleMetadata(databaseType, sourceEnvironment, targetEnvironment, tableMetadataFile string, authContext beneCopyJobAuthContext) (*movedataStyleDatabaseMetadata, func(), error) {
	sourceReader, err := openDatabaseWithCredentials(sourceEnvironment, databaseType, authContext.UserID, authContext.Password)
	if err != nil {
		return nil, nil, err
	}

	sourceWriter, err := openDatabaseWithCredentials(sourceEnvironment, databaseType, authContext.UserID, authContext.Password)
	if err != nil {
		sourceReader.Close()
		return nil, nil, err
	}

	destinationWriter, err := openDatabaseWithCredentials(targetEnvironment, databaseType, authContext.UserID, authContext.Password)
	if err != nil {
		sourceWriter.Close()
		sourceReader.Close()
		return nil, nil, err
	}

	metadata, err := newMovedataStyleDatabaseMetadata(databaseType, sourceReader, sourceWriter, destinationWriter, tableMetadataFile)
	if err != nil {
		destinationWriter.Close()
		sourceWriter.Close()
		sourceReader.Close()
		return nil, nil, err
	}

	cleanup := func() {
		destinationWriter.Close()
		sourceWriter.Close()
		sourceReader.Close()
	}

	return metadata, cleanup, nil
}

func startMovedataStyleTransaction(dbWriter *sql.DB) (*sql.Tx, error) {
	if dbWriter == nil {
		return nil, fmt.Errorf("cannot start movedata-style transaction on nil db")
	}

	dbTxWriter, err := dbWriter.Begin()
	if err != nil {
		return nil, fmt.Errorf("start movedata-style transaction: %w", err)
	}

	return dbTxWriter, nil
}

func stopMovedataStyleTransaction(dbTxWriter *sql.Tx, runErr error) {
	if dbTxWriter == nil {
		return
	}

	if runErr != nil {
		if rollbackErr := dbTxWriter.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			log.Printf("rollback movedata-style transaction failed: %v", rollbackErr)
		}
		return
	}

	if commitErr := dbTxWriter.Commit(); commitErr != nil && commitErr != sql.ErrTxDone {
		log.Printf("commit movedata-style transaction failed: %v", commitErr)
	}
}