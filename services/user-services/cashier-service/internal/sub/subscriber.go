// cashier-service/internal/subscriber/transaction_subscriber. go
package subscriber

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"cashier-service/internal/handler"

	"github.com/redis/go-redis/v9"
)

const (
	TransactionEventsChannel = "transaction_events"
)

type TransactionEventSubscriber struct {
	rdb    *redis.Client
	hub    *handler.Hub
	pubsub *redis.PubSub
}

func NewTransactionEventSubscriber(rdb *redis.Client, hub *handler.Hub) *TransactionEventSubscriber {
	return &TransactionEventSubscriber{
		rdb: rdb,
		hub: hub,
	}
}

type TransactionEvent struct {
	EventType       string                 `json:"event_type"`
	UserID          string                 `json:"user_id"`
	ReceiptCode     string                 `json:"receipt_code"`
	TransactionID   int64                  `json:"transaction_id"`
	TransactionType string                 `json:"transaction_type"`
	Status          string                 `json:"status"`
	Amount          float64                `json:"amount"`
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

// Start subscribes to transaction events and forwards them to WebSocket clients
func (s *TransactionEventSubscriber) Start(ctx context.Context) error {
	s.pubsub = s.rdb.Subscribe(ctx, TransactionEventsChannel)

	// Wait for confirmation that subscription is created
	_, err := s.pubsub.Receive(ctx)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	log.Printf("[TransactionSubscriber]  Subscribed to channel: %s", TransactionEventsChannel)

	// Start listening for messages
	go s.listen(ctx)

	return nil
}

// listen processes incoming Redis messages
func (s *TransactionEventSubscriber) listen(ctx context.Context) {
	ch := s.pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			log.Println("[TransactionSubscriber] Stopping subscriber...")
			s.pubsub.Close()
			return

		case msg := <-ch:
			if msg == nil {
				continue
			}

			// Parse event
			var event TransactionEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("[TransactionSubscriber] Failed to parse event: %v", err)
				continue
			}

			// Process event
			s.processEvent(&event)
		}
	}
}

// processEvent handles the event and sends it to the appropriate WebSocket client
func (s *TransactionEventSubscriber) processEvent(event *TransactionEvent) {
	log.Printf("[TransactionSubscriber] Processing event: %s for user=%s", event.EventType, event.UserID)

	// Format amount from atomic units to decimal
	amount := float64(event.Amount) / 100.0
	balanceAfter := float64(event.BalanceAfter) / 100.0
	fee := float64(event.Fee) / 100.0

	// Create WebSocket message based on event type
	var wsMessage map[string]interface{}

	switch event.EventType {
	case "deposit.completed":
		wsMessage = map[string]interface{}{
			"type": "deposit_completed",
			"data": map[string]interface{}{
				"receipt_code":   event.ReceiptCode,
				"transaction_id": event.TransactionID,
				"amount":         amount,
				"currency":       event.Currency,
				"account_number": event.AccountNumber,
				"balance_after":  balanceAfter,
				"timestamp":      event.Timestamp.Unix(),
			},
		}

	case "withdrawal.completed":
		wsMessage = map[string]interface{}{
			"type": "withdrawal_completed",
			"data": map[string]interface{}{
				"receipt_code":   event.ReceiptCode,
				"transaction_id": event.TransactionID,
				"amount":         amount,
				"currency":       event.Currency,
				"account_number": event.AccountNumber,
				"balance_after":  balanceAfter,
				"timestamp":      event.Timestamp.Unix(),
			},
		}

	case "transfer.completed":
		wsMessage = map[string]interface{}{
			"type": "transfer_completed",
			"data": map[string]interface{}{
				"receipt_code":   event.ReceiptCode,
				"transaction_id": event.TransactionID,
				"amount":         amount,
				"currency":       event.Currency,
				"from_account":   event.FromAccount,
				"to_account":     event.ToAccount,
				"fee":            fee,
				"timestamp":      event.Timestamp.Unix(),
			},
		}

	case "transaction.completed":
		wsMessage = map[string]interface{}{
			"type": "transaction_completed",
			"data": map[string]interface{}{
				"receipt_code":     event.ReceiptCode,
				"transaction_id":   event.TransactionID,
				"transaction_type": event.TransactionType,
				"amount":           amount,
				"currency":         event.Currency,
				"balance_after":    balanceAfter,
				"fee":              fee,
				"timestamp":        event.Timestamp.Unix(),
			},
		}

	case "transaction.failed":
		wsMessage = map[string]interface{}{
			"type": "transaction_failed",
			"data": map[string]interface{}{
				"receipt_code":     event.ReceiptCode,
				"transaction_id":   event.TransactionID,
				"transaction_type": event.TransactionType,
				"amount":           amount,
				"currency":         event.Currency,
				"error":            event.ErrorMessage,
				"timestamp":        event.Timestamp.Unix(),
			},
		}

	default:
		log.Printf("[TransactionSubscriber] Unknown event type: %s", event.EventType)
		return
	}

	// Convert to JSON
	messageBytes, err := json.Marshal(wsMessage)
	if err != nil {
		log.Printf("[TransactionSubscriber] Failed to marshal WebSocket message: %v", err)
		return
	}

	// Send to user's WebSocket connection
	s.hub.SendToUser(event.UserID, messageBytes)
	log.Printf("[TransactionSubscriber]  Sent %s notification to user=%s", event.EventType, event.UserID)
}

// Stop gracefully stops the subscriber
func (s *TransactionEventSubscriber) Stop() error {
	if s.pubsub != nil {
		return s.pubsub.Close()
	}
	return nil
}
