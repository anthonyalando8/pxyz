package middleware

import (
	service "auth-service/internal/service/app_oauth2_client"
	"context"
	"net/http"
	"strings"
	"x/shared/response"
	globalmid "x/shared/auth/middleware"
)


type contextKey string

const (
	// ContextUserID contextKey = "userID"
	// ContextToken  contextKey = "token"
	ContextScope contextKey = "scope"
	ContextClientID contextKey = "clientID"
)

type OAuth2Middleware struct {
	oauth2Svc *service.OAuth2Service
}

func NewOAuth2Middleware(oauth2Svc *service.OAuth2Service) *OAuth2Middleware {
	return &OAuth2Middleware{oauth2Svc: oauth2Svc}
}

// ValidateOAuth2Token validates OAuth2 access tokens for API access
func (m *OAuth2Middleware) ValidateOAuth2Token(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			response.Error(w, http.StatusUnauthorized, "Authorization header required")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Error(w, http.StatusUnauthorized, "Invalid authorization header format")
			return
		}

		token := parts[1]

		// Validate the access token
		accessToken, err := m.oauth2Svc.ValidateAccessToken(r.Context(), token)
		if err != nil {
			response.Error(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		// Add user and client info to context
		ctx := r.Context()
		if accessToken.UserID != nil {
			ctx = context.WithValue(ctx, globalmid.ContextUserID, *accessToken.UserID)
		}
		ctx = context.WithValue(ctx, ContextClientID, accessToken.ClientID)
		ctx = context.WithValue(ctx, ContextScope, accessToken.Scope)
		ctx = context.WithValue(ctx, globalmid.ContextToken, token)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}


// CheckScope ensures the token has required scope
func (m *OAuth2Middleware) CheckScope(requiredScope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scope, ok := r.Context().Value(ContextScope).(string)
			if !ok || scope == "" {
				response.Error(w, http.StatusForbidden, "Insufficient permissions")
				return
			}

			// Check if required scope is present
			scopes := strings.Split(scope, " ")
			hasScope := false
			for _, s := range scopes {
				if s == requiredScope {
					hasScope = true
					break
				}
			}

			if !hasScope {
				response.Error(w, http.StatusForbidden, "Insufficient scope")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
