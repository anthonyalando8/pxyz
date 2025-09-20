package domain

import "time"

// Balance represents current balance of an account
type Balance struct {
    AccountID int64     `json:"account_id"`
    Balance   float64   `json:"balance"`
    UpdatedAt time.Time `json:"updated_at"`
}
