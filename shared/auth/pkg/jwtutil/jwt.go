// internal/token/jwt.go
package jwtutil


import (
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID string `json:"uid"`
	Device string `json:"device,omitempty"`
	jwt.RegisteredClaims
}

type JWTConfig struct {
	PubPath  string
	Issuer   string
	Audience string
}