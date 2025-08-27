// apple/verify.go
package apple

import (
	"context"
	"fmt"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

const appleKeysURL = "https://appleid.apple.com/auth/keys"

type AppleClaims struct {
	Sub   string
	Email string
	EmailVerified bool
	Raw   jwt.Token
}

// VerifyIDToken validates signature + standard claims, then checks audience.
func VerifyIDToken(ctx context.Context, idToken, clientID string) (*AppleClaims, error) {
	keyset, err := jwk.Fetch(ctx, appleKeysURL)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: %w", err)
	}

	t, err := jwt.ParseString(idToken, jwt.WithKeySet(keyset), jwt.WithValidate(true))
	if err != nil {
		return nil, fmt.Errorf("parse/validate id_token: %w", err)
	}

	// aud must contain your client_id (ServiceID)
	found := false
	for _, aud := range t.Audience() {
		if aud == clientID {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("invalid audience")
	}

	sub := t.Subject()
	email, _ := t.Get("email")
	ev, _ := t.Get("email_verified")

	ac := &AppleClaims{
		Sub:   sub,
		Email: str(email),
		EmailVerified: boolVal(ev),
		Raw:   t,
	}
	return ac, nil
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func boolVal(v any) bool {
	if b, ok := v.(bool); ok { return b }
	if s, ok := v.(string); ok { return s == "true" }
	return false
}
