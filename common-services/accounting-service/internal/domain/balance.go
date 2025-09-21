package domain

import "time"

// Balance represents current balance of an account
type Balance struct {
    AccountID int64     `json:"account_id"`
	AccountNumber string `json:"account_number"` // new field

    Balance   float64   `json:"balance"`
    UpdatedAt time.Time `json:"updated_at"`
}
