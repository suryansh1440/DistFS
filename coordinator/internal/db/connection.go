package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

// InitDB connects to the PostgreSQL metadata database.
func InitDB() error {
	host := getEnv("POSTGRES_HOST", "localhost")
	port := getEnv("POSTGRES_PORT", "5432")
	user := getEnv("POSTGRES_USER", "distfs")
	password := getEnv("POSTGRES_PASSWORD", "distfs_secret")
	dbname := getEnv("POSTGRES_DB", "distfs")

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)

	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)

	log.Println("[OK] Connected to PostgreSQL")
	return nil
}

// RunMigrations applies SQL migration files to initialize tables.
func RunMigrations() error {
	migrations := []string{
		"migrations/001_create_nodes.sql",
		"migrations/002_create_files.sql",
		"migrations/003_create_chunks.sql",
		"migrations/004_create_shards.sql",
	}

	for _, file := range migrations {
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", file, err)
		}

		_, err = DB.Exec(string(content))
		if err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", file, err)
		}

		log.Printf("[OK] Migration applied: %s", file)
	}

	return nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
