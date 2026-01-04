//go:build integration

package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestMain(m *testing.M) {
	// Setup integration test environment
	setupIntegration()

	// Run tests
	code := m.Run()

	// Cleanup
	cleanupIntegration()

	os.Exit(code)
}

func setupIntegration() {
	// Initialize integration test configuration
	integrationConfig = IntegrationConfig{
		DBConnString: getEnvOrDefault("TEST_DB_CONN", "host=localhost port=5432 user=scorefrost password=secret123 dbname=scorefrost sslmode=disable"),
		SolutionSalt: getEnvOrDefault("SOLUTION_SALT", "your_secret_salt_change_in_production"),
	}

	// Set environment variable for the application code to use
	os.Setenv("SOLUTION_SALT", integrationConfig.SolutionSalt)

	// Wait for database to be ready
	waitForDatabase(30 * time.Second)

	// Clean and prepare database
	prepareIntegrationDatabase()

	// Create test users
	createIntegrationTestUsers()
}

func cleanupIntegration() {
	// Clean up test data
	cleanupIntegrationData()
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func waitForDatabase(timeout time.Duration) {
	db, err := connectToIntegrationDB()
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to test database: %v", err))
	}
	defer db.Close()

	start := time.Now()
	for time.Since(start) < timeout {
		if err := db.Ping(); err == nil {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	panic(fmt.Sprintf("Database not ready after %v", timeout))
}
