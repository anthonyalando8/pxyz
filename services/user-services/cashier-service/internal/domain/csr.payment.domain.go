// domain/payment.go
package domain
import "errors"



var (
    ErrProviderNotFound = errors.New("payment provider not found")
    ErrInvalidRequest   = errors.New("invalid request")
    ErrProcessingFailed = errors.New("payment processing failed")
)


type DepositRequest struct {
    Provider   string `json:"provider"` // e.g., "mpesa"
    UserID     string `json:"user_id"`
    Phone      string `json:"phone"`
    Amount     float64 `json:"amount"`
    AccountRef string `json:"account_ref"`
}

type DepositResponse struct {
    TransactionID string
    Status        string
    ProviderRef   string
}

type WithdrawRequest struct {
    Provider   string `json:"provider"` // e.g., "mpesa"
    UserID     string `json:"user_id"`
    Phone      string `json:"phone"`
    Amount     float64 `json:"amount"`
    AccountRef string `json:"account_ref"`
}

type WithdrawResponse struct {
    TransactionID string
    Status        string
    ProviderRef   string
}
