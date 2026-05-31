package main

import (
	"encoding/json"
	"fmt"
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

type beneCopyProgress struct {
	CurrentTable string
	CopiedRows   int
	SkippedRows  int
}

type beneCopyProgressFunc func(progress beneCopyProgress)

func isReadOnlyMARxEndpoint(host string) bool {
	normalizedHost := strings.ToLower(strings.TrimSpace(host))
	return strings.Contains(normalizedHost, ".cluster-ro-")
}
