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

	// parse default config
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Printf("[DB] ❌ Failed to parse config: %v", err)
		return nil, err
	}

	// connection pool settings
	config.MaxConns = 50                // max simultaneous connections
	config.MinConns = 10                // keep some idle connections
	config.MaxConnLifetime = time.Hour  // recycle connections after 1h
	config.MaxConnIdleTime = 5 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbpool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Printf("[DB] ❌ Failed to create pool: %v", err)
		return nil, err
	}

	log.Println("[DB] Pool created, testing connection...")

	if err := dbpool.Ping(ctx); err != nil {
		log.Printf("[DB] ❌ Failed to ping database: %v", err)
		return nil, err
	}

	log.Println("[DB] ✅ Connected successfully!")
	return dbpool, nil
}


