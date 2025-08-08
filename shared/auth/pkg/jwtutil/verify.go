// internal/token/verify.go
package jwtutil

import (
	"crypto/rsa"
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

type Verifier struct {
	pubKeys  map[string]*rsa.PublicKey // kid -> pub
	defPub   *rsa.PublicKey            // fallback if no kid
	issuer   string
	audience string
}

func NewVerifier(def *rsa.PublicKey, issuer, audience string) *Verifier {
	return &Verifier{
		pubKeys:  map[string]*rsa.PublicKey{},
		defPub:   def,
		issuer:   issuer,
		audience: audience,
	}
}

func (v *Verifier) AddKey(kid string, pub *rsa.PublicKey) {
	v.pubKeys[kid] = pub
}

func (v *Verifier) ParseAndValidate(tokenStr string) (*Claims, error) {
	claims := new(Claims)
	parser := jwt.NewParser(jwt.WithAudience(v.audience), jwt.WithIssuer(v.issuer))

	token, err := parser.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid != "" {
			if k, ok := v.pubKeys[kid]; ok {
				return k, nil
			}
		}
		return v.defPub, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
