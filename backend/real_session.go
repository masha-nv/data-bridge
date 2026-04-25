package main

import "sync"

type realSession struct {
	UserID      string
	Password    string
	DisplayName string
}

var activeRealSession struct {
	mu      sync.RWMutex
	session *realSession
}

func setActiveRealSession(userID, password, displayName string) {
	activeRealSession.mu.Lock()
	defer activeRealSession.mu.Unlock()

	activeRealSession.session = &realSession{
		UserID:      userID,
		Password:    password,
		DisplayName: displayName,
	}
}

func getActiveRealSession() (realSession, bool) {
	activeRealSession.mu.RLock()
	defer activeRealSession.mu.RUnlock()

	if activeRealSession.session == nil {
		return realSession{}, false
	}

	return *activeRealSession.session, true
}