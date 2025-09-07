package repository

import (
	"context"
	"encoding/json"
	//"errors"
	"fmt"
	"strings"
	"time"

	"account-service/internal/domain"
	"x/shared/utils/errors"

	//"github.com/jackc/pgx/v5"
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

func (r *UserProfileRepository) UpdateNationality(ctx context.Context, userID string, nationality *string) error {
	const q = `
		UPDATE user_profiles
		SET nationality = $2,
		    updated_at = NOW()
		WHERE user_id = $1
	`

	tag, err := r.db.Exec(ctx, q, userID, nationality)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

// GetByUserID fetches a profile by user_id
func (r *UserProfileRepository) GetByUserID(ctx context.Context, userID string) (*domain.UserProfile, error) {
	const q = `
		SELECT user_id, date_of_birth, profile_image_url, first_name, last_name,
		       surname, sys_username, address, gender, bio,
		       nationality, created_at, updated_at
		FROM user_profiles
		WHERE user_id = $1
	`

	profile := &domain.UserProfile{}
	var dob *time.Time
	var addrJSON []byte

	err := r.db.QueryRow(ctx, q, userID).Scan(
		&profile.UserID,
		&dob,
		&profile.ProfileImageURL,
		&profile.FirstName,
		&profile.LastName,
		&profile.Surname,
		&profile.SysUsername,
		&addrJSON,
		&profile.Gender,
		&profile.Bio,
		&profile.Nationality,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
	if err != nil {
		// if errors.Is(err, pgx.ErrNoRows) {
		// 	return nil, xerrors.ErrNotFound
		// }
		return nil, err
	}

	if dob != nil {
		profile.DateOfBirth = dob
	}

	// decode JSONB address into map
	if len(addrJSON) > 0 {
		if err := json.Unmarshal(addrJSON, &profile.Address); err != nil {
			return nil, fmt.Errorf("failed to decode address JSON: %w", err)
		}
	}

	return profile, nil
}



// Update modifies fields in a user profile
func (r *UserProfileRepository) Update(ctx context.Context, profile *domain.UserProfile) error {
	setClauses := []string{}
	args := []interface{}{}
	argPos := 1

	// user_id is always required
	args = append(args, profile.UserID)

	if profile.DateOfBirth != nil {
		setClauses = append(setClauses, fmt.Sprintf("date_of_birth = $%d", argPos+len(args)))
		args = append(args, profile.DateOfBirth)
	}
	if profile.ProfileImageURL != "" {
		setClauses = append(setClauses, fmt.Sprintf("profile_image_url = $%d", argPos+len(args)))
		args = append(args, profile.ProfileImageURL)
	}
	if profile.FirstName != "" {
		setClauses = append(setClauses, fmt.Sprintf("first_name = $%d", argPos+len(args)))
		args = append(args, profile.FirstName)
	}
	if profile.LastName != "" {
		setClauses = append(setClauses, fmt.Sprintf("last_name = $%d", argPos+len(args)))
		args = append(args, profile.LastName)
	}
	if profile.Surname != "" {
		setClauses = append(setClauses, fmt.Sprintf("surname = $%d", argPos+len(args)))
		args = append(args, profile.Surname)
	}
	if profile.SysUsername != "" {
		setClauses = append(setClauses, fmt.Sprintf("sys_username = $%d", argPos+len(args)))
		args = append(args, profile.SysUsername)
	}
	if profile.Address != nil {
		setClauses = append(setClauses, fmt.Sprintf("address = $%d", argPos+len(args)))
		args = append(args, profile.Address)
	}
	if profile.Gender != "" {
		setClauses = append(setClauses, fmt.Sprintf("gender = $%d", argPos+len(args)))
		args = append(args, profile.Gender)
	}
	if profile.Bio != "" {
		setClauses = append(setClauses, fmt.Sprintf("bio = $%d", argPos+len(args)))
		args = append(args, profile.Bio)
	}

	// always update timestamp
	setClauses = append(setClauses, "updated_at = NOW()")

	if len(setClauses) == 1 { // means only updated_at was added
		return r.db.QueryRow(ctx, "UPDATE user_profiles SET updated_at = NOW() WHERE user_id = $1 RETURNING updated_at", profile.UserID).Scan(&profile.UpdatedAt)
	}

	q := fmt.Sprintf(`
		UPDATE user_profiles
		SET %s
		WHERE user_id = $1
		RETURNING updated_at
	`, strings.Join(setClauses, ", "))

	return r.db.QueryRow(ctx, q, args...).Scan(&profile.UpdatedAt)
}

// Delete removes a profile by user_id
func (r *UserProfileRepository) Delete(ctx context.Context, userID string) error {
	const q = `DELETE FROM user_profiles WHERE user_id = $1`
	_, err := r.db.Exec(ctx, q, userID)
	return err
}

// UpdateProfilePicture updates the user's profile picture URL.
func (r *UserProfileRepository) UpdateProfilePicture(ctx context.Context, userID, imageURL string) error {
	const q = `
		UPDATE user_profiles
		SET profile_image_url = $1, updated_at = NOW()
		WHERE user_id = $2
	`

	res, err := r.db.Exec(ctx, q, imageURL, userID)
	if err != nil {
		return err
	}

	if rows := res.RowsAffected(); rows == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}


// DeleteProfilePicture removes the profile picture URL and returns the old one.
func (r *UserProfileRepository) DeleteProfilePicture(ctx context.Context, userID string) (string, error) {
	const q = `
		UPDATE user_profiles
		SET profile_image_url = NULL, updated_at = NOW()
		WHERE user_id = $1
		RETURNING COALESCE(profile_image_url, '')
	`

	var oldURL string
	err := r.db.QueryRow(ctx, q, userID).Scan(&oldURL)
	if err != nil {
		return "", err
	}

	return oldURL, nil
}