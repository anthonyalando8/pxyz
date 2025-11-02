package jwtutil

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

func LoadRSAPrivateKeyFromPEM(path string) (*rsa.PrivateKey, error) {
	// Try to find the key in multiple locations
	actualPath := findKeyFile(path)
	
	b, err := os.ReadFile(actualPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key from %s: %w", actualPath, err)
	}

	block, _ := pem.Decode(b)
	if block == nil || (block.Type != "RSA PRIVATE KEY" && block.Type != "PRIVATE KEY") {
		return nil, fmt.Errorf("invalid PEM private key type: %s", block.Type)
	}

	if block.Type == "PRIVATE KEY" {
		// PKCS8 format
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS8 private key: %w", err)
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA private key")
		}
		return rsaKey, nil
	}

	// PKCS1 format
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// findKeyFile tries multiple locations for the key file
func findKeyFile(originalPath string) string {
	// List of paths to try (in order)
	locations := []string{
		originalPath,                          // Original path (e.g., ../../shared/secrets/jwt_public.pem)
		"/shared/secrets/jwt_private.pem",    // Init container location (private)
		"/app/secrets/JWT_PRIVATE_KEY",       // Direct Kubernetes mount (private)
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