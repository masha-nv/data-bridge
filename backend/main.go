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
		SearchBy []string `json:"searchBy"`
		BeneId   string   `json:"beneId"`
		BeneName string   `json:"beneName"`
		Envs     []string `json:"envs"`
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
		query := "SELECT * FROM beneficiaries WHERE 1=1"
		args := []any{}
		if contains(req.SearchBy, "beneId") && req.BeneId != "" {
			query += " AND bene_id = ?"
			args = append(args, req.BeneId)
		}
		if contains(req.SearchBy, "beneName") && req.BeneName != "" {
			query += " AND bene_name LIKE ?"
			args = append(args, "%"+req.BeneName+"%")
		}
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

func contains(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

func main() {
	http.HandleFunc("/api/search", searchHandler)
	http.HandleFunc("/api/tables", tablesHandler)
	http.HandleFunc("/api/move", moveHandler)
	log.Println("Go backend running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}