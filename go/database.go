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

	log.Println("Database schema initialized successfully")
	return nil
}
