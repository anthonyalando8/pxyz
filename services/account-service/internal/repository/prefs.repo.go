package repository

import (
	"account-service/internal/domain"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/structpb"
)

type PreferencesRepository struct {
	db *pgxpool.Pool
}

func NewPreferencesRepository(db *pgxpool.Pool) *PreferencesRepository {
	return &PreferencesRepository{db: db}
}

/* ======================
   CRUD METHODS
====================== */

// CreatePreferences inserts a new record with defaults.
func (r *PreferencesRepository) CreatePreferences(ctx context.Context, prefs *domain.UserPreferences) error {
	const q = `
		INSERT INTO user_preferences (user_id, preferences, updated_at)
		VALUES ($1, $2, NOW())
		RETURNING updated_at
	`

	prefsJSON, _ := json.Marshal(prefs.Preferences)

	return r.db.QueryRow(ctx, q,
		prefs.UserID,
		prefsJSON,
	).Scan(&prefs.UpdatedAt)
}

// GetPreferences fetches preferences for a user.
func (r *PreferencesRepository) GetPreferences(ctx context.Context, userID string) (*domain.UserPreferences, error) {
	const q = `
		SELECT user_id, preferences, updated_at
		FROM user_preferences
		WHERE user_id = $1
	`

	var rec domain.UserPreferences
	var prefsJSON []byte

	err := r.db.QueryRow(ctx, q, userID).Scan(
		&rec.UserID,
		&prefsJSON,
		&rec.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows { // or sql.ErrNoRows if using database/sql
			return nil, xerrors.ErrNotFound
		}
		return nil, err
	}

	// decode JSON -> map
	if err := json.Unmarshal(prefsJSON, &rec.Preferences); err != nil {
		return nil, fmt.Errorf("failed to unmarshal preferences: %w", err)
	}

	return &rec, nil
}


// UpdatePreferences overwrites entire JSON.
func (r *PreferencesRepository) UpdatePreferences(ctx context.Context, prefs *domain.UserPreferences) error {
	const q = `
		UPDATE user_preferences
		SET preferences = $2, updated_at = NOW()
		WHERE user_id = $1
		RETURNING updated_at
	`

	prefsJSON, err := json.Marshal(prefs.Preferences)
	if err != nil {
		return fmt.Errorf("failed to marshal preferences: %w", err)
	}

	err = r.db.QueryRow(ctx, q, prefs.UserID, prefsJSON).Scan(&prefs.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows { // or sql.ErrNoRows
			return xerrors.ErrNotFound
		}
		return err
	}

	return nil
}


// DeletePreferences removes preferences row for a user.
func (r *PreferencesRepository) DeletePreferences(ctx context.Context, userID string) error {
	const q = `DELETE FROM user_preferences WHERE user_id = $1`
	_, err := r.db.Exec(ctx, q, userID)
	return err
}

/* ======================
   HELPERS
====================== */

// UpsertPreferences inserts or updates depending on existence.
func (r *PreferencesRepository) UpsertPreferences(ctx context.Context, prefs *domain.UserPreferences) error {
	const q = `
		INSERT INTO user_preferences (user_id, preferences, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id)
		DO UPDATE SET preferences = EXCLUDED.preferences, updated_at = NOW()
		RETURNING updated_at
	`

	prefsJSON, err := json.Marshal(prefs.Preferences)
	if err != nil {
		return fmt.Errorf("failed to marshal preferences: %w", err)
	}

	return r.db.QueryRow(ctx, q, prefs.UserID, prefsJSON).Scan(&prefs.UpdatedAt)
}

func (r *PreferencesRepository) UpdateMultiplePreferences(ctx context.Context, userID string, prefs map[string]*structpb.Value) error {
	if len(prefs) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for key, val := range prefs {
		const q = `
			UPDATE user_preferences
			SET preferences = jsonb_set(
				COALESCE(preferences, '{}'::jsonb),
				ARRAY[$2],
				to_jsonb($3::jsonb),
				true
			),
			updated_at = NOW()
			WHERE user_id = $1
		`
		if _, err := tx.Exec(ctx, q, userID, key, val.AsInterface()); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *PreferencesRepository) UpdateSinglePreference(ctx context.Context, userID string, key string, value *structpb.Value) error {
	const q = `
		UPDATE user_preferences
		SET preferences = jsonb_set(
			COALESCE(preferences, '{}'::jsonb),
			ARRAY[$2],
			to_jsonb($3::jsonb),
			true
		),
		updated_at = NOW()
		WHERE user_id = $1
		RETURNING updated_at
	`

	var updatedAt time.Time
	err := r.db.QueryRow(ctx, q, userID, key, value.AsInterface()).Scan(&updatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return xerrors.ErrNotFound
		}
		return err
	}
	return nil
}
