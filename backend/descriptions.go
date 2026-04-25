package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	descriptionTypeTRC      = "TRC"
	descriptionTypePwrReply = "PW-R Reply Codes"
)


// descriptionLookupRequest defines the expected payload for a description lookup.
type descriptionLookupRequest struct {
	Environment string `json:"environment"` // Environment (e.g., "dev", "test")
	Type        string `json:"type"`        // Description type (e.g., "TRC", "PW-R Reply Codes")
	Code        string `json:"code"`        // Code to look up
}

// descriptionLookupResponse defines the response contract for a description lookup.
type descriptionLookupResponse struct {
	Environment string `json:"environment"`
	Type        string `json:"type"`
	Code        string `json:"code"`
	Description string `json:"description"`
}

// descriptionsLookupHandler handles POST requests for description lookups.
// It validates input, delegates to getDescriptionValue, and returns a structured JSON response.
func descriptionsLookupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req descriptionLookupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	req.Environment = strings.TrimSpace(req.Environment)
	req.Type = strings.TrimSpace(req.Type)
	req.Code = strings.TrimSpace(req.Code)
	if req.Environment == "" || req.Type == "" || req.Code == "" {
		http.Error(w, "environment, type, and code are required", http.StatusBadRequest)
		return
	}

	description, err := getDescriptionValue(req.Environment, req.Type, req.Code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, descriptionLookupResponse{
		Environment: req.Environment,
		Type:        req.Type,
		Code:        req.Code,
		Description: description,
	})
}

// getDescriptionValue routes the lookup to the correct handler based on type.
// Returns the description string or an error.
func getDescriptionValue(env, descriptionType, code string) (string, error) {
	db, err := openMARxDB(env)
	if err != nil {
		return "", fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	switch descriptionType {
	case descriptionTypeTRC:
		return lookupTrcDescription(env, db, code)
	case descriptionTypePwrReply:
		return lookupPwrReplyDescription(env, db, code)
	default:
		return "", fmt.Errorf("unsupported description type: %s", descriptionType)
	}
}

// lookupTrcDescription queries the TRC description table for the given code.
func lookupTrcDescription(env string, db *sql.DB, code string) (string, error) {
	query, err := rebindQueryForDatabase(env, databaseNameMARx, `SELECT tran_rply_desc FROM mcs_tran_reply WHERE tran_rply_cd = ?`)
	if err != nil {
		return "", err
	}

	var description string
	err = db.QueryRow(
		query,
		strings.TrimSpace(strings.ToUpper(code)),
	).Scan(&description)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("no description found for code %s", code)
		}
		return "", fmt.Errorf("TRC lookup error: %w", err)
	}
	return description, nil
}

// lookupPwrReplyDescription queries the PW-R Reply Codes table for the given input value.
func lookupPwrReplyDescription(env string, db *sql.DB, inputValue string) (string, error) {
	replyCode, agency, premiumType := parsePwrReplyCode(inputValue)
	query, err := rebindQueryForDatabase(env, databaseNameMARx, `SELECT reply_code_description
		 FROM marx_pwr_agency_reply
		 WHERE TRIM(deductibility_code || agency_reply_code) = ?
		   AND withholding_agency = ?
		   AND premium_type_code = ?
		   AND obsolete_date IS NULL`)
	if err != nil {
		return "", err
	}

	var description string
	err = db.QueryRow(
		query,
		replyCode,
		agency,
		premiumType,
	).Scan(&description)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("no description found for code %s", inputValue)
		}
		return "", fmt.Errorf("PW-R Reply lookup error: %w", err)
	}
	return description, nil
}

// parsePwrReplyCode parses the input value for PW-R Reply Codes into its components.
// Returns replyCode, agency, premiumType.
func parsePwrReplyCode(inputValue string) (string, string, string) {
	replyCode := ""
	agency := "S"
	premiumType := "CD"

	inputValue = strings.ToUpper(strings.TrimSpace(inputValue))
	parts := strings.Split(inputValue, ",")
	if len(parts) > 0 {
		replyCode = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		value := strings.TrimSpace(parts[1])
		if len(value) > 1 {
			value = value[0:1]
		}
		if value != "" {
			agency = value
		}
	}
	if len(parts) > 2 {
		value := strings.TrimSpace(parts[2])
		if value != "" {
			premiumType = value
		}
	}
	return replyCode, agency, premiumType
}