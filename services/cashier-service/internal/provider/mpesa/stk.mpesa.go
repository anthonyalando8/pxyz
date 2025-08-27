package mpesa

import(
	"time"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
) 

// Deposit via STK Push
func (c *MpesaClient) StkPush(phone string, amount float64, accountRef, callbackURL string) (map[string]interface{}, error) {
	token, err := c.getToken()
	if err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("20060102150405")
	password := base64.StdEncoding.EncodeToString([]byte(c.ShortCode + c.PassKey + timestamp))

	payload := map[string]interface{}{
		"BusinessShortCode": c.ShortCode,
		"Password":          password,
		"Timestamp":         timestamp,
		"TransactionType":   "CustomerPayBillOnline",
		"Amount":            amount,
		"PartyA":            phone,
		"PartyB":            c.ShortCode,
		"PhoneNumber":       phone,
		"CallBackURL":       callbackURL,
		"AccountReference":  accountRef,
		"TransactionDesc":   "Deposit",
	}

	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", c.BaseURL+"/mpesa/stkpush/v1/processrequest", bytes.NewBuffer(body))
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
