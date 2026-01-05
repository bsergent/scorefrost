package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAdminMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		hasAuth        bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "dev user access allowed",
			userID:         devUserID,
			hasAuth:        true,
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name:           "non-dev user access forbidden",
			userID:         "some-other-user-id",
			hasAuth:        true,
			expectedStatus: http.StatusForbidden,
			expectedBody:   "Forbidden: Admin access required",
		},
		{
			name:           "no authorization header",
			userID:         "",
			hasAuth:        false,
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Unauthorized: Missing or invalid Authorization header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("Failed to create mock database: %v", err)
			}
			defer db.Close()

			// For authenticated cases, mock the auth middleware database call
			if tt.hasAuth {
				mock.ExpectQuery(`SELECT id, COALESCE\(display_name, ''\), friend_code FROM "user" WHERE api_key_hash`).
					WillReturnRows(sqlmock.NewRows([]string{"id", "display_name", "friend_code"}).
						AddRow(tt.userID, "TestUser", "ABC123"))
			}

			// Create test handler that returns success if reached
			testHandler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			}

			// Wrap with admin middleware
			handler := adminMiddleware(db, testHandler)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
			if tt.hasAuth {
				// Add Bearer token header for auth middleware
				req.Header.Set("Authorization", "Bearer test-api-key")
			}

			// Record response
			rr := httptest.NewRecorder()
			handler(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Check response body
			body := strings.TrimSpace(rr.Body.String())
			if !strings.Contains(body, tt.expectedBody) {
				t.Errorf("Expected body to contain %q, got %q", tt.expectedBody, body)
			}

			// Verify all expectations were met
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("Unfulfilled mock expectations: %v", err)
			}
		})
	}
}

func TestGetPendingDisplayNamesHandler_WithRouting(t *testing.T) {
	tests := []struct {
		name           string
		mockRows       *sqlmock.Rows
		mockError      error
		expectedStatus int
		expectedCount  int
		shouldHaveData bool
		hasAuth        bool
	}{
		{
			name: "successful retrieval with data",
			mockRows: sqlmock.NewRows([]string{
				"user_id", "friend_code", "current_display_name", "pending_display_name", "display_name_status",
			}).
				AddRow("user1", "ABC123", "OldName1", "NewName1", 1).
				AddRow("user2", "DEF456", "OldName2", "NewName2", 1),
			expectedStatus: http.StatusOK,
			expectedCount:  2,
			shouldHaveData: true,
			hasAuth:        true,
		},
		{
			name:           "successful retrieval with no data",
			mockRows:       sqlmock.NewRows([]string{"user_id", "friend_code", "current_display_name", "pending_display_name", "display_name_status"}),
			expectedStatus: http.StatusOK,
			expectedCount:  0,
			shouldHaveData: true,
			hasAuth:        true,
		},
		{
			name:           "database error",
			mockError:      sql.ErrConnDone,
			expectedStatus: http.StatusInternalServerError,
			shouldHaveData: false,
			hasAuth:        true,
		},
		{
			name:           "unauthorized access",
			expectedStatus: http.StatusUnauthorized,
			shouldHaveData: false,
			hasAuth:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("Failed to create mock database: %v", err)
			}
			defer db.Close()

			// Mock auth middleware if needed
			if tt.hasAuth {
				mock.ExpectQuery(`SELECT id, COALESCE\(display_name, ''\), friend_code FROM "user" WHERE api_key_hash`).
					WillReturnRows(sqlmock.NewRows([]string{"id", "display_name", "friend_code"}).
						AddRow(devUserID, "AdminUser", "ADMIN"))
			}

			// Set up expectation for the admin endpoint
			if tt.hasAuth {
				query := mock.ExpectQuery("SELECT \\* FROM get_pending_display_names\\(\\)")
				if tt.mockError != nil {
					query.WillReturnError(tt.mockError)
				} else {
					query.WillReturnRows(tt.mockRows)
				}
			}

			// Set up the actual router
			mux := setupRoutes(db)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/admin/v1/names", nil)

			// Add auth header if needed
			if tt.hasAuth {
				req.Header.Set("Authorization", "Bearer test-api-key")
			}

			rr := httptest.NewRecorder()

			// Execute request through the actual router
			mux.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Check response body for successful cases
			if tt.shouldHaveData {
				var response GetPendingDisplayNamesResponse
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.Count != tt.expectedCount {
					t.Errorf("Expected count %d, got %d", tt.expectedCount, response.Count)
				}

				if len(response.PendingNames) != tt.expectedCount {
					t.Errorf("Expected %d pending names, got %d", tt.expectedCount, len(response.PendingNames))
				}

				// Verify content-type header
				expectedContentType := "application/json"
				if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
					t.Errorf("Expected Content-Type %s, got %s", expectedContentType, contentType)
				}
			}

			// Verify all expectations were met
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("Unfulfilled mock expectations: %v", err)
			}
		})
	}
}

func TestEvaluateDisplayNameHandler_WithRouting(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		requestBody    interface{}
		approve        bool
		mockResult     []interface{}
		mockError      error
		expectedStatus int
		expectedMsg    string
		hasAuth        bool
	}{
		{
			name:           "approve display name success",
			userID:         "test-user-id",
			requestBody:    EvaluateDisplayNameRequest{Approve: true},
			approve:        true,
			mockResult:     []interface{}{"ApprovedName", 2},
			expectedStatus: http.StatusOK,
			expectedMsg:    "approved",
			hasAuth:        true,
		},
		{
			name:           "reject display name success",
			userID:         "test-user-id",
			requestBody:    EvaluateDisplayNameRequest{Approve: false},
			approve:        false,
			mockResult:     []interface{}{"OriginalName", 0},
			expectedStatus: http.StatusOK,
			expectedMsg:    "rejected",
			hasAuth:        true,
		},
		{
			name:           "invalid JSON body",
			userID:         "test-user-id",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Invalid request body",
			hasAuth:        true,
		},
		{
			name:           "URL route mismatch",
			userID:         "", // This creates URL "/admin/v1/names/" which hits the fallback route
			requestBody:    EvaluateDisplayNameRequest{Approve: true},
			expectedStatus: http.StatusOK, // The fallback route returns 200 with service info
			expectedMsg:    "scorefrost",  // Service name from fallback route
			hasAuth:        false,         // Don't set up auth expectations since it won't reach admin middleware
		},
		{
			name:           "unauthorized access",
			userID:         "test-user-id",
			requestBody:    EvaluateDisplayNameRequest{Approve: true},
			expectedStatus: http.StatusUnauthorized,
			expectedMsg:    "Unauthorized",
			hasAuth:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("Failed to create mock database: %v", err)
			}
			defer db.Close()

			// Mock auth middleware if needed
			if tt.hasAuth {
				mock.ExpectQuery(`SELECT id, COALESCE\(display_name, ''\), friend_code FROM "user" WHERE api_key_hash`).
					WillReturnRows(sqlmock.NewRows([]string{"id", "display_name", "friend_code"}).
						AddRow(devUserID, "AdminUser", "ADMIN"))
			}

			// Set up database expectation for the actual admin operation
			if tt.hasAuth && tt.userID != "" && tt.requestBody != "invalid json" {
				var query string
				if tt.approve {
					query = "SELECT \\* FROM approve_display_name\\(\\$1\\)"
				} else {
					query = "SELECT \\* FROM reject_display_name\\(\\$1\\)"
				}

				expectation := mock.ExpectQuery(query).WithArgs(tt.userID)
				if tt.mockError != nil {
					expectation.WillReturnError(tt.mockError)
				} else if tt.mockResult != nil {
					expectation.WillReturnRows(
						sqlmock.NewRows([]string{"display_name", "status"}).
							AddRow(tt.mockResult[0], tt.mockResult[1]),
					)
				}
			}

			// Set up the actual router to get proper PathValue support
			mux := setupRoutes(db)

			// Prepare request body
			var body []byte
			if tt.requestBody == "invalid json" {
				body = []byte("invalid json")
			} else if tt.requestBody != nil {
				body, _ = json.Marshal(tt.requestBody)
			}

			// Create request with proper URL for routing
			url := "/admin/v1/names/" + tt.userID
			req := httptest.NewRequest(http.MethodPut, url, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Add auth header if needed
			if tt.hasAuth {
				req.Header.Set("Authorization", "Bearer test-api-key")
			}

			rr := httptest.NewRecorder()

			// Execute request through the actual router (this enables PathValue)
			mux.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Check response content
			body_str := strings.TrimSpace(rr.Body.String())
			if tt.expectedStatus == http.StatusOK && tt.userID != "" {
				// For successful admin responses, check JSON structure
				var response User
				if err := json.NewDecoder(strings.NewReader(body_str)).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.ID != tt.userID {
					t.Errorf("Expected user ID %s, got %s", tt.userID, response.ID)
				}

				if response.Message != tt.expectedMsg {
					t.Errorf("Expected message %s, got %s", tt.expectedMsg, response.Message)
				}

				// Verify content-type header
				expectedContentType := "application/json"
				if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
					t.Errorf("Expected Content-Type %s, got %s", expectedContentType, contentType)
				}
			} else {
				// For error responses or non-admin responses, check for expected message
				if !strings.Contains(body_str, tt.expectedMsg) {
					t.Errorf("Expected body to contain %q, got %q", tt.expectedMsg, body_str)
				}
			}

			// Verify all expectations were met
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("Unfulfilled mock expectations: %v", err)
			}
		})
	}
}

// Test the specific struct types
func TestAdminTypes(t *testing.T) {
	t.Run("PendingDisplayName struct", func(t *testing.T) {
		pending := PendingDisplayName{
			UserID:             "test-user",
			FriendCode:         "ABC123",
			CurrentDisplayName: "OldName",
			PendingDisplayName: "NewName",
			DisplayNameStatus:  1,
		}

		// Test JSON marshaling
		data, err := json.Marshal(pending)
		if err != nil {
			t.Fatalf("Failed to marshal PendingDisplayName: %v", err)
		}

		// Test JSON unmarshaling
		var unmarshaled PendingDisplayName
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("Failed to unmarshal PendingDisplayName: %v", err)
		}

		if unmarshaled.UserID != pending.UserID {
			t.Errorf("Expected UserID %s, got %s", pending.UserID, unmarshaled.UserID)
		}
	})

	t.Run("EvaluateDisplayNameRequest struct", func(t *testing.T) {
		req := EvaluateDisplayNameRequest{Approve: true}

		// Test JSON marshaling
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("Failed to marshal EvaluateDisplayNameRequest: %v", err)
		}

		// Test JSON unmarshaling
		var unmarshaled EvaluateDisplayNameRequest
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("Failed to unmarshal EvaluateDisplayNameRequest: %v", err)
		}

		if unmarshaled.Approve != req.Approve {
			t.Errorf("Expected Approve %v, got %v", req.Approve, unmarshaled.Approve)
		}
	})

	t.Run("User struct in admin response", func(t *testing.T) {
		resp := User{
			ApiResponse: ApiResponse{
				Message: "approved",
			},
			ID:          "test-user",
			DisplayName: "TestName",
			FriendCode:  "ABCD-1234",
		}

		// Test JSON marshaling
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Failed to marshal User: %v", err)
		}

		// Test JSON unmarshaling
		var unmarshaled User
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("Failed to unmarshal User: %v", err)
		}

		if unmarshaled.Message != resp.Message {
			t.Errorf("Expected Message %s, got %s", resp.Message, unmarshaled.Message)
		}
		if unmarshaled.ID != resp.ID {
			t.Errorf("Expected ID %s, got %s", resp.ID, unmarshaled.ID)
		}
	})
}

// Test dev user constant
func TestDevUserConstant(t *testing.T) {
	expected := "00000000-0000-0000-0000-000000000001"
	if devUserID != expected {
		t.Errorf("Expected devUserID %s, got %s", expected, devUserID)
	}
}
