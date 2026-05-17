package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	defaultAppConfigFileName = "backend-config.json"
	defaultDBConfigFileName  = "backend-db-parms.json"
	databaseNameMARx         = "MARx"
	databaseNamePWA          = "PWA"
	databaseNameBatch        = "Batch"
	appModeDemo              = "demo"
	appModeReal              = "real"
)

type backendAppConfig struct {
	Mode              string `json:"mode"`
	SupportedEnvs     string `json:"supportedEnvironments"`
	DBConfigFile      string `json:"dbConfigFile"`
	DBCertificateFile string `json:"dbCertificateFile"`
}

type databaseDefinition struct {
	DBDriver string `json:"dbDriver"`
	DBHost   string `json:"dbHost"`
	DBPort   string `json:"dbPort"`
	DBName   string `json:"dbName"`
	DBSchema string `json:"dbSchema"`
	DBParams string `json:"dbParams"`
	DBFile   string `json:"dbFile"`
}

type databaseInventory map[string]map[string]databaseDefinition

type runtimeConfig struct {
	appConfig backendAppConfig
	dbConfigs databaseInventory
}

var (
	runtimeConfigOnce sync.Once
	runtimeConfigData runtimeConfig
	runtimeConfigErr  error
)

func loadRuntimeConfig() error {
	runtimeConfigOnce.Do(func() {
		appConfig, err := loadAppConfig()
		if err != nil {
			runtimeConfigErr = err
			return
		}

		dbConfigs, err := loadDatabaseInventory(appConfig.DBConfigFile)
		if err != nil {
			runtimeConfigErr = err
			return
		}

		runtimeConfigData = runtimeConfig{
			appConfig: appConfig,
			dbConfigs: dbConfigs,
		}
	})

	return runtimeConfigErr
}

func loadAppConfig() (backendAppConfig, error) {
	appConfig := defaultAppConfig()
	configPath, found, err := resolveConfigPath(defaultAppConfigFileName)
	if err != nil {
		return backendAppConfig{}, err
	}
	if !found {
		return appConfig, nil
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return backendAppConfig{}, fmt.Errorf("read app config: %w", err)
	}

	if err := json.Unmarshal(content, &appConfig); err != nil {
		return backendAppConfig{}, fmt.Errorf("parse app config %s: %w", configPath, err)
	}

	appConfig.Mode = strings.ToLower(strings.TrimSpace(appConfig.Mode))
	if appConfig.Mode == "" {
		appConfig.Mode = appModeDemo
	}
	if appConfig.DBConfigFile == "" {
		appConfig.DBConfigFile = defaultDBConfigFileName
	}

	return appConfig, nil
}

func loadDatabaseInventory(fileName string) (databaseInventory, error) {
	configPath, found, err := resolveConfigPath(fileName)
	if err != nil {
		return nil, err
	}
	if !found {
		return defaultDatabaseInventory(), nil
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read database inventory: %w", err)
	}

	var inventory databaseInventory
	if err := json.Unmarshal(content, &inventory); err != nil {
		return nil, fmt.Errorf("parse database inventory %s: %w", configPath, err)
	}

	return inventory, nil
}

func resolveConfigPath(fileName string) (string, bool, error) {
	if strings.TrimSpace(fileName) == "" {
		return "", false, nil
	}

	searchDirs := make([]string, 0, 2)
	workingDir, err := os.Getwd()
	if err != nil {
		return "", false, fmt.Errorf("get current directory: %w", err)
	}
	searchDirs = append(searchDirs, workingDir)

	execPath, err := os.Executable()
	if err == nil {
		searchDirs = append(searchDirs, filepath.Dir(execPath))
	}

	for _, directory := range searchDirs {
		candidate := filepath.Join(directory, fileName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", false, err
		}
	}

	return "", false, nil
}

func defaultAppConfig() backendAppConfig {
	return backendAppConfig{
		Mode:              appModeDemo,
		SupportedEnvs:     "Dev2,Test2,Impl2,Prod2",
		DBConfigFile:      defaultDBConfigFileName,
		DBCertificateFile: "rds-ca-2019-root.pem",
	}
}

func defaultDatabaseInventory() databaseInventory {
	return databaseInventory{
		"Dev2": {
			databaseNameMARx:  {DBDriver: sqliteDriverName, DBFile: "dev2.db"},
			databaseNamePWA:   {DBDriver: sqliteDriverName, DBFile: "dev2.db"},
			databaseNameBatch: {DBDriver: sqliteDriverName, DBFile: "dev2.db"},
		},
		"Test2": {
			databaseNameMARx:  {DBDriver: sqliteDriverName, DBFile: "test2.db"},
			databaseNamePWA:   {DBDriver: sqliteDriverName, DBFile: "test2.db"},
			databaseNameBatch: {DBDriver: sqliteDriverName, DBFile: "test2.db"},
		},
		"Impl2": {
			databaseNameMARx:  {DBDriver: sqliteDriverName, DBFile: "impl2.db"},
			databaseNamePWA:   {DBDriver: sqliteDriverName, DBFile: "impl2.db"},
			databaseNameBatch: {DBDriver: sqliteDriverName, DBFile: "impl2.db"},
		},
		"Prod2": {
			databaseNameMARx:  {DBDriver: sqliteDriverName, DBFile: "prod2.db"},
			databaseNamePWA:   {DBDriver: sqliteDriverName, DBFile: "prod2.db"},
			databaseNameBatch: {DBDriver: sqliteDriverName, DBFile: "prod2.db"},
		},
	}
}

func getAppMode() string {
	return runtimeConfigData.appConfig.Mode
}

func getSupportedEnvironments() []string {
	if strings.TrimSpace(runtimeConfigData.appConfig.SupportedEnvs) != "" {
		parts := strings.Split(runtimeConfigData.appConfig.SupportedEnvs, ",")
		envs := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				envs = append(envs, trimmed)
			}
		}
		return envs
	}

	envs := make([]string, 0, len(runtimeConfigData.dbConfigs))
	for env := range runtimeConfigData.dbConfigs {
		envs = append(envs, env)
	}
	sort.Strings(envs)
	return envs
}

func getDatabaseDefinition(environment, databaseName string) (databaseDefinition, error) {
	environmentConfigs, ok := runtimeConfigData.dbConfigs[environment]
	if !ok {
		return databaseDefinition{}, fmt.Errorf("unknown environment: %s", environment)
	}

	definition, ok := environmentConfigs[databaseName]
	if !ok {
		return databaseDefinition{}, fmt.Errorf("database %s is not configured for environment %s", databaseName, environment)
	}

	return definition, nil
}

func getCertificateFileName() string {
	return runtimeConfigData.appConfig.DBCertificateFile
}