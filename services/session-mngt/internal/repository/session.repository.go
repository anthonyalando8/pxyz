package repository

import (
	"context"

	"session-service/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"

)

type SessionRepository struct {
	db *pgxpool.Pool
}

func NewSessionRepository(db *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) CreateOrUpdateSession(ctx context.Context, session *domain.Session) error {
	query := `
	INSERT INTO sessions (
		id, user_id, auth_token, device_id, ip_address, user_agent, geo_location,
		device_metadata, last_seen_at, is_active, created_at
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	ON CONFLICT (user_id, device_id)
	DO UPDATE SET
		auth_token = EXCLUDED.auth_token,
		ip_address = EXCLUDED.ip_address,
		user_agent = EXCLUDED.user_agent,
		geo_location = EXCLUDED.geo_location,
		device_metadata = EXCLUDED.device_metadata,
		last_seen_at = EXCLUDED.last_seen_at,
		is_active = EXCLUDED.is_active;
	`

	_, err := r.db.Exec(ctx, query,
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
		session.CreatedAt,
	)
	return err
}



func (r *SessionRepository) GetSessionByToken(ctx context.Context, token string) (*domain.Session, error) {
	var session domain.Session

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
			created_at
		FROM sessions
		WHERE auth_token = $1
	`

	err := r.db.QueryRow(ctx, query, token).Scan(
		&session.ID,
		&session.UserID,
		&session.AuthToken,
		&session.DeviceID,
		&session.IPAddress,
		&session.GeoLocation,
		&session.DeviceMeta,
		&session.UserAgent,
		&session.LastSeenAt,
		&session.IsActive,
		&session.CreatedAt,
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

func (r *SessionRepository) GetSessionsByUserID(ctx context.Context, userID string) ([]*domain.Session, error) {
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
			created_at
		FROM sessions
		WHERE user_id = $1
	`

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
			&s.CreatedAt,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, &s)
	}

	return sessions, nil
}



func (r *SessionRepository) DeleteByToken(ctx context.Context, token string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
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
