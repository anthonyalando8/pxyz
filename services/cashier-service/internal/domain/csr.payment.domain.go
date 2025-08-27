// domain/payment.go
package domain
import "errors"



var (
    ErrProviderNotFound = errors.New("payment provider not found")
    ErrInvalidRequest   = errors.New("invalid request")
    ErrProcessingFailed = errors.New("payment processing failed")
)


type DepositRequest struct {
    Provider   string
    UserID     string
    Phone      string
    Amount     float64
    AccountRef string
}

type DepositResponse struct {
    TransactionID string
    Status        string
    ProviderRef   string
}

type WithdrawRequest struct {
    Provider   string
    UserID     string
    Phone      string
    Amount     float64
    AccountRef string
}

type WithdrawResponse struct {
    TransactionID string
    Status        string
    ProviderRef   string
}
