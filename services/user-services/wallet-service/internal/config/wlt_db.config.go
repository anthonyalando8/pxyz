package config

import (
	"context"
	"fmt"

	"time"

	"github.com/joho/godotenv"
	"github.com/jackc/pgx/v5/pgxpool"
)

func LoadEnv() error {
	return godotenv.Load()
}

func ConnectDB(cfg *Config) *pgxpool.Pool {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbpool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		panic("Failed to initialize DB: " + err.Error())
	}
	if err := dbpool.Ping(ctx); err != nil {
		panic("DB unreachable: " + err.Error())
	}
	return dbpool
}