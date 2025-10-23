package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
)

// initializeDatabase creates all database tables
func initializeDatabase(db *sql.DB) error {
	log.Println("Initializing database schema...")

	// Read the schema file
	schemaSQL, err := os.ReadFile("sql/schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema.sql: %w", err)
	}

	// Execute the schema
	if _, err := db.Exec(string(schemaSQL)); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

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
