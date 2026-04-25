package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
)

const completedJobStatus = 4

type devopsJobsRequest struct {
	Environment string `json:"environment"`
	Status      string `json:"status"`
}

type devopsJobRecord struct {
	StatusCode      int    `json:"statusCode"`
	JobID           int64  `json:"jobId"`
	InFilePath      string `json:"inFilePath"`
	InFileURI       string `json:"inFileUri"`
	ThreadKey       string `json:"threadKey"`
	BatchName       string `json:"batchName"`
	BatchTitle      string `json:"batchTitle"`
	ThreadPoolID    int64  `json:"threadPoolId"`
	ThreadID        int64  `json:"threadId"`
	StartDateTime   string `json:"startDateTime"`
	StatusDateTime  string `json:"statusDateTime"`
	CreatedDateTime string `json:"createdDateTime"`
	EndDateTime     string `json:"endDateTime"`
}

type devopsJobsResponse struct {
	Environment string            `json:"environment"`
	Status      string            `json:"status"`
	Rows        []devopsJobRecord `json:"rows"`
}

func devopsJobsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req devopsJobsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	req.Environment = strings.TrimSpace(req.Environment)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	if req.Environment == "" || req.Status == "" {
		http.Error(w, "environment and status are required", http.StatusBadRequest)
		return
	}

	rows, err := getDevopsJobs(req.Environment, req.Status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, devopsJobsResponse{
		Environment: req.Environment,
		Status:      req.Status,
		Rows:        rows,
	})
}

func getDevopsJobs(env, status string) ([]devopsJobRecord, error) {
	db, err := openBatchDB(env)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query, args, err := buildDevopsJobsQuery(status)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]devopsJobRecord, 0)
	for rows.Next() {
		var row devopsJobRecord
		var endDateTime sql.NullString
		if err := rows.Scan(
			&row.StatusCode,
			&row.JobID,
			&row.InFilePath,
			&row.InFileURI,
			&row.ThreadKey,
			&row.BatchName,
			&row.BatchTitle,
			&row.ThreadPoolID,
			&row.ThreadID,
			&row.StartDateTime,
			&row.StatusDateTime,
			&row.CreatedDateTime,
			&endDateTime,
		); err != nil {
			return nil, err
		}
		row.EndDateTime = nullableStringValue(endDateTime)
		results = append(results, row)
	}

	return results, nil
}

func buildDevopsJobsQuery(status string) (string, []any, error) {
	baseQuery := `SELECT
		a.status_cd,
		a.job_id,
		c.in_file_path,
		c.in_file_uri,
		a.thread_key,
		b.name,
		b.title,
		a.thread_pool_id,
		a.id,
		a.start_date_time,
		a.status_date_time,
		a.created_date_time,
		a.end_date_time
	FROM sr_batch_thread a
	JOIN sr_batch_type b ON a.thread_pool_id = b.id
	JOIN sr_batch_job c ON a.job_id = c.id
	WHERE `

	orderBy := ` ORDER BY a.status_date_time DESC, a.status_cd DESC, a.start_date_time`
	switch status {
	case "active":
		return baseQuery + "a.status_cd <> ?" + orderBy, []any{completedJobStatus}, nil
	case "completed":
		return baseQuery + "a.status_cd = ?" + orderBy, []any{completedJobStatus}, nil
	default:
		return "", nil, http.ErrNotSupported
	}
}