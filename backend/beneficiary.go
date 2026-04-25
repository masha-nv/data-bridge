package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	beneIDTypeBLK     = "BLK"
	beneIDTypeMBI     = "MBI"
	beneIDTypeHICN    = "HICN"
	beneIDTypeSSN     = "SSN"
	beneIDTypeRRBHICN = "RRB-HICN"
)

type beneficiaryLookupRequest struct {
	Environment string `json:"environment"`
	IDType      string `json:"idType"`
	IDValue     string `json:"idValue"`
}

type beneficiaryLookupResponse struct {
	Environment     string `json:"environment"`
	IDType          string `json:"idType"`
	IDValue         string `json:"idValue"`
	BeneLinkPartKey int64  `json:"beneLinkPartKey"`
	BeneLinkKey     int64  `json:"beneLinkKey"`
	HICN            string `json:"hicn"`
	BeneDeathDate   string `json:"beneDeathDate"`
	BeneBirthDate   string `json:"beneBirthDate"`
	BeneLastName    string `json:"beneLastName"`
	BeneFirstName   string `json:"beneFirstName"`
	MiddleName      string `json:"middleName"`
	SSN             string `json:"ssn"`
	BeneSex         string `json:"beneSex"`
	ArchiveStatus   string `json:"archiveStatus"`
	LastUpdateTS    string `json:"lastUpdateTs"`
	MBI             string `json:"mbi"`
	RRBHICN         string `json:"rrbHicn"`
}

func beneficiaryLookupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req beneficiaryLookupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	req.Environment = strings.TrimSpace(req.Environment)
	req.IDType = strings.ToUpper(strings.TrimSpace(req.IDType))
	req.IDValue = strings.ToUpper(strings.TrimSpace(req.IDValue))

	if req.Environment == "" || req.IDType == "" || req.IDValue == "" {
		http.Error(w, "environment, idType, and idValue are required", http.StatusBadRequest)
		return
	}

	response, err := getBeneficiaryDetails(req.Environment, req.IDType, req.IDValue)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, response)
}

func getBeneficiaryDetails(environment, idType, idValue string) (beneficiaryLookupResponse, error) {
	db, err := openMARxDB(environment)
	if err != nil {
		return beneficiaryLookupResponse{}, err
	}
	defer db.Close()

	query, args, err := buildBeneficiaryLookupQuery(idType, idValue)
	if err != nil {
		return beneficiaryLookupResponse{}, err
	}

	response := beneficiaryLookupResponse{
		Environment: environment,
		IDType:      idType,
		IDValue:     idValue,
	}

	var beneCanNum string
	var bicCode string
	var beneDeathDate sql.NullString
	var middleName sql.NullString
	var rrbHicn sql.NullString

	query, err = rebindQueryForDatabase(environment, databaseNameMARx, query)
	if err != nil {
		return beneficiaryLookupResponse{}, err
	}

	err = db.QueryRow(query, args...).Scan(
		&response.BeneLinkPartKey,
		&response.BeneLinkKey,
		&beneCanNum,
		&bicCode,
		&beneDeathDate,
		&response.BeneBirthDate,
		&response.BeneLastName,
		&response.BeneFirstName,
		&middleName,
		&response.SSN,
		&response.BeneSex,
		&response.ArchiveStatus,
		&response.LastUpdateTS,
		&response.MBI,
		&rrbHicn,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return beneficiaryLookupResponse{}, fmt.Errorf("no beneficiary found for %s %s", idType, idValue)
		}

		return beneficiaryLookupResponse{}, err
	}

	response.HICN = strings.TrimSpace(beneCanNum + bicCode)
	response.BeneDeathDate = nullableStringValue(beneDeathDate)
	response.MiddleName = nullableStringValue(middleName)
	response.RRBHICN = nullableStringValue(rrbHicn)

	return response, nil
}

func buildBeneficiaryLookupQuery(idType, idValue string) (string, []any, error) {
	baseQuery := `SELECT
		bene_link_part_key,
		bene_link_key,
		bene_can_num,
		bic_cd,
		bene_death_dt,
		bene_birth_dt,
		bene_last_name,
		bene_1st_name,
		mdl_name,
		ssn_num,
		CASE bene_sex_cd WHEN '1' THEN 'Male' WHEN '2' THEN 'Female' ELSE 'Other' END AS bene_sex_cd,
		CASE bene_arcv_stus_ind WHEN '1' THEN 'Active' ELSE 'Archived' END AS bene_arcv_stus_ind,
		rec_updt_ts,
		mbi_id,
		rrb_hic_num
	FROM cme_bene_stus
	WHERE `

	switch idType {
	case beneIDTypeBLK:
		blpk := makeBlpkFromBlk(idValue)
		return baseQuery + "bene_link_part_key = ? AND bene_link_key = ?", []any{blpk, idValue}, nil
	case beneIDTypeMBI:
		return baseQuery + "mbi_id = ?", []any{idValue}, nil
	case beneIDTypeHICN:
		if len(idValue) < 10 {
			return "", nil, fmt.Errorf("invalid HICN value %s", idValue)
		}

		return baseQuery + "bene_can_num = ? AND bic_cd = ?", []any{idValue[0:9], idValue[9:]}, nil
	case beneIDTypeSSN:
		return baseQuery + "ssn_num = ?", []any{idValue}, nil
	case beneIDTypeRRBHICN:
		return baseQuery + "rrb_hic_num = ?", []any{idValue}, nil
	default:
		return "", nil, fmt.Errorf("unsupported beneficiary id type: %s", idType)
	}
}

func makeBlpkFromBlk(blk string) string {
	blk = strings.TrimSpace(blk)

	switch blkLength := len(blk); {
	case blkLength >= 3:
		return fmt.Sprintf("%s%s%s", string(blk[blkLength-1]), string(blk[blkLength-2]), string(blk[blkLength-3]))
	case blkLength == 2:
		return fmt.Sprintf("%s%s", string(blk[blkLength-1]), string(blk[blkLength-2]))
	case blkLength == 1:
		return string(blk[blkLength-1])
	default:
		return ""
	}
}

func nullableStringValue(value sql.NullString) string {
	if value.Valid {
		return value.String
	}

	return ""
}