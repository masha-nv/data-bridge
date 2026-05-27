package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.cms.gov/MEPS/marx-db/dbconn"
	"github.cms.gov/MEPS/marx-logging-interface/logging"
	"github.cms.gov/MEPS/marx-move-data/movedata"
)

var marxMovedataLoggingInit sync.Once

func copyBeneCopyMarxMovedataRows(ctx context.Context, jobID, sourceEnvironment, targetEnvironment, beneLinkPartKey, beneLinkKey string, progress beneCopyProgressFunc, authContext beneCopyJobAuthContext) (beneCopyExecutionResult, error) {
	_ = jobID
	_ = progress

	ensureMarxMovedataLogging()

	if err := ctx.Err(); err != nil {
		return beneCopyExecutionResult{}, err
	}

	parsedBeneLinkPartKey, err := strconv.ParseInt(strings.TrimSpace(beneLinkPartKey), 10, 16)
	if err != nil {
		return beneCopyExecutionResult{}, fmt.Errorf("beneLinkPartKey must be numeric: %w", err)
	}
	parsedBeneLinkKey, err := strconv.ParseInt(strings.TrimSpace(beneLinkKey), 10, 32)
	if err != nil {
		return beneCopyExecutionResult{}, fmt.Errorf("beneLinkKey must be numeric: %w", err)
	}

	marxMetadata, marxClose, err := openMarxMovedataMetadata(databaseNameMARx, sourceEnvironment, targetEnvironment, defaultBeneCopyMARxConfigFileName, authContext)
	if err != nil {
		return beneCopyExecutionResult{}, err
	}
	defer marxClose()

	pwaMetadata, pwaClose, err := openMarxMovedataMetadata(databaseNamePWA, sourceEnvironment, targetEnvironment, defaultBeneCopyPWAConfigFileName, authContext)
	if err != nil {
		return beneCopyExecutionResult{}, err
	}
	defer pwaClose()

	databasesMetadata := movedata.DatabasesMetadata{marxMetadata, pwaMetadata}
	if err := databasesMetadata.Copy(int16(parsedBeneLinkPartKey), int32(parsedBeneLinkKey)); err != nil {
		return beneCopyExecutionResult{}, err
	}

	if err := ctx.Err(); err != nil {
		return beneCopyExecutionResult{}, err
	}

	return beneCopyExecutionResult{
		Success:     true,
		CopiedRows:  0,
		SkippedRows: 0,
		Message:     "Bene copy complete via marx-move-data",
	}, nil
}

func ensureMarxMovedataLogging() {
	marxMovedataLoggingInit.Do(func() {
		logger := logging.Log()
		logging.ExternalLoggingDisabled = true
		if logger.ErrorService == nil {
			logger.ErrorService = &logging.ErrorQueueWriter{
				SystemID:    logging.SystemIDMarx,
				SubSystemID: logging.SubSystemCore,
				ServiceName: "data-bridge-marx-movedata",
			}
		}
	})
}

func openMarxMovedataMetadata(databaseName, sourceEnvironment, targetEnvironment, tableMetadataFile string, authContext beneCopyJobAuthContext) (*movedata.DatabaseMetadata, func(), error) {
	configPath, found, err := resolveBeneCopyConfigPath(tableMetadataFile)
	if err != nil {
		return nil, nil, err
	}
	if !found {
		return nil, nil, fmt.Errorf("bene copy table config %s was not found", tableMetadataFile)
	}

	sourceReader, err := openDatabaseWithCredentials(sourceEnvironment, databaseName, authContext.UserID, authContext.Password)
	if err != nil {
		return nil, nil, err
	}

	sourceWriter, err := openDatabaseWithCredentials(sourceEnvironment, databaseName, authContext.UserID, authContext.Password)
	if err != nil {
		sourceReader.Close()
		return nil, nil, err
	}

	destinationWriter, err := openDatabaseWithCredentials(targetEnvironment, databaseName, authContext.UserID, authContext.Password)
	if err != nil {
		sourceWriter.Close()
		sourceReader.Close()
		return nil, nil, err
	}

	databaseType, err := marxMovedataDatabaseType(databaseName)
	if err != nil {
		destinationWriter.Close()
		sourceWriter.Close()
		sourceReader.Close()
		return nil, nil, err
	}

	metadata, err := movedata.New(databaseType, sourceReader, sourceWriter, destinationWriter, configPath)
	if err != nil {
		destinationWriter.Close()
		sourceWriter.Close()
		sourceReader.Close()
		return nil, nil, err
	}

	cleanup := func() {
		destinationWriter.Close()
		sourceWriter.Close()
		sourceReader.Close()
	}

	return metadata, cleanup, nil
}

func marxMovedataDatabaseType(databaseName string) (dbconn.DatabaseType, error) {
	switch strings.ToLower(strings.TrimSpace(databaseName)) {
	case strings.ToLower(databaseNameMARx):
		return dbconn.Postgres, nil
	case strings.ToLower(databaseNamePWA):
		return dbconn.MySQL, nil
	default:
		return "", fmt.Errorf("unsupported marx-move-data database %s", databaseName)
	}
}