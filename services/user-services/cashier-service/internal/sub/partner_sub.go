// cashier-service/internal/events/subscriber. go
package subscriber

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// Deposit channels
	ChannelDepositCompleted = "partner:deposit:completed"
	ChannelDepositFailed    = "partner:deposit:failed"
	
	// Withdrawal channels
	ChannelWithdrawalCompleted = "partner:withdrawal:completed"
	ChannelWithdrawalFailed    = "partner: withdrawal:failed"
	
	// General transaction events
	ChannelTransactionEvent = "partner:transaction:event"
)

type EventSubscriber struct {
	rdb     *redis.Client
	logger  *zap.Logger
	handler EventHandler
}

type EventHandler interface {
	// Deposit handlers
	HandleDepositCompleted(ctx context.Context, event *DepositCompletedEvent) error
	HandleDepositFailed(ctx context.Context, event *DepositFailedEvent) error
	
	// Withdrawal handlers
	HandleWithdrawalCompleted(ctx context.Context, event *WithdrawalCompletedEvent) error
	HandleWithdrawalFailed(ctx context.Context, event *WithdrawalFailedEvent) error
}

// ============================================
// DEPOSIT EVENTS
// ============================================

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

// ============================================
// WITHDRAWAL EVENTS
// ============================================

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
	// ReceiptCode    string                 `json:"receipt_code"`
    // JournalID      int64                  `json:"journal_id"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CompletedAt    time.Time              `json:"completed_at"`
	Timestamp      int64                  `json:"timestamp"`
}

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

// ============================================
// SUBSCRIBER IMPLEMENTATION
// ============================================

func NewEventSubscriber(rdb *redis.Client, logger *zap.Logger, handler EventHandler) *EventSubscriber {
	return &EventSubscriber{
		rdb:     rdb,
		logger:  logger,
		handler:  handler,
	}
}

// Start begins listening for events
func (s *EventSubscriber) Start(ctx context.Context) error {
	// Subscribe to all channels
	channels := []string{
		ChannelDepositCompleted,
		ChannelDepositFailed,
		ChannelWithdrawalCompleted,
		ChannelWithdrawalFailed,
	}

	pubsub := s.rdb. Subscribe(ctx, channels...)
	defer pubsub.Close()

	// Wait for confirmation that subscription is created
	_, err := pubsub.Receive(ctx)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	s.logger.Info("event subscriber started",
		zap. Strings("channels", channels))

	// Start consuming messages
	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("event subscriber stopping")
			return ctx.Err()

		case msg := <-ch:
			s.handleMessage(context.Background(), msg)
		}
	}
}

// handleMessage processes incoming Redis messages
func (s *EventSubscriber) handleMessage(ctx context. Context, msg *redis.Message) {
	s.logger.Debug("received event",
		zap.String("channel", msg.Channel),
		zap.String("payload", msg.Payload))

	switch msg.Channel {
	case ChannelDepositCompleted:
		s.handleDepositCompleted(ctx, []byte(msg.Payload))

	case ChannelDepositFailed: 
		s.handleDepositFailed(ctx, []byte(msg.Payload))

	case ChannelWithdrawalCompleted:
		s.handleWithdrawalCompleted(ctx, []byte(msg.Payload))

	case ChannelWithdrawalFailed:
		s. handleWithdrawalFailed(ctx, []byte(msg. Payload))

	default:
		s.logger.Warn("received message on unknown channel",
			zap.String("channel", msg. Channel))
	}
}

// ============================================
// DEPOSIT EVENT HANDLERS
// ============================================

// handleDepositCompleted processes deposit completed events
func (s *EventSubscriber) handleDepositCompleted(ctx context.Context, payload []byte) {
	var event DepositCompletedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		s.logger.Error("failed to unmarshal deposit completed event",
			zap.Error(err),
			zap.String("payload", string(payload)))
		return
	}

	s.logger.Info("processing deposit completed event",
		zap. String("transaction_ref", event. TransactionRef),
		zap.String("user_id", event.UserID),
		zap.Float64("amount", event. Amount),
		zap.String("currency", event.Currency))

	// Process event with timeout
	processCtx, cancel := context. WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.handler.HandleDepositCompleted(processCtx, &event); err != nil {
		s.logger.Error("failed to handle deposit completed event",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		// TODO: Implement retry mechanism or dead letter queue
		return
	}

	s.logger.Info("deposit completed event processed successfully",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID))
}

// handleDepositFailed processes deposit failed events
func (s *EventSubscriber) handleDepositFailed(ctx context.Context, payload []byte) {
	var event DepositFailedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		s. logger.Error("failed to unmarshal deposit failed event",
			zap.Error(err),
			zap.String("payload", string(payload)))
		return
	}

	s.logger.Info("processing deposit failed event",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID),
		zap.String("error", event.ErrorMessage))

	processCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.handler.HandleDepositFailed(processCtx, &event); err != nil {
		s.logger. Error("failed to handle deposit failed event",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return
	}

	s.logger.Info("deposit failed event processed successfully",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID))
}

// ============================================
// WITHDRAWAL EVENT HANDLERS
// ============================================

// handleWithdrawalCompleted processes withdrawal completed events
func (s *EventSubscriber) handleWithdrawalCompleted(ctx context.Context, payload []byte) {
	var event WithdrawalCompletedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		s. logger.Error("failed to unmarshal withdrawal completed event",
			zap.Error(err),
			zap.String("payload", string(payload)))
		return
	}

	s.logger.Info("processing withdrawal completed event",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID),
		zap.Float64("amount", event.Amount),
		zap.String("currency", event. Currency),
		zap.String("external_ref", event.ExternalRef))

	// Process event with timeout
	processCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.handler.HandleWithdrawalCompleted(processCtx, &event); err != nil {
		s.logger.Error("failed to handle withdrawal completed event",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		// TODO: Implement retry mechanism or dead letter queue
		return
	}

	s.logger.Info("withdrawal completed event processed successfully",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID),
		zap.String("external_ref", event.ExternalRef))
}

// handleWithdrawalFailed processes withdrawal failed events
func (s *EventSubscriber) handleWithdrawalFailed(ctx context.Context, payload []byte) {
	var event WithdrawalFailedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		s.logger.Error("failed to unmarshal withdrawal failed event",
			zap.Error(err),
			zap.String("payload", string(payload)))
		return
	}

	s.logger.Info("processing withdrawal failed event",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID),
		zap.String("error", event.ErrorMessage))

	processCtx, cancel := context.WithTimeout(ctx, 30*time. Second)
	defer cancel()

	if err := s.handler. HandleWithdrawalFailed(processCtx, &event); err != nil {
		s.logger.Error("failed to handle withdrawal failed event",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return
	}

	s.logger. Info("withdrawal failed event processed successfully",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID))
}