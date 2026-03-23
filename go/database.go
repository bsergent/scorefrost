package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// expectedSchemaVersion returns the highest migration version found in *.up.sql files.
func expectedSchemaVersion(migrationsDir string) (int64, error) {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return 0, fmt.Errorf("failed to enumerate migration files: %w", err)
	}
	if len(files) == 0 {
		return 0, fmt.Errorf("no migration files found in %s", migrationsDir)
	}

	var maxVersion int64
	for _, path := range files {
		base := filepath.Base(path)
		parts := strings.SplitN(base, "_", 2)
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid migration filename format: %s", base)
		}

		version, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid migration version in filename %s: %w", base, err)
		}
		if version > maxVersion {
			maxVersion = version
		}
	}

	return maxVersion, nil
}

// verifySchemaCompatibility ensures the connected DB schema matches the bundled migration set.
func verifySchemaCompatibility(db *sql.DB, migrationsDir string) error {
	expectedVersion, err := expectedSchemaVersion(migrationsDir)
	if err != nil {
		return err
	}

	var schemaMigrationsExists bool
	err = db.QueryRow(`SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&schemaMigrationsExists)
	if err != nil {
		return fmt.Errorf("failed to check schema_migrations existence: %w", err)
	}
	if !schemaMigrationsExists {
		return fmt.Errorf("schema_migrations table not found; run migrations up to version %d before starting API", expectedVersion)
	}

	var currentVersion int64
	var dirty bool
	err = db.QueryRow(`SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&currentVersion, &dirty)
	if err == sql.ErrNoRows {
		return fmt.Errorf("schema_migrations is empty; run migrations up to version %d before starting API", expectedVersion)
	}
	if err != nil {
		return fmt.Errorf("failed to read schema_migrations state: %w", err)
	}

	if dirty {
		return fmt.Errorf("database migration state is dirty at version %d; fix migration state before starting API", currentVersion)
	}

	if currentVersion != expectedVersion {
		return fmt.Errorf("database schema version mismatch: database=%d expected=%d; run migrations before starting API", currentVersion, expectedVersion)
	}

	return nil
}

// syncDevAPIKeyHash updates the dev user's API key hash from environment variable.
func syncDevAPIKeyHash(db *sql.DB) error {
	devAPIKey := os.Getenv("DEV_API_KEY")
	if devAPIKey == "" {
		log.Println("Warning: DEV_API_KEY not set, dev user will not have valid authentication")
		return nil
	}

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
	return nil
}

// applyMigrationsUp applies all *.up.sql files in lexical order from migrationsDir.
// It maintains a golang-migrate compatible schema_migrations table.
func applyMigrationsUp(db *sql.DB, migrationsDir string) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT NOT NULL PRIMARY KEY,
			dirty BOOLEAN NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("failed to ensure schema_migrations table: %w", err)
	}

	var currentVersion int64
	var dirty bool
	err := db.QueryRow(`SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&currentVersion, &dirty)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to read schema_migrations state: %w", err)
	}
	if dirty {
		return fmt.Errorf("database migration state is dirty at version %d", currentVersion)
	}

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("failed to enumerate migration files: %w", err)
	}
	sort.Strings(files)

	for _, path := range files {
		base := filepath.Base(path)
		parts := strings.SplitN(base, "_", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid migration filename format: %s", base)
		}

		version, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid migration version in filename %s: %w", base, err)
		}

		if version <= currentVersion {
			continue
		}

		migrationSQL, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", base, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin tx for migration %s: %w", base, err)
		}

		if _, err := tx.Exec(`DELETE FROM schema_migrations`); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to update migration state before %s: %w", base, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, dirty) VALUES ($1, TRUE)`, version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to mark migration dirty for %s: %w", base, err)
		}

		if _, err := tx.Exec(string(migrationSQL)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", base, err)
		}

		if _, err := tx.Exec(`DELETE FROM schema_migrations`); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to finalize migration state for %s: %w", base, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, dirty) VALUES ($1, FALSE)`, version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to mark migration complete for %s: %w", base, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", base, err)
		}

		currentVersion = version
		log.Printf("Applied migration: %s", base)
	}

	return nil
}

// applyMigrationsDownAll applies all *.down.sql files in reverse lexical order,
// then clears migration state.
func applyMigrationsDownAll(db *sql.DB, migrationsDir string) error {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.down.sql"))
	if err != nil {
		return fmt.Errorf("failed to enumerate down migration files: %w", err)
	}
	sort.Strings(files)

	for i := len(files) - 1; i >= 0; i-- {
		path := files[i]
		base := filepath.Base(path)

		downSQL, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read down migration %s: %w", base, err)
		}

		if _, err := db.Exec(string(downSQL)); err != nil {
			return fmt.Errorf("failed to execute down migration %s: %w", base, err)
		}

		log.Printf("Applied down migration: %s", base)
	}

	if _, err := db.Exec(`DROP TABLE IF EXISTS schema_migrations CASCADE`); err != nil {
		return fmt.Errorf("failed to clear schema_migrations after down: %w", err)
	}

	return nil
}
