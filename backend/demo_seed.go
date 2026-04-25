package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func ensureDemoSeedData(env string, db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin demo seed transaction for %s: %w", env, err)
	}

	if err := seedDescriptions(tx); err != nil {
		tx.Rollback()
		return fmt.Errorf("seed descriptions for %s: %w", env, err)
	}

	if err := seedBeneficiaries(env, tx); err != nil {
		tx.Rollback()
		return fmt.Errorf("seed beneficiaries for %s: %w", env, err)
	}

	if err := seedBatchJobs(env, tx); err != nil {
		tx.Rollback()
		return fmt.Errorf("seed batch jobs for %s: %w", env, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit demo seed transaction for %s: %w", env, err)
	}

	return nil
}

func seedDescriptions(tx *sql.Tx) error {
	trcRows := [][]any{
		{"T001", "Transaction accepted for downstream processing"},
		{"T101", "Transaction suspended for manual review"},
		{"E201", "Enrollment reply indicates missing beneficiary data"},
	}

	for _, row := range trcRows {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO mcs_tran_reply (tran_rply_cd, tran_rply_desc) VALUES (?, ?)`,
			row...,
		); err != nil {
			return err
		}
	}

	pwrRows := [][]any{
		{"I", "0002", "S", "CD", "SSA Part C/D premium updated successfully", nil},
		{"E", "0002", "S", "CD", "SSA reported premium adjustment error", nil},
		{"C", "100", "R", "CD", "RRB withholding accepted for Part C/D", nil},
		{"B", "0005", "S", "B", "SSA Part B withholding confirmed", nil},
		{"ES", "008", "R", "B", "RRB Part B withholding exception needs review", nil},
	}

	for _, row := range pwrRows {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO marx_pwr_agency_reply (
				deductibility_code,
				agency_reply_code,
				withholding_agency,
				premium_type_code,
				reply_code_description,
				obsolete_date
			) VALUES (?, ?, ?, ?, ?, ?)`,
			row...,
		); err != nil {
			return err
		}
	}

	return nil
}

func seedBeneficiaries(env string, tx *sql.Tx) error {
	envTag := strings.ToUpper(env)
	now := time.Now().UTC().Format(time.RFC3339)

	beneRows := [][]any{
		{321, 123123123, "123456789", "A", nil, "1950-03-14", fmt.Sprintf("%s-JOHNSON", envTag), "ALICE", "MARIE", "111223333", "2", "1", now, fmt.Sprintf("1EG4TE5MK7%d", len(envTag)), fmt.Sprintf("RRB-%s-1001", envTag)},
		{654, 456456456, "987654321", "B", nil, "1942-11-02", fmt.Sprintf("%s-SMITH", envTag), "ROBERT", nil, "222334444", "1", "1", now, fmt.Sprintf("1EG4TE5MK8%d", len(envTag)), nil},
		{987, 789789789, "555667777", "C", "2022-05-22", "1938-07-09", fmt.Sprintf("%s-WALKER", envTag), "EVELYN", "J", "333445555", "2", "0", now, fmt.Sprintf("1EG4TE5MK9%d", len(envTag)), fmt.Sprintf("RRB-%s-1003", envTag)},
	}

	for _, row := range beneRows {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO cme_bene_stus (
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
				bene_sex_cd,
				bene_arcv_stus_ind,
				rec_updt_ts,
				mbi_id,
				rrb_hic_num
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			row...,
		); err != nil {
			return err
		}
	}

	return nil
}

func seedBatchJobs(env string, tx *sql.Tx) error {
	envTag := strings.ToUpper(env)
	now := time.Now().UTC()
	activeStatusTime := now.Add(-15 * time.Minute).Format(time.RFC3339)
	completedStatusTime := now.Add(-90 * time.Minute).Format(time.RFC3339)
	createdTime := now.Add(-2 * time.Hour).Format(time.RFC3339)
	startTime := now.Add(-105 * time.Minute).Format(time.RFC3339)
	endTime := now.Add(-80 * time.Minute).Format(time.RFC3339)

	batchTypes := [][]any{
		{1, "Eligibility Update", "Eligibility Update Batch"},
		{2, "Premium Reconciliation", "Premium Reconciliation Batch"},
	}

	for _, row := range batchTypes {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO sr_batch_type (id, name, title) VALUES (?, ?, ?)`, row...); err != nil {
			return err
		}
	}

	batchJobs := [][]any{
		{1001, fmt.Sprintf("/%s/inbound/eligibility_1001.dat", strings.ToLower(envTag)), fmt.Sprintf("s3://demo-%s/eligibility_1001.dat", strings.ToLower(envTag))},
		{1002, fmt.Sprintf("/%s/inbound/reconcile_1002.dat", strings.ToLower(envTag)), fmt.Sprintf("s3://demo-%s/reconcile_1002.dat", strings.ToLower(envTag))},
	}

	for _, row := range batchJobs {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO sr_batch_job (id, in_file_path, in_file_uri) VALUES (?, ?, ?)`, row...); err != nil {
			return err
		}
	}

	threads := [][]any{
		{2001, 2, 1001, fmt.Sprintf("%s-THREAD-01", envTag), 1, startTime, activeStatusTime, createdTime, nil},
		{2002, 4, 1002, fmt.Sprintf("%s-THREAD-02", envTag), 2, startTime, completedStatusTime, createdTime, endTime},
	}

	for _, row := range threads {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO sr_batch_thread (
				id,
				status_cd,
				job_id,
				thread_key,
				thread_pool_id,
				start_date_time,
				status_date_time,
				created_date_time,
				end_date_time
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			row...,
		); err != nil {
			return err
		}
	}

	return nil
}