package _2faservice

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

const (
	BackupCodeLength = 10  // visible length of each code
	BackupCodeCount  = 10  // how many codes to generate
)

// GenerateBackupCodes generates n backup codes (plaintext + hashed form).
func GenerateBackupCodes(n int) ([]string, []string, error) {
	plainCodes := make([]string, 0, n)
	hashes := make([]string, 0, n)

	for i := 0; i < n; i++ {
		// Generate random bytes
		b := make([]byte, BackupCodeLength)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, fmt.Errorf("failed to generate random bytes: %w", err)
		}

		// Encode to base32 (or hex) to make it human friendly
		code := base64.StdEncoding.WithPadding(base64.NoPadding).EncodeToString(b)
		if len(code) > BackupCodeLength {
			code = code[:BackupCodeLength]
		}

		// Hash it (so we never store plaintext)
		hash := sha256.Sum256([]byte(code))
		hashStr := hex.EncodeToString(hash[:])

		plainCodes = append(plainCodes, code)
		hashes = append(hashes, hashStr)
	}

	return plainCodes, hashes, nil
}
