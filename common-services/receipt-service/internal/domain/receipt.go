package domain

import "time"

// Receipt represents a transaction receipt
type PartyInfo struct {
	ID            string `json:"id"`
	Type          string `json:"type"`           // user, partner, system
	Name          string `json:"name,omitempty"`
	Phone         string `json:"phone,omitempty"`
	Email         string `json:"email,omitempty"`
	AccountNumber string `json:"account_number,omitempty"` // optional: account identifier
	IsCreditor    bool   `json:"is_creditor"`               // true if this party is creditor
}

type Receipt struct {
	ID           int64     `json:"id"`
	Code         string    `json:"code"`          // e.g., TIJ5LW4VDT
	JournalID    int64     `json:"journal_id"`
	Type         string    `json:"type"`          // deposit, withdrawal, admin_credit
	Amount       float64   `json:"amount"`
	Currency     string    `json:"currency"`
	Status       string    `json:"status"`        // pending, success, failed
	CreatedAt    time.Time `json:"created_at"`

	// Additional info (not stored in DB)
	Creditor PartyInfo `json:"creditor,omitempty"`
	Debitor  PartyInfo `json:"debitor,omitempty"`

	CodedType   string `json:"coded_type"`
	ExternalRef string `json:"external_ref"`

	// Optional payload for notifications or metadata
	Payload map[string]interface{} `json:"payload,omitempty"`
}


