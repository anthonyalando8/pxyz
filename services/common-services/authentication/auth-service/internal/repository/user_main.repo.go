package repository

import (
	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func NewSignupError(stage, next string) *xerrors.SignupError {
	return &xerrors.SignupError{Stage: stage, NextStage: next}
}
