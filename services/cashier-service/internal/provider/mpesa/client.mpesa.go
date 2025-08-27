// internal/provider/mpesa/client.go
package mpesa

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type MpesaClient struct {
	BaseURL      string
	ConsumerKey  string
	ConsumerSecret string
	PassKey      string
	ShortCode    string
	HttpClient   *http.Client
	token        string
	tokenExpiry  time.Time
}

func NewMpesaClient(baseURL, key, secret, passkey, shortcode string) *MpesaClient {
	return &MpesaClient{
		BaseURL:        baseURL,
		ConsumerKey:    key,
		ConsumerSecret: secret,
		PassKey:        passkey,
		ShortCode:      shortcode,
		HttpClient:     &http.Client{Timeout: 10 * time.Second},
	}
}


func (c *MpesaClient) getToken() (string, error) {
	// if token still valid, reuse
	if time.Now().Before(c.tokenExpiry) && c.token != "" {
		return c.token, nil
	}

	url := c.BaseURL + "/oauth/v1/generate?grant_type=client_credentials"
	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(c.ConsumerKey, c.ConsumerSecret)

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get token: %s", string(body))
	}

	var res struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   string `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	// save token in memory
	c.token = res.AccessToken
	c.tokenExpiry = time.Now().Add(50 * time.Minute) // expire before 1hr

	return c.token, nil
}
