// internal/token/loader.go
package jwtutil

import (
	"log"
)

func LoadAndBuild(cfg JWTConfig) (*Verifier) {
	pub, err := LoadRSAPublicKeyFromPEM(cfg.PubPath)
	if err != nil || pub == nil {
		log.Fatalf("failed to load public key from %s: %v", cfg.PubPath, err)
	}

	ver := NewVerifier(pub, cfg.Issuer, cfg.Audience)

	return ver
}
