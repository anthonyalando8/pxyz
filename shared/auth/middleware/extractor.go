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

func (am *AuthMiddleware) extractAndVerifyToken(r *http.Request, w http.ResponseWriter) (string, *jwtutil.Claims, bool) {
	token := extractToken(r)
	if token == "" {
		response.Error(w, http.StatusUnauthorized, "No token provided")
		return "", nil, false
	}

	claims, err := am.verifier.ParseAndValidate(token)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "Invalid or expired token")
		return "", nil, false
	}

	return token, claims, true
}
