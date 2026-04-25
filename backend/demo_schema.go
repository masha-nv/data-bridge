package main

import (
	"database/sql"
	"fmt"
)

var demoSchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS mcs_tran_reply (
		tran_rply_cd TEXT PRIMARY KEY,
		tran_rply_desc TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS marx_pwr_agency_reply (
		deductibility_code TEXT NOT NULL,
		agency_reply_code TEXT NOT NULL,
		withholding_agency TEXT NOT NULL,
		premium_type_code TEXT NOT NULL,
		reply_code_description TEXT NOT NULL,
		obsolete_date TEXT,
		PRIMARY KEY (deductibility_code, agency_reply_code, withholding_agency, premium_type_code)
	)`,
	`CREATE TABLE IF NOT EXISTS cme_bene_stus (
		bene_link_part_key INTEGER NOT NULL,
		bene_link_key INTEGER NOT NULL,
		bene_can_num TEXT,
		bic_cd TEXT,
		bene_death_dt TEXT,
		bene_birth_dt TEXT,
		bene_last_name TEXT,
		bene_1st_name TEXT,
		mdl_name TEXT,
		ssn_num TEXT,
		bene_sex_cd TEXT,
		bene_arcv_stus_ind TEXT,
		rec_updt_ts TEXT,
		mbi_id TEXT,
		rrb_hic_num TEXT,
		PRIMARY KEY (bene_link_part_key, bene_link_key)
	)`,
	`CREATE TABLE IF NOT EXISTS sr_batch_type (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		title TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS sr_batch_job (
		id INTEGER PRIMARY KEY,
		in_file_path TEXT,
		in_file_uri TEXT
	)`,
	`CREATE TABLE IF NOT EXISTS sr_batch_thread (
		id INTEGER PRIMARY KEY,
		status_cd INTEGER NOT NULL,
		job_id INTEGER NOT NULL,
		thread_key TEXT,
		thread_pool_id INTEGER NOT NULL,
		start_date_time TEXT,
		status_date_time TEXT,
		created_date_time TEXT,
		end_date_time TEXT,
		FOREIGN KEY (job_id) REFERENCES sr_batch_job(id),
		FOREIGN KEY (thread_pool_id) REFERENCES sr_batch_type(id)
	)`,
}

func ensureDemoSchema(db *sql.DB) error {
	for _, statement := range demoSchemaStatements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("create demo schema: %w", err)
		}
	}

	return nil
}
