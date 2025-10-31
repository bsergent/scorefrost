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
	mux.HandleFunc("POST /user", createUserHandler(db))
	mux.HandleFunc("GET /user/{id}", getUserHandler(db))
	mux.HandleFunc("PUT /user/{id}/name", authMiddleware(db, updateDisplayNameHandler(db)))

	// Score routes
	mux.HandleFunc("POST /score/submit", authMiddleware(db, submitScoreHandler(db)))
	mux.HandleFunc("GET /score/best", authMiddleware(db, bestScoresHandler(db)))

	// Admin routes
	mux.HandleFunc("GET /admin/names", adminMiddleware(db, getPendingDisplayNamesHandler(db)))
	mux.HandleFunc("PUT /admin/names/{user_id}", adminMiddleware(db, evaluateDisplayNameHandler(db)))

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