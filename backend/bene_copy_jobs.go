package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type beneCopyJobState string

const (
	beneCopyEngineMarxMovedata  = "marx-movedata"

	beneCopyJobStateQueued    beneCopyJobState = "queued"
	beneCopyJobStateRunning   beneCopyJobState = "running"
	beneCopyJobStateCompleted beneCopyJobState = "completed"
	beneCopyJobStateFailed    beneCopyJobState = "failed"
)

type beneCopyJob struct {
	ID                string
	Engine            string
	State             beneCopyJobState
	SourceEnvironment string
	TargetEnvironment string
	BeneLinkPartKey   string
	BeneLinkKey       string
	AuthContext       beneCopyJobAuthContext
	SubmittedAt       time.Time
	StartedAt         time.Time
	CompletedAt       time.Time
	UpdatedAt         time.Time
	CurrentTable      string
	CopiedRows        int
	SkippedRows       int
	Message           string
	Error             string
}

type beneCopyJobSubmissionResponse struct {
	JobID             string           `json:"jobId"`
	Engine            string           `json:"engine"`
	Status            beneCopyJobState `json:"status"`
	SubmittedAt       time.Time        `json:"submittedAt"`
	SourceEnvironment string           `json:"sourceEnvironment"`
	TargetEnvironment string           `json:"targetEnvironment"`
	BeneLinkPartKey   string           `json:"beneLinkPartKey,omitempty"`
	BeneLinkKey       string           `json:"beneLinkKey"`
	Message           string           `json:"message"`
}

type beneCopyJobStatusResponse struct {
	JobID             string           `json:"jobId"`
	Engine            string           `json:"engine"`
	Status            beneCopyJobState `json:"status"`
	SubmittedAt       time.Time        `json:"submittedAt"`
	UpdatedAt         time.Time        `json:"updatedAt"`
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

var beneCopyJobs struct {
	mu    sync.RWMutex
	items map[string]beneCopyJob
}

var beneCopyJobSequence atomic.Uint64

const beneCopyWorkerPollInterval = 500 * time.Millisecond

type beneCopyJobAuthContext struct {
	UserID      string
	Password    string
	DisplayName string
}

func enqueueBeneCopyJob(req beneCopyRequest, authContext beneCopyJobAuthContext) beneCopyJob {
	now := time.Now().UTC()
	engine := normalizeBeneCopyEngine(req.Engine)
	job := beneCopyJob{
		ID:                fmt.Sprintf("bene-copy-%s-%d-%d", engine, now.UnixMilli(), beneCopyJobSequence.Add(1)),
		Engine:            engine,
		State:             beneCopyJobStateQueued,
		SourceEnvironment: req.SourceEnvironment,
		TargetEnvironment: req.TargetEnvironment,
		BeneLinkPartKey:   req.BeneLinkPartKey,
		BeneLinkKey:       req.BeneLinkKey,
		AuthContext:       authContext,
		SubmittedAt:       now,
		UpdatedAt:         now,
		Message:           beneCopyQueuedMessage(engine),
	}

	beneCopyJobs.mu.Lock()
	defer beneCopyJobs.mu.Unlock()

	if beneCopyJobs.items == nil {
		beneCopyJobs.items = make(map[string]beneCopyJob)
	}

	beneCopyJobs.items[job.ID] = job
	persistBeneCopyJobHistory(job)
	return job
}

func getBeneCopyJob(jobID string) (beneCopyJob, bool) {
	beneCopyJobs.mu.RLock()
	defer beneCopyJobs.mu.RUnlock()

	job, ok := beneCopyJobs.items[jobID]
	return job, ok
}

func updateBeneCopyJob(jobID string, update func(*beneCopyJob)) {
	beneCopyJobs.mu.Lock()

	job, ok := beneCopyJobs.items[jobID]
	if !ok {
		beneCopyJobs.mu.Unlock()
		return
	}

	update(&job)
	job.UpdatedAt = time.Now().UTC()
	beneCopyJobs.items[jobID] = job
	beneCopyJobs.mu.Unlock()

	persistBeneCopyJobHistory(job)
}

func claimNextQueuedBeneCopyJob() (beneCopyJob, bool) {
	beneCopyJobs.mu.Lock()
	defer beneCopyJobs.mu.Unlock()

	for jobID, job := range beneCopyJobs.items {
		if job.State != beneCopyJobStateQueued {
			continue
		}

		job.State = beneCopyJobStateRunning
		job.StartedAt = time.Now().UTC()
		job.Message = beneCopyRunningMessage(job.Engine)
		job.Error = ""
		job.UpdatedAt = job.StartedAt
		beneCopyJobs.items[jobID] = job
		persistBeneCopyJobHistory(job)
		return job, true
	}

	return beneCopyJob{}, false
}

func startBeneCopyWorker() {
	go runBeneCopyWorker()
}

func runBeneCopyWorker() {
	ticker := time.NewTicker(beneCopyWorkerPollInterval)
	defer ticker.Stop()

	for {
		job, ok := claimNextQueuedBeneCopyJob()
		if !ok {
			<-ticker.C
			continue
		}

		processBeneCopyJob(job)
	}
}

func processBeneCopyJob(job beneCopyJob) {
	jobStart := time.Now()
	log.Printf(
		"bene copy job started: jobId=%s engine=%s source=%s target=%s beneLinkPartKey=%s beneLinkKey=%s timeout=%s",
		job.ID,
		job.Engine,
		job.SourceEnvironment,
		job.TargetEnvironment,
		job.BeneLinkPartKey,
		job.BeneLinkKey,
		beneCopyExecutionTimeout,
	)

	jobContext, cancel := context.WithTimeout(context.Background(), beneCopyExecutionTimeout)
	defer cancel()

	progress := func(progress beneCopyProgress) {
		updateBeneCopyJob(job.ID, func(current *beneCopyJob) {
			current.CurrentTable = progress.CurrentTable
			current.CopiedRows = progress.CopiedRows
			current.SkippedRows = progress.SkippedRows
			current.Message = fmt.Sprintf("processing table %s", progress.CurrentTable)
		})
	}

	var (
		result beneCopyExecutionResult
		err    error
	)
	result, err = copyBeneCopyMarxMovedataRows(jobContext, job.ID, job.SourceEnvironment, job.TargetEnvironment, job.BeneLinkPartKey, job.BeneLinkKey, progress, job.AuthContext)
	if err != nil {
		log.Printf("bene copy job failed: jobId=%s duration=%s err=%v", job.ID, time.Since(jobStart).Round(time.Millisecond), err)
		failBeneCopyJob(job.ID, err.Error())
		return
	}

	log.Printf(
		"bene copy job completed: jobId=%s duration=%s copied=%d skipped=%d",
		job.ID,
		time.Since(jobStart).Round(time.Millisecond),
		result.CopiedRows,
		result.SkippedRows,
	)

	updateBeneCopyJob(job.ID, func(current *beneCopyJob) {
		current.State = beneCopyJobStateCompleted
		current.CompletedAt = time.Now().UTC()
		current.CurrentTable = ""
		current.CopiedRows = result.CopiedRows
		current.SkippedRows = result.SkippedRows
		current.Message = result.Message
		current.Error = ""
	})
}

func failBeneCopyJob(jobID, failure string) {
	updateBeneCopyJob(jobID, func(current *beneCopyJob) {
		current.State = beneCopyJobStateFailed
		current.CompletedAt = time.Now().UTC()
		current.CurrentTable = ""
		current.Message = "bene copy job failed"
		current.Error = strings.TrimSpace(failure)
	})
}

func beneCopyStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := strings.TrimSpace(r.URL.Query().Get("jobId"))
	if jobID == "" {
		http.Error(w, "jobId is required", http.StatusBadRequest)
		return
	}

	job, ok := getBeneCopyJob(jobID)
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	writeJSON(w, beneCopyJobStatusResponse{
		JobID:             job.ID,
		Engine:            normalizeBeneCopyEngine(job.Engine),
		Status:            job.State,
		SubmittedAt:       job.SubmittedAt,
		UpdatedAt:         job.UpdatedAt,
		SourceEnvironment: job.SourceEnvironment,
		TargetEnvironment: job.TargetEnvironment,
		BeneLinkPartKey:   job.BeneLinkPartKey,
		BeneLinkKey:       job.BeneLinkKey,
		CurrentTable:      job.CurrentTable,
		CopiedRows:        job.CopiedRows,
		SkippedRows:       job.SkippedRows,
		Message:           job.Message,
		Error:             job.Error,
	})
}

func normalizeBeneCopyEngine(engine string) string {
	return beneCopyEngineMarxMovedata
}

func beneCopyQueuedMessage(engine string) string {
	return "marx-move-data bene copy job queued"
}

func beneCopyRunningMessage(engine string) string {
	return "marx-move-data bene copy job running"
}

func buildBeneCopyJobSubmissionResponse(job beneCopyJob) beneCopyJobSubmissionResponse {
	return beneCopyJobSubmissionResponse{
		JobID:             job.ID,
		Engine:            normalizeBeneCopyEngine(job.Engine),
		Status:            job.State,
		SubmittedAt:       job.SubmittedAt,
		SourceEnvironment: job.SourceEnvironment,
		TargetEnvironment: job.TargetEnvironment,
		BeneLinkPartKey:   job.BeneLinkPartKey,
		BeneLinkKey:       job.BeneLinkKey,
		Message:           job.Message,
	}
}