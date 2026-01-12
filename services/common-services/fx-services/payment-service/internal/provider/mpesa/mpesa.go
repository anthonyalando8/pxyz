// internal/provider/mpesa/mpesa. go
package mpesa

import (
    "bytes"
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
    
    "payment-service/config"
)

type MpesaProvider struct {
    config     config.MpesaConfig
    baseURL    string
    httpClient *http.Client
}

func NewMpesaProvider(cfg config.MpesaConfig) *MpesaProvider {
    baseURL := "https://sandbox.safaricom.co.ke"
    if cfg.Environment == "production" {
        baseURL = "https://api.safaricom.co.ke"
    }

    return &MpesaProvider{
        config:     cfg,
        baseURL:    baseURL,
        httpClient: &http.Client{Timeout: 30 * time. Second},
    }
}

// ============================================
// STK PUSH (Lipa Na M-Pesa Online)
// ============================================

// STKPushRequest represents M-Pesa STK Push request
type STKPushRequest struct {
    BusinessShortCode string `json:"BusinessShortCode"`
    Password          string `json:"Password"`
    Timestamp         string `json:"Timestamp"`
    TransactionType   string `json:"TransactionType"`
    Amount            int    `json:"Amount"`
    PartyA            string `json:"PartyA"`
    PartyB            string `json:"PartyB"`
    PhoneNumber       string `json:"PhoneNumber"`
    CallBackURL       string `json:"CallBackURL"`
    AccountReference  string `json:"AccountReference"`
    TransactionDesc   string `json:"TransactionDesc"`
}

// STKPushResponse represents M-Pesa STK Push response
type STKPushResponse struct {
    MerchantRequestID   string `json:"MerchantRequestID"`
    CheckoutRequestID   string `json:"CheckoutRequestID"`
    ResponseCode        string `json:"ResponseCode"`
    ResponseDescription string `json:"ResponseDescription"`
    CustomerMessage     string `json:"CustomerMessage"`
}

// InitiateSTKPush initiates M-Pesa STK Push (Lipa Na M-Pesa Online)
func (m *MpesaProvider) InitiateSTKPush(ctx context.Context, phoneNumber, accountRef string, amount float64, callbackURL string) (*STKPushResponse, error) {
    // Get access token
    token, err := m.getAccessToken(ctx, "stk")
    if err != nil {
        return nil, fmt. Errorf("failed to get access token: %w", err)
    }

    // Prepare request
    timestamp := time.Now().Format("20060102150405")
    password := base64.StdEncoding. EncodeToString([]byte(
        m.config.ShortCode + m.config.Passkey + timestamp,
    ))

    request := STKPushRequest{
        BusinessShortCode: m.config.ShortCode,
        Password:          password,
        Timestamp:         timestamp,
        TransactionType:   "CustomerPayBillOnline",
        Amount:            int(amount),
        PartyA:            phoneNumber,
        PartyB:            m.config.ShortCode,
        PhoneNumber:       phoneNumber,
        CallBackURL:       callbackURL,
        AccountReference:  accountRef,
        TransactionDesc:   fmt.Sprintf("Payment for %s", accountRef),
    }

    // Make API call
    url := fmt. Sprintf("%s/mpesa/stkpush/v1/processrequest", m.baseURL)
    respData, err := m.makeRequest(ctx, "POST", url, token, request)
    if err != nil {
        return nil, err
    }

    // Parse response
    var response STKPushResponse
    responseBytes, _ := json.Marshal(respData)
    if err := json.Unmarshal(responseBytes, &response); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    return &response, nil
}

// STKCallbackRequest represents M-Pesa STK callback
type STKCallbackRequest struct {
    Body struct {
        StkCallback struct {
            MerchantRequestID string `json:"MerchantRequestID"`
            CheckoutRequestID string `json:"CheckoutRequestID"`
            ResultCode        int    `json:"ResultCode"`
            ResultDesc        string `json:"ResultDesc"`
            CallbackMetadata  struct {
                Item []struct {
                    Name  string      `json:"Name"`
                    Value interface{} `json:"Value"`
                } `json:"Item"`
            } `json:"CallbackMetadata"`
        } `json:"stkCallback"`
    } `json:"Body"`
}

// ParseSTKCallback parses M-Pesa STK callback
func (m *MpesaProvider) ParseSTKCallback(payload []byte) (*CallbackResult, error) {
    var callback STKCallbackRequest
    if err := json.Unmarshal(payload, &callback); err != nil {
        return nil, fmt.Errorf("failed to parse callback: %w", err)
    }

    stkCallback := callback.Body.StkCallback
    
    result := &CallbackResult{
        CheckoutRequestID: stkCallback. CheckoutRequestID,
        MerchantRequestID: stkCallback.MerchantRequestID,
        ResultCode:        fmt.Sprintf("%d", stkCallback.ResultCode),
        ResultDescription: stkCallback.ResultDesc,
        Success:           stkCallback.ResultCode == 0,
        RawData:           make(map[string]interface{}),
    }

    // Extract metadata
    if stkCallback.ResultCode == 0 {
        for _, item := range stkCallback.CallbackMetadata.Item {
            switch item.Name {
            case "Amount":
                if val, ok := item.Value.(float64); ok {
                    result.Amount = val
                }
            case "MpesaReceiptNumber":
                if val, ok := item.Value.(string); ok {
                    result.ProviderTxID = val
                }
            case "PhoneNumber":
                if val, ok := item.Value.(string); ok {
                    result.PhoneNumber = val
                }
            }
            result.RawData[item. Name] = item.Value
        }
    }

    return result, nil
}

// ============================================
// B2C (Business to Customer)
// ============================================

// B2CRequest represents M-Pesa B2C request
type B2CRequest struct {
    InitiatorName      string `json:"InitiatorName"`
    SecurityCredential string `json:"SecurityCredential"`
    CommandID          string `json:"CommandID"`
    Amount             int    `json:"Amount"`
    PartyA             string `json:"PartyA"`
    PartyB             string `json:"PartyB"`
    Remarks            string `json:"Remarks"`
    QueueTimeOutURL    string `json:"QueueTimeOutURL"`
    ResultURL          string `json:"ResultURL"`
    Occasion           string `json:"Occasion"`
}

// B2CResponse represents M-Pesa B2C response
type B2CResponse struct {
    ConversationID           string `json:"ConversationID"`
    OriginatorConversationID string `json:"OriginatorConversationID"`
    ResponseCode             string `json:"ResponseCode"`
    ResponseDescription      string `json:"ResponseDescription"`
}

// InitiateB2C initiates M-Pesa B2C (Business to Customer) payment
func (m *MpesaProvider) InitiateB2C(ctx context.Context, phoneNumber, accountRef string, amount float64, resultURL, timeoutURL string) (*B2CResponse, error) {
    // Get access token
    token, err := m.getAccessToken(ctx, "b2c")
    if err != nil {
        return nil, fmt.Errorf("failed to get access token: %w", err)
    }

    request := B2CRequest{
        InitiatorName:      m. config.B2CInitiatorName,
        SecurityCredential: m.config.B2CSecurityCredential,
        CommandID:          "BusinessPayment",
        Amount:             int(amount),
        PartyA:             m.config.B2CShortCode,
        PartyB:             phoneNumber,
        Remarks:            fmt.Sprintf("Withdrawal for %s", accountRef),
        QueueTimeOutURL:    timeoutURL,
        ResultURL:          resultURL,
        Occasion:           accountRef,
    }

    // Make API call
    url := fmt.Sprintf("%s/mpesa/b2c/v1/paymentrequest", m.baseURL)
    respData, err := m. makeRequest(ctx, "POST", url, token, request)
    if err != nil {
        return nil, err
    }

    // Parse response
    var response B2CResponse
    responseBytes, _ := json.Marshal(respData)
    if err := json. Unmarshal(responseBytes, &response); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    return &response, nil
}

// B2CCallbackRequest represents M-Pesa B2C callback
type B2CCallbackRequest struct {
    Result struct {
        ResultType               int    `json:"ResultType"`
        ResultCode               int    `json:"ResultCode"`
        ResultDesc               string `json:"ResultDesc"`
        OriginatorConversationID string `json:"OriginatorConversationID"`
        ConversationID           string `json:"ConversationID"`
        TransactionID            string `json:"TransactionID"`
        ResultParameters         struct {
            ResultParameter []struct {
                Key   string      `json:"Key"`
                Value interface{} `json:"Value"`
            } `json:"ResultParameter"`
        } `json:"ResultParameters"`
    } `json:"Result"`
}

// ParseB2CCallback parses M-Pesa B2C callback
func (m *MpesaProvider) ParseB2CCallback(payload []byte) (*CallbackResult, error) {
    var callback B2CCallbackRequest
    if err := json. Unmarshal(payload, &callback); err != nil {
        return nil, fmt.Errorf("failed to parse callback: %w", err)
    }

    result := &CallbackResult{
        ConversationID:    callback.Result.ConversationID,
        TransactionID:     callback.Result. TransactionID,
        ProviderTxID:      callback. Result.TransactionID,
        ResultCode:        fmt.Sprintf("%d", callback.Result.ResultCode),
        ResultDescription: callback.Result.ResultDesc,
        Success:           callback.Result.ResultCode == 0,
        RawData:           make(map[string]interface{}),
    }

    // Extract parameters
    for _, param := range callback.Result. ResultParameters.ResultParameter {
        result.RawData[param.Key] = param.Value
        
        switch param.Key {
        case "TransactionAmount":
            if val, ok := param.Value.(float64); ok {
                result.Amount = val
            }
        case "TransactionReceipt":
            if val, ok := param.Value.(string); ok {
                result.ProviderTxID = val
            }
        case "ReceiverPartyPublicName":
            if val, ok := param.Value.(string); ok {
                result.PhoneNumber = val
            }
        }
    }

    return result, nil
}

// ============================================
// B2B (Business to Business)
// ============================================

// B2BRequest represents M-Pesa B2B request
type B2BRequest struct {
    Initiator              string `json:"Initiator"`
    SecurityCredential     string `json:"SecurityCredential"`
    CommandID              string `json:"CommandID"`
    Amount                 int    `json:"Amount"`
    PartyA                 string `json:"PartyA"`
    SenderIdentifierType   int    `json:"SenderIdentifierType"`
    PartyB                 string `json:"PartyB"`
    RecieverIdentifierType int    `json:"RecieverIdentifierType"` // ✅ Note:  Typo is intentional (Safaricom API uses "Reciever")
    AccountReference       string `json:"AccountReference"`
    Remarks                string `json:"Remarks"`
    QueueTimeOutURL        string `json:"QueueTimeOutURL"`
    ResultURL              string `json:"ResultURL"`
}

// B2BResponse represents M-Pesa B2B response
type B2BResponse struct {
    ConversationID           string `json:"ConversationID"`
    OriginatorConversationID string `json:"OriginatorConversationID"`
    ResponseCode             string `json:"ResponseCode"`
    ResponseDescription      string `json:"ResponseDescription"`
}

// InitiateB2B initiates M-Pesa B2B (Business to Business) payment
func (m *MpesaProvider) InitiateB2B(ctx context.Context, paybill, accountNumber string, amount float64, remarks string, resultURL, timeoutURL string) (*B2BResponse, error) {
    // Get access token (use B2B credentials)
    token, err := m.getAccessToken(ctx, "b2b")
    if err != nil {
        return nil, fmt.Errorf("failed to get access token:  %w", err)
    }

    if remarks == "" {
        remarks = "B2B Payment"
    }

    request := B2BRequest{
        Initiator:              m.config.B2BInitiatorName,
        SecurityCredential:      m.config.B2BSecurityCredential,
        CommandID:              "BusinessPayBill",
        Amount:                 int(amount),
        PartyA:                 m.config.B2BShortCode,
        SenderIdentifierType:   4, // 4 = Shortcode
        PartyB:                 paybill,
        RecieverIdentifierType: 4, // ✅ Typo is intentional
        AccountReference:       accountNumber,
        Remarks:                 remarks,
        QueueTimeOutURL:        timeoutURL,
        ResultURL:              resultURL,
    }

    // Make API call
    url := fmt. Sprintf("%s/mpesa/b2b/v1/paymentrequest", m.baseURL)
    respData, err := m.makeRequest(ctx, "POST", url, token, request)
    if err != nil {
        return nil, err
    }

    // Parse response
    var response B2BResponse
    responseBytes, _ := json.Marshal(respData)
    if err := json.Unmarshal(responseBytes, &response); err != nil {
        return nil, fmt. Errorf("failed to parse response: %w", err)
    }

    return &response, nil
}

// B2BCallbackRequest represents M-Pesa B2B callback (same structure as B2C)
type B2BCallbackRequest struct {
    Result struct {
        ResultType               int    `json:"ResultType"`
        ResultCode               int    `json:"ResultCode"`
        ResultDesc               string `json:"ResultDesc"`
        OriginatorConversationID string `json:"OriginatorConversationID"`
        ConversationID           string `json:"ConversationID"`
        TransactionID            string `json:"TransactionID"`
        ResultParameters         struct {
            ResultParameter []struct {
                Key   string      `json:"Key"`
                Value interface{} `json:"Value"`
            } `json:"ResultParameter"`
        } `json:"ResultParameters"`
    } `json:"Result"`
}

// ParseB2BCallback parses M-Pesa B2B callback
func (m *MpesaProvider) ParseB2BCallback(payload []byte) (*CallbackResult, error) {
    var callback B2BCallbackRequest
    if err := json.Unmarshal(payload, &callback); err != nil {
        return nil, fmt. Errorf("failed to parse callback: %w", err)
    }

    result := &CallbackResult{
        ConversationID:    callback.Result.ConversationID,
        TransactionID:     callback.Result.TransactionID,
        ProviderTxID:      callback.Result. TransactionID,
        ResultCode:        fmt.Sprintf("%d", callback.Result.ResultCode),
        ResultDescription: callback.Result.ResultDesc,
        Success:           callback.Result.ResultCode == 0,
        RawData:           make(map[string]interface{}),
    }

    // Extract parameters
    for _, param := range callback.Result.ResultParameters.ResultParameter {
        result. RawData[param.Key] = param.Value
        
        switch param.Key {
        case "TransactionAmount":
            if val, ok := param.Value.(float64); ok {
                result.Amount = val
            }
        case "TransactionReceipt":
            if val, ok := param.Value.(string); ok {
                result.ProviderTxID = val
            }
        case "InitiatorAccountCurrentBalance", "DebitAccountCurrentBalance":
            // Store balance info
            result.RawData[param.Key] = param.Value
        }
    }

    return result, nil
}

// ============================================
// SHARED TYPES & HELPERS
// ============================================

// CallbackResult represents parsed callback result (used by all callback types)
type CallbackResult struct {
    CheckoutRequestID string
    MerchantRequestID string
    ConversationID    string
    TransactionID     string
    ProviderTxID      string
    ResultCode        string
    ResultDescription string
    Success           bool
    Amount            float64
    PhoneNumber       string
    RawData           map[string]interface{}
}

// getAccessToken gets M-Pesa OAuth token
// apiType can be "stk", "b2c", or "b2b"
func (m *MpesaProvider) getAccessToken(ctx context.Context, apiType string) (string, error) {
    url := fmt.Sprintf("%s/oauth/v1/generate? grant_type=client_credentials", m.baseURL)
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return "", err
    }

    // Select appropriate credentials based on API type
    var consumerKey, consumerSecret string
    switch apiType {
    case "b2c":
        consumerKey = m.config.B2CConsumerKey
        consumerSecret = m.config.B2CConsumerSecret
    case "b2b":
        consumerKey = m.config.B2BConsumerKey
        consumerSecret = m.config.B2BConsumerSecret
    default:  // "stk" or default
        consumerKey = m. config.ConsumerKey
        consumerSecret = m.config.ConsumerSecret
    }

    // Basic auth with consumer key and secret
    auth := base64.StdEncoding.EncodeToString([]byte(
        consumerKey + ":" + consumerSecret,
    ))
    req.Header.Set("Authorization", "Basic "+auth)

    resp, err := m.httpClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("failed to get token: %s", string(body))
    }

    var result struct {
        AccessToken string `json:"access_token"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", err
    }

    return result.AccessToken, nil
}

// makeRequest makes HTTP request to M-Pesa API
func (m *MpesaProvider) makeRequest(ctx context.Context, method, url, token string, payload interface{}) (map[string]interface{}, error) {
    var body io.Reader
    if payload != nil {
        jsonData, err := json.Marshal(payload)
        if err != nil {
            return nil, err
        }
        body = bytes. NewBuffer(jsonData)
    }

    req, err := http.NewRequestWithContext(ctx, method, url, body)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+token)

    resp, err := m.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    responseBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    var result map[string]interface{}
    if err := json.Unmarshal(responseBody, &result); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    // Check for error in response
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API error: %v", result)
    }

    return result, nil
}