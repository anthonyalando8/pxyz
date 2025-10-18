// internal/token/loader.go
package jwtutil

import (
	"time"
	"log"
)

type JWTConfig struct {
	PrivPath string
	PubPath  string
	Issuer   string
	Audience string
	TTL      time.Duration
	KID      string
}

func LoadAndBuild(cfg JWTConfig) (*Generator) {
	priv, err := LoadRSAPrivateKeyFromPEM(cfg.PrivPath)
	if err != nil || priv == nil {
		log.Fatalf("failed to load private key from %s: %v", cfg.PrivPath, err)
	}

	gen := NewGenerator(priv, cfg.Issuer, cfg.Audience, cfg.KID, cfg.TTL)

	return gen
}
