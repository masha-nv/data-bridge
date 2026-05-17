package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	beneCopyHistoryDBFileName   = "bene_copy_history.db"
	defaultBeneCopyHistoryLimit = 20
	maxBeneCopyHistoryLimit     = 100
)

type beneCopyHistoryJobSummary struct {
	JobID             string           `json:"jobId"`
	Engine            string           `json:"engine"`
	Status            beneCopyJobState `json:"status"`
	SubmittedAt       time.Time        `json:"submittedAt"`
	StartedAt         *time.Time       `json:"startedAt,omitempty"`
	CompletedAt       *time.Time       `json:"completedAt,omitempty"`
	UpdatedAt         time.Time        `json:"updatedAt"`
	DurationMs        int64            `json:"durationMs,omitempty"`
	SourceEnvironment string           `json:"sourceEnvironment"`
	TargetEnvironment string           `json:"targetEnvironment"`
	BeneLinkPartKey   string           `json:"beneLinkPartKey,omitempty"`
	BeneLinkKey       string           `json:"beneLinkKey"`
	CurrentTable      string           `json:"currentTable,omitempty"`
	CopiedRows        int              `json:"copiedRows"`
	SkippedRows       int              `json:"skippedRows"`
	Message           string           `json:"message,omitempty"`
	Error             string           `json:"error,omitempty"`
}

type beneCopyTableHistoryEntry struct {
	TableName   string     `json:"tableName"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	DurationMs  int64      `json:"durationMs,omitempty"`
	CopiedRows  int        `json:"copiedRows"`
	SkippedRows int        `json:"skippedRows"`
	Error       string     `json:"error,omitempty"`
}

type beneCopyHistoryListResponse struct {
	Jobs []beneCopyHistoryJobSummary `json:"jobs"`
}

type beneCopyHistoryDetailResponse struct {
	Job    beneCopyHistoryJobSummary   `json:"job"`
	Tables []beneCopyTableHistoryEntry `json:"tables"`
}

type beneCopyHistoryStore struct {
	db *sql.DB
}

var beneCopyHistory beneCopyHistoryStore

func initBeneCopyHistoryStore() error {
	historyPath, err := resolveBeneCopyHistoryDBPath()
	if err != nil {
		return err
	}

	db, err := sql.Open(sqliteDriverName, historyPath)
	if err != nil {
		return fmt.Errorf("open bene copy history db %s: %w", historyPath, err)
	}

	if _, err := db.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		db.Close()
		return fmt.Errorf("enable bene copy history WAL mode: %w", err)
	}

	if _, err := db.Exec(`PRAGMA busy_timeout = 5000;`); err != nil {
		db.Close()
		return fmt.Errorf("set bene copy history busy timeout: %w", err)
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS bene_copy_job_runs (
			job_id TEXT PRIMARY KEY,
			engine TEXT NOT NULL DEFAULT 'legacy',
			status TEXT NOT NULL,
			source_environment TEXT NOT NULL,
			target_environment TEXT NOT NULL,
			bene_link_part_key TEXT NOT NULL DEFAULT '',
			bene_link_key TEXT NOT NULL,
			submitted_at TEXT NOT NULL,
			started_at TEXT,
			completed_at TEXT,
			updated_at TEXT NOT NULL,
			current_table TEXT,
			copied_rows INTEGER NOT NULL DEFAULT 0,
			skipped_rows INTEGER NOT NULL DEFAULT 0,
			message TEXT,
			error TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS bene_copy_table_events (
			job_id TEXT NOT NULL,
			table_name TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TEXT NOT NULL,
			completed_at TEXT,
			duration_ms INTEGER NOT NULL DEFAULT 0,
			copied_rows INTEGER NOT NULL DEFAULT 0,
			skipped_rows INTEGER NOT NULL DEFAULT 0,
			error TEXT,
			PRIMARY KEY (job_id, table_name)
		);`,
	}

	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			return fmt.Errorf("initialize bene copy history schema: %w", err)
		}
	}

	if err := ensureBeneCopyHistoryColumns(db); err != nil {
		db.Close()
		return err
	}

	beneCopyHistory.db = db
	log.Printf("bene copy history db ready at %s", historyPath)
	return nil
}

func resolveBeneCopyHistoryDBPath() (string, error) {
	basePath := ""
	execPath, err := os.Executable()
	if err == nil {
		basePath = filepath.Dir(execPath)
	} else {
		workingDir, workingDirErr := os.Getwd()
		if workingDirErr != nil {
			return "", fmt.Errorf("resolve bene copy history db path: %w", workingDirErr)
		}
		basePath = workingDir
	}

	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return "", fmt.Errorf("create bene copy history db directory %s: %w", basePath, err)
	}

	return filepath.Join(basePath, beneCopyHistoryDBFileName), nil
}

func persistBeneCopyJobHistory(job beneCopyJob) {
	if beneCopyHistory.db == nil {
		return
	}

	if err := beneCopyHistory.upsertJobRun(job); err != nil {
		log.Printf("bene copy history job upsert failed: jobId=%s err=%v", job.ID, err)
	}
}

func persistBeneCopyTableStarted(jobID, tableName string, startedAt time.Time) {
	if beneCopyHistory.db == nil {
		return
	}

	if err := beneCopyHistory.upsertTableEventStart(jobID, tableName, startedAt); err != nil {
		log.Printf("bene copy history table start persist failed: jobId=%s table=%s err=%v", jobID, tableName, err)
	}
}

func persistBeneCopyTableCompleted(jobID, tableName string, startedAt, completedAt time.Time, copiedRows, skippedRows int) {
	if beneCopyHistory.db == nil {
		return
	}

	if err := beneCopyHistory.upsertTableEventCompletion(jobID, tableName, startedAt, completedAt, copiedRows, skippedRows, ""); err != nil {
		log.Printf("bene copy history table completion persist failed: jobId=%s table=%s err=%v", jobID, tableName, err)
	}
}

func persistBeneCopyTableFailed(jobID, tableName string, startedAt, completedAt time.Time, copiedRows, skippedRows int, failure error) {
	if beneCopyHistory.db == nil {
		return
	}

	if err := beneCopyHistory.upsertTableEventCompletion(jobID, tableName, startedAt, completedAt, copiedRows, skippedRows, strings.TrimSpace(failure.Error())); err != nil {
		log.Printf("bene copy history table failure persist failed: jobId=%s table=%s err=%v", jobID, tableName, err)
	}
}

func (store beneCopyHistoryStore) upsertJobRun(job beneCopyJob) error {
	_, err := store.db.Exec(
		`INSERT INTO bene_copy_job_runs (
			job_id, engine, status, source_environment, target_environment, bene_link_part_key, bene_link_key,
			submitted_at, started_at, completed_at, updated_at, current_table,
			copied_rows, skipped_rows, message, error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(job_id) DO UPDATE SET
			engine = excluded.engine,
			status = excluded.status,
			source_environment = excluded.source_environment,
			target_environment = excluded.target_environment,
			bene_link_part_key = excluded.bene_link_part_key,
			bene_link_key = excluded.bene_link_key,
			submitted_at = excluded.submitted_at,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			updated_at = excluded.updated_at,
			current_table = excluded.current_table,
			copied_rows = excluded.copied_rows,
			skipped_rows = excluded.skipped_rows,
			message = excluded.message,
			error = excluded.error`,
		job.ID,
		normalizeBeneCopyEngine(job.Engine),
		string(job.State),
		job.SourceEnvironment,
		job.TargetEnvironment,
		job.BeneLinkPartKey,
		job.BeneLinkKey,
		formatBeneCopyHistoryTime(job.SubmittedAt),
		formatOptionalBeneCopyHistoryTime(job.StartedAt),
		formatOptionalBeneCopyHistoryTime(job.CompletedAt),
		formatBeneCopyHistoryTime(job.UpdatedAt),
		strings.TrimSpace(job.CurrentTable),
		job.CopiedRows,
		job.SkippedRows,
		strings.TrimSpace(job.Message),
		strings.TrimSpace(job.Error),
	)
	if err != nil {
		return fmt.Errorf("upsert bene copy job run %s: %w", job.ID, err)
	}

	return nil
}

func (store beneCopyHistoryStore) upsertTableEventStart(jobID, tableName string, startedAt time.Time) error {
	_, err := store.db.Exec(
		`INSERT INTO bene_copy_table_events (job_id, table_name, status, started_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(job_id, table_name) DO UPDATE SET
			status = excluded.status,
			started_at = excluded.started_at,
			completed_at = NULL,
			duration_ms = 0,
			copied_rows = 0,
			skipped_rows = 0,
			error = NULL`,
		jobID,
		tableName,
		string(beneCopyJobStateRunning),
		formatBeneCopyHistoryTime(startedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert bene copy table start %s/%s: %w", jobID, tableName, err)
	}

	return nil
}

func (store beneCopyHistoryStore) upsertTableEventCompletion(jobID, tableName string, startedAt, completedAt time.Time, copiedRows, skippedRows int, failure string) error {
	status := "completed"
	if strings.TrimSpace(failure) != "" {
		status = "failed"
	}

	_, err := store.db.Exec(
		`INSERT INTO bene_copy_table_events (
			job_id, table_name, status, started_at, completed_at, duration_ms, copied_rows, skipped_rows, error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(job_id, table_name) DO UPDATE SET
			status = excluded.status,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			duration_ms = excluded.duration_ms,
			copied_rows = excluded.copied_rows,
			skipped_rows = excluded.skipped_rows,
			error = excluded.error`,
		jobID,
		tableName,
		status,
		formatBeneCopyHistoryTime(startedAt),
		formatBeneCopyHistoryTime(completedAt),
		completedAt.Sub(startedAt).Milliseconds(),
		copiedRows,
		skippedRows,
		strings.TrimSpace(failure),
	)
	if err != nil {
		return fmt.Errorf("upsert bene copy table completion %s/%s: %w", jobID, tableName, err)
	}

	return nil
}

func beneCopyHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := defaultBeneCopyHistoryLimit
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil || parsedLimit <= 0 {
			http.Error(w, "limit must be a positive integer", http.StatusBadRequest)
			return
		}
		if parsedLimit > maxBeneCopyHistoryLimit {
			parsedLimit = maxBeneCopyHistoryLimit
		}
		limit = parsedLimit
	}

	engine := normalizeBeneCopyEngine(r.URL.Query().Get("engine"))
	jobs, err := beneCopyHistory.listRecentJobs(limit, engine)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, beneCopyHistoryListResponse{Jobs: jobs})
}

func beneCopyHistoryDetailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/marx/beneficiaries/copy/history/"))
	if jobID == "" {
		http.Error(w, "jobId is required", http.StatusBadRequest)
		return
	}

	job, tables, err := beneCopyHistory.getJobHistory(jobID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, beneCopyHistoryDetailResponse{Job: job, Tables: tables})
}

func (store beneCopyHistoryStore) listRecentJobs(limit int, engine string) ([]beneCopyHistoryJobSummary, error) {
	if store.db == nil {
		return nil, fmt.Errorf("bene copy history db is not initialized")
	}

	rows, err := store.db.Query(
		`SELECT job_id, engine, status, source_environment, target_environment, bene_link_part_key, bene_link_key,
			submitted_at, started_at, completed_at, updated_at, current_table,
			copied_rows, skipped_rows, message, error
		 FROM bene_copy_job_runs
		 WHERE engine = ?
		 ORDER BY updated_at DESC
		 LIMIT ?`,
		engine,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list bene copy job history: %w", err)
	}
	defer rows.Close()

	jobs := make([]beneCopyHistoryJobSummary, 0)
	for rows.Next() {
		job, err := scanBeneCopyHistoryJobSummary(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bene copy job history: %w", err)
	}

	return jobs, nil
}

func (store beneCopyHistoryStore) getJobHistory(jobID string) (beneCopyHistoryJobSummary, []beneCopyTableHistoryEntry, error) {
	if store.db == nil {
		return beneCopyHistoryJobSummary{}, nil, fmt.Errorf("bene copy history db is not initialized")
	}

	row := store.db.QueryRow(
		`SELECT job_id, engine, status, source_environment, target_environment, bene_link_part_key, bene_link_key,
			submitted_at, started_at, completed_at, updated_at, current_table,
			copied_rows, skipped_rows, message, error
		 FROM bene_copy_job_runs
		 WHERE job_id = ?`,
		jobID,
	)

	job, err := scanBeneCopyHistoryJobSummary(row)
	if err != nil {
		return beneCopyHistoryJobSummary{}, nil, err
	}

	rows, err := store.db.Query(
		`SELECT table_name, status, started_at, completed_at, duration_ms, copied_rows, skipped_rows, error
		 FROM bene_copy_table_events
		 WHERE job_id = ?
		 ORDER BY started_at ASC, table_name ASC`,
		jobID,
	)
	if err != nil {
		return beneCopyHistoryJobSummary{}, nil, fmt.Errorf("query bene copy table history for %s: %w", jobID, err)
	}
	defer rows.Close()

	tables := make([]beneCopyTableHistoryEntry, 0)
	for rows.Next() {
		entry, scanErr := scanBeneCopyTableHistoryEntry(rows)
		if scanErr != nil {
			return beneCopyHistoryJobSummary{}, nil, scanErr
		}
		tables = append(tables, entry)
	}

	if err := rows.Err(); err != nil {
		return beneCopyHistoryJobSummary{}, nil, fmt.Errorf("iterate bene copy table history for %s: %w", jobID, err)
	}

	return job, tables, nil
}

type beneCopyHistoryScanner interface {
	Scan(dest ...any) error
}

func scanBeneCopyHistoryJobSummary(scanner beneCopyHistoryScanner) (beneCopyHistoryJobSummary, error) {
	var (
		jobID             string
		engine            string
		status            string
		sourceEnvironment string
		targetEnvironment string
		beneLinkPartKey   string
		beneLinkKey       string
		submittedAt       string
		startedAt         sql.NullString
		completedAt       sql.NullString
		updatedAt         string
		currentTable      sql.NullString
		copiedRows        int
		skippedRows       int
		message           sql.NullString
		errorText         sql.NullString
	)

	if err := scanner.Scan(
		&jobID,
		&engine,
		&status,
		&sourceEnvironment,
		&targetEnvironment,
		&beneLinkPartKey,
		&beneLinkKey,
		&submittedAt,
		&startedAt,
		&completedAt,
		&updatedAt,
		&currentTable,
		&copiedRows,
		&skippedRows,
		&message,
		&errorText,
	); err != nil {
		return beneCopyHistoryJobSummary{}, err
	}

	submittedTime, err := parseBeneCopyHistoryTime(submittedAt)
	if err != nil {
		return beneCopyHistoryJobSummary{}, err
	}
	updatedTime, err := parseBeneCopyHistoryTime(updatedAt)
	if err != nil {
		return beneCopyHistoryJobSummary{}, err
	}

	startedTime, err := parseOptionalBeneCopyHistoryTime(startedAt)
	if err != nil {
		return beneCopyHistoryJobSummary{}, err
	}
	completedTime, err := parseOptionalBeneCopyHistoryTime(completedAt)
	if err != nil {
		return beneCopyHistoryJobSummary{}, err
	}

	summary := beneCopyHistoryJobSummary{
		JobID:             jobID,
		Engine:            normalizeBeneCopyEngine(engine),
		Status:            beneCopyJobState(status),
		SubmittedAt:       submittedTime,
		StartedAt:         startedTime,
		CompletedAt:       completedTime,
		UpdatedAt:         updatedTime,
		SourceEnvironment: sourceEnvironment,
		TargetEnvironment: targetEnvironment,
		BeneLinkPartKey:   beneLinkPartKey,
		BeneLinkKey:       beneLinkKey,
		CurrentTable:      strings.TrimSpace(currentTable.String),
		CopiedRows:        copiedRows,
		SkippedRows:       skippedRows,
		Message:           strings.TrimSpace(message.String),
		Error:             strings.TrimSpace(errorText.String),
	}

	if startedTime != nil {
		endTime := updatedTime
		if completedTime != nil {
			endTime = *completedTime
		}
		summary.DurationMs = endTime.Sub(*startedTime).Milliseconds()
	}

	return summary, nil
}

func ensureBeneCopyHistoryColumns(db *sql.DB) error {
	columnStatements := map[string]string{
		"engine":             "ALTER TABLE bene_copy_job_runs ADD COLUMN engine TEXT NOT NULL DEFAULT 'legacy'",
		"bene_link_part_key": "ALTER TABLE bene_copy_job_runs ADD COLUMN bene_link_part_key TEXT NOT NULL DEFAULT ''",
	}

	for columnName, statement := range columnStatements {
		exists, err := beneCopyHistoryColumnExists(db, "bene_copy_job_runs", columnName)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("add bene copy history column %s: %w", columnName, err)
		}
	}

	return nil
}

func beneCopyHistoryColumnExists(db *sql.DB, tableName, columnName string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return false, fmt.Errorf("query history table info for %s: %w", tableName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return false, fmt.Errorf("scan history table info for %s: %w", tableName, err)
		}
		if strings.EqualFold(name, columnName) {
			return true, nil
		}
	}

	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate history table info for %s: %w", tableName, err)
	}

	return false, nil
}

func scanBeneCopyTableHistoryEntry(scanner beneCopyHistoryScanner) (beneCopyTableHistoryEntry, error) {
	var (
		tableName    string
		status       string
		startedAt    string
		completedAt  sql.NullString
		durationMs   int64
		copiedRows   int
		skippedRows  int
		errorText    sql.NullString
	)

	if err := scanner.Scan(&tableName, &status, &startedAt, &completedAt, &durationMs, &copiedRows, &skippedRows, &errorText); err != nil {
		return beneCopyTableHistoryEntry{}, err
	}

	startedTime, err := parseBeneCopyHistoryTime(startedAt)
	if err != nil {
		return beneCopyTableHistoryEntry{}, err
	}
	completedTime, err := parseOptionalBeneCopyHistoryTime(completedAt)
	if err != nil {
		return beneCopyTableHistoryEntry{}, err
	}

	return beneCopyTableHistoryEntry{
		TableName:   tableName,
		Status:      status,
		StartedAt:   startedTime,
		CompletedAt: completedTime,
		DurationMs:  durationMs,
		CopiedRows:  copiedRows,
		SkippedRows: skippedRows,
		Error:       strings.TrimSpace(errorText.String),
	}, nil
}

func formatBeneCopyHistoryTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func formatOptionalBeneCopyHistoryTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}

	return formatBeneCopyHistoryTime(value)
}

func parseBeneCopyHistoryTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("parse bene copy history time %q: %w", value, err)
	}

	return parsed, nil
}

func parseOptionalBeneCopyHistoryTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil, nil
	}

	parsed, err := parseBeneCopyHistoryTime(value.String)
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}