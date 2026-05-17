package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

type connectRequest struct {
	UserID   string `json:"userId"`
	Password string `json:"password"`
}

type databaseConnectionState struct {
	Name      string `json:"name"`
	Connected bool   `json:"connected"`
}

type environmentConnectionState struct {
	Environment string                    `json:"environment"`
	Databases   []databaseConnectionState `json:"databases"`
}

type sessionResponse struct {
	Connected              bool                         `json:"connected"`
	UserID                 string                       `json:"userId"`
	DisplayName            string                       `json:"displayName"`
	Mode                   string                       `json:"mode"`
	EnvironmentConnections []environmentConnectionState `json:"environmentConnections"`
}

type demoUser struct {
	UserID      string
	Password    string
	DisplayName string
}

var demoUsers = map[string]demoUser{
	"demo": {
		UserID:      "demo",
		Password:    "demo1234",
		DisplayName: "Demo User",
	},
	"admin": {
		UserID:      "admin",
		Password:    "admin1234",
		DisplayName: "Demo Admin",
	},
}

func getDemoEnvironmentConnections() []environmentConnectionState {
	envs := getSupportedEnvironments()
	sort.Strings(envs)

	connections := make([]environmentConnectionState, 0, len(envs))
	for _, env := range envs {
		databases := []databaseConnectionState{
			{
				Name:      databaseNameMARx,
				Connected: true,
			},
			{
				Name:      databaseNamePWA,
				Connected: true,
			},
			{
				Name:      databaseNameBatch,
				Connected: true,
			},
		}
		connections = append(connections, environmentConnectionState{
			Environment: env,
			Databases:   databases,
		})
	}

	return connections
}

func getRealEnvironmentConnections(userID, password string) ([]environmentConnectionState, bool, []string) {
	envs := getSupportedEnvironments()
	sort.Strings(envs)

	connections := make([]environmentConnectionState, 0, len(envs))
	results := make([]environmentConnectionState, len(envs))
	failures := make([]string, 0)
	hasAnyConnection := false
	var failureMu sync.Mutex
	var connectionMu sync.Mutex
	var waitGroup sync.WaitGroup

	for index, env := range envs {
		waitGroup.Add(1)
		go func(index int, env string) {
			defer waitGroup.Done()

			databases := make([]databaseConnectionState, 0, 3)
			for _, databaseName := range []string{databaseNameMARx, databaseNamePWA, databaseNameBatch} {
				definition, err := getDatabaseDefinition(env, databaseName)
				if err != nil {
					databases = append(databases, databaseConnectionState{Name: databaseName, Connected: false})
					failureMu.Lock()
					failures = append(failures, fmt.Sprintf("%s/%s: %v", env, databaseName, err))
					failureMu.Unlock()
					continue
				}

				db, err := openConnectionWithDefinition(userID, password, definition)
				connected := err == nil
				if connected {
					connectionMu.Lock()
					hasAnyConnection = true
					connectionMu.Unlock()
					db.Close()
				} else {
					failureMu.Lock()
					failures = append(failures, fmt.Sprintf("%s/%s: %v", env, databaseName, err))
					failureMu.Unlock()
				}

				databases = append(databases, databaseConnectionState{
					Name:      databaseName,
					Connected: connected,
				})
			}

			results[index] = environmentConnectionState{
				Environment: env,
				Databases:   databases,
			}
		}(index, env)
	}

	waitGroup.Wait()
	connections = append(connections, results...)

	return connections, hasAnyConnection, failures
}

func validateDemoCredentials(userID, password string) (demoUser, bool) {
	normalizedUserID := strings.ToLower(strings.TrimSpace(userID))
	user, ok := demoUsers[normalizedUserID]
	if !ok {
		return demoUser{}, false
	}

	if user.Password != password {
		return demoUser{}, false
	}

	return user, true
}

func authLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req connectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.UserID) == "" || strings.TrimSpace(req.Password) == "" {
		http.Error(w, "userId and password are required", http.StatusBadRequest)
		return
	}

	if getAppMode() == appModeReal {
		connections, hasAnyConnection, failures := getRealEnvironmentConnections(req.UserID, req.Password)
		if !hasAnyConnection {
			message := "unable to connect to any configured databases with the supplied credentials"
			if len(failures) > 0 {
				message += ":\n" + strings.Join(failures, "\n")
			}
			http.Error(w, message, http.StatusUnauthorized)
			return
		}

		setActiveRealSession(req.UserID, req.Password, req.UserID)
		writeJSON(w, sessionResponse{
			Connected:              true,
			UserID:                 req.UserID,
			DisplayName:            req.UserID,
			Mode:                   getAppMode(),
			EnvironmentConnections: connections,
		})
		return
	}

	user, ok := validateDemoCredentials(req.UserID, req.Password)
	if !ok {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	writeJSON(w, sessionResponse{
		Connected:              true,
		UserID:                 user.UserID,
		DisplayName:            user.DisplayName,
		Mode:                   getAppMode(),
		EnvironmentConnections: getDemoEnvironmentConnections(),
	})
}