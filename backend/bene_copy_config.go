package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultBeneCopyMARxConfigFileName = "marx_bene_recall_marx_table_config.json"
	defaultBeneCopyPWAConfigFileName  = "marx_bene_recall_pwa_table_config.json"
)

type beneCopyTableConfig struct {
	Tables []beneCopyTableNode `json:"Tables"`
}

type beneCopyTableNode struct {
	Table beneCopyTableDefinition `json:"Table"`
}

type beneCopyTableDefinition struct {
	TableName          string                   `json:"TableName"`
	TableHasBeneLinkKey bool                    `json:"TableHasBeneLinkKey"`
	ParentTable        *beneCopyParentTable     `json:"ParentTable,omitempty"`
	Tables             []beneCopyTableNode      `json:"Tables,omitempty"`
}

type beneCopyParentTable struct {
	TableName string                  `json:"TableName"`
	Columns   []beneCopyColumnMapping `json:"Columns"`
}

type beneCopyColumnMapping struct {
	NameParent string `json:"NameParent"`
	NameChild  string `json:"NameChild"`
}

func loadBeneCopyMARxConfig() (beneCopyTableConfig, error) {
	return loadBeneCopyTableConfig(defaultBeneCopyMARxConfigFileName)
}

func loadBeneCopyPWAConfig() (beneCopyTableConfig, error) {
	return loadBeneCopyTableConfig(defaultBeneCopyPWAConfigFileName)
}

func loadBeneCopyTableConfig(fileName string) (beneCopyTableConfig, error) {
	configPath, found, err := resolveBeneCopyConfigPath(fileName)
	if err != nil {
		return beneCopyTableConfig{}, err
	}
	if !found {
		return beneCopyTableConfig{}, fmt.Errorf("bene copy table config %s was not found", fileName)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return beneCopyTableConfig{}, fmt.Errorf("read bene copy table config %s: %w", configPath, err)
	}

	var config beneCopyTableConfig
	if err := json.Unmarshal(content, &config); err != nil {
		return beneCopyTableConfig{}, fmt.Errorf("parse bene copy table config %s: %w", configPath, err)
	}

	return config, nil
}

func resolveBeneCopyConfigPath(fileName string) (string, bool, error) {
	trimmedFileName := strings.TrimSpace(fileName)
	if trimmedFileName == "" {
		return "", false, nil
	}

	searchBases := make([]string, 0, 4)
	workingDir, err := os.Getwd()
	if err != nil {
		return "", false, fmt.Errorf("get current directory for bene copy config: %w", err)
	}
	searchBases = append(searchBases, workingDir)

	execPath, err := os.Executable()
	if err == nil {
		searchBases = append(searchBases, filepath.Dir(execPath))
	}

	relativeCandidates := []string{
		trimmedFileName,
		filepath.Join("..", trimmedFileName),
		filepath.Join("..", "marx-bene-recall", trimmedFileName),
		filepath.Join("..", "..", "marx-bene-recall", trimmedFileName),
	}

	seen := map[string]bool{}
	for _, base := range searchBases {
		for _, relativeCandidate := range relativeCandidates {
			candidate := filepath.Clean(filepath.Join(base, relativeCandidate))
			if seen[candidate] {
				continue
			}
			seen[candidate] = true

			if _, err := os.Stat(candidate); err == nil {
				return candidate, true, nil
			} else if !os.IsNotExist(err) {
				return "", false, err
			}
		}
	}

	return "", false, nil
}