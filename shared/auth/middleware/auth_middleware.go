package middleware

import (
	"context"
	"fmt"
	"net/http"

	"x/shared/auth/pkg/jwtutil"
	"x/shared/response"
	authpb "x/shared/genproto/sessionpb"
)

type contextKey string

const (
	ContextUserID contextKey = "userID"
	ContextToken  contextKey = "token"
)

type AuthMiddleware struct {
	verifier   *jwtutil.Verifier
	authClient authpb.AuthServiceClient
}

func NewAuthMiddleware(verifier *jwtutil.Verifier, authClient authpb.AuthServiceClient) *AuthMiddleware {
	return &AuthMiddleware{verifier: verifier, authClient: authClient}
}

func (am *AuthMiddleware) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			fmt.Println("No token provided")
			response.Error(w, http.StatusUnauthorized, "Unauthorized: no token provided")
			return
		}

		claims, err := am.verifier.ParseAndValidate(token)
		if err != nil {
			fmt.Printf("Invalid token: %v\n", err)
			response.Error(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		resp, err := am.authClient.ValidateSession(r.Context(), &authpb.ValidateSessionRequest{Token: token})
		if err != nil || !resp.Valid {
			fmt.Printf("Session validation failed: %v\n", err)
			response.Error(w, http.StatusUnauthorized, "Unauthorized: session not found")
			return
		}

		ctx := context.WithValue(r.Context(), ContextUserID, claims.UserID)
		ctx = context.WithValue(ctx, ContextToken, token)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
