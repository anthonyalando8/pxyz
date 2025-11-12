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
	//"x/shared/genproto/authpb"
	sessionpb "x/shared/genproto/sessionpb"
	urbacservice "x/shared/urbac/utils"
	"x/shared/utils/cache"

	"x/shared/utils/id"
)

type SessionUsecase struct {
	SessionRepo *repository.SessionRepository
	Sf		  *id.Snowflake
	jwtGen *jwtutil.Generator
	authClient *authclient.AuthService
	urbacservice  *urbacservice.Service
	cache 	*cache.Cache
}

func NewSessionUsecase(sessionRepo *repository.SessionRepository, sf *id.Snowflake, jwtGen *jwtutil.Generator, authClient *authclient.AuthService, 	urbacservice  *urbacservice.Service, cache *cache.Cache) *SessionUsecase {
	return &SessionUsecase{
		SessionRepo: sessionRepo,
		Sf:          sf,
		jwtGen: jwtGen,
		authClient: authClient,
		urbacservice: urbacservice,
		cache: cache,
	}
}

func (u *SessionUsecase) CreateSession(ctx context.Context, req *sessionpb.CreateSessionRequest) (*sessionpb.CreateSessionResponse, error) {
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
	
	role := "temp" // fallback if something fails

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
				"kyc_unverified": 2,
				"trader":         3,
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

	// Generate token
	token, _, err := u.jwtGen.Generate(req.UserId, role, deviceID,req.Purpose, req.IsTemp, req.ExtraData)
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

	// Cache session data immediately
	if u.cache != nil {
		// Cache token for the user:device combination
		tokenKey := fmt.Sprintf("%s:%s", req.UserId, deviceID)
		if err := u.cache.Set(ctx, "session_tokens", tokenKey, token, u.jwtGen.Ttl); err != nil {
			log.Printf("[WARN] failed to cache token for user %s device %s: %v", req.UserId, deviceID, err)
		} else {
			log.Printf("[CACHE SET] Cached token for user %s device %s", req.UserId, deviceID)
		}

		// Cache session ID mapping for quick lookup
		sessionKey := fmt.Sprintf("session:%s", session.ID)
		if err := u.cache.Set(ctx, "sessions", sessionKey, token, u.jwtGen.Ttl); err != nil {
			log.Printf("[WARN] failed to cache session %s: %v", session.ID, err)
		}

		// Cache user info (basic session metadata)
		userInfoKey := fmt.Sprintf("user:%s:info", req.UserId)
		userInfo := fmt.Sprintf(`{"user_id":"%s","role":"%s","last_login":"%s"}`, 
			req.UserId, role, now.Format(time.RFC3339))
		if err := u.cache.Set(ctx, "user_info", userInfoKey, userInfo, 24*time.Hour); err != nil {
			log.Printf("[WARN] failed to cache user info for %s: %v", req.UserId, err)
		} else {
			log.Printf("[CACHE SET] Cached user info for %s", req.UserId)
		}

		// Maintain a set of active devices per user
		devicesKey := fmt.Sprintf("user:%s:devices", req.UserId)
		if err := u.cache.Set(ctx, "user_devices", devicesKey, deviceID, 24*time.Hour); err != nil {
			log.Printf("[WARN] failed to cache device list for user %s: %v", req.UserId, err)
		}
	}

	// Write to DB asynchronously in the background
	go func() {
		// Create a new context for the background operation with timeout
		bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := u.SessionRepo.CreateOrUpdateSession(bgCtx, session, u.jwtGen.Ttl); err != nil {
			log.Printf("[ERROR] failed to persist session to DB for user %s: %v", req.UserId, err)
			
			// Optional: Implement retry logic or dead letter queue here
			// For now, just log the error
		} else {
			log.Printf("[DB WRITE] Successfully persisted session %s for user %s", session.ID, req.UserId)
		}
	}()

	// Return response immediately with cached data
	return &sessionpb.CreateSessionResponse{
		Session: domainToProtoSession(session),
	}, nil
}

// Helper method to get cached token
func (u *SessionUsecase) GetCachedToken(ctx context.Context, userID, deviceID string) (string, error) {
	if u.cache == nil {
		return "", errors.New("cache not available")
	}
	tokenKey := fmt.Sprintf("%s:%s", userID, deviceID)
	return u.cache.Get(ctx, "session_tokens", tokenKey)
}

// Helper method to invalidate cached token
func (u *SessionUsecase) InvalidateCachedToken(ctx context.Context, userID, deviceID string) error {
	if u.cache == nil {
		return nil
	}
	tokenKey := fmt.Sprintf("%s:%s", userID, deviceID)
	return u.cache.Delete(ctx, "session_tokens", tokenKey)
}

// Helper method to invalidate all user caches
func (u *SessionUsecase) InvalidateUserCache(ctx context.Context, userID string) error {
	if u.cache == nil {
		return nil
	}
	
	var errs []error
	
	// Delete role cache
	if err := u.cache.Delete(ctx, "user_roles", fmt.Sprintf("user:%s:role", userID)); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete role cache: %w", err))
	}
	
	// Delete user info cache
	if err := u.cache.Delete(ctx, "user_info", fmt.Sprintf("user:%s:info", userID)); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete user info cache: %w", err))
	}
	
	// Delete devices cache
	if err := u.cache.Delete(ctx, "user_devices", fmt.Sprintf("user:%s:devices", userID)); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete devices cache: %w", err))
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("cache invalidation errors: %v", errs)
	}
	
	return nil
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