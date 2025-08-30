package repository

import (
	"context"
	"database/sql"
	"time"

	"session-service/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionRepository struct {
	db *pgxpool.Pool
}

func NewSessionRepository(db *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) CreateOrUpdateSession(ctx context.Context, session *domain.Session, tempTTL time.Duration) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if session.IsTemp {
		// Remove expired temp sessions first
		_, err = tx.Exec(ctx, `
			DELETE FROM sessions
			WHERE user_id = $1 AND is_temp = TRUE AND expires_at <= NOW()
		`, session.UserID)
		if err != nil {
			return err
		}

		// Enforce temp session limit
		var tempCount int
		err = tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM sessions
			WHERE user_id = $1 AND is_active = TRUE AND is_temp = TRUE
		`, session.UserID).Scan(&tempCount)
		if err != nil {
			return err
		}

		if tempCount >= 5 { // configurable
			_, err = tx.Exec(ctx, `
				DELETE FROM sessions
				WHERE id = (
					SELECT id FROM sessions
					WHERE user_id = $1 AND is_active = TRUE AND is_temp = TRUE
					ORDER BY last_seen_at ASC
					LIMIT 1
				)
			`, session.UserID)
			if err != nil {
				return err
			}
		}

		// Set expiry
		session.ExpiresAt = sql.NullTime{
			Time:  time.Now().Add(tempTTL),
			Valid: true,
		}

	} else {
		// Remove inactive main sessions if over limit
		var mainCount int
		err = tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM sessions
			WHERE user_id = $1 AND is_active = TRUE AND is_temp = FALSE
		`, session.UserID).Scan(&mainCount)
		if err != nil {
			return err
		}

		if mainCount >= 3 {
			_, err = tx.Exec(ctx, `
				DELETE FROM sessions
				WHERE id = (
					SELECT id FROM sessions
					WHERE user_id = $1 AND is_active = TRUE AND is_temp = FALSE
					ORDER BY last_seen_at ASC
					LIMIT 1
				)
			`, session.UserID)
			if err != nil {
				return err
			}
		}

		// No expiry for main sessions
		session.ExpiresAt = sql.NullTime{Valid: false}
	}

	// Insert or update only non-empty fields
	query := `
	INSERT INTO sessions (
		id, user_id, auth_token, device_id, ip_address, user_agent, geo_location,
		device_metadata, last_seen_at, is_active, is_single_use, is_temp, is_used,
		created_at, expires_at, purpose
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	ON CONFLICT (user_id, device_id, is_temp)
	DO UPDATE SET
		auth_token      = COALESCE(NULLIF(EXCLUDED.auth_token, ''), sessions.auth_token),
		ip_address      = COALESCE(NULLIF(EXCLUDED.ip_address, ''), sessions.ip_address),
		user_agent      = COALESCE(NULLIF(EXCLUDED.user_agent, ''), sessions.user_agent),
		geo_location    = COALESCE(NULLIF(EXCLUDED.geo_location, ''), sessions.geo_location),
		device_metadata = COALESCE(EXCLUDED.device_metadata, sessions.device_metadata),
		last_seen_at    = COALESCE(EXCLUDED.last_seen_at, sessions.last_seen_at),
		-- boolean fields: update unconditionally, unless you make them nullable
		is_active       = EXCLUDED.is_active,
		is_single_use   = EXCLUDED.is_single_use,
		is_temp         = EXCLUDED.is_temp,
		is_used         = EXCLUDED.is_used,
		expires_at      = COALESCE(EXCLUDED.expires_at, sessions.expires_at),
		purpose         = COALESCE(NULLIF(EXCLUDED.purpose, ''), sessions.purpose),
		updated_at      = NOW();
	`

	_, err = tx.Exec(ctx, query,
		session.ID,
		session.UserID,
		session.AuthToken,
		session.DeviceID,
		session.IPAddress,
		session.UserAgent,
		session.GeoLocation,
		session.DeviceMeta,
		session.LastSeenAt,
		session.IsActive,
		session.IsSingleUse,
		session.IsTemp,
		session.IsUsed,
		session.CreatedAt,
		session.ExpiresAt,
		session.Purpose,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}





func (r *SessionRepository) GetSessionByToken(ctx context.Context, token string) (*domain.Session, error) {
	var session domain.Session

	query := `
		SELECT 
			id,
			user_id,
			auth_token,
			device_id,
			ip_address,
			user_agent,
			geo_location,
			device_metadata,
			is_active,
			is_single_use,
			is_temp,
			is_used,
			purpose,
			last_seen_at,
			created_at,
			expires_at
		FROM sessions
		WHERE auth_token = $1
		LIMIT 1
	`

	err := r.db.QueryRow(ctx, query, token).Scan(
		&session.ID,
		&session.UserID,
		&session.AuthToken,
		&session.DeviceID,
		&session.IPAddress,
		&session.UserAgent,
		&session.GeoLocation,
		&session.DeviceMeta,
		&session.IsActive,
		&session.IsSingleUse,
		&session.IsTemp,
		&session.IsUsed,
		&session.Purpose,
		&session.LastSeenAt,
		&session.CreatedAt,
		&session.ExpiresAt,
	)

	if err != nil {
		return nil, err
	}

	return &session, nil
}



func (r *SessionRepository) GetUserDetailsByToken(ctx context.Context, token string) (*domain.User, error) {
	session, err := r.GetSessionByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	var user domain.User
	err = r.db.QueryRow(ctx, `SELECT id, email FROM users WHERE id=$1`, session.UserID).
		Scan(&user.ID, &user.Email)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *SessionRepository) GetSessionsByUserID(ctx context.Context, userID string, includeTemp bool) ([]*domain.Session, error) {
	query := `
		SELECT 
			id,
			user_id,
			auth_token,
			device_id,
			ip_address::text,
			geo_location,
			device_metadata,
			user_agent,
			last_seen_at,
			is_active,
			is_temp,
			created_at,
			purpose
		FROM sessions
		WHERE user_id = $1
	`

	if !includeTemp {
		query += " AND is_temp = FALSE"
	}

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*domain.Session
	for rows.Next() {
		var s domain.Session
		if err := rows.Scan(
			&s.ID,
			&s.UserID,
			&s.AuthToken,
			&s.DeviceID,
			&s.IPAddress,
			&s.GeoLocation,
			&s.DeviceMeta,
			&s.UserAgent,
			&s.LastSeenAt,
			&s.IsActive,
			&s.IsTemp,
			&s.CreatedAt,
			&s.Purpose,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, &s)
	}

	return sessions, nil
}




func (r *SessionRepository) DeleteByToken(ctx context.Context, token string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE auth_token = $1`, token)
	return err
}

func (r *SessionRepository) DeleteAllByUser(ctx context.Context, userId string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userId)
	return err
}

func (r *SessionRepository) DeleteByID(ctx context.Context, sessionID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID)
	return err
}

func (r *SessionRepository) UpdateSessionUsed(ctx context.Context, sessionID string, used bool) error {
	_, err := r.db.Exec(ctx, `
		UPDATE sessions
		SET is_used = $1
		WHERE id = $2
	`, used, sessionID)
	return err
}
