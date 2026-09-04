package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

var devUserID = UserID(uuid.MustParse("00000000-0000-0000-0000-000000000001"))

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

// EvaluateDisplayNameRequest represents the request body for PUT /admin/names/{userID}
type EvaluateDisplayNameRequest struct {
	Approve bool `json:"approve"`
}

// GetPendingDisplayNamesResponse represents the response for GET /admin/names
type GetPendingDisplayNamesResponse struct {
	PendingNames []PendingDisplayName `json:"pending_names"`
	Count        int                  `json:"count"`
}

// getPendingDisplayNamesHandler handles GET /admin/names
func getPendingDisplayNamesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pendingNames, err := getPendingDisplayNames(db)

		if err != nil {
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

// evaluateDisplayNameHandler handles PUT /admin/names/{user_id}
func evaluateDisplayNameHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		// Extract user ID from URL path parameter (Go 1.22+)
		userIdStr, err := uuid.Parse(r.PathValue("user_id"))
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}
		userId := UserID(userIdStr)

		// Parse request body
		var req EvaluateDisplayNameRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Call appropriate stored procedure based on approval decision
		var finalDisplayName DisplayName
		var statusMessage string

		if req.Approve {
			// Call approve_display_name stored procedure
			finalDisplayName, err = approveDisplayName(db, userId)
			statusMessage = "approved"
		} else {
			// Call reject_display_name stored procedure
			finalDisplayName, err = rejectDisplayName(db, userId)
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
		response := User{
			ApiResponse: ApiResponse{
				Message: statusMessage,
			},
			ID:          &userId,
			DisplayName: finalDisplayName,
		}

		// Send JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}

		log.Printf("Display name %s for user %s by admin", statusMessage, userId)
	}
}
