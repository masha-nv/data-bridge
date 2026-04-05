package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var envDBs = map[string]string{
	"develop": "dev.db",
	"test":    "test.db",
	"prod":    "prod.db",
}

func openDB(env string) (*sql.DB, error) {
	dbFile, ok := envDBs[env]
	if !ok {
		return nil, fmt.Errorf("unknown environment: %s", env)
	}
	return sql.Open("sqlite3", dbFile)
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func tablesHandler(w http.ResponseWriter, r *http.Request) {
	env := r.URL.Query().Get("env")
	db, err := openDB(env)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer db.Close()
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tables)
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
	rows, err:= db.Query(fmt.Sprintf("SELECT * FROM %s", table))
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"moved": count})
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"cleared": tables})
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
       // Use develop as default DB for schema
       db, err := openDB("develop")
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
       w.Header().Set("Content-Type", "application/json")
       json.NewEncoder(w).Encode(columns)
}

func main() {
	http.HandleFunc("/api/search", searchHandler)
	http.HandleFunc("/api/tables", tablesHandler)
	http.HandleFunc("/api/move", moveHandler)
	http.HandleFunc("/api/clear", clearHandler)
	http.HandleFunc("/api/rows", tableRowsHandler)
	http.HandleFunc("/api/columls", columnsHandler)
	log.Println("Go backend running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}