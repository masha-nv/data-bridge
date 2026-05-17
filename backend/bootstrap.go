package main

import (
	"encoding/json"
	"net/http"
)

const (
	backendAddr = ":8080"
)

type backendStatus struct {
	Name                  string   `json:"name"`
	Mode                  string   `json:"mode"`
	SupportedEnvironments []string `json:"supportedEnvironments"`
	LegacyRoutes          []string `json:"legacyRoutes"`
	MarxRoutesEnabled     bool     `json:"marxRoutesEnabled"`
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func backendStatusHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, backendStatus{
		Name:                  "data-bridge-backend",
		Mode:                  getAppMode(),
		SupportedEnvironments: getSupportedEnvironments(),
		LegacyRoutes: []string{
			"/api/search",
			"/api/tables",
			"/api/move",
			"/api/clear",
			"/api/rows",
			"/api/columls",
		},
		MarxRoutesEnabled: true,
	})
}