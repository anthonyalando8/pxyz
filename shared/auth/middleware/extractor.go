package middleware

import (
	"net/http"
	"strings"
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
