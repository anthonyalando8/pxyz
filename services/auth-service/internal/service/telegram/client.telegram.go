package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

type TelegramClient struct {
	BotToken string
}

func NewTelegramClient(botToken string) *TelegramClient {
	return &TelegramClient{BotToken: botToken}
}

// VerifyTelegramAuth checks the authenticity of Telegram login data
func (c *TelegramClient) VerifyTelegramAuth(data map[string]string) bool {
	hash := data["hash"]
	delete(data, "hash")

	// Build data-check-string
	var pairs []string
	for k, v := range data {
		pairs = append(pairs, k+"="+v)
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	// Compute secret key
	secret := sha256.Sum256([]byte(c.BotToken))

	h := hmac.New(sha256.New, secret[:])
	h.Write([]byte(dataCheckString))
	calculatedHash := hex.EncodeToString(h.Sum(nil))

	return calculatedHash == hash
}
