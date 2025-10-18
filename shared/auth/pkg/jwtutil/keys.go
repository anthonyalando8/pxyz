// internal/token/keys.go
package jwtutil

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

func LoadRSAPublicKeyFromPEM(path string) (*rsa.PublicKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	block, _ := pem.Decode(b)
	if block == nil || (block.Type != "PUBLIC KEY" && block.Type != "RSA PUBLIC KEY") {
		return nil, fmt.Errorf("invalid PEM public key type: %s", block.Type)
	}

	pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// Try PKCS1 as fallback
		rsaPub, err2 := x509.ParsePKCS1PublicKey(block.Bytes)
		if err2 == nil {
			return rsaPub, nil
		}
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	pub, ok := pubAny.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not RSA public key")
	}
	return pub, nil
}
