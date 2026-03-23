package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
)

// setupRoutes creates and configures the HTTP router with all application routes
// This function is shared between the main application and tests
func setupRoutes(db *sql.DB) *http.ServeMux {
	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", health)

	// User routes
	mux.HandleFunc("POST "+APIBasePath+"/user", loginUserHandler(db))
	mux.HandleFunc("GET "+APIBasePath+"/user/{user_id}", getUserHandler(db))
	mux.HandleFunc("PUT "+APIBasePath+"/user/name", authMiddleware(db, updateDisplayNameHandler(db)))

	// Score routes
	mux.HandleFunc("PUT "+APIBasePath+"/score", authMiddleware(db, submitScoreHandler(db)))
	mux.HandleFunc("GET "+APIBasePath+"/score/best", authMiddleware(db, bestScoresHandler(db)))
	mux.HandleFunc("GET "+APIBasePath+"/score/leaderboard", authMiddleware(db, leaderboardHandler(db)))

	// Admin routes
	mux.HandleFunc("GET "+AdminAPIBasePath+"/names", adminMiddleware(db, getPendingDisplayNamesHandler(db)))
	mux.HandleFunc("PUT "+AdminAPIBasePath+"/names/{user_id}", adminMiddleware(db, evaluateDisplayNameHandler(db)))

	// Documentation routes
	setupDocsRoutes(mux)

	// Root endpoint - provides service info
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		apiPort := os.Getenv("API_PORT")
		if apiPort == "" {
			apiPort = "8080"
		}
		fmt.Fprintf(w, `{"service":"scorefrost","env_port":"%s"}`, apiPort)
	})

	return mux
}
