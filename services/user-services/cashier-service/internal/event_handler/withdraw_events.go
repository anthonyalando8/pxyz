// cashier-service/internal/usecase/withdrawal_event_handler. go
package eventhandler

import (
	"context"
	"fmt"

	"cashier-service/internal/domain"
	"cashier-service/internal/handler"
	"cashier-service/internal/repository"
	"cashier-service/internal/sub"

	"go.uber.org/zap"
)

type WithdrawalEventHandler struct {
	userRepo *repository.UserRepo
	hub      *handler.Hub // WebSocket hub
	logger   *zap.Logger
}

func NewWithdrawalEventHandler(
	userRepo *repository. UserRepo,
	hub *handler.Hub,
	logger *zap.Logger,
) *WithdrawalEventHandler {
	return &WithdrawalEventHandler{
		userRepo: userRepo,
		hub:      hub,
		logger:   logger,
	}
}

// HandleWithdrawalCompleted processes withdrawal completed events from partner service
func (h *WithdrawalEventHandler) HandleWithdrawalCompleted(ctx context.Context, event *subscriber. WithdrawalCompletedEvent) error {
	h.logger.Info("handling withdrawal completed event",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID),
		zap.Float64("amount", event.Amount),
		zap.String("currency", event.Currency),
		zap.String("external_ref", event.ExternalRef))

	// 1. Get withdrawal request by transaction ref
	withdrawal, err := h.userRepo.GetWithdrawalByRef(ctx, event.TransactionRef)
	if err != nil {
		h.logger.Error("failed to get withdrawal request",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt.Errorf("withdrawal request not found: %w", err)
	}

	// 2. Verify withdrawal is in correct status
	if withdrawal.Status != domain.WithdrawalStatusSentToPartner && withdrawal.Status != domain.WithdrawalStatusProcessing {
		h.logger. Warn("withdrawal not in expected status",
			zap.String("transaction_ref", event.TransactionRef),
			zap.String("current_status", withdrawal.Status))
		// Still process to ensure idempotency
	}

	// 3. Update withdrawal record with completion details
	if err := h.userRepo.MarkWithdrawalCompleted(ctx, event.TransactionRef, event. ExternalRef); err != nil {
		h.logger.Error("failed to mark withdrawal completed",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt.Errorf("failed to update withdrawal: %w", err)
	}

	// 4. Send WebSocket notification to user
	h.sendWithdrawalCompletedWebSocket(withdrawal.UserID, event)

	// 5. Log success
	h.logger.Info("withdrawal completed event processed successfully",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID),
		zap.String("external_ref", event.ExternalRef))

	return nil
}

// HandleWithdrawalFailed processes withdrawal failed events from partner service
func (h *WithdrawalEventHandler) HandleWithdrawalFailed(ctx context.Context, event *subscriber.WithdrawalFailedEvent) error {
	h.logger.Info("handling withdrawal failed event",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event. UserID),
		zap.String("error", event.ErrorMessage))

	// 1. Get withdrawal request
	withdrawal, err := h.userRepo.GetWithdrawalByRef(ctx, event.TransactionRef)
	if err != nil {
		h. logger.Error("failed to get withdrawal request",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt.Errorf("withdrawal request not found: %w", err)
	}

	// 2. Update withdrawal as failed
	if err := h.userRepo.MarkWithdrawalFailed(ctx, event.TransactionRef, event.ErrorMessage); err != nil {
		h.logger.Error("failed to mark withdrawal failed",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt.Errorf("failed to update withdrawal: %w", err)
	}

	// 3. Send WebSocket notification to user
	h.sendWithdrawalFailedWebSocket(withdrawal.UserID, event)

	// 4. Log failure
	h.logger.Info("withdrawal failed event processed successfully",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID),
		zap.String("error", event.ErrorMessage))

	return nil
}

// sendWithdrawalCompletedWebSocket sends real-time notification to user via WebSocket
func (h *WithdrawalEventHandler) sendWithdrawalCompletedWebSocket(userID int64, event *subscriber. WithdrawalCompletedEvent) {
	if h.hub == nil {
		return
	}

	userIDStr := fmt.Sprintf("%d", userID)

	message := map[string]interface{}{
		"type": "withdrawal_completed",
		"data": map[string]interface{}{
			"transaction_ref": event.TransactionRef,
			"external_ref":    event.ExternalRef, // M-Pesa code or bank reference
			"amount":          event. Amount,
			"currency":        event.Currency,
			"payment_method":  event.PaymentMethod,
			"completed_at":    event.CompletedAt.Unix(),
			"timestamp":       event.Timestamp,
			"metadata":        event.Metadata,
		},
	}

	// Send to specific user
	if client, exists := h.hub.GetClient(userIDStr); exists {
		if err := client.SendJSON(message); err != nil {
			h.logger.Error("failed to send websocket message",
				zap.String("user_id", userIDStr),
				zap.Error(err))
		} else {
			h. logger.Debug("websocket message sent",
				zap.String("user_id", userIDStr),
				zap.String("type", "withdrawal_completed"))
		}
	} else {
		h.logger.Debug("user not connected to websocket",
			zap.String("user_id", userIDStr))
	}
}

// sendWithdrawalFailedWebSocket sends real-time failure notification to user
func (h *WithdrawalEventHandler) sendWithdrawalFailedWebSocket(userID int64, event *subscriber. WithdrawalFailedEvent) {
	if h.hub == nil {
		return
	}

	userIDStr := fmt. Sprintf("%d", userID)

	message := map[string]interface{}{
		"type":  "withdrawal_failed",
		"data": map[string]interface{}{
			"transaction_ref":  event.TransactionRef,
			"amount":          event.Amount,
			"currency":        event.Currency,
			"error_message":   event.ErrorMessage,
			"failed_at":       event. FailedAt.Unix(),
			"timestamp":       event.Timestamp,
		},
	}

	// Send to specific user
	if client, exists := h.hub.GetClient(userIDStr); exists {
		if err := client.SendJSON(message); err != nil {
			h.logger. Error("failed to send websocket message",
				zap.String("user_id", userIDStr),
				zap.Error(err))
		} else {
			h.logger.Debug("websocket message sent",
				zap. String("user_id", userIDStr),
				zap.String("type", "withdrawal_failed"))
		}
	} else {
		h.logger.Debug("user not connected to websocket",
			zap.String("user_id", userIDStr))
	}
}