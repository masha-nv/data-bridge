package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type restartJobsRequest struct {
	Environment string `json:"environment"`
	JobIDs      string `json:"jobIds"`
}

type markJobsCompleteRequest struct {
	Environment   string `json:"environment"`
	CurrentStatus string `json:"currentStatus"`
	JobIDs        string `json:"jobIds"`
}

type devopsActionResponse struct {
	Environment   string `json:"environment"`
	ReturnCode    int    `json:"returnCode"`
	ReturnMessage string `json:"returnMessage"`
}

func restartFailedJobsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req restartJobsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	req.Environment = strings.TrimSpace(req.Environment)
	req.JobIDs = strings.TrimSpace(req.JobIDs)
	if req.Environment == "" || req.JobIDs == "" {
		http.Error(w, "environment and jobIds are required", http.StatusBadRequest)
		return
	}

	rc, rm, err := restartFailedJobs(req.Environment, req.JobIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, devopsActionResponse{
		Environment:   req.Environment,
		ReturnCode:    rc,
		ReturnMessage: rm,
	})
}

func markJobsCompleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req markJobsCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	req.Environment = strings.TrimSpace(req.Environment)
	req.CurrentStatus = strings.TrimSpace(req.CurrentStatus)
	req.JobIDs = strings.TrimSpace(req.JobIDs)
	if req.Environment == "" || req.CurrentStatus == "" || req.JobIDs == "" {
		http.Error(w, "environment, currentStatus, and jobIds are required", http.StatusBadRequest)
		return
	}

	rc, rm, err := markJobsComplete(req.Environment, req.CurrentStatus, req.JobIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, devopsActionResponse{
		Environment:   req.Environment,
		ReturnCode:    rc,
		ReturnMessage: rm,
	})
}

func restartFailedJobs(env, jobIDs string) (int, string, error) {
	db, err := openBatchDB(env)
	if err != nil {
		return 9, "", err
	}
	defer db.Close()

	jobIDList := splitJobIDs(jobIDs)
	return runJobMutation(db, jobIDList, func(tx *sql.Tx, jobID int64) (string, error) {
		now := time.Now().UTC().Format(time.RFC3339)
		result, err := tx.Exec(
			`UPDATE sr_batch_thread
			 SET status_cd = ?, status_date_time = ?, end_date_time = NULL
			 WHERE job_id = ?`,
			2,
			now,
			jobID,
		)
		if err != nil {
			return "", err
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return "", err
		}
		if rowsAffected == 0 {
			return "job not found", nil
		}
		return "successful", nil
	})
}

func markJobsComplete(env, currentStatus, jobIDs string) (int, string, error) {
	statusCode, err := strconv.ParseInt(currentStatus, 10, 32)
	if err != nil {
		return 9, "", fmt.Errorf("current job status is not an int. value = %s. error = %w", currentStatus, err)
	}

	db, err := openBatchDB(env)
	if err != nil {
		return 9, "", err
	}
	defer db.Close()

	jobIDList := splitJobIDs(jobIDs)
	return runJobMutation(db, jobIDList, func(tx *sql.Tx, jobID int64) (string, error) {
		now := time.Now().UTC().Format(time.RFC3339)
		result, err := tx.Exec(
			`UPDATE sr_batch_thread
			 SET status_cd = ?, status_date_time = ?, end_date_time = ?
			 WHERE job_id = ? AND status_cd = ?`,
			completedJobStatus,
			now,
			now,
			jobID,
			statusCode,
		)
		if err != nil {
			return "", err
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return "", err
		}
		if rowsAffected == 0 {
			return fmt.Sprintf("job not found in current status %d", statusCode), nil
		}
		return "successful", nil
	})
}

func splitJobIDs(jobIDs string) []string {
	parts := strings.Split(jobIDs, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}

	return values
}

func runJobMutation(db *sql.DB, jobIDList []string, mutate func(tx *sql.Tx, jobID int64) (string, error)) (int, string, error) {
	var rc int
	var rm []string

	for _, jobID := range jobIDList {
		jobIDAsInt, err := strconv.ParseInt(jobID, 10, 32)
		if err != nil {
			rc += -100
			rm = append(rm, fmt.Sprintf("job-id %s - error job-id is not a number. error = %v", jobID, err))
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			return rc, strings.Join(rm, "\n"), err
		}

		message, err := mutate(tx, jobIDAsInt)
		if err != nil {
			tx.Rollback()
			rc += -100
			rm = append(rm, fmt.Sprintf("job-id %s - error = %v", jobID, err))
			continue
		}

		if err := tx.Commit(); err != nil {
			rc += -100
			rm = append(rm, fmt.Sprintf("job-id %s - commit error = %v", jobID, err))
			continue
		}

		if strings.TrimSpace(message) != "successful" {
			rc += -100
		}
		rm = append(rm, fmt.Sprintf("job-id %s - %s", jobID, message))
	}

	return rc, strings.Join(rm, "\n"), nil
}