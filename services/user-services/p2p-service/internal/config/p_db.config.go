package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ConnectDB() (*pgxpool.Pool, error) {
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	log.Printf("[DB] Connecting to database: host=%s port=%s db=%s user=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_USER"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbpool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Printf("[DB] ❌ Failed to create pool: %v", err)
		return nil, err
	}

	log.Println("[DB] Pool created, testing connection...")

	if err := dbpool.Ping(ctx); err != nil {
		log.Printf("[DB] ❌ Failed to ping database: %v", err)
		return nil, err
	}

	log.Println("[DB] Connected successfully!")
	return dbpool, nil
}