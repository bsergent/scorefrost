package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

const devUserID = "00000000-0000-0000-0000-000000000001"

// adminMiddleware ensures only the dev user can access admin endpoints
func adminMiddleware(db *sql.DB, next http.HandlerFunc) http.HandlerFunc {
	return authMiddleware(db, func(w http.ResponseWriter, r *http.Request) {
		// Get authenticated user ID from context
		userID, ok := GetUserID(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check if user is the dev user
		if userID != devUserID {
			log.Printf("Admin access denied for user %s", userID)
			http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
			return
		}

		// User is dev, proceed to handler
		next(w, r)
	})
}

// PendingDisplayName represents a pending display name change
type PendingDisplayName struct {
	UserID             string `json:"user_id"`
	FriendCode         string `json:"friend_code"`
	CurrentDisplayName string `json:"current_display_name"`
	PendingDisplayName string `json:"pending_display_name"`
	DisplayNameStatus  int    `json:"display_name_status"`
}

// GetPendingDisplayNamesResponse represents the response for GET /admin/pending-display-names
type GetPendingDisplayNamesResponse struct {
	PendingNames []PendingDisplayName `json:"pending_names"`
	Count        int                  `json:"count"`
}

// EvaluateDisplayNameRequest represents the request body for PUT /admin/display-names/{user_id}
type EvaluateDisplayNameRequest struct {
	Approve bool `json:"approve"`
}

// EvaluateDisplayNameResponse represents the response for PUT /admin/display-names/{user_id}
type EvaluateDisplayNameResponse struct {
	UserID        string `json:"user_id"`
	DisplayName   string `json:"display_name"`
	Status        int    `json:"status"`
	StatusMessage string `json:"status_message"`
}

// adminRouter routes admin requests
func adminRouter(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// GET /admin/pending-display-names - Get all pending display name changes
		if r.URL.Path == "/admin/pending-display-names" && r.Method == http.MethodGet {
			adminMiddleware(db, getPendingDisplayNamesHandler(db))(w, r)
			return
		}

		// PUT /admin/display-names/{user_id} - Approve/reject a pending display name
		if matchesPattern(r.URL.Path, "/admin/display-names/{user_id}") && r.Method == http.MethodPut {
			params := extractPathParams(r.URL.Path, "/admin/display-names/{user_id}")
			ctx := r.Context()
			for key, value := range params {
				ctx = context.WithValue(ctx, contextKey("path_"+key), value)
			}
			adminMiddleware(db, evaluateDisplayNameHandler(db))(w, r.WithContext(ctx))
			return
		}

		// No match
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// getPendingDisplayNamesHandler handles GET /admin/pending-display-names
func getPendingDisplayNamesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only accept GET requests
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Call stored procedure to get all pending display names
		rows, err := db.Query(`SELECT * FROM get_pending_display_names()`)
		if err != nil {
			log.Printf("Failed to query pending display names: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// Collect all pending names
		var pendingNames []PendingDisplayName
		for rows.Next() {
			var pending PendingDisplayName
			if err := rows.Scan(
				&pending.UserID,
				&pending.FriendCode,
				&pending.CurrentDisplayName,
				&pending.PendingDisplayName,
				&pending.DisplayNameStatus,
			); err != nil {
				log.Printf("Failed to scan pending display name: %v", err)
				continue
			}
			pendingNames = append(pendingNames, pending)
		}

		if err := rows.Err(); err != nil {
			log.Printf("Error iterating pending display names: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Prepare response
		response := GetPendingDisplayNamesResponse{
			PendingNames: pendingNames,
			Count:        len(pendingNames),
		}

		// Send JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}

// evaluateDisplayNameHandler handles PUT /admin/display-names/{user_id}
func evaluateDisplayNameHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only accept PUT requests
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract user ID from URL path
		userID := GetPathParam(r, "user_id")
		if userID == "" {
			http.Error(w, "User ID is required", http.StatusBadRequest)
			return
		}

		// Parse request body
		var req EvaluateDisplayNameRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Call appropriate stored procedure based on approval decision
		var finalDisplayName string
		var newStatus int
		var statusMessage string
		var err error

		if req.Approve {
			// Call approve_display_name stored procedure
			err = db.QueryRow(`SELECT * FROM approve_display_name($1)`, userID).
				Scan(&finalDisplayName, &newStatus)
			statusMessage = "approved"
		} else {
			// Call reject_display_name stored procedure
			err = db.QueryRow(`SELECT * FROM reject_display_name($1)`, userID).
				Scan(&finalDisplayName, &newStatus)
			statusMessage = "rejected"
		}

		// Handle errors from stored procedures
		if err != nil {
			errMsg := err.Error()
			if strings.Contains(errMsg, "User not found") {
				http.Error(w, "User not found", http.StatusNotFound)
				return
			}
			if strings.Contains(errMsg, "No pending display name") {
				http.Error(w, "No pending display name for this user", http.StatusBadRequest)
				return
			}
			log.Printf("Failed to %s display name: %v", statusMessage, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Prepare response
		response := EvaluateDisplayNameResponse{
			UserID:        userID,
			DisplayName:   finalDisplayName,
			Status:        newStatus,
			StatusMessage: statusMessage,
		}

		// Send JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}

		log.Printf("Display name %s for user %s by admin", statusMessage, userID)
	}
}
