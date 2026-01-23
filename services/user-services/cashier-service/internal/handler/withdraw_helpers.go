// handler/withdrawal_helpers.go
package handler

import (
	"cashier-service/internal/domain"
	"context"
	"fmt"
	"log"
	"math"
	"time"

	// "math/rand"
	// partnersvcpb "x/shared/genproto/partner/svcpb"
	accountingpb "x/shared/genproto/shared/accounting/v1"
	"x/shared/utils/id"

	"go.uber.org/zap"
)

// buildDirectWithdrawalContext builds context for direct withdrawal
func (h *PaymentHandler) buildDirectWithdrawalContext(ctx context.Context, wctx *WithdrawalContext) (*WithdrawalContext, error) {
	req := wctx.Request

	// Direct withdrawals are in USD
	wctx.AmountInUSD = req.Amount
	wctx.ExchangeRate = 1.0
	req.LocalCurrency = "USD"

	// Get user USD account
	userAccount, err := h.GetAccountByCurrency(ctx, wctx.UserID, "user", "USD",nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get user account: %v", err)
	}
	wctx.UserAccount = userAccount

	// Validate balance
	balanceResp, err := h.accountingClient.Client.GetBalance(ctx, &accountingpb.GetBalanceRequest{
		Identifier: &accountingpb.GetBalanceRequest_AccountNumber{
			AccountNumber: userAccount,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check balance: %v", err)
	}

	if balanceResp.Balance.AvailableBalance < wctx.AmountInUSD {
		return nil, fmt.Errorf("insufficient balance: need %.2f USD, have %.2f USD",
			wctx.AmountInUSD, balanceResp.Balance.AvailableBalance)
	}

	h.logger.Info("direct withdrawal context built",
		zap.Float64("amount_usd", wctx.AmountInUSD))

	return wctx, nil
}

// processDirectWithdrawal processes direct withdrawal
func (h *PaymentHandler) processDirectWithdrawal(ctx context.Context, client *Client, wctx *WithdrawalContext) {
	req := wctx.Request

	// Create withdrawal request
	withdrawalReq := &domain.WithdrawalRequest{
		UserID:      wctx.UserIDInt,
		RequestRef:  id.GenerateTransactionID("WD-DR"),
		Amount:      wctx.AmountInUSD,
		Currency:    "USD",
		Destination: req.Destination,
		Service:     req.Service,
		Status:      domain.WithdrawalStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Store metadata
	withdrawalReq.SetOriginalAmount(req.Amount, "USD", 1.0)
	if withdrawalReq.Metadata == nil {
		withdrawalReq.Metadata = make(map[string]interface{})
	}
	withdrawalReq.Metadata["withdrawal_type"] = "direct"

	// Save to database
	if err := h.userUc.CreateWithdrawalRequest(ctx, withdrawalReq); err != nil {
		h.logger.Error("failed to create withdrawal request", zap.Error(err))
		client.SendError(fmt.Sprintf("failed to create withdrawal request: %v", err))
		return
	}

	// Process asynchronously
	go h.executeDirectWithdrawal(withdrawalReq, wctx)

	// Send success response
	client.SendSuccess("direct withdrawal request created", map[string]interface{}{
		"request_ref":     withdrawalReq.RequestRef,
		"amount_usd":      wctx.AmountInUSD,
		"withdrawal_type": "direct",
		"status":          "processing",
	})
}

// executeDirectWithdrawal executes the direct withdrawal
func (h *PaymentHandler) executeDirectWithdrawal(withdrawal *domain.WithdrawalRequest, wctx *WithdrawalContext) {
	ctx := context.Background()

	if err := h.userUc.MarkWithdrawalProcessing(ctx, withdrawal.RequestRef); err != nil {
		log.Printf("[DirectWithdrawal] Failed to mark as processing: %v", err)
		return
	}

	// Execute debit (in USD)
	debitReq := &accountingpb.DebitRequest{
		AccountNumber:       wctx.UserAccount,
		Amount:              withdrawal.Amount,
		AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		Description:         fmt.Sprintf("Direct withdrawal %.2f USD to %s",
			withdrawal.Amount, withdrawal.Destination),
		ExternalRef:         &withdrawal.RequestRef,
		CreatedByExternalId: fmt.Sprintf("%d", withdrawal.UserID),
		CreatedByType:       accountingpb.OwnerType_OWNER_TYPE_USER,
		TransactionType:     accountingpb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL,
	}

	resp, err := h.accountingClient.Client.Debit(ctx, debitReq)
	if err != nil {
		errMsg := err.Error()
		h.userUc.FailWithdrawal(ctx, withdrawal.RequestRef, errMsg)

		h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
            "type": "withdrawal_failed",
            "data": {
                "request_ref": "%s",
                "error": "%s"
            }
        }`, withdrawal.RequestRef, errMsg)))
		return
	}

	if err := h.userUc.UpdateWithdrawalWithReceipt(ctx, withdrawal.ID, resp.ReceiptCode, resp.JournalId, true); err != nil {
		log.Printf("[DirectWithdrawal] Failed to mark as completed: %v", err)
		return
	}

	h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
        "type": "withdrawal_completed",
        "data": {
            "request_ref": "%s",
            "receipt_code": "%s",
            "amount_usd": %.2f,
            "balance_after": %.2f
        }
    }`, withdrawal.RequestRef, resp.ReceiptCode, withdrawal.Amount, resp.BalanceAfter)))
}

// getPaymentMethod converts service to payment method
func getPaymentMethod(service *string) string {
	if service != nil {
		return *service
	}
	return "default"
}

// roundTo2Decimals rounds to 2 decimal places
func roundTo2Decimals(value float64) float64 {
	return math.Round(value*100) / 100
}

// roundTo8Decimals rounds to 8 decimal places (for crypto)
func roundTo8Decimals(value float64) float64 {
	return math.Round(value*100000000) / 100000000
}
