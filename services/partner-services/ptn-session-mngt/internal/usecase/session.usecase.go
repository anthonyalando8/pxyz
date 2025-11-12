package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"ptn-session-service/internal/domain"
	"ptn-session-service/internal/repository"
	"ptn-session-service/pkg/jwtutil"
	"time"
	authclient "x/shared/auth"
	//authpb "x/shared/genproto/partner/authpb"
	urbacservice "x/shared/factory/partner/urbac/utils"
	"x/shared/utils/cache"

	//"x/shared/genproto/authpb"
	sessionpb "x/shared/genproto/partner/sessionpb"

	"x/shared/utils/id"
)

type SessionUsecase struct {
	SessionRepo  *repository.SessionRepository
	Sf           *id.Snowflake
	jwtGen       *jwtutil.Generator
	authClient   *authclient.AuthService
	urbacservice  *urbacservice.Service
	cache 	*cache.Cache
}

func NewSessionUsecase(sessionRepo *repository.SessionRepository, sf *id.Snowflake, jwtGen *jwtutil.Generator, authClient *authclient.AuthService, urbacservice  *urbacservice.Service, cache 	*cache.Cache,) *SessionUsecase {
	return &SessionUsecase{
		SessionRepo:  sessionRepo,
		Sf:           sf,
		jwtGen:       jwtGen,
		authClient:   authClient,
		urbacservice: urbacservice,
		cache: cache,
	}
}

func (u *SessionUsecase) CreateSession(ctx context.Context, req *sessionpb.CreateSessionRequest) (*sessionpb.CreateSessionResponse, error) {
	//log.Printf("Request to create session: %+v", req)
	if req.UserId == "" {
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
	role := "any" // fallback if something fails

	// Try to get role from cache first
	var rolesFromCache bool
	cacheKey := fmt.Sprintf("user:%s:role", req.UserId)
	if u.cache != nil {
		cachedRole, err := u.cache.Get(ctx, "user_roles", cacheKey)
		if err == nil && cachedRole != "" {
			role = cachedRole
			rolesFromCache = true
			log.Printf("[CACHE HIT] Retrieved role for user %s from cache: %s", req.UserId, role)
		}
	}

	// If not in cache, fetch from urbac service
	if !rolesFromCache && u.urbacservice != nil && req.UserId != "" {
		rolesRes, err := u.urbacservice.GetUserRoles(ctx, req.UserId)
		if err != nil {
			log.Printf("[WARN] failed to fetch roles for user %s: %v", req.UserId, err)
		} else if len(rolesRes) > 0 {
			// Define priority: lower number = higher priority
			rolePriority := map[string]int{
				"any":            1,
				"partner_user": 2,
				"partner_admin":         3,
			}

			// Start with lowest
			highest := "any"
			highestRank := rolePriority[highest]

			for _, r := range rolesRes {
				roleName := r.GetRoleName()
				if rank, ok := rolePriority[roleName]; ok && rank > highestRank {
					highest = roleName
					highestRank = rank
				}
			}

			role = highest
			
			// Cache the role for 1 hour
			if u.cache != nil {
				if err := u.cache.Set(ctx, "user_roles", cacheKey, role, time.Hour); err != nil {
					log.Printf("[WARN] failed to cache role for user %s: %v", req.UserId, err)
				} else {
					log.Printf("[CACHE SET] Cached role for user %s: %s", req.UserId, role)
				}
			}
		} else {
			log.Printf("[INFO] user %s has no roles assigned", req.UserId)
		}
	}

	log.Printf("User %s has role: %s", req.UserId, role)

	token, _, err := u.jwtGen.Generate(req.UserId, role, deviceID, req.IsTemp, req.ExtraData)

	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	isSingleUse := false
	isTemp := false

	if req.IsSingleUse {
		isSingleUse = true
	}
	if req.IsTemp {
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
		Purpose:     req.Purpose,
		IsTemp:      isTemp,
		IsUsed:      false,
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
		IsSingleUse:    s.IsSingleUse,
		IsUsed:         s.IsUsed,
		Purpose:        s.Purpose,
		IsTemp:         s.IsTemp,
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
	return uc.SessionRepo.GetSessionsByUserID(ctx, userID, false)
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
