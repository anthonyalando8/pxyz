package domain

import "time"

// Receipt represents a transaction receipt
type Receipt struct {
    ID           int64     `json:"id"`
    Code         string    `json:"code"`          // e.g., TIJ5LW4VDT
    JournalID    int64     `json:"journal_id"`
    AccountID    int64     `json:"account_id"`
    AccountType  string    `json:"account_type"`  // system, partner, user
    Type         string    `json:"type"`          // deposit, withdrawal, admin_credit
    Amount       float64   `json:"amount"`
    Currency     string    `json:"currency"`
    Status       string    `json:"status"`        // pending, success, failed
    CreatedAt    time.Time `json:"created_at"`
}
