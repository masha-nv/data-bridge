package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"

	moderncsqlite "modernc.org/sqlite"
)

func init() {
	sql.Register("sqlite3", &moderncsqlite.Driver{})
}

func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

func openDB(env string) (*sql.DB, error) {
	return openMARxDB(env)
}

func openMARxDB(env string) (*sql.DB, error) {
	return getDatabaseAdapter().Open(env, databaseNameMARx)
}

func openBatchDB(env string) (*sql.DB, error) {
	return getDatabaseAdapter().Open(env, databaseNameBatch)
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	type Req struct {
		Value map[string]string `json:"value"`
		Envs  []string          `json:"envs"`
		Table string            `json:"table"`
	}
	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	results := make(map[string][]map[string]any)
	for _, env := range req.Envs {
		db, err := openDB(env)
		if err != nil {
			results[env] = []map[string]any{{"error": err.Error()}}
			continue
		}
		defer db.Close()
		table := req.Table
		if table == "" {
			http.Error(w, "table is required", 400)
			return
		}
		query := "SELECT * FROM " + table + " WHERE 1=1"
		args := []any{}
		// Get actual columns for the table
		schemaDB, err := openDB(env)
		if err != nil {
			results[env] = []map[string]any{{"error": err.Error()}}
			continue
		}
		colRows, err := schemaDB.Query("PRAGMA table_info(" + table + ")")
		if err != nil {
			results[env] = []map[string]any{{"error": err.Error()}}
			schemaDB.Close()
			continue
		}
		actualCols := map[string]bool{}
		for colRows.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dfltValue any
			if err := colRows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err == nil {
				actualCols[name] = true
			}
		}
		colRows.Close()
		schemaDB.Close()
		for col, val := range req.Value {
			if val == "" {
				continue
			}
			if !actualCols[col] {
				continue // skip keys not in table
			}
			// Use LIKE for columns containing 'name', exact match otherwise
			if strings.Contains(strings.ToLower(col), "name") {
				query += " AND " + col + " LIKE ?"
				args = append(args, "%"+val+"%")
			} else {
				query += " AND " + col + " = ?"
				args = append(args, val)
			}
		}
		results[env] = []map[string]any{} // clear before appending
		rows, err := db.Query(query, args...)
		if err != nil {
			results[env] = []map[string]any{{"error": err.Error()}}
			continue
		}
		defer rows.Close()
		cols, _ := rows.Columns()
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			rows.Scan(ptrs...)
			rowMap := map[string]any{}
			for i, col := range cols {
				rowMap[col] = vals[i]
			}
			results[env] = append(results[env], rowMap)
		}
	}
	writeJSON(w, results)
}

func tablesHandler(w http.ResponseWriter, r *http.Request) {
	env := r.URL.Query().Get("env")
	db, err := openDB(env)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer db.Close()
	tables, err := listTableNames(db, env, databaseNameMARx)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, tables)
}

func listTableNames(db *sql.DB, environment, databaseName string) ([]string, error) {
	definition, err := getDatabaseDefinition(environment, databaseName)
	if err != nil {
		return nil, err
	}

	switch definition.DBDriver {
	case sqliteDriverName:
		return listSQLiteTableNames(db)
	case "postgres":
		return listPostgresTableNames(db, definition.DBSchema)
	case "mysql":
		return listMySQLTableNames(db)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", definition.DBDriver)
	}
}

func listSQLiteTableNames(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}

	return tables, rows.Err()
}

func listPostgresTableNames(db *sql.DB, configuredSchemas string) ([]string, error) {
	schemas := splitConfiguredSchemas(configuredSchemas)
	if len(schemas) == 0 {
		schemas = []string{"public"}
	}

	query := fmt.Sprintf(
		"SELECT table_schema, table_name FROM information_schema.tables WHERE table_type = 'BASE TABLE' AND table_schema IN (%s) ORDER BY table_schema, table_name",
		buildSQLStringList(schemas),
	)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]string, 0)
	for rows.Next() {
		var schemaName string
		var tableName string
		if err := rows.Scan(&schemaName, &tableName); err != nil {
			return nil, err
		}
		tables = append(tables, schemaName+"."+tableName)
	}

	return tables, rows.Err()
}

func listMySQLTableNames(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() ORDER BY table_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]string, 0)
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		tables = append(tables, tableName)
	}

	return tables, rows.Err()
}

func splitConfiguredSchemas(configuredSchemas string) []string {
	parts := strings.Split(configuredSchemas, ",")
	schemas := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			schemas = append(schemas, trimmed)
		}
	}

	sort.Strings(schemas)
	return schemas
}

func buildSQLStringList(values []string) string {
	quotedValues := make([]string, 0, len(values))
	for _, value := range values {
		quotedValues = append(quotedValues, quoteSQLString(value))
	}

	return strings.Join(quotedValues, ", ")
}

func quoteSQLString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func tableRowsHandler(w http.ResponseWriter, r *http.Request) {
	env := r.URL.Query().Get("env")
	db, err := openDB(env)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer db.Close()
	table := r.URL.Query().Get("table")
	rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s", table))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var results []map[string]interface{}
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		rows.Scan(ptrs...)
		rowMap := map[string]interface{}{}
		for i, col := range cols {
			rowMap[col] = vals[i]
		}
		results = append(results, rowMap)
	}
	writeJSON(w, results)
}

func moveHandler(w http.ResponseWriter, r *http.Request) {
	type Req struct {
		Table   string `json:"table"`
		FromEnv string `json:"fromEnv"`
		ToEnv   string `json:"toEnv"`
	}
	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	fromDB, err := openDB(req.FromEnv)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer fromDB.Close()
	toDB, err := openDB(req.ToEnv)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer toDB.Close()
	tx, err := toDB.Begin()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	rows, err := fromDB.Query(fmt.Sprintf("SELECT * FROM %s", req.Table))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	count := 0
	for rows.Next() {
		rows.Scan(ptrs...)
		placeholders := make([]string, len(cols))
		for i := range cols {
			placeholders[i] = "?"
		}
		insert := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", req.Table, strings.Join(cols, ","), strings.Join(placeholders, ","))
		_, err := tx.Exec(insert, vals...)
		if err != nil {
			// Skip duplicate unique/primary key errors
			if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "UNIQUE constraint violation") || strings.Contains(err.Error(), "constraint failed") {
				continue
			}
			tx.Rollback()
			http.Error(w, err.Error(), 500)
			return
		}
		count++
	}
	tx.Commit()
	writeJSON(w, map[string]any{"moved": count})
}

// clearHandler deletes all rows from all tables in the specified environment's database
func clearHandler(w http.ResponseWriter, r *http.Request) {
	type Req struct {
		Env string `json:"env"`
	}
	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	db, err := openDB(req.Env)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer db.Close()
	// Get all table names
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		tables = append(tables, name)
	}
	for _, table := range tables {
		_, err := db.Exec("DELETE FROM " + table)
		if err != nil {
			http.Error(w, "failed to clear table "+table+": "+err.Error(), 500)
			return
		}
	}
	writeJSON(w, map[string]any{"cleared": tables})
}

func contains(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

func columnsHandler(w http.ResponseWriter, r *http.Request) {
 	table := r.URL.Query().Get("table")
	if table == "" {
		http.Error(w, "table is required", 400)
		return
	}
	defaultEnvironment := ""
	for _, environment := range getSupportedEnvironments() {
		defaultEnvironment = environment
		break
	}
	if defaultEnvironment == "" {
		http.Error(w, "no environments are configured", 500)
		return
	}

	db, err := openDB(defaultEnvironment)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer db.Close()
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		columns = append(columns, name)
	}
	writeJSON(w, columns)
}


func registerSharedRoutes() {
	http.HandleFunc("/api/auth/login", enableCORS(authLoginHandler))
	http.HandleFunc("/api/status", enableCORS(backendStatusHandler))
}

func registerLegacyRoutes() {
	http.HandleFunc("/api/search", enableCORS(searchHandler))
	http.HandleFunc("/api/tables", enableCORS(tablesHandler))
	http.HandleFunc("/api/move", enableCORS(moveHandler))
	http.HandleFunc("/api/clear", enableCORS(clearHandler))
	http.HandleFunc("/api/rows", enableCORS(tableRowsHandler))
	http.HandleFunc("/api/columls", enableCORS(columnsHandler))
}

func registerMarxRoutes() {
	http.HandleFunc("/api/marx/descriptions/lookup", enableCORS(descriptionsLookupHandler))
	http.HandleFunc("/api/marx/beneficiaries/lookup", enableCORS(beneficiaryLookupHandler))
	http.HandleFunc("/api/marx/beneficiaries/copy", enableCORS(beneCopyHandler))
	http.HandleFunc("/api/marx/beneficiaries/copy/movedata", enableCORS(beneCopyMovedataHandler))
	http.HandleFunc("/api/marx/beneficiaries/copy/status", enableCORS(beneCopyStatusHandler))
	http.HandleFunc("/api/marx/beneficiaries/copy/history", enableCORS(beneCopyHistoryHandler))
	http.HandleFunc("/api/marx/beneficiaries/copy/history/", enableCORS(beneCopyHistoryDetailHandler))
	http.HandleFunc("/api/marx/devops/jobs", enableCORS(devopsJobsHandler))
	http.HandleFunc("/api/marx/devops/restart-jobs", enableCORS(restartFailedJobsHandler))
	http.HandleFunc("/api/marx/devops/mark-jobs-complete", enableCORS(markJobsCompleteHandler))
	http.HandleFunc("/api/marx/sql/run", enableCORS(sqlRunnerHandler))
}

func registerRoutes() {
	registerSharedRoutes()
	registerLegacyRoutes()
	registerMarxRoutes()
}

func main() {
	if err := loadRuntimeConfig(); err != nil {
		log.Fatal(err)
	}

	if err := validateDemoDatabaseSetup(); err != nil {
		log.Fatal(err)
	}

	if err := initBeneCopyHistoryStore(); err != nil {
		log.Fatal(err)
	}

	startBeneCopyWorker()
	registerRoutes()
	log.Printf("Go backend running on %s in %s mode", backendAddr, getAppMode())
	log.Fatal(http.ListenAndServe(backendAddr, nil))
}