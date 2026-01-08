// internal/events/publisher.go
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// Channel names
	ChannelDepositCompleted    = "partner:deposit:completed"
	ChannelDepositFailed       = "partner:deposit: failed"
	ChannelWithdrawalCompleted = "partner:withdrawal:completed"
	ChannelWithdrawalFailed    = "partner:withdrawal:failed"
	ChannelTransactionEvent    = "partner:transaction: event"
)

type EventPublisher struct {
	rdb    *redis.Client
	logger *zap.Logger
}

func NewEventPublisher(rdb *redis.Client, logger *zap.Logger) *EventPublisher {
	return &EventPublisher{
		rdb:    rdb,
		logger: logger,
	}
}

// ============================================
// DEPOSIT EVENTS
// ============================================

// DepositCompletedEvent represents a completed deposit event
type DepositCompletedEvent struct {
	EventType      string                 `json:"event_type"`
	TransactionRef string                 `json:"transaction_ref"`
	TransactionID  int64                  `json:"transaction_id"`
	PartnerID      string                 `json:"partner_id"`
	UserID         string                 `json:"user_id"`
	Amount         float64                `json:"amount"`
	Currency       string                 `json:"currency"`
	ReceiptCode    string                 `json:"receipt_code"`
	JournalID      int64                  `json:"journal_id"`
	FeeAmount      float64                `json:"fee_amount"`
	UserBalance    float64                `json:"user_balance"`
	PaymentMethod  string                 `json:"payment_method,omitempty"`
	ExternalRef    string                 `json:"external_ref,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CompletedAt    time.Time              `json:"completed_at"`
	Timestamp      int64                  `json:"timestamp"`
}

// DepositFailedEvent represents a failed deposit event
type DepositFailedEvent struct {
	EventType      string    `json:"event_type"`
	TransactionRef string    `json:"transaction_ref"`
	TransactionID  int64     `json:"transaction_id"`
	PartnerID      string    `json:"partner_id"`
	UserID         string    `json:"user_id"`
	Amount         float64   `json:"amount"`
	Currency       string    `json:"currency"`
	ErrorMessage   string    `json:"error_message"`
	FailedAt       time.Time `json:"failed_at"`
	Timestamp      int64     `json:"timestamp"`
}

// PublishDepositCompleted publishes a deposit completed event
func (p *EventPublisher) PublishDepositCompleted(ctx context.Context, event *DepositCompletedEvent) error {
	event.EventType = "deposit.completed"
	event. Timestamp = time.Now().Unix()

	payload, err := json.Marshal(event)
	if err != nil {
		p. logger.Error("failed to marshal deposit completed event",
			zap. String("transaction_ref", event. TransactionRef),
			zap.Error(err))
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish to specific channel
	if err := p.rdb. Publish(ctx, ChannelDepositCompleted, payload).Err(); err != nil {
		p.logger.Error("failed to publish deposit completed event",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt.Errorf("failed to publish event: %w", err)
	}

	// Also publish to general transaction event channel
	if err := p.rdb.Publish(ctx, ChannelTransactionEvent, payload).Err(); err != nil {
		p.logger.Warn("failed to publish to general channel",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
	}

	p.logger.Info("deposit completed event published",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID),
		zap.Float64("amount", event.Amount),
		zap.String("currency", event.Currency),
		zap.String("receipt_code", event.ReceiptCode))

	return nil
}

// PublishDepositFailed publishes a deposit failed event
func (p *EventPublisher) PublishDepositFailed(ctx context.Context, event *DepositFailedEvent) error {
	event.EventType = "deposit. failed"
	event.Timestamp = time.Now().Unix()

	payload, err := json.Marshal(event)
	if err != nil {
		p.logger. Error("failed to marshal deposit failed event",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish to specific channel
	if err := p. rdb.Publish(ctx, ChannelDepositFailed, payload).Err(); err != nil {
		p.logger.Error("failed to publish deposit failed event",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt.Errorf("failed to publish event:  %w", err)
	}

	// Also publish to general transaction event channel
	if err := p.rdb.Publish(ctx, ChannelTransactionEvent, payload).Err(); err != nil {
		p.logger. Warn("failed to publish to general channel",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
	}

	p.logger.Info("deposit failed event published",
		zap. String("transaction_ref", event. TransactionRef),
		zap.String("user_id", event.UserID),
		zap.String("error", event.ErrorMessage))

	return nil
}

// ============================================
// WITHDRAWAL EVENTS
// ============================================

// WithdrawalCompletedEvent represents a completed withdrawal event
type WithdrawalCompletedEvent struct {
	EventType      string                 `json:"event_type"`
	TransactionRef string                 `json:"transaction_ref"`
	TransactionID  int64                  `json:"transaction_id"`
	PartnerID      string                 `json:"partner_id"`
	UserID         string                 `json:"user_id"`
	Amount         float64                `json:"amount"`
	Currency       string                 `json:"currency"`
	ExternalRef    string                 `json:"external_ref"` // M-Pesa code or bank reference
	PaymentMethod  string                 `json:"payment_method,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CompletedAt    time.Time              `json:"completed_at"`
	Timestamp      int64                  `json:"timestamp"`
}

// WithdrawalFailedEvent represents a failed withdrawal event
type WithdrawalFailedEvent struct {
	EventType      string    `json:"event_type"`
	TransactionRef string    `json:"transaction_ref"`
	TransactionID  int64     `json:"transaction_id"`
	PartnerID      string    `json:"partner_id"`
	UserID         string    `json:"user_id"`
	Amount         float64   `json:"amount"`
	Currency       string    `json:"currency"`
	ErrorMessage   string    `json:"error_message"`
	FailedAt       time.Time `json:"failed_at"`
	Timestamp      int64     `json:"timestamp"`
}

// PublishWithdrawalCompleted publishes a withdrawal completed event
func (p *EventPublisher) PublishWithdrawalCompleted(ctx context.Context, event *WithdrawalCompletedEvent) error {
	event.EventType = "withdrawal.completed"
	event.Timestamp = time.Now().Unix()

	payload, err := json.Marshal(event)
	if err != nil {
		p.logger.Error("failed to marshal withdrawal completed event",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish to specific channel
	if err := p.rdb. Publish(ctx, ChannelWithdrawalCompleted, payload).Err(); err != nil {
		p.logger.Error("failed to publish withdrawal completed event",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt.Errorf("failed to publish event: %w", err)
	}

	// Also publish to general transaction event channel
	if err := p.rdb.Publish(ctx, ChannelTransactionEvent, payload).Err(); err != nil {
		p.logger.Warn("failed to publish to general channel",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
	}

	p.logger.Info("withdrawal completed event published",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID),
		zap.Float64("amount", event.Amount),
		zap.String("currency", event.Currency),
		zap.String("external_ref", event.ExternalRef))

	return nil
}

// PublishWithdrawalFailed publishes a withdrawal failed event
func (p *EventPublisher) PublishWithdrawalFailed(ctx context.Context, event *WithdrawalFailedEvent) error {
	event.EventType = "withdrawal.failed"
	event. Timestamp = time.Now().Unix()

	payload, err := json. Marshal(event)
	if err != nil {
		p.logger.Error("failed to marshal withdrawal failed event",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish to specific channel
	if err := p.rdb.Publish(ctx, ChannelWithdrawalFailed, payload).Err(); err != nil {
		p. logger.Error("failed to publish withdrawal failed event",
			zap. String("transaction_ref", event. TransactionRef),
			zap.Error(err))
		return fmt.Errorf("failed to publish event: %w", err)
	}

	// Also publish to general transaction event channel
	if err := p.rdb. Publish(ctx, ChannelTransactionEvent, payload).Err(); err != nil {
		p. logger.Warn("failed to publish to general channel",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
	}

	p.logger. Warn("withdrawal failed event published",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID),
		zap.String("error", event.ErrorMessage))

	return nil
}

// ============================================
// GENERIC TRANSACTION EVENT
// ============================================

// TransactionEvent represents a generic transaction event
type TransactionEvent struct {
	EventType      string                 `json:"event_type"`
	TransactionRef string                 `json:"transaction_ref"`
	TransactionID  int64                  `json:"transaction_id"`
	PartnerID      string                 `json:"partner_id"`
	UserID         string                 `json:"user_id"`
	Amount         float64                `json:"amount"`
	Currency       string                 `json:"currency"`
	Status         string                 `json:"status"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Timestamp      int64                  `json:"timestamp"`
}

// PublishTransactionEvent publishes a generic transaction event
func (p *EventPublisher) PublishTransactionEvent(ctx context.Context, event *TransactionEvent) error {
	event. Timestamp = time.Now().Unix()

	payload, err := json. Marshal(event)
	if err != nil {
		p.logger.Error("failed to marshal transaction event",
			zap.String("transaction_ref", event.TransactionRef),
			zap.String("event_type", event.EventType),
			zap.Error(err))
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := p.rdb. Publish(ctx, ChannelTransactionEvent, payload).Err(); err != nil {
		p. logger.Error("failed to publish transaction event",
			zap.String("transaction_ref", event.TransactionRef),
			zap.String("event_type", event. EventType),
			zap.Error(err))
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger. Info("transaction event published",
		zap.String("event_type", event.EventType),
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("status", event.Status))

	return nil
}