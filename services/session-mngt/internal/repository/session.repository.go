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
	INSERT INTO sessions (id, user_id, token, device, ip, created_at, last_active)
	VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
	ON CONFLICT (user_id, device)
	DO UPDATE SET 
		token = EXCLUDED.token,
		ip = EXCLUDED.ip,
		last_active = NOW();
	`
	_, err := r.db.Exec(ctx, query,
		session.ID,
		session.UserID,
		session.Token,
		session.Device,
		session.IP,
	)
	return err
}


func (r *SessionRepository) GetSessionByToken(ctx context.Context, token string) (*domain.Session, error) {
	var session domain.Session
	err := r.db.QueryRow(ctx, `SELECT id, user_id, token, device, ip::text, last_active, created_at FROM sessions WHERE token=$1`, token).
		Scan(&session.ID, &session.UserID, &session.Token, &session.Device, &session.IP, &session.LastActive, &session.CreatedAt)
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
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, device, ip, last_active, created_at 
		FROM sessions 
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*domain.Session
	for rows.Next() {
		var s domain.Session
		err := rows.Scan(&s.ID, &s.UserID, &s.Device, &s.IP, &s.LastActive, &s.CreatedAt)
		if err != nil {
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
