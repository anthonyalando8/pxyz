// internal/config/audit_db.config.go
package config

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
}

func LoadDBConfig() DBConfig {
	return DBConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "sam"),
		Password: getEnv("DB_PASSWORD", ""),
		Database: getEnv("DB_NAME", "pxyz_user"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}
}

func ConnectDB() (*pgxpool.Pool, error) {
	dbConfig := LoadDBConfig()

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		dbConfig.User,
		dbConfig.Password,
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.Database,
		dbConfig.SSLMode,
	)

	log.Printf("🔌 Connecting to database: host=%s port=%s db=%s user=%s",
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.Database,
		dbConfig.User,
	)

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Optimized connection pool settings
	config.MaxConns = int32(getEnvInt("DB_MAX_CONNS", 50))
	config.MinConns = int32(getEnvInt("DB_MIN_CONNS", 10))
	config.MaxConnLifetime = getEnvDuration("DB_MAX_CONN_LIFETIME", 1*time.Hour)
	config.MaxConnIdleTime = getEnvDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute)
	config.HealthCheckPeriod = getEnvDuration("DB_HEALTH_CHECK_PERIOD", 1*time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("✅ Database connected successfully!")

	// Log pool stats
	stats := pool.Stat()
	log.Printf("📊 Pool stats - Total: %d, Idle: %d, Acquired: %d",
		stats.TotalConns(),
		stats.IdleConns(),
		stats.AcquiredConns(),
	)

	return pool, nil
}

// HealthCheck performs a database health check
func HealthCheck(pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	// Check pool stats
	stats := pool.Stat()
	if stats.TotalConns() == 0 {
		return fmt.Errorf("no database connections available")
	}

	return nil
}
