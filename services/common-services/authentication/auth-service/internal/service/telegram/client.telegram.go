package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"
)

type TelegramClient struct {
	BotToken string
}

func NewTelegramClient(botToken string) *TelegramClient {
	return &TelegramClient{BotToken: botToken}
}

// VerifyTelegramAuth checks the authenticity of Telegram login data
// VerifyTelegramAuth checks the authenticity of Telegram login data
func (c *TelegramClient) VerifyTelegramAuth(data map[string]string) bool {
	hashHex, ok := data["hash"]
	if !ok || hashHex == "" {
		return false
	}
	delete(data, "hash")

	// Build data-check-string
	var pairs []string
	for k, v := range data {
		pairs = append(pairs, k+"="+v)
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	// Compute secret key: SHA-256 of bot token
	secret := sha256.Sum256([]byte(c.BotToken))

	// Compute HMAC-SHA256 of data-check-string using secret
	h := hmac.New(sha256.New, secret[:])
	h.Write([]byte(dataCheckString))
	calculatedHash := h.Sum(nil)

	// Decode Telegram hash from hex
	hashBytes, err := hex.DecodeString(hashHex)
	if err != nil {
		return false
	}

	// Compare using hmac.Equal to prevent timing attacks
	if !hmac.Equal(calculatedHash, hashBytes) {
		return false
	}

	// Validate auth_date (within 24 hours)
	authDateStr, ok := data["auth_date"]
	if !ok {
		return false
	}
	authDateInt, err := strconv.ParseInt(authDateStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Since(time.Unix(authDateInt, 0)) > 24*time.Hour {
		return false
	}

	return true
}
