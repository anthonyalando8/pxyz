// cashier-service/internal/usecase/combined_event_handler.go
package eventhandler

import (
	"context"

	"cashier-service/internal/handler"
	"cashier-service/internal/repository"
	"cashier-service/internal/sub"

	"go.uber.org/zap"
)

// CombinedEventHandler implements EventHandler interface for both deposits and withdrawals
type CombinedEventHandler struct {
	depositHandler    *DepositEventHandler
	withdrawalHandler *WithdrawalEventHandler
}

func NewCombinedEventHandler(
	userRepo *repository.UserRepo,
	hub *handler.Hub,
	logger *zap.Logger,
) *CombinedEventHandler {
	return &CombinedEventHandler{
		depositHandler:     NewDepositEventHandler(userRepo, hub, logger),
		withdrawalHandler: NewWithdrawalEventHandler(userRepo, hub, logger),
	}
}

// Deposit methods
func (h *CombinedEventHandler) HandleDepositCompleted(ctx context.Context, event *subscriber.DepositCompletedEvent) error {
	return h.depositHandler.HandleDepositCompleted(ctx, event)
}

func (h *CombinedEventHandler) HandleDepositFailed(ctx context.Context, event *subscriber.DepositFailedEvent) error {
	return h.depositHandler.HandleDepositFailed(ctx, event)
}

// Withdrawal methods
func (h *CombinedEventHandler) HandleWithdrawalCompleted(ctx context.Context, event *subscriber.WithdrawalCompletedEvent) error {
	return h.withdrawalHandler.HandleWithdrawalCompleted(ctx, event)
}

func (h *CombinedEventHandler) HandleWithdrawalFailed(ctx context.Context, event *subscriber. WithdrawalFailedEvent) error {
	return h.withdrawalHandler.HandleWithdrawalFailed(ctx, event)
}