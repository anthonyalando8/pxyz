package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectDB creates an optimized PostgreSQL connection pool for 4000+ TPS
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

	// Parse connection pool settings from environment (OPTIMIZED FOR 4000+ TPS)
	maxConns := getEnvAsInt("DB_MAX_CONNS", 100)        // Increased from 150 to 100 (optimal)
	minConns := getEnvAsInt("DB_MIN_CONNS", 20)         // Keep warm connections
	maxConnLifetime := getEnvAsDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute)  // Reduced from 1h
	maxConnIdleTime := getEnvAsDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute)  // Reduced from 30m

	// Configure connection pool
	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Printf("[DB] ❌ Failed to parse config: %v", err)
		return nil, err
	}

	// CRITICAL: Connection pool tuning for high throughput
	poolConfig.MaxConns = int32(maxConns)
	poolConfig.MinConns = int32(minConns)
	poolConfig.MaxConnLifetime = maxConnLifetime
	poolConfig.MaxConnIdleTime = maxConnIdleTime
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	// OPTIMIZATION: Enable statement caching for better performance
	poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheStatement
	poolConfig.ConnConfig.StatementCacheCapacity = 1000

	// OPTIMIZATION: Connection timeouts
	poolConfig.ConnConfig.ConnectTimeout = 10 * time.Second

	log.Printf("[DB] Pool config: max_conns=%d min_conns=%d max_lifetime=%s max_idle_time=%s statement_cache=enabled",
		maxConns, minConns, maxConnLifetime, maxConnIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbpool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Printf("[DB] ❌ Failed to create pool: %v", err)
		return nil, err
	}

	log.Println("[DB] Pool created, testing connection...")

	if err := dbpool.Ping(ctx); err != nil {
		log.Printf("[DB] ❌ Failed to ping database: %v", err)
		return nil, err
	}

	log.Printf("[DB] ✅ Connected successfully! Stats: idle=%d active=%d total=%d max=%d",
		dbpool.Stat().IdleConns(),
		dbpool.Stat().AcquiredConns(),
		dbpool.Stat().TotalConns(),
		dbpool.Stat().MaxConns(),
	)

	return dbpool, nil
}

func getEnvAsInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}