// internal/token/jwt.go
package jwtutil


import (
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID    string            `json:"uid"`
	Device    string            `json:"device,omitempty"`
	IsTemp    bool              `json:"is_temp"`
	Role      string			`json:"role,omitempty"`
	SessionPurpose string 			`json:"purpose,omitempty"`
	UserType  string 			`json:"type"`
	ExtraData map[string]string `json:"data,omitempty"`
	jwt.RegisteredClaims
}

type JWTConfig struct {
	PubPath  string
	Issuer   string
	Audience string
}