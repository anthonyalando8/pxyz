package middleware

import (
	"net/http"
	"strings"
	"x/shared/auth/pkg/jwtutil"
	"x/shared/response"
)

func extractToken(r *http.Request) string {
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}
	if cookie, err := r.Cookie("token"); err == nil {
		return cookie.Value
	}
	if q := r.URL.Query().Get("token"); q != "" {
		return q
	}
	return ""
}

func (am *AuthMiddleware) extractAndVerifyToken(r *http.Request, w http.ResponseWriter) (string, *jwtutil.Claims, string, bool) {
	token := extractToken(r)
	if token == "" {
		response.Error(w, http.StatusUnauthorized, "No token provided")
		return "", nil, "", false
	}

	var claims *jwtutil.Claims
	var err error
	var tokenType string

	// Try User verifier
	claims, err = am.UserVerifier.ParseAndValidate(token)
	if err == nil {
		tokenType = "user"
		return token, claims, tokenType, true
	}

	// Try Partner verifier
	claims, err = am.PartnerVerifier.ParseAndValidate(token)
	if err == nil {
		tokenType = "partner"
		return token, claims, tokenType, true
	}

	// Try Admin verifier
	claims, err = am.AdminVerifier.ParseAndValidate(token)
	if err == nil {
		tokenType = "admin"
		return token, claims, tokenType, true
	}

	// If all fail
	response.Error(w, http.StatusUnauthorized, "Invalid or expired token")
	return "", nil, "", false
}

