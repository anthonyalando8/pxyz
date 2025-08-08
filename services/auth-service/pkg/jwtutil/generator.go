// internal/token/jwt.go
package jwtutil


import (
	"crypto/rsa"
	"time"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oklog/ulid/v2"
)

type Claims struct {
	UserID string `json:"uid"`
	Device string `json:"device,omitempty"`
	jwt.RegisteredClaims
}

type Generator struct {
	priv     *rsa.PrivateKey
	issuer   string
	audience string
	kid      string // key id for rotation
	ttl      time.Duration
}

func NewGenerator(priv *rsa.PrivateKey, issuer, audience, kid string, ttl time.Duration) *Generator {
	return &Generator{priv: priv, issuer: issuer, audience: audience, kid: kid, ttl: ttl}
}

func (g *Generator) Generate(userID, device string) (string, string, error) {
	if g.priv == nil {
        return "", "", fmt.Errorf("jwt generator has nil private key")
    }
	now := time.Now()
	jti := ulid.Make().String()

	claims := &Claims{
		UserID: userID,
		Device: device,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    g.issuer,
			Subject:   userID,
			Audience:  []string{g.audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(g.ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        jti,
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	if g.kid != "" {
		tok.Header["kid"] = g.kid
	}
	signed, err := tok.SignedString(g.priv)
	return signed, jti, err
}
