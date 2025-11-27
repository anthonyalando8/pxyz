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

	var dbpool *pgxpool.Pool
	var err error

	maxRetries := 5
	delay := 2 * time.Second

	for i := 1; i <= maxRetries; i++ {
		log.Printf("[DB] Attempt %d/%d: connecting to database...", i, maxRetries)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		config, parseErr := pgxpool.ParseConfig(dbURL)
		if parseErr != nil {
			log.Printf("[DB] ❌ Failed to parse config: %v", parseErr)
			return nil, parseErr
		}

		// tuning pool settings
		config.MaxConns = 50
		config.MinConns = 10
		config.MaxConnLifetime = time.Hour
		config.MaxConnIdleTime = 5 * time.Minute

		dbpool, err = pgxpool.NewWithConfig(ctx, config)
		if err == nil {
			// test connection
			if pingErr := dbpool.Ping(ctx); pingErr == nil {
				log.Println("[DB] ✅ Connected successfully!")
				return dbpool, nil
			}
			err = fmt.Errorf("ping failed: %w", err)
		}

		log.Printf("[DB] ❌ Connection failed: %v", err)

		if i < maxRetries {
			log.Printf("[DB] Retrying in %s...", delay)
			time.Sleep(delay)
			delay *= 2 // exponential backoff
		}
	}

	return nil, fmt.Errorf("failed to connect to DB after %d attempts: %w", maxRetries, err)
}
