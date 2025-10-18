package mpesa

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// Withdraw to customer (B2C)
func (c *MpesaClient) B2C(phone string, amount float64, callbackURL string) (map[string]interface{}, error) {
	token, err := c.getToken()
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"InitiatorName":      "testapi",      // comes from Daraja portal
		"SecurityCredential": "ENC_CREDENTIAL", // encrypted credential
		"CommandID":          "BusinessPayment",
		"Amount":             amount,
		"PartyA":             c.ShortCode,
		"PartyB":             phone,
		"Remarks":            "Withdrawal",
		"QueueTimeOutURL":    callbackURL,
		"ResultURL":          callbackURL,
		"Occasion":           "Withdraw",
	}

	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", c.BaseURL+"/mpesa/b2c/v1/paymentrequest", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return res, nil
}
