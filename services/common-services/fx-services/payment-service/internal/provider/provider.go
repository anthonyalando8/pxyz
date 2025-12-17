// internal/provider/provider.go
package provider

import (
	"context"
	"payment-service/internal/domain"
	"time"
)

// PaymentProvider defines the interface all payment providers must implement
type PaymentProvider interface {
    // GetName returns the provider name
    GetName() string
    
    // InitiateDeposit initiates a deposit transaction
    InitiateDeposit(ctx context.Context, req *domain.DepositRequest) (*ProviderResponse, error)
    
    // InitiateWithdrawal initiates a withdrawal transaction
    //InitiateWithdrawal(ctx context. Context, req *domain.WithdrawalRequest) (*ProviderResponse, error)
    
    // ProcessCallback processes provider callback
    ProcessCallback(ctx context.Context, payload []byte) (*CallbackResult, error)
    
    // QueryTransaction queries transaction status from provider
    QueryTransaction(ctx context.Context, providerTxID string) (*TransactionStatus, error)
}

type ProviderResponse struct {
    Success           bool
    ProviderTxID      string
    CheckoutRequestID string  // For M-Pesa STK
    Message           string
    RawResponse       map[string]interface{}
}

type CallbackResult struct {
    PaymentRef        string
    ProviderTxID      string
    CheckoutRequestID string
    Success           bool
    ResultCode        string
    ResultDescription string
    Amount            float64
    PhoneNumber       string
    RawData           map[string]interface{}
}

type TransactionStatus struct {
    ProviderTxID      string
    Status            string
    ResultCode        string
    ResultDescription string
    Amount            float64
    CompletedAt       *time.Time
}