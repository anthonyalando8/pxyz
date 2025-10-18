// internal/domain/wallet.go
package domain
import (
    "time"
)

type Wallet struct {
    ID     string  `json:"id"`
    UserID  string  `json:"user_id"`
    Balance float64 `json:"balance"`
    Currency  string `json:"currency"`
    Available float64 `json:"available"`
    Locked    float64 `json:"locked"`
    Type      string `json:"type"` // e.g., "main", "savings"
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type WalletTransaction struct {
    ID        string `json:"id"`
    WalletID  string `json:"wallet_id"`
    UserID    string `json:"user_id"`
    Currency  string `json:"currency"`
    Amount    float64 `json:"amount"`
    TxStatus    string `json:"tx_status"` // e.g., "pending", "completed", "failed"
    TxType    string `json:"tx_type"` // e.g., "deposit", "withdrawal"
    Description string `json:"description"`
    RefID     *string `json:"ref_id,omitempty"` // Optional reference ID for external transactions
    CreatedAt time.Time `json:"created_at"`
}
