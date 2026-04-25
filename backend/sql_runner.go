package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	sqlRunnerQueryTimeout = 10 * time.Second
	sqlRunnerMaxRows      = 200
)

type sqlRunnerRequest struct {
	Environment string `json:"environment"`
	SQL         string `json:"sql"`
}

type sqlStatementResult struct {
	StatementNumber int        `json:"statementNumber"`
	Statement       string     `json:"statement"`
	Columns         []string   `json:"columns,omitempty"`
	Rows            [][]string `json:"rows,omitempty"`
	RowCount        int        `json:"rowCount"`
	Truncated       bool       `json:"truncated,omitempty"`
	Error           string     `json:"error,omitempty"`
}

type sqlRunnerResponse struct {
	Environment string               `json:"environment"`
	Results     []sqlStatementResult `json:"results"`
}

func sqlRunnerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req sqlRunnerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	req.Environment = strings.TrimSpace(req.Environment)
	req.SQL = strings.TrimSpace(req.SQL)
	if req.Environment == "" || req.SQL == "" {
		http.Error(w, "environment and sql are required", http.StatusBadRequest)
		return
	}

	results, err := runSQLStatements(req.Environment, req.SQL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, sqlRunnerResponse{
		Environment: req.Environment,
		Results:     results,
	})
}

func runSQLStatements(env, allSQL string) ([]sqlStatementResult, error) {
	db, err := openDB(env)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	statements := splitSQLStatements(allSQL)
	results := make([]sqlStatementResult, 0, len(statements))

	for i, statement := range statements {
		statementType := strings.ToLower(strings.Fields(statement)[0])
		result := sqlStatementResult{
			StatementNumber: i + 1,
			Statement:       statement,
		}

		switch statementType {
		case "select":
			columns, rows, rowCount, truncated, err := runSelectSQL(db, statement)
			if err != nil {
				result.Error = fmt.Sprintf("Error in running sql %s in %s. Error = %v", statement, env, err)
				results = append(results, result)
				return results, fmt.Errorf("error running sql")
			}
			result.Columns = columns
			result.Rows = rows
			result.RowCount = rowCount
			result.Truncated = truncated
		default:
			result.Error = fmt.Sprintf("unsupported sql statement type: %s", statementType)
			results = append(results, result)
			return results, fmt.Errorf("error running sql")
		}

		results = append(results, result)
	}

	return results, nil
}

func splitSQLStatements(allSQL string) []string {
	parts := strings.Split(allSQL, ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		statement := strings.TrimSpace(part)
		if statement != "" {
			statements = append(statements, statement)
		}
	}

	return statements
}

func runSelectSQL(db *sql.DB, statement string) ([]string, [][]string, int, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sqlRunnerQueryTimeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, statement)
	if err != nil {
		return nil, nil, 0, false, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, 0, false, err
	}

	results := make([][]string, 0)
	rowCount := 0
	truncated := false
	for rows.Next() {
		if rowCount >= sqlRunnerMaxRows {
			truncated = true
			break
		}

		values := make([]any, len(columns))
		valuePointers := make([]any, len(columns))
		for i := range values {
			valuePointers[i] = &values[i]
		}

		if err := rows.Scan(valuePointers...); err != nil {
			return nil, nil, rowCount, truncated, err
		}

		row := make([]string, len(columns))
		for i, value := range values {
			row[i] = sqlValueToString(value)
		}
		results = append(results, row)
		rowCount++
	}

	if err := rows.Err(); err != nil {
		return nil, nil, rowCount, truncated, err
	}

	return columns, results, rowCount, truncated, nil
}

func sqlValueToString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case []byte:
		return string(typed)
	default:
		return fmt.Sprintf("%v", typed)
	}
}