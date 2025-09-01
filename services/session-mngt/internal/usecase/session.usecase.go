package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"session-service/internal/domain"
	"session-service/internal/repository"
	"session-service/pkg/jwtutil"
	"time"
	authclient "x/shared/auth"
	"x/shared/genproto/authpb"
	sessionpb "x/shared/genproto/sessionpb"
	"x/shared/utils/id"
)

type SessionUsecase struct {
	SessionRepo *repository.SessionRepository
	Sf		  *id.Snowflake
	jwtGen *jwtutil.Generator
	authClient *authclient.AuthService
}

func NewSessionUsecase(sessionRepo *repository.SessionRepository, sf *id.Snowflake, jwtGen *jwtutil.Generator, authClient *authclient.AuthService) *SessionUsecase {
	return &SessionUsecase{
		SessionRepo: sessionRepo,
		Sf:          sf,
		jwtGen: jwtGen,
		authClient: authClient,
	}
}

func (u *SessionUsecase) CreateSession(ctx context.Context, req *sessionpb.CreateSessionRequest) (*sessionpb.CreateSessionResponse, error) {
	//log.Printf("Request to create session: %+v", req)
	if req.UserId == ""{
		return nil, errors.New("user ID required")
	}


	deviceID := req.DeviceId
	if deviceID == "" {
		deviceID = "unknown"
	}

	ipAddress := req.IpAddress
	if ipAddress == "" {
		ipAddress = "unknown"
	}
	role := "temp" // fallback if anything fails
	var perms []string

	if u.authClient != nil && req.UserId != "" && !req.IsTemp {
		permRes, err := u.authClient.GetUserRolesPermissions(ctx, &authpb.GetUserRolesPermissionsRequest{
			UserId: req.UserId,
		})
		if err != nil {
			log.Printf("[WARN] failed to fetch role/permissions for user %s: %v", req.UserId, err)
		} else if permRes.Ok && permRes.RoleWithPermissions != nil {
			// Assign role
			role = permRes.RoleWithPermissions.RoleName

			// Collect permissions
			for _, p := range permRes.RoleWithPermissions.Permissions {
				perms = append(perms, p.Name)
			}
		}
	}

log.Printf("User %s has role: %s and permissions: %v", req.UserId, role, perms)


	token, _, err := u.jwtGen.Generate(req.UserId, role, deviceID, req.IsTemp, req.ExtraData)

	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	isSingleUse := false
	isTemp := false

	if req.IsSingleUse{
		isSingleUse = true
	}
	if req.IsTemp{
		isTemp = true
	}

	now := time.Now()
	session := &domain.Session{
		ID:          u.Sf.Generate(),
		UserID:      req.UserId,
		AuthToken:   token,
		DeviceID:    &deviceID,
		IPAddress:   &ipAddress,
		UserAgent:   strPtr(req.UserAgent),
		GeoLocation: strPtr(req.GeoLocation),
		DeviceMeta:  strPtrAny(req.DeviceMetadata),
		IsActive:    req.IsActive,
		IsSingleUse: isSingleUse,
		Purpose: req.Purpose,
		IsTemp: isTemp,
		IsUsed: false,
		LastSeenAt:  &now,
		CreatedAt:   now,
	}
	if err := u.SessionRepo.CreateOrUpdateSession(ctx, session, u.jwtGen.Ttl); err != nil {
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
		IsSingleUse: s.IsSingleUse,
		IsUsed: s.IsUsed,
		Purpose: s.Purpose,
		IsTemp: s.IsTemp,
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
	return uc.SessionRepo.GetSessionsByUserID(ctx,userID, false)
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