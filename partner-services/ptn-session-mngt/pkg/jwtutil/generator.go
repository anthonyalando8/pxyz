package jwtutil

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oklog/ulid/v2"
)

// Claims now includes IsTemp flag and ExtraData map
type Claims struct {
	UserID    string            `json:"uid"`
	Device    string            `json:"device,omitempty"`
	IsTemp    bool              `json:"is_temp"`
	UserType  string 			`json:"type"`
	Role      string			`json:"role,omitempty"`
	ExtraData map[string]string `json:"data,omitempty"`
	jwt.RegisteredClaims
}

type Generator struct {
	priv     *rsa.PrivateKey
	issuer   string
	audience string
	kid      string // key id for rotation
	Ttl      time.Duration
}

func NewGenerator(priv *rsa.PrivateKey, issuer, audience, kid string, ttl time.Duration) *Generator {
	return &Generator{priv: priv, issuer: issuer, audience: audience, kid: kid, Ttl: ttl}
}

// Updated Generate function to handle isTemp and extraData
func (g *Generator) Generate(userID, role, device string, isTemp bool, extraData map[string]string) (string, string, error) {
	if g.priv == nil {
		return "", "", fmt.Errorf("jwt generator has nil private key")
	}
	now := time.Now()
	jti := ulid.Make().String()
	expiresIn := g.Ttl
	if isTemp {
		expiresIn = 30 * time.Minute
	}

	claims := &Claims{
		UserID:    userID,
		Device:    device,
		IsTemp:    isTemp,
		Role:      role,
		UserType: "partner",
		ExtraData: extraData,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    g.issuer,
			Subject:   userID,
			Audience:  []string{g.audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(expiresIn)),
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
