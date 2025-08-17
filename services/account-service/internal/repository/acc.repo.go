package repository

import (
	"context"
	"time"

	"account-service/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserProfileRepository struct {
	db *pgxpool.Pool
}

func NewUserProfileRepository(db *pgxpool.Pool) *UserProfileRepository {
	return &UserProfileRepository{db: db}
}

// Create inserts a new user profile
func (r *UserProfileRepository) Create(ctx context.Context, profile *domain.UserProfile) error {
	const q = `
		INSERT INTO user_profiles (
			user_id, date_of_birth, profile_image_url, first_name, last_name,
			surname, sys_username, address, gender, bio, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, NOW(), NOW()
		)
		RETURNING created_at, updated_at
	`

	var dob *time.Time
	if profile.DateOfBirth != nil {
		dob = profile.DateOfBirth
	}

	return r.db.QueryRow(ctx, q,
		profile.UserID,
		dob,
		profile.ProfileImageURL,
		profile.FirstName,
		profile.LastName,
		profile.Surname,
		profile.SysUsername,
		profile.Address,
		profile.Gender,
		profile.Bio,
	).Scan(&profile.CreatedAt, &profile.UpdatedAt)
}

// GetByUserID fetches a profile by user_id
func (r *UserProfileRepository) GetByUserID(ctx context.Context, userID int64) (*domain.UserProfile, error) {
	const q = `
		SELECT user_id, date_of_birth, profile_image_url, first_name, last_name,
		       surname, sys_username, address, gender, bio, created_at, updated_at
		FROM user_profiles
		WHERE user_id = $1
	`

	profile := &domain.UserProfile{}
	var dob *time.Time

	err := r.db.QueryRow(ctx, q, userID).Scan(
		&profile.UserID,
		&dob,
		&profile.ProfileImageURL,
		&profile.FirstName,
		&profile.LastName,
		&profile.Surname,
		&profile.SysUsername,
		&profile.Address,
		&profile.Gender,
		&profile.Bio,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if dob != nil {
		profile.DateOfBirth = dob
	}

	return profile, nil
}

// Update modifies fields in a user profile
func (r *UserProfileRepository) Update(ctx context.Context, profile *domain.UserProfile) error {
	const q = `
		UPDATE user_profiles
		SET date_of_birth = $2,
		    profile_image_url = $3,
		    first_name = $4,
		    last_name = $5,
		    surname = $6,
		    sys_username = $7,
		    address = $8,
		    gender = $9,
		    bio = $10,
		    updated_at = NOW()
		WHERE user_id = $1
		RETURNING updated_at
	`

	var dob *time.Time
	if profile.DateOfBirth != nil {
		dob = profile.DateOfBirth
	}

	return r.db.QueryRow(ctx, q,
		profile.UserID,
		dob,
		profile.ProfileImageURL,
		profile.FirstName,
		profile.LastName,
		profile.Surname,
		profile.SysUsername,
		profile.Address,
		profile.Gender,
		profile.Bio,
	).Scan(&profile.UpdatedAt)
}

// Delete removes a profile by user_id
func (r *UserProfileRepository) Delete(ctx context.Context, userID int64) error {
	const q = `DELETE FROM user_profiles WHERE user_id = $1`
	_, err := r.db.Exec(ctx, q, userID)
	return err
}
