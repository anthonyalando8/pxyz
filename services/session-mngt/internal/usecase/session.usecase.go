package usecase

import (
	"context"
	"errors"
	"session-service/internal/domain"
	"session-service/internal/repository"
	"time"
	authpb "x/shared/genproto/authpb"
	"x/shared/utils/id"
)

type SessionUsecase struct {
	SessionRepo *repository.SessionRepository
	Sf		  *id.Snowflake
}

func NewSessionUsecase(sessionRepo *repository.SessionRepository, sf *id.Snowflake) *SessionUsecase {
	return &SessionUsecase{
		SessionRepo: sessionRepo,
		Sf:          sf,
	}
}

func (u *SessionUsecase) CreateSession(ctx context.Context, req *authpb.CreateSessionRequest) (*authpb.CreateSessionResponse, error) {
	session := &domain.Session{
		ID:         u.Sf.Generate(), // "sess_xyz..."
		UserID:     req.UserId,
		Device:     req.Device,
		IP:         req.Ip,
		Token:      req.Token,
		LastActive: time.Now(),
	}
	if session.UserID == "" || session.Token == "" {
		return nil, errors.New("user ID and token are required")
	}
	if session.Device == "" {
		session.Device = "unknown"
	}
	if session.IP == "" {
		session.IP = "unknown"
	}

	err := u.SessionRepo.CreateOrUpdateSession(ctx, session)
	if err != nil {
		return nil, err
	}

	return &authpb.CreateSessionResponse{SessionId: session.ID}, nil
}

func (uc *SessionUsecase) GetSessionsByUserID(ctx context.Context, userID string) ([]*domain.Session, error) {
	return uc.SessionRepo.GetSessionsByUserID(ctx,userID)
}

func (u *SessionUsecase) DeleteSession(ctx context.Context, token string) error {
	return u.SessionRepo.DeleteByToken(ctx, token)
}

func (u *SessionUsecase) DeleteAllSessions(ctx context.Context, userId string) error {
	return u.SessionRepo.DeleteAllByUser(ctx, userId)
}

func (u *SessionUsecase) DeleteSessionByID(ctx context.Context, sessionID string) error {
	return u.SessionRepo.DeleteByID(ctx, sessionID)
}