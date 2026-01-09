// user-service/internal/usecase/deposit_event_handler.go
package eventhandler

import (
	"context"
	"fmt"
	// "strconv"
	// "time"

	"cashier-service/internal/domain"
	"cashier-service/internal/handler"
	"cashier-service/internal/repository"
	"cashier-service/internal/sub"

	"go.uber.org/zap"
)

type DepositEventHandler struct {
	userRepo *repository.UserRepo
	hub      *handler.Hub // WebSocket hub
	logger   *zap. Logger
}

func NewDepositEventHandler(
	userRepo *repository.UserRepo,
	hub *handler.Hub,
	logger *zap.Logger,
) *DepositEventHandler {
	return &DepositEventHandler{
		userRepo: userRepo,
		hub:      hub,
		logger:   logger,
	}
}

// HandleDepositCompleted processes deposit completed events from partner service
func (h *DepositEventHandler) HandleDepositCompleted(ctx context.Context, event *subscriber. DepositCompletedEvent) error {
	h.logger.Info("handling deposit completed event",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID),
		zap.Float64("amount", event.Amount),
		zap.String("currency", event.Currency),
		zap.String("receipt_code", event.ReceiptCode))

	// 1. Get deposit request by transaction ref
	deposit, err := h.userRepo. GetDepositByRef(ctx, event.TransactionRef)
	if err != nil {
		h.logger.Error("failed to get deposit request",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt.Errorf("deposit request not found: %w", err)
	}

	// 2. Verify deposit is in correct status
	if deposit.Status != domain.DepositStatusSentToPartner && deposit.Status != domain.DepositStatusProcessing {
		h.logger. Warn("deposit not in expected status",
			zap.String("transaction_ref", event.TransactionRef),
			zap. String("current_status", deposit.Status))
		// Still process to ensure idempotency
	}

	// 3. Update deposit record with completion details
	if err := h.userRepo. MarkDepositCompleted(ctx, event.TransactionRef, event. ReceiptCode, event.JournalID); err != nil {
		h.logger.Error("failed to mark deposit completed",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt. Errorf("failed to update deposit: %w", err)
	}

	// 4.  Send WebSocket notification to user
	h.sendDepositCompletedWebSocket(deposit. UserID, event)

	// 5. Log success
	h.logger.Info("deposit completed event processed successfully",
		zap.String("transaction_ref", event.TransactionRef),
		zap. String("user_id", event. UserID),
		zap.String("receipt_code", event. ReceiptCode),
		zap.Int64("journal_id", event.JournalID))

	return nil
}

// HandleDepositFailed processes deposit failed events from partner service
func (h *DepositEventHandler) HandleDepositFailed(ctx context.Context, event *subscriber. DepositFailedEvent) error {
	h.logger.Info("handling deposit failed event",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID),
		zap.String("error", event.ErrorMessage))

	// 1. Get deposit request
	deposit, err := h.userRepo.GetDepositByRef(ctx, event.TransactionRef)
	if err != nil {
		h.logger.Error("failed to get deposit request",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt.Errorf("deposit request not found: %w", err)
	}

	// 2. Update deposit as failed
	if err := h. userRepo.MarkDepositFailed(ctx, event.TransactionRef, event.ErrorMessage); err != nil {
		h.logger.Error("failed to mark deposit failed",
			zap.String("transaction_ref", event.TransactionRef),
			zap.Error(err))
		return fmt. Errorf("failed to update deposit: %w", err)
	}

	// 3. Send WebSocket notification to user
	h.sendDepositFailedWebSocket(deposit.UserID, event)

	// 4. Log failure
	h.logger.Info("deposit failed event processed successfully",
		zap.String("transaction_ref", event.TransactionRef),
		zap.String("user_id", event.UserID),
		zap.String("error", event.ErrorMessage))

	return nil
}

// sendDepositCompletedWebSocket sends real-time notification to user via WebSocket
func (h *DepositEventHandler) sendDepositCompletedWebSocket(userID int64, event *subscriber.DepositCompletedEvent) {
	if h.hub == nil {
		return
	}

	userIDStr := fmt.Sprintf("%d", userID)

	message := map[string]interface{}{
		"type": "deposit_completed",
		"data": map[string]interface{}{
			"transaction_ref": event.TransactionRef,
			"receipt_code":    event.ReceiptCode,
			"journal_id":      event.JournalID,
			"amount":          event.Amount,
			"currency":        event.Currency,
			"user_balance":    event.UserBalance,
			"fee_amount":      event.FeeAmount,
			"payment_method":  event.PaymentMethod,
			"completed_at":    event.CompletedAt. Unix(),
			"timestamp":       event.Timestamp,
		},
	}

	// Send to specific user
	if client, exists := h.hub.GetClient(userIDStr); exists {
		if err := client.SendJSON(message); err != nil {
			h.logger.Error("failed to send websocket message",
				zap.String("user_id", userIDStr),
				zap. Error(err))
		} else {
			h.logger. Debug("websocket message sent",
				zap.String("user_id", userIDStr),
				zap.String("type", "deposit_completed"))
		}
	} else {
		h.logger. Debug("user not connected to websocket",
			zap.String("user_id", userIDStr))
	}
}

// sendDepositFailedWebSocket sends real-time failure notification to user
func (h *DepositEventHandler) sendDepositFailedWebSocket(userID int64, event *subscriber. DepositFailedEvent) {
	if h.hub == nil {
		return
	}

	userIDStr := fmt.Sprintf("%d", userID)

	message := map[string]interface{}{
		"type": "deposit_failed",
		"data": map[string]interface{}{
			"transaction_ref": event. TransactionRef,
			"amount":          event.Amount,
			"currency":        event.Currency,
			"error_message":   event.ErrorMessage,
			"failed_at":       event.FailedAt.Unix(),
			"timestamp":       event.Timestamp,
		},
	}

	// Send to specific user
	if client, exists := h.hub.GetClient(userIDStr); exists {
		if err := client. SendJSON(message); err != nil {
			h.logger.Error("failed to send websocket message",
				zap.String("user_id", userIDStr),
				zap.Error(err))
		} else {
			h.logger.Debug("websocket message sent",
				zap.String("user_id", userIDStr),
				zap.String("type", "deposit_failed"))
		}
	} else {
		h.logger.Debug("user not connected to websocket",
			zap.String("user_id", userIDStr))
	}
}