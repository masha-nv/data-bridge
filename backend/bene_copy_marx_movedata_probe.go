package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const marxMovedataProbeCommand = "marx-movedata-probe"

func init() {
	if len(os.Args) < 2 || os.Args[1] != marxMovedataProbeCommand {
		return
	}

	if err := runMarxMovedataProbe(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "marx-movedata probe failed: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func runMarxMovedataProbe(args []string) error {
	if len(args) != 6 {
		return fmt.Errorf("usage: go run . %s <sourceEnvironment> <targetEnvironment> <beneLinkPartKey> <beneLinkKey> <userId> <password>", marxMovedataProbeCommand)
	}

	if err := loadRuntimeConfig(); err != nil {
		return fmt.Errorf("load runtime config: %w", err)
	}

	sourceEnvironment := args[0]
	targetEnvironment := args[1]
	beneLinkPartKey := args[2]
	beneLinkKey := args[3]
	userID := args[4]
	password := args[5]

	ctx, cancel := context.WithTimeout(context.Background(), beneCopyExecutionTimeout)
	defer cancel()

	result, err := copyBeneCopyMarxMovedataRows(
		ctx,
		"terminal-probe",
		sourceEnvironment,
		targetEnvironment,
		beneLinkPartKey,
		beneLinkKey,
		func(progress beneCopyProgress) {
			fmt.Fprintf(os.Stderr, "progress: table=%s copied=%d skipped=%d\n", progress.CurrentTable, progress.CopiedRows, progress.SkippedRows)
		},
		beneCopyJobAuthContext{
			UserID:      userID,
			Password:    password,
			DisplayName: userID,
		},
	)
	if err != nil {
		return err
	}

	output := struct {
		RanAt  time.Time               `json:"ranAt"`
		Result beneCopyExecutionResult `json:"result"`
	}{
		RanAt:  time.Now().UTC(),
		Result: result,
	}

	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal probe result: %w", err)
	}

	fmt.Fprintln(os.Stdout, string(encoded))
	return nil
}