package repository

import (
	"account-service/internal/domain"
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
func (r *PreferencesRepository) GetPreferences(ctx context.Context, userID int64) (*domain.UserPreferences, error) {
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
		return nil, err
	}

	// decode JSON -> map
	_ = json.Unmarshal(prefsJSON, &rec.Preferences)
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

	prefsJSON, _ := json.Marshal(prefs.Preferences)

	return r.db.QueryRow(ctx, q,
		prefs.UserID,
		prefsJSON,
	).Scan(&prefs.UpdatedAt)
}

// DeletePreferences removes preferences row for a user.
func (r *PreferencesRepository) DeletePreferences(ctx context.Context, userID int64) error {
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

	prefsJSON, _ := json.Marshal(prefs.Preferences)

	return r.db.QueryRow(ctx, q,
		prefs.UserID,
		prefsJSON,
	).Scan(&prefs.UpdatedAt)
}

// UpdateSinglePreference sets a single key:value in JSONB (atomic).
func (r *PreferencesRepository) UpdateSinglePreference(ctx context.Context, userID int64, key, value string) error {
	const q = `
		UPDATE user_preferences
		SET preferences = jsonb_set(preferences, ARRAY[$2], to_jsonb($3::text), true),
		    updated_at = NOW()
		WHERE user_id = $1
		RETURNING updated_at
	`

	var updatedAt time.Time
	return r.db.QueryRow(ctx, q, userID, key, value).Scan(&updatedAt)
}
