// accounting-service/internal/publisher/transaction_publisher.go
package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	TransactionEventsChannel = "transaction_events"
)

type TransactionEventPublisher struct {
	rdb *redis.Client
}

func NewTransactionEventPublisher(rdb *redis.Client) *TransactionEventPublisher {
	return &TransactionEventPublisher{rdb: rdb}
}

type TransactionEvent struct {
	EventType       string                 `json:"event_type"` // transaction. created, transaction.completed, transaction.failed
	UserID          string                 `json:"user_id"`
	ReceiptCode     string                 `json:"receipt_code"`
	TransactionID   int64                  `json:"transaction_id"`
	TransactionType string                 `json:"transaction_type"` // deposit, withdrawal, transfer, etc
	Status          string                 `json:"status"`
	Amount          float64                `json:"amount"` // Atomic units
	Currency        string                 `json:"currency"`
	AccountNumber   string                 `json:"account_number,omitempty"`
	FromAccount     string                 `json:"from_account,omitempty"`
	ToAccount       string                 `json:"to_account,omitempty"`
	BalanceAfter    float64                `json:"balance_after,omitempty"`
	Fee             float64                `json:"fee,omitempty"`
	ErrorMessage    string                 `json:"error_message,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	Timestamp       time.Time              `json:"timestamp"`
}

// PublishTransactionEvent publishes a transaction event to Redis
func (p *TransactionEventPublisher) PublishTransactionEvent(ctx context.Context, event *TransactionEvent) error {
	event.Timestamp = time.Now()

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := p.rdb.Publish(ctx, TransactionEventsChannel, payload).Err(); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	log.Printf("[TransactionEvent] Published: %s for user=%s, receipt=%s",
		event.EventType, event.UserID, event.ReceiptCode)

	return nil
}

// PublishTransactionCompleted publishes a successful transaction completion
func (p *TransactionEventPublisher) PublishTransactionCompleted(ctx context.Context, userID, receiptCode string, transactionID int64, transactionType, currency string, amount, balanceAfter, fee float64) error {
	return p.PublishTransactionEvent(ctx, &TransactionEvent{
		EventType:       "transaction.completed",
		UserID:          userID,
		ReceiptCode:     receiptCode,
		TransactionID:   transactionID,
		TransactionType: transactionType,
		Status:          "completed",
		Amount:          amount,
		Currency:        currency,
		BalanceAfter:    balanceAfter,
		Fee:             fee,
	})
}

// PublishTransactionFailed publishes a failed transaction
func (p *TransactionEventPublisher) PublishTransactionFailed(ctx context.Context, userID, receiptCode string, transactionID int64, transactionType, currency string, amount float64, errorMsg string) error {
	return p.PublishTransactionEvent(ctx, &TransactionEvent{
		EventType:       "transaction.failed",
		UserID:          userID,
		ReceiptCode:     receiptCode,
		TransactionID:   transactionID,
		TransactionType: transactionType,
		Status:          "failed",
		Amount:          amount,
		Currency:        currency,
		ErrorMessage:    errorMsg,
	})
}

// PublishDepositCompleted publishes a deposit completion
func (p *TransactionEventPublisher) PublishDepositCompleted(ctx context.Context, userID, receiptCode, accountNumber string, amount float64, currency string, balanceAfter float64) error {
	return p.PublishTransactionEvent(ctx, &TransactionEvent{
		EventType:       "deposit.completed",
		UserID:          userID,
		ReceiptCode:     receiptCode,
		TransactionType: "deposit",
		Status:          "completed",
		Amount:          amount,
		Currency:        currency,
		AccountNumber:   accountNumber,
		BalanceAfter:    balanceAfter,
	})
}

// PublishWithdrawalCompleted publishes a withdrawal completion
func (p *TransactionEventPublisher) PublishWithdrawalCompleted(ctx context.Context, userID, receiptCode, accountNumber string, amount float64, currency string, balanceAfter float64) error {
	return p.PublishTransactionEvent(ctx, &TransactionEvent{
		EventType:       "withdrawal.completed",
		UserID:          userID,
		ReceiptCode:     receiptCode,
		TransactionType: "withdrawal",
		Status:          "completed",
		Amount:          amount,
		Currency:        currency,
		AccountNumber:   accountNumber,
		BalanceAfter:    balanceAfter,
	})
}

// PublishTransferCompleted publishes a transfer completion
func (p *TransactionEventPublisher) PublishTransferCompleted(ctx context.Context, userID, receiptCode, fromAccount, toAccount string, amount float64, currency string, fee float64) error {
	return p.PublishTransactionEvent(ctx, &TransactionEvent{
		EventType:       "transfer.completed",
		UserID:          userID,
		ReceiptCode:     receiptCode,
		TransactionType: "transfer",
		Status:          "completed",
		Amount:          amount,
		Currency:        currency,
		FromAccount:     fromAccount,
		ToAccount:       toAccount,
		Fee:             fee,
	})
}
