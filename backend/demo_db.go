package main

import (
	"database/sql"
	"fmt"
	"sort"
)

func getDemoDatabaseFiles() map[string]string {
	databaseFiles := make(map[string]string, len(getSupportedEnvironments()))
	for _, env := range getSupportedEnvironments() {
		definition, err := getDatabaseDefinition(env, databaseNameMARx)
		if err != nil {
			continue
		}
		databaseFiles[env] = definition.DBFile
	}

	return databaseFiles
}

func getSortedDemoEnvironments() []string {
	envs := append([]string(nil), getSupportedEnvironments()...)
	sort.Strings(envs)
	return envs
}

func ensureDemoDatabaseFile(env, dbFile string) error {
	db, err := sql.Open(sqliteDriverName, dbFile)
	if err != nil {
		return fmt.Errorf("open demo database for %s: %w", env, err)
	}
	defer db.Close()

	if err := ensureDemoSchema(db); err != nil {
		return fmt.Errorf("ensure demo schema for %s: %w", env, err)
	}

	if err := ensureDemoSeedData(env, db); err != nil {
		return fmt.Errorf("ensure demo seed data for %s: %w", env, err)
	}

	return nil
}

func validateDemoDatabaseSetup() error {
	if getAppMode() != appModeDemo {
		return nil
	}

	databaseFiles := getDemoDatabaseFiles()
	for _, env := range getSortedDemoEnvironments() {
		dbFile, ok := databaseFiles[env]
		if !ok {
			return fmt.Errorf("missing database mapping for environment %s", env)
		}
		if err := ensureDemoDatabaseFile(env, dbFile); err != nil {
			return err
		}
	}

	return nil
}