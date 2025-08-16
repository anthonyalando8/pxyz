package middleware

import (
	"context"
	"errors"
	"net/http"

	"x/shared/auth/pkg/jwtutil"
	"x/shared/response"
	"x/shared/utils/errors"

	authpb "x/shared/genproto/sessionpb"
)



type AuthMiddleware struct {
	verifier   *jwtutil.Verifier
	authClient authpb.AuthServiceClient
}

func NewAuthMiddleware(verifier *jwtutil.Verifier, authClient authpb.AuthServiceClient) *AuthMiddleware {
	return &AuthMiddleware{verifier: verifier, authClient: authClient}
}


func (am *AuthMiddleware) validateSession(ctx context.Context, token string, w http.ResponseWriter) (*authpb.ValidateSessionResponse, bool) {
	resp, err := am.authClient.ValidateSession(ctx, &authpb.ValidateSessionRequest{Token: token})
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

func (am *AuthMiddleware) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, claims, ok := am.extractAndVerifyToken(r, w)
		if !ok {
			return
		}

		resp, ok := am.validateSession(r.Context(), token, w)
		if !ok {
			return
		}

		next.ServeHTTP(w, setContextValues(r, claims, token, resp))
	})
}

func (am *AuthMiddleware) RequireWithChecks(allowedTypes, allowedPurposes []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, claims, ok := am.extractAndVerifyToken(r, w)
			if !ok {
				return
			}

			resp, ok := am.validateSession(r.Context(), token, w)
			if !ok {
				return
			}

			if !contains(allowedTypes, resp.SessionType) {
				response.Error(w, http.StatusForbidden, "Not allowed for this session type")
				return
			}
			if len(allowedPurposes) > 0 && !contains(allowedPurposes, resp.Purpose) {
				response.Error(w, http.StatusForbidden, "Session not allowed for this purpose")
				return
			}

			next.ServeHTTP(w, setContextValues(r, claims, token, resp))
		})
	}
}