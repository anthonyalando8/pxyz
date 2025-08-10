package usecase

import (
	"context"
	"errors"
	"fmt"
	"session-service/internal/domain"
	"session-service/internal/repository"
	"time"
	sessionpb "x/shared/genproto/sessionpb"
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

func (u *SessionUsecase) CreateSession(ctx context.Context, req *sessionpb.CreateSessionRequest) (*sessionpb.CreateSessionResponse, error) {
	if req.UserId == "" || req.AuthToken == "" {
		return nil, errors.New("user ID and token are required")
	}

	deviceID := req.DeviceId
	if deviceID == "" {
		deviceID = "unknown"
	}

	ipAddress := req.IpAddress
	if ipAddress == "" {
		ipAddress = "unknown"
	}

	now := time.Now()
	session := &domain.Session{
		UserID:      req.UserId,
		AuthToken:   req.AuthToken,
		DeviceID:    &deviceID,
		IPAddress:   &ipAddress,
		UserAgent:   strPtr(req.UserAgent),
		GeoLocation: strPtr(req.GeoLocation),
		DeviceMeta:  strPtrAny(req.DeviceMetadata),
		IsActive:    req.IsActive,
		LastSeenAt:  &now,
		CreatedAt:   now,
	}

	if err := u.SessionRepo.CreateOrUpdateSession(ctx, session); err != nil {
		return nil, err
	}

	return &sessionpb.CreateSessionResponse{
		Session: domainToProtoSession(session),
	}, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// since DeviceMeta in domain is *any, you may want to store it as string in proto
func strPtrAny(s string) *any {
	if s == "" {
		return nil
	}
	var i any = s
	return &i
}

func domainToProtoSession(s *domain.Session) *sessionpb.Session {
	return &sessionpb.Session{
		Id:             s.ID,
		UserId:         s.UserID,
		AuthToken:      s.AuthToken,
		DeviceId:       deref(s.DeviceID),
		IpAddress:      deref(s.IPAddress),
		UserAgent:      deref(s.UserAgent),
		GeoLocation:    deref(s.GeoLocation),
		DeviceMetadata: anyToString(s.DeviceMeta),
		IsActive:       s.IsActive,
		LastSeenAt:     formatTime(s.LastSeenAt),
		CreatedAt:      s.CreatedAt.Format(time.RFC3339),
	}
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func anyToString(a *any) string {
	if a == nil {
		return ""
	}
	if str, ok := (*a).(string); ok {
		return str
	}
	return fmt.Sprintf("%v", *a)
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
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