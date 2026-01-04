package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
)

// initializeDatabase creates all database tables and functions
func initializeDatabase(db *sql.DB) error {
	log.Println("Initializing database schema...")

	// Read and execute tables first
	tablesSQL, err := os.ReadFile("sql/tables.sql")
	if err != nil {
		return fmt.Errorf("failed to read tables.sql: %w", err)
	}

	// Execute the tables schema
	if _, err := db.Exec(string(tablesSQL)); err != nil {
		return fmt.Errorf("failed to execute tables schema: %w", err)
	}
	log.Println("Database tables initialized successfully")

	// Read and execute functions
	functionsSQL, err := os.ReadFile("sql/functions.sql")
	if err != nil {
		return fmt.Errorf("failed to read functions.sql: %w", err)
	}

	// Execute the functions schema
	if _, err := db.Exec(string(functionsSQL)); err != nil {
		return fmt.Errorf("failed to execute functions schema: %w", err)
	}
	log.Println("Database functions initialized successfully")

	// Update dev user's API key hash from environment variable
	devAPIKey := os.Getenv("DEV_API_KEY")
	if devAPIKey == "" {
		log.Println("Warning: DEV_API_KEY not set, dev user will not have valid authentication")
	} else {
		devAPIKeyHash := hashAPIKey(devAPIKey)
		_, err := db.Exec(`
			UPDATE "user"
			SET api_key_hash = $1
			WHERE id = '00000000-0000-0000-0000-000000000001'
		`, devAPIKeyHash)
		if err != nil {
			return fmt.Errorf("failed to update dev user API key: %w", err)
		}
		log.Println("Dev user API key hash updated successfully")
	}

	log.Println("Database schema initialized successfully")
	return nil
}
