package middleware

import (
	"context"
	"errors"
	"net/http"


	"x/shared/auth/pkg/jwtutil"
	"x/shared/response"
	xerrors "x/shared/utils/errors"

	authpb "x/shared/genproto/sessionpb"
	ptnsessionpb "x/shared/genproto/partner/sessionpb"
	adminsessionpb "x/shared/genproto/admin/sessionpb"
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
}

// Constructor
func NewAuthMiddleware(
	userVerifier, partnerVerifier, adminVerifier *jwtutil.Verifier,
	userClient authpb.AuthServiceClient,
	partnerClient ptnsessionpb.PartnerSessionServiceClient,
	adminClient adminsessionpb.AdminSessionServiceClient,
	rbacClient urbacpb.RBACServiceClient,
) *AuthMiddleware {
	return &AuthMiddleware{
		UserVerifier:    userVerifier,
		PartnerVerifier: partnerVerifier,
		AdminVerifier:   adminVerifier,
		UserClient:      userClient,
		PartnerClient:   partnerClient,
		AdminClient:     adminClient,
		RBACClient:      rbacClient,
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
				// include other fields if needed
			}
		}
	default:
		response.Error(w, http.StatusUnauthorized, "Unknown token type")
		return nil, false
	}

	if err != nil {
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

	// Extract token and determine token type
	token, claims, tokenType, ok := am.extractAndVerifyToken(r, w)
	if !ok {
		return "", nil, nil, false
	}

	// Validate session with the appropriate client
	resp, ok := am.validateSession(r.Context(), token, tokenType, w)
	if !ok {
		return "", nil, nil, false
	}

	// Check optional session type restrictions
	if len(allowedTypes) > 0 && !contains(allowedTypes, resp.SessionType) {
		response.Error(w, http.StatusForbidden, "Not allowed for this session type")
		return "", nil, nil, false
	}

	// Check optional session purpose restrictions
	if len(allowedPurposes) > 0 && !contains(allowedPurposes, resp.Purpose) {
		response.Error(w, http.StatusForbidden, "Session not allowed for this purpose")
		return "", nil, nil, false
	}

	// Check optional role restrictions
	if len(allowedRoles) > 0 {
		roleVal := r.Context().Value(ContextRole)
		if roleVal == nil {
			response.Error(w, http.StatusForbidden, "role not found in context")
			return "", nil, nil, false
		}
		role, ok := roleVal.(string)
		if !ok || role == "" {
			response.Error(w, http.StatusForbidden, "invalid role in context")
			return "", nil, nil, false
		}
		if !contains(allowedRoles, role) {
			response.Error(w, http.StatusForbidden, "insufficient role")
			return "", nil, nil, false
		}
	}

	return token, claims, resp, true
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


