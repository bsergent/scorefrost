package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	if os.Getenv("DOCKER_ENV") != "true" {
		err := godotenv.Load()
		if err != nil {
			log.Printf("Warning: Could not load .env file, relying on environment variables")
		}
	}

	apiPort := os.Getenv("API_PORT")
	if apiPort == "" {
		apiPort = "8080"
	}

	db, err := connectToDB()
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	_ = db // Will be used later

	http.HandleFunc("/health", health)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"service":"scorefrost","env_port":"%s"}`, apiPort)
	})

	log.Printf("Starting server on :%s", apiPort)
	log.Fatal(http.ListenAndServe(":"+apiPort, nil))
}

func connectToDB() (*sql.DB, error) {
	dbHost := os.Getenv("PGHOST")
	dbPort := os.Getenv("PGPORT")
	dbUser := os.Getenv("PGUSER")
	dbPassword := os.Getenv("PGPASSWORD")
	dbName := os.Getenv("PGDB")

	// PostgreSQL DSN
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName,
	)
	log.Printf("Connection String: %s", dsn)

	var db *sql.DB
	var err error

	// Try to connect with retries
	const maxRetries = 5
	for range maxRetries {
		db, err = sql.Open("postgres", dsn)
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	defer db.Close()

	// Return any connection errors
	if err != nil {
		return nil, err
	}

	log.Println("Connected to PostgreSQL!")
	return db, nil
}

func health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","time":"%s"}`, time.Now().UTC().Format(time.RFC3339))
}

// TODO Gracefully clean shut down the server on SIGINT/SIGTERM
