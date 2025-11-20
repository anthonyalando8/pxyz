package middleware

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"x/shared/auth/pkg/jwtutil"
	"x/shared/response"
	"x/shared/utils/cache"
	xerrors "x/shared/utils/errors"

	adminsessionpb "x/shared/genproto/admin/sessionpb"
	ptnsessionpb "x/shared/genproto/partner/sessionpb"
	authpb "x/shared/genproto/sessionpb"
	"x/shared/genproto/urbacpb"
)




type AuthMiddleware struct {
	// JWT verifiers
	UserVerifier    *jwtutil.Verifier
	PartnerVerifier *jwtutil.Verifier
	AdminVerifier   *jwtutil.Verifier

	// gRPC clients
	UserClient    authpb.AuthServiceClient
	PartnerClient ptnsessionpb.PartnerSessionServiceClient
	AdminClient   adminsessionpb.AdminSessionServiceClient
	RBACClient    urbacpb.RBACServiceClient
	cache *cache.Cache
}

// Constructor
func NewAuthMiddleware(
	userVerifier, partnerVerifier, adminVerifier *jwtutil.Verifier,
	userClient authpb.AuthServiceClient,
	partnerClient ptnsessionpb.PartnerSessionServiceClient,
	adminClient adminsessionpb.AdminSessionServiceClient,
	rbacClient urbacpb.RBACServiceClient,
	cache *cache.Cache,
) *AuthMiddleware {
	return &AuthMiddleware{
		UserVerifier:    userVerifier,
		PartnerVerifier: partnerVerifier,
		AdminVerifier:   adminVerifier,
		UserClient:      userClient,
		PartnerClient:   partnerClient,
		AdminClient:     adminClient,
		RBACClient:      rbacClient,
		cache: cache,
	}
}


// validateSession calls the relevant client based on tokenType
func (am *AuthMiddleware) validateSession(ctx context.Context, token, tokenType string, w http.ResponseWriter) (*authpb.ValidateSessionResponse, bool) {
	var (
		resp *authpb.ValidateSessionResponse
		err  error
	)

	switch tokenType {
	case "user":
		resp, err = am.UserClient.ValidateSession(ctx, &authpb.ValidateSessionRequest{Token: token})
	case "partner":
		partnerResp, e := am.PartnerClient.ValidateSession(ctx, &ptnsessionpb.ValidateSessionRequest{Token: token})
		err = e
		if err == nil {
			// map partner response to shared authpb.ValidateSessionResponse
			resp = &authpb.ValidateSessionResponse{
				Valid: partnerResp.Valid,
				Error: partnerResp.Error,
				SessionType: partnerResp.SessionType,
				Purpose: partnerResp.Purpose,
				// include other fields if needed
			}
		}
	case "admin":
		adminResp, e := am.AdminClient.ValidateSession(ctx, &adminsessionpb.ValidateSessionRequest{Token: token})
		err = e
		if err == nil {
			// map admin response to shared authpb.ValidateSessionResponse
			resp = &authpb.ValidateSessionResponse{
				Valid: adminResp.Valid,
				Error: adminResp.Error,
				SessionType: adminResp.SessionType,
				Purpose: adminResp.Purpose,
				// include other fields if needed
			}
		}
	default:
		response.Error(w, http.StatusUnauthorized, "Unknown token type")
		return nil, false
	}

	if err != nil {
		log.Printf("Session validation error for token type %s: %v", tokenType, err)
		switch {
		case errors.Is(err, xerrors.ErrNotFound):
			response.Error(w, http.StatusUnauthorized, "Session not found")
		case errors.Is(err, xerrors.ErrSessionUsed):
			response.Error(w, http.StatusUnauthorized, "Session already used")
		default:
			response.Error(w, http.StatusUnauthorized, "Session validation failed")
		}
		return nil, false
	}

	if !resp.Valid {
		response.Error(w, http.StatusUnauthorized, resp.Error)
		return nil, false
	}

	return resp, true
}


// handleAuth validates token, session, optional type/purpose, and optional roles
func (am *AuthMiddleware) handleAuth(
	w http.ResponseWriter,
	r *http.Request,
	allowedTypes, allowedPurposes, allowedRoles []string,
) (string, *jwtutil.Claims, *authpb.ValidateSessionResponse, bool) {

	// --- 1. Derive expected token type from URL prefix ---
	path := r.URL.Path
	var expectedType string

	switch {
	case strings.HasPrefix(path, "/admin/"):
		expectedType = "admin"
	case strings.HasPrefix(path, "/user/"):
		expectedType = "user"
	case strings.HasPrefix(path, "/partner/"):
		expectedType = "partner"
	default:
		log.Printf("⚠️ Unauthorized: invalid API prefix for path %s", path)
		expectedType = "user"
	}

	// --- 2. Extract token and validate against expected type ---
	token, claims, tokenType, ok := am.extractAndVerifyToken(r, w, []string{expectedType})
	if !ok {
		return "", nil, nil, false
	}

	// --- 3. Ensure token type matches API prefix ---
	if tokenType != expectedType {
		response.Error(w, http.StatusForbidden, "Token type not allowed for this API")
		return "", nil, nil, false
	}

	// --- 4. Validate session with cache-first approach ---
	resp, ok := am.validateSessionWithCache(r.Context(), token, tokenType, claims, w)
	if !ok {
		return "", nil, nil, false
	}

	// --- 5. Check optional session type restrictions ---
	if len(allowedTypes) > 0 {
		log.Printf("Allowed session types: %v, current session type: %s", allowedTypes, resp.SessionType)
		if !contains(allowedTypes, resp.SessionType) {
			response.Error(w, http.StatusForbidden, "Not allowed for this session type")
			return "", nil, nil, false
		}
	}

	// --- 6. Check optional session purpose restrictions ---
	if len(allowedPurposes) > 0 {
		log.Printf("Allowed session purposes: %v, current session purpose: %s", allowedPurposes, resp.Purpose)
		if !contains(allowedPurposes, resp.Purpose) {
			response.Error(w, http.StatusForbidden, "Session not allowed for this purpose")
			return "", nil, nil, false
		}
	}

	// --- 7. Check optional role restrictions ---
	if len(allowedRoles) > 0 {
		var role string
		log.Printf("Allowed roles: %v", allowedRoles)
		// Try context role first
		if roleVal := r.Context().Value(ContextRole); roleVal != nil {
			if rStr, ok := roleVal.(string); ok && rStr != "" {
				role = rStr
			}
		}

		// Fallback to claims role if context role is empty
		if role == "" && claims != nil && claims.Role != "" {
			role = claims.Role
		}

		// Final check
		if role == "" {
			response.Error(w, http.StatusForbidden, "role not found in context or claims")
			return "", nil, nil, false
		}

		if !contains(allowedRoles, role) {
			response.Error(w, http.StatusForbidden, "insufficient role")
			return "", nil, nil, false
		}
	}

	return token, claims, resp, true
}

// validateSessionWithCache checks cache first before calling validation service
func (am *AuthMiddleware) validateSessionWithCache(
	ctx context.Context,
	token, tokenType string,
	claims *jwtutil.Claims,
	w http.ResponseWriter,
) (*authpb.ValidateSessionResponse, bool) {
	
	// Try cache first if available
	if am.cache != nil && claims != nil {
		cacheKey := fmt.Sprintf("%s:%s", claims.UserID, claims.Device)
		
		// Check if token exists in cache
		cachedToken, err := am.cache.Get(ctx, "session_tokens", cacheKey)
		if err == nil && cachedToken == token {
			log.Printf("[CACHE HIT] Valid session found for user %s device %s", claims.UserID, claims.Device)
			
			// Construct response from cached data
			sessionType := "main"
			if claims.IsTemp{
				sessionType = "temp"
			}
			resp := &authpb.ValidateSessionResponse{
				Valid: true,
				SessionType: sessionType,
				Purpose: claims.SessionPurpose,
			}
			
			// Optionally get additional session metadata from cache
			sessionMetaKey := fmt.Sprintf("user:%s:info", claims.UserID)
			if userInfo, err := am.cache.Get(ctx, "user_info", sessionMetaKey); err == nil && userInfo != "" {
				log.Printf("[CACHE HIT] User info found: %s", userInfo)
				// Parse JSON if needed for additional fields
			}
			
			return resp, true
		}
		
		if err != nil {
			log.Printf("[CACHE MISS] Session not in cache for user %s device %s: %v", claims.UserID, claims.Device, err)
		} else {
			log.Printf("[CACHE INVALID] Token mismatch for user %s device %s", claims.UserID, claims.Device)
			// Token in cache doesn't match - possible revocation or new session
			// Fall through to validation service
		}
	}

	// Cache miss or not available - call validation service
	log.Printf("[VALIDATION SERVICE] Calling service for token type: %s", tokenType)
	return am.validateSession(ctx, token, tokenType, w)
}

// AuthMiddleware without restrictions
func (am *AuthMiddleware) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, claims, resp, ok := am.handleAuth(w, r, nil, nil, nil)
		if !ok {
			return
		}
		next.ServeHTTP(w, setContextValues(r, claims, token, resp))
	})
}

// RequireWithChecks allows optional session type, purpose, and role restrictions
func (am *AuthMiddleware) RequireWithChecks(
	allowedTypes, allowedPurposes, allowedRoles []string,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, claims, resp, ok := am.handleAuth(w, r, allowedTypes, allowedPurposes, allowedRoles)
			if !ok {
				return
			}
			next.ServeHTTP(w, setContextValues(r, claims, token, resp))
		})
	}
}



// WithRole sets the role in context (to be called after JWT verification)
func WithRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ContextRole, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}


