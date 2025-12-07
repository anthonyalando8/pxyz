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
	ChannelDepositCompleted = "partner:deposit:completed"
	ChannelDepositFailed    = "partner:deposit:failed"
	ChannelTransactionEvent = "partner:transaction:event"
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
		p.logger.Error("failed to marshal deposit completed event",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt. Errorf("failed to marshal event: %w", err)
	}

	// Publish to specific channel
	if err := p. rdb. Publish(ctx, ChannelDepositCompleted, payload).Err(); err != nil {
		p.logger.Error("failed to publish deposit completed event",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt.Errorf("failed to publish event: %w", err)
	}

	// Also publish to general transaction event channel
	if err := p. rdb.Publish(ctx, ChannelTransactionEvent, payload).Err(); err != nil {
		p.logger. Warn("failed to publish to general channel",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
	}

	p.logger.Info("deposit completed event published",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID),
		zap.Float64("amount", event.Amount),
		zap.String("currency", event.Currency))

	return nil
}

// PublishDepositFailed publishes a deposit failed event
func (p *EventPublisher) PublishDepositFailed(ctx context.Context, event *DepositFailedEvent) error {
	event. EventType = "deposit.failed"
	event.Timestamp = time.Now().Unix()

	payload, err := json.Marshal(event)
	if err != nil {
		p.logger.Error("failed to marshal deposit failed event",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish to specific channel
	if err := p.rdb.Publish(ctx, ChannelDepositFailed, payload). Err(); err != nil {
		p.logger.Error("failed to publish deposit failed event",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt.Errorf("failed to publish event: %w", err)
	}

	// Also publish to general transaction event channel
	if err := p.rdb.Publish(ctx, ChannelTransactionEvent, payload). Err(); err != nil {
		p.logger.Warn("failed to publish to general channel",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
	}

	p. logger.Info("deposit failed event published",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID),
		zap.String("error", event.ErrorMessage))

	return nil
}