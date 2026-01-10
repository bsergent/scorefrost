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
	mux.HandleFunc("POST /api/v1/user", loginUserHandler(db))
	mux.HandleFunc("GET /api/v1/user/{user_id}", getUserHandler(db))
	mux.HandleFunc("PUT /api/v1/user/name", authMiddleware(db, updateDisplayNameHandler(db)))

	// Score routes
	mux.HandleFunc("POST /api/v1/score/submit", authMiddleware(db, submitScoreHandler(db)))
	mux.HandleFunc("GET /api/v1/score/best", authMiddleware(db, bestScoresHandler(db)))
	mux.HandleFunc("GET /api/v1/score/leaderboard", authMiddleware(db, leaderboardHandler(db)))

	// Admin routes
	mux.HandleFunc("GET /admin/v1/names", adminMiddleware(db, getPendingDisplayNamesHandler(db)))
	mux.HandleFunc("PUT /admin/v1/names/{user_id}", adminMiddleware(db, evaluateDisplayNameHandler(db)))

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
