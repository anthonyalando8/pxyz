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
	urbacpb "x/shared/genproto/urbacpb"
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

// CreateSession creates a new session with optimized caching
func (u *SessionUsecase) CreateSession(ctx context.Context, req *sessionpb.CreateSessionRequest) (*sessionpb.CreateSessionResponse, error) {
	if req.UserId == "" {
		return nil, errors.New("user ID required")
	}

	// Normalize input
	deviceID := normalizeDeviceID(req.DeviceId)
	ipAddress := normalizeIPAddress(req.IpAddress)

	// Get user role (with cache)
	role := u.getUserRole(ctx, req.UserId)

	// Generate token
	token, _, err := u.jwtGen.Generate(req.UserId, role, deviceID, req.Purpose, req.IsTemp, req.ExtraData)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Build session object
	session := u.buildSession(req, token, deviceID, ipAddress)

	// Cache operations in background (fire and forget)
	go u.cacheSessionData(context.Background(), session, token, role, deviceID)

	// DB write in background (with retry)
	go u.persistSessionToDB(context.Background(), session)

	// Return immediately
	return &sessionpb.CreateSessionResponse{
		Session: domainToProtoSession(session),
	}, nil
}

// ===== Helper Functions =====

// normalizeDeviceID ensures device ID is never empty
func normalizeDeviceID(deviceID string) string {
	if deviceID == "" {
		return "unknown"
	}
	return deviceID
}

// normalizeIPAddress ensures IP address is never empty
func normalizeIPAddress(ipAddress string) string {
	if ipAddress == "" {
		return "unknown"
	}
	return ipAddress
}

// getUserRole fetches user role with cache fallback
func (u *SessionUsecase) getUserRole(ctx context.Context, userID string) string {
	// Try cache first
	if role := u.getRoleFromCache(ctx, userID); role != "" {
		return role
	}

	// Fetch from urbac service
	role := u.getRoleFromService(ctx, userID)
	
	// Cache the result in background
	go u.cacheUserRole(context.Background(), userID, role)

	return role
}

// getRoleFromCache attempts to get role from cache
func (u *SessionUsecase) getRoleFromCache(ctx context.Context, userID string) string {
	if u.cache == nil {
		return ""
	}

	cacheKey := fmt.Sprintf("user:%s:role", userID)
	cachedRole, err := u.cache.Get(ctx, "user_roles", cacheKey)
	if err == nil && cachedRole != "" {
		log.Printf("[CACHE HIT] Retrieved role for user %s from cache: %s", userID, cachedRole)
		return cachedRole
	}

	return ""
}

// getRoleFromService fetches role from urbac service with fallback
func (u *SessionUsecase) getRoleFromService(ctx context.Context, userID string) string {
	if u.urbacservice == nil {
		return "temp"
	}

	rolesRes, err := u.urbacservice.GetUserRoles(ctx, userID)
	if err != nil {
		log.Printf("[WARN] failed to fetch roles for user %s: %v", userID, err)
		return "temp"
	}

	if len(rolesRes) == 0 {
		log.Printf("[INFO] user %s has no roles assigned", userID)
		return "temp"
	}

	return u.selectHighestPriorityRole(rolesRes)
}

// selectHighestPriorityRole picks the highest priority role from a list
func (u *SessionUsecase) selectHighestPriorityRole(roles []*urbacpb.UserRole) string {
	// Define priority: higher number = higher priority
	rolePriority := map[string]int{
		"any":            1,
		"kyc_unverified": 2,
		"trader":         3,
	}

	highest := "any"
	highestRank := rolePriority[highest]

	for _, r := range roles {
		roleName := r.GetRoleName()
		if rank, ok := rolePriority[roleName]; ok && rank > highestRank {
			highest = roleName
			highestRank = rank
		}
	}

	return highest
}

// cacheUserRole caches user role in background
func (u *SessionUsecase) cacheUserRole(ctx context.Context, userID, role string) {
	if u.cache == nil {
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cacheKey := fmt.Sprintf("user:%s:role", userID)
	if err := u.cache.Set(ctx, "user_roles", cacheKey, role, time.Hour); err != nil {
		log.Printf("[WARN] failed to cache role for user %s: %v", userID, err)
	} else {
		log.Printf("[CACHE SET] Cached role for user %s: %s", userID, role)
	}
}

// buildSession constructs the session domain object
func (u *SessionUsecase) buildSession(req *sessionpb.CreateSessionRequest, token, deviceID, ipAddress string) *domain.Session {
	now := time.Now()

	return &domain.Session{
		ID:          u.Sf.Generate(),
		UserID:      req.UserId,
		AuthToken:   token,
		DeviceID:    &deviceID,
		IPAddress:   &ipAddress,
		UserAgent:   strPtr(req.UserAgent),
		GeoLocation: strPtr(req.GeoLocation),
		DeviceMeta:  strPtrAny(req.DeviceMetadata),
		IsActive:    req.IsActive,
		IsSingleUse: req.IsSingleUse,
		Purpose:     req.Purpose,
		IsTemp:      req.IsTemp,
		IsUsed:      false,
		LastSeenAt:  &now,
		CreatedAt:   now,
	}
}

// cacheSessionData caches all session-related data in background
func (u *SessionUsecase) cacheSessionData(ctx context.Context, session *domain.Session, token, role, deviceID string) {
	if u.cache == nil {
		return
	}

	// Create context with timeout for all cache operations
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Cache operations can run in parallel
	errChan := make(chan error, 4)

	// 1. Cache token for user:device combination
	go func() {
		errChan <- u.cacheSessionToken(ctx, session.UserID, deviceID, token)
	}()

	// 2. Cache session ID mapping
	go func() {
		errChan <- u.cacheSessionMapping(ctx, session.ID, token)
	}()

	// 3. Cache user info
	go func() {
		errChan <- u.cacheUserInfo(ctx, session.UserID, role, session.CreatedAt)
	}()

	// 4. Cache device list
	go func() {
		errChan <- u.cacheUserDevice(ctx, session.UserID, deviceID)
	}()

	// Collect errors (non-blocking, just for logging)
	go func() {
		for i := 0; i < 4; i++ {
			if err := <-errChan; err != nil {
				log.Printf("[CACHE ERROR] %v", err)
			}
		}
	}()
}

// cacheSessionToken caches the session token
func (u *SessionUsecase) cacheSessionToken(ctx context.Context, userID, deviceID, token string) error {
	tokenKey := fmt.Sprintf("%s:%s", userID, deviceID)
	if err := u.cache.Set(ctx, "session_tokens", tokenKey, token, u.jwtGen.Ttl); err != nil {
		return fmt.Errorf("failed to cache token for user %s device %s: %w", userID, deviceID, err)
	}
	log.Printf("[CACHE SET] Cached token for user %s device %s", userID, deviceID)
	return nil
}

// cacheSessionMapping caches the session ID to token mapping
func (u *SessionUsecase) cacheSessionMapping(ctx context.Context, sessionID, token string) error {
	sessionKey := fmt.Sprintf("session:%s", sessionID)
	if err := u.cache.Set(ctx, "sessions", sessionKey, token, u.jwtGen.Ttl); err != nil {
		return fmt.Errorf("failed to cache session %s: %w", sessionID, err)
	}
	log.Printf("[CACHE SET] Cached session %s", sessionID)
	return nil
}

// cacheUserInfo caches basic user information
func (u *SessionUsecase) cacheUserInfo(ctx context.Context, userID, role string, lastLogin time.Time) error {
	userInfoKey := fmt.Sprintf("user:%s:info", userID)
	userInfo := fmt.Sprintf(`{"user_id":"%s","role":"%s","last_login":"%s"}`,
		userID, role, lastLogin.Format(time.RFC3339))
	
	if err := u.cache.Set(ctx, "user_info", userInfoKey, userInfo, 24*time.Hour); err != nil {
		return fmt.Errorf("failed to cache user info for %s: %w", userID, err)
	}
	log.Printf("[CACHE SET] Cached user info for %s", userID)
	return nil
}

// cacheUserDevice caches the device for a user
func (u *SessionUsecase) cacheUserDevice(ctx context.Context, userID, deviceID string) error {
	devicesKey := fmt.Sprintf("user:%s:devices", userID)
	if err := u.cache.Set(ctx, "user_devices", devicesKey, deviceID, 24*time.Hour); err != nil {
		return fmt.Errorf("failed to cache device list for user %s: %w", userID, err)
	}
	log.Printf("[CACHE SET] Cached device %s for user %s", deviceID, userID)
	return nil
}

// persistSessionToDB writes session to database with retry logic
func (u *SessionUsecase) persistSessionToDB(ctx context.Context, session *domain.Session) {
	const maxRetries = 3
	backoff := time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create new context for each attempt
		dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		
		err := u.SessionRepo.CreateOrUpdateSession(dbCtx, session, u.jwtGen.Ttl)
		cancel()

		if err == nil {
			log.Printf("[DB WRITE] Successfully persisted session %s for user %s", session.ID, session.UserID)
			return
		}

		// Log the error
		log.Printf("[DB ERROR] Attempt %d/%d failed to persist session %s: %v", 
			attempt, maxRetries, session.ID, err)

		// Don't retry if it's the last attempt
		if attempt == maxRetries {
			log.Printf("[DB FATAL] Failed to persist session %s after %d attempts", session.ID, maxRetries)
			// TODO: Send to dead letter queue or alerting system
			return
		}

		// Exponential backoff
		time.Sleep(backoff)
		backoff *= 2
	}
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