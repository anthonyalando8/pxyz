package repository

import (

	"github.com/jackc/pgx/v5/pgxpool"
)

type PartnerRepo struct {
	db *pgxpool.Pool
}

func NewPartnerRepo(db *pgxpool.Pool) *PartnerRepo {
	return &PartnerRepo{db: db}
}

