// internal/domain/crypto_deposit.go
package domain

import (
	"math/big"
	"time"
)

// DepositStatus represents deposit processing status
type DepositStatus string

const (
	DepositStatusDetected  DepositStatus = "detected"
	DepositStatusPending   DepositStatus = "pending"
	DepositStatusConfirmed DepositStatus = "confirmed"
	DepositStatusCredited  DepositStatus = "credited"
	DepositStatusFailed    DepositStatus = "failed"
)

// CryptoDeposit represents an incoming deposit
type CryptoDeposit struct {
	ID                     int64
	DepositID              string  // UUID
	WalletID               int64
	UserID                 string
	
	// Deposit details
	Chain                  string
	Asset                  string
	FromAddress            string
	ToAddress              string
	Amount                 *big.Int
	
	// Blockchain
	TxHash                 string
	BlockNumber            int64
	BlockTimestamp         *time.Time
	Confirmations          int
	RequiredConfirmations  int
	
	// Status
	Status                 DepositStatus
	TransactionID          *int64
	
	// Notifications
	UserNotified           bool
	NotifiedAt             *time.Time
	NotificationSent       bool
	
	// Timestamps
	DetectedAt             time.Time
	ConfirmedAt            *time.Time
	CreditedAt             *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// DepositNotification for user alerts
type DepositNotification struct {
	UserID        string
	DepositID     string
	Chain         string
	Asset         string
	Amount        string  // Formatted
	TxHash        string
	Confirmations int
	Required      int
	Status        DepositStatus
	DetectedAt    time.Time
}