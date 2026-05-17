package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
)

const sqliteDriverName = "sqlite3"

const databaseConnectTimeout = 3 * time.Second

type databaseAdapter interface {
	Open(environment, databaseName string) (*sql.DB, error)
}

type demoSQLiteAdapter struct{}

type realDatabaseAdapter struct{}

var (
	mysqlTLSRegistrationOnce sync.Once
	mysqlTLSRegistrationErr  error
)

func (demoSQLiteAdapter) Open(environment, databaseName string) (*sql.DB, error) {
	definition, err := getDatabaseDefinition(environment, databaseName)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(definition.DBFile) == "" {
		return nil, fmt.Errorf("database %s in %s does not define a dbFile", databaseName, environment)
	}

	return sql.Open(sqliteDriverName, definition.DBFile)
}

func (realDatabaseAdapter) Open(environment, databaseName string) (*sql.DB, error) {
	session, ok := getActiveRealSession()
	if !ok {
		return nil, fmt.Errorf("no active real database session; log in first")
	}

	definition, err := getDatabaseDefinition(environment, databaseName)
	if err != nil {
		return nil, err
	}

	return openConnectionWithDefinition(session.UserID, session.Password, definition)
}

func openConnectionWithDefinition(userID, password string, definition databaseDefinition) (*sql.DB, error) {
	switch definition.DBDriver {
	case sqliteDriverName:
		return sql.Open(sqliteDriverName, definition.DBFile)
	case "postgres":
		return openPostgresConnection(userID, password, definition)
	case "mysql":
		return openMySQLConnection(userID, password, definition)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", definition.DBDriver)
	}
}

func openPostgresConnection(userID, password string, definition databaseDefinition) (*sql.DB, error) {
	certificatePath, err := resolveCertificatePath()
	if err != nil {
		return nil, err
	}

	connectionURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(userID, password),
		Host:   net.JoinHostPort(strings.TrimSpace(definition.DBHost), defaultPort(definition.DBPort, "5432")),
		Path:   strings.TrimSpace(definition.DBName),
	}

	queryValues := connectionURL.Query()
	queryValues.Set("sslmode", "verify-full")
	queryValues.Set("sslrootcert", certificatePath)
	queryValues.Set("connect_timeout", strconv.Itoa(int(databaseConnectTimeout/time.Second)))
	if strings.TrimSpace(definition.DBSchema) != "" {
		queryValues.Set("search_path", definition.DBSchema)
	}
	if err := mergeQueryParams(queryValues, definition.DBParams); err != nil {
		return nil, fmt.Errorf("parse postgres connection parameters: %w", err)
	}
	connectionURL.RawQuery = queryValues.Encode()

	connector, err := pq.NewConnector(connectionURL.String())
	if err != nil {
		return nil, fmt.Errorf("create postgres connector: %w", err)
	}

	db := sql.OpenDB(connector)
	if err := pingWithTimeout(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres database %s: %w", definition.DBHost, err)
	}

	return db, nil
}

func openMySQLConnection(userID, password string, definition databaseDefinition) (*sql.DB, error) {
	certificatePath, err := resolveCertificatePath()
	if err != nil {
		return nil, err
	}

	if err := registerMySQLTLSConfig(certificatePath); err != nil {
		return nil, err
	}

	config := mysql.NewConfig()
	config.User = userID
	config.Passwd = password
	config.Net = "tcp"
	config.Addr = net.JoinHostPort(strings.TrimSpace(definition.DBHost), defaultPort(definition.DBPort, "3306"))
	config.DBName = strings.TrimSpace(definition.DBName)
	config.TLSConfig = "rds-custom"
	config.Timeout = databaseConnectTimeout
	config.ReadTimeout = databaseConnectTimeout
	config.WriteTimeout = databaseConnectTimeout
	if err := applyMySQLParams(config, definition.DBParams); err != nil {
		return nil, fmt.Errorf("parse mysql connection parameters: %w", err)
	}

	db, err := sql.Open(definition.DBDriver, config.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("open mysql connection: %w", err)
	}
	if err := pingWithTimeout(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping mysql database %s: %w", definition.DBHost, err)
	}

	return db, nil
}

func registerMySQLTLSConfig(certificatePath string) error {
	mysqlTLSRegistrationOnce.Do(func() {
		pem, err := os.ReadFile(certificatePath)
		if err != nil {
			mysqlTLSRegistrationErr = fmt.Errorf("read mysql certificate %s: %w", certificatePath, err)
			return
		}

		rootCertPool := x509.NewCertPool()
		if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
			mysqlTLSRegistrationErr = fmt.Errorf("append mysql certificate PEM failed")
			return
		}

		mysqlTLSRegistrationErr = mysql.RegisterTLSConfig("rds-custom", &tls.Config{RootCAs: rootCertPool})
	})

	return mysqlTLSRegistrationErr
}

func resolveCertificatePath() (string, error) {
	certificateFile := strings.TrimSpace(getCertificateFileName())
	if certificateFile == "" {
		return "", fmt.Errorf("certificate file is not configured")
	}

	path, found, err := resolveConfigPath(certificateFile)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("certificate file %s was not found", certificateFile)
	}

	if runtime.GOOS == "windows" {
		return filepath.Clean(path), nil
	}

	return path, nil
}

func pingWithTimeout(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), databaseConnectTimeout)
	defer cancel()

	return db.PingContext(ctx)
}

func defaultPort(port, fallback string) string {
	trimmedPort := strings.TrimSpace(port)
	if trimmedPort == "" {
		return fallback
	}

	return trimmedPort
}

func mergeQueryParams(queryValues url.Values, rawParams string) error {
	parsedValues, err := parseRawQueryParams(rawParams)
	if err != nil {
		return err
	}

	for key, values := range parsedValues {
		if len(values) == 0 {
			continue
		}
		queryValues.Del(key)
		for _, value := range values {
			queryValues.Add(key, value)
		}
	}

	return nil
}

func applyMySQLParams(config *mysql.Config, rawParams string) error {
	parsedValues, err := parseRawQueryParams(rawParams)
	if err != nil {
		return err
	}

	for key, values := range parsedValues {
		if len(values) == 0 {
			continue
		}

		value := values[len(values)-1]
		switch strings.ToLower(key) {
		case "interpolateparams":
			parsedValue, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("%s=%q: %w", key, value, err)
			}
			config.InterpolateParams = parsedValue
		case "parsetime":
			parsedValue, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("%s=%q: %w", key, value, err)
			}
			config.ParseTime = parsedValue
		case "multistatements":
			parsedValue, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("%s=%q: %w", key, value, err)
			}
			config.MultiStatements = parsedValue
		case "timeout":
			parsedValue, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("%s=%q: %w", key, value, err)
			}
			config.Timeout = parsedValue
		case "readtimeout":
			parsedValue, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("%s=%q: %w", key, value, err)
			}
			config.ReadTimeout = parsedValue
		case "writetimeout":
			parsedValue, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("%s=%q: %w", key, value, err)
			}
			config.WriteTimeout = parsedValue
		default:
			if config.Params == nil {
				config.Params = map[string]string{}
			}
			config.Params[key] = value
		}
	}

	return nil
}

func parseRawQueryParams(rawParams string) (url.Values, error) {
	trimmedParams := strings.TrimSpace(rawParams)
	if trimmedParams == "" {
		return url.Values{}, nil
	}

	parsedValues, err := url.ParseQuery(trimmedParams)
	if err != nil {
		return nil, err
	}

	return parsedValues, nil
}

func getDatabaseAdapter() databaseAdapter {
	switch getAppMode() {
	case appModeDemo:
		return demoSQLiteAdapter{}
	case appModeReal:
		return realDatabaseAdapter{}
	default:
		return demoSQLiteAdapter{}
	}
}

func openDatabaseWithCredentials(environment, databaseName, userID, password string) (*sql.DB, error) {
	definition, err := getDatabaseDefinition(environment, databaseName)
	if err != nil {
		return nil, err
	}

	switch getAppMode() {
	case appModeDemo:
		return demoSQLiteAdapter{}.Open(environment, databaseName)
	case appModeReal:
		if strings.TrimSpace(userID) == "" || strings.TrimSpace(password) == "" {
			return nil, fmt.Errorf("no captured real database credentials; log in first")
		}
		return openConnectionWithDefinition(userID, password, definition)
	default:
		return demoSQLiteAdapter{}.Open(environment, databaseName)
	}
}