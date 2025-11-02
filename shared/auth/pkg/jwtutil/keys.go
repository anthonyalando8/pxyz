package jwtutil

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)


func LoadRSAPublicKeyFromPEM(path string) (*rsa.PublicKey, error) {
	// Try to find the key in multiple locations
	actualPath := findKeyFile(path)
	
	b, err := os.ReadFile(actualPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key from %s: %w", actualPath, err)
	}

	block, _ := pem.Decode(b)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("invalid PEM public key type")
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaKey, nil
}

// findKeyFile tries multiple locations for the key file
func findKeyFile(originalPath string) string {
	// List of paths to try (in order)
	locations := []string{
		originalPath,                          // Original path (e.g., ../../shared/secrets/jwt_public.pem)
		"/shared/secrets/jwt_public.pem",     // Init container location (public)
		"/app/secrets/JWT_PUBLIC_KEY",        // Direct Kubernetes mount (public)
	}
	
	// Try each location
	for _, path := range locations {
		if fileExists(path) {
			return path
		}
	}
	
	// Return original path (will fail with clear error message)
	return originalPath
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}