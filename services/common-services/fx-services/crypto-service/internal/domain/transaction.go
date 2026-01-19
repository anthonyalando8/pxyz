// internal/domain/crypto_transaction.go
package domain

import (
	"math/big"
	"time"
)

// TransactionType represents the type of crypto transaction
type TransactionType string

const (
	TransactionTypeDeposit          TransactionType = "deposit"
	TransactionTypeWithdrawal       TransactionType = "withdrawal"
	TransactionTypeInternalTransfer TransactionType = "internal_transfer"
	TransactionTypeConversion       TransactionType = "conversion"
	TransactionTypeFeePayment       TransactionType = "fee_payment"
)

// TransactionStatus represents transaction status
type TransactionStatus string

const (
	TransactionStatusPending      TransactionStatus = "pending"
	TransactionStatusBroadcasting TransactionStatus = "broadcasting"
	TransactionStatusBroadcasted  TransactionStatus = "broadcasted"
	TransactionStatusConfirming   TransactionStatus = "confirming"
	TransactionStatusConfirmed    TransactionStatus = "confirmed"
	TransactionStatusCompleted    TransactionStatus = "completed"
	TransactionStatusFailed       TransactionStatus = "failed"
	TransactionStatusCancelled    TransactionStatus = "cancelled"
)

// CryptoTransaction represents a cryptocurrency transaction
type CryptoTransaction struct {
	ID            int64
	TransactionID string  // UUID
	UserID        string
	
	// Classification
	Type          TransactionType
	Chain         string
	Asset         string
	
	// Addresses
	FromWalletID  *int64
	FromAddress   string
	ToWalletID    *int64
	ToAddress     string
	IsInternal    bool
	
	// Amounts (smallest unit)
	Amount        *big.Int
	
	// Fees
	NetworkFee         *big.Int
	NetworkFeeCurrency *string
	PlatformFee        *big.Int
	PlatformFeeCurrency *string
	TotalFee           *big.Int
	
	// Blockchain details
	TxHash                 *string
	BlockNumber            *int64
	BlockTimestamp         *time.Time
	Confirmations          int
	RequiredConfirmations  int
	
	// Chain-specific
	GasUsed        *int64
	GasPrice       *big.Int
	EnergyUsed     *int64  // TRON
	BandwidthUsed  *int64  // TRON
	
	// Status
	Status         TransactionStatus
	StatusMessage  *string
	
	// Accounting link
	AccountingTxID *string
	
	// Metadata
	Memo           *string
	Metadata       map[string]interface{}
	
	// Timestamps
	InitiatedAt    time.Time
	BroadcastedAt  *time.Time
	ConfirmedAt    *time.Time
	CompletedAt    *time.Time
	FailedAt       *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// TransactionSummary for list views
type TransactionSummary struct {
	ID                string
	Type              TransactionType
	Chain             string
	Asset             string
	Amount            string  // Formatted
	Fee               string  // Formatted
	Status            TransactionStatus
	TxHash            *string
	IsInternal        bool
	CreatedAt         time.Time
	ConfirmedAt       *time.Time
}