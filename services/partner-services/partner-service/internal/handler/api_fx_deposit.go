package handler

import (
	//"encoding/json"
	"context"
	"encoding/json"
	"fmt"

	//"log"
	"net/http"
	domain "partner-service/internal/domain"

	//"strconv"
	"time"

	// emailclient "x/shared/email"
	// smsclient "x/shared/sms"

	//"partner-service/internal/usecase"
	partnerMiddleware "partner-service/pkg/auth"

	//"x/shared/auth/middleware"
	"partner-service/internal/events"

	"x/shared/response"

	//accountingclient "x/shared/common/accounting"
	//authpb "x/shared/genproto/partner/authpb"
	accountingpb "x/shared/genproto/shared/accounting/v1"

	//"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	//"google.golang.org/protobuf/types/known/timestamppb"
)

// ============================================================================
// TRANSACTION OPERATIONS (Credit User)
// ============================================================================
type CreditUser struct {
	UserID         string                 `json:"user_id"`
	Amount         float64                `json:"amount"`
	Currency       string                 `json:"currency"`
	TransactionRef string                 `json:"transaction_ref"`
	Description    string                 `json:"description"`
	PaymentMethod  string                 `json:"payment_method,omitempty"`
	ExternalRef    string                 `json:"external_ref,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}
// CreditUser allows partner to credit a user's wallet (via API key authentication)
// This is called after user initiates deposit and partner collects payment
// handler/partner_handler.go

// CreditUser allows partner to credit a user's wallet (via API key authentication)
func (h *PartnerHandler) CreditUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Get and validate partner
	partner, ok := partnerMiddleware.GetPartnerFromContext(ctx)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	h.logger.Info("credit request received",
		zap.String("partner_id", partner.ID),
		zap.String("partner_name", partner.Name))

	// 2. Parse and validate request
	var req CreditUser
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http. StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validateCreditRequest(&req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// 3. Get and validate existing transaction
	partnerTx, err := h.getAndValidateTransaction(ctx, partner.ID, &req)
	if err != nil {
		h.handleCreditError(ctx, w, 0, err. Error(), http.StatusBadRequest)
		return
	}

	// 4. Update status to processing
	if err := h.uc.UpdateTransactionStatus(ctx, partnerTx. ID, "processing", ""); err != nil {
		h.logger.Error("failed to update transaction status",
			zap.Int64("transaction_id", partnerTx.ID),
			zap.Error(err))
		response.Error(w, http.StatusInternalServerError, "failed to update transaction status")
		return
	}

	// 5.  Get partner and user accounts
	partnerAccount, userAccount, err := h.getTransferAccounts(ctx, partner.ID, req.UserID, req.Currency)
	if err != nil {
		h.handleCreditError(ctx, w, partnerTx.ID, err. Error(), http.StatusNotFound)
		return
	}

	// 6.  Verify partner balance
	if err := h. verifyPartnerBalance(ctx, partnerAccount. AccountNumber, req.Amount); err != nil {
		h.handleCreditError(ctx, w, partnerTx.ID, err.Error(), http.StatusPaymentRequired)
		return
	}

	// 7. Execute transfer
	transferResp, err := h.executeTransfer(ctx, partner, partnerAccount, userAccount, &req)
	if err != nil {
		h.handleCreditError(ctx, w, partnerTx.ID, err.Error(), http.StatusBadGateway)
		return
	}

	// 8. Complete transaction
	if err := h. completeTransaction(ctx, partnerTx. ID, transferResp, req.ExternalRef); err != nil {
		h.logger.Error("failed to complete transaction",
			zap.Int64("transaction_id", partnerTx.ID),
			zap. Error(err))
		// Don't fail - money already transferred
	}

	// 9. Get final balances
	balances := h.getFinalBalances(ctx, partnerAccount.AccountNumber, userAccount.AccountNumber)

	// 10. Publish success events
	h.publishDepositCompleted(ctx, partner, &req, partnerTx, transferResp, balances. UserBalance)

	// 11. Send webhook
	h.sendDepositWebhook(ctx, partner. ID, &req, partnerTx, transferResp, balances)

	// 12. Log API activity
	h.logAPIActivity(ctx, partner.ID, "POST", "/transactions/credit", http.StatusOK, req, transferResp)

	// 13. Return success response
	h.sendCreditResponse(w, &req, partnerTx, transferResp, balances)

	h.logger.Info("credit transaction completed",
		zap. String("partner_id", partner. ID),
		zap.String("user_id", req.UserID),
		zap.Int64("transaction_id", partnerTx.ID),
		zap.Float64("amount", req.Amount),
		zap. String("receipt", transferResp. ReceiptCode))
}

// ============================================
// HELPER METHODS
// ============================================

// getAndValidateTransaction retrieves and validates the transaction
func (h *PartnerHandler) getAndValidateTransaction(ctx context.Context, partnerID string, req *CreditUser) (*domain. PartnerTransaction, error) {
	// Get transaction
	partnerTx, err := h. uc.GetTransactionByRef(ctx, partnerID, req.TransactionRef)
	if err != nil {
		h.logger.Error("transaction not found",
			zap. String("partner_id", partnerID),
			zap.String("transaction_ref", req.TransactionRef),
			zap. Error(err))
		return nil, fmt.Errorf("transaction with ref %s not found", req. TransactionRef)
	}

	// Verify status
	if partnerTx. Status != "pending" {
		h.logger.Warn("transaction not in pending status",
			zap.String("partner_id", partnerID),
			zap.String("transaction_ref", req.TransactionRef),
			zap.String("current_status", partnerTx. Status))
		return nil, fmt.Errorf("transaction already %s", partnerTx. Status)
	}

	// Verify amount
	if partnerTx.Amount != req.Amount {
		h.logger. Warn("amount mismatch",
			zap.Float64("expected", partnerTx.Amount),
			zap.Float64("provided", req.Amount))
		return nil, fmt. Errorf("amount mismatch: expected %.2f, got %.2f", partnerTx.Amount, req.Amount)
	}

	// Verify currency
	if partnerTx. Currency != req.Currency {
		h.logger.Warn("currency mismatch",
			zap.String("expected", partnerTx.Currency),
			zap.String("provided", req.Currency))
		return nil, fmt. Errorf("currency mismatch: expected %s, got %s", partnerTx.Currency, req.Currency)
	}

	// Verify user ID
	if partnerTx. UserID != req.UserID {
		h.logger.Warn("user_id mismatch",
			zap.String("expected", partnerTx.UserID),
			zap.String("provided", req.UserID))
		return nil, fmt.Errorf("user_id mismatch")
	}

	return partnerTx, nil
}

// getTransferAccounts retrieves partner and user accounts
func (h *PartnerHandler) getTransferAccounts(ctx context.Context, partnerID, userID, currency string) (*accountingpb.Account, *accountingpb.Account, error) {
	// Get partner account
	partnerAccount, err := h.getWalletAccount(ctx, accountingpb. OwnerType_OWNER_TYPE_PARTNER, accountingpb.AccountPurpose_ACCOUNT_PURPOSE_SETTLEMENT, partnerID, currency)
	if err != nil {
		h.logger.Error("failed to get partner account",
			zap.String("partner_id", partnerID),
			zap.String("currency", currency),
			zap.Error(err))
		return nil, nil, fmt. Errorf("partner has no %s wallet account", currency)
	}

	// Get user account
	userAccount, err := h.getWalletAccount(ctx, accountingpb.OwnerType_OWNER_TYPE_USER, accountingpb.AccountPurpose_ACCOUNT_PURPOSE_WALLET, userID, currency)
	if err != nil {
		h.logger. Error("failed to get user account",
			zap.String("user_id", userID),
			zap.String("currency", currency),
			zap.Error(err))
		return nil, nil, fmt.Errorf("user has no %s wallet account", currency)
	}

	return partnerAccount, userAccount, nil
}

// getWalletAccount retrieves a wallet account for an owner
func (h *PartnerHandler) getWalletAccount(ctx context.Context, ownerType accountingpb.OwnerType, accountPurpose accountingpb.AccountPurpose, ownerID, currency string) (*accountingpb.Account, error) {
	accountsResp, err := h.accountingClient.Client.GetAccountsByOwner(ctx, &accountingpb.GetAccountsByOwnerRequest{
		OwnerType:   ownerType,
		OwnerId:     ownerID,
		AccountType: accountingpb.AccountType_ACCOUNT_TYPE_REAL,
	})
	if err != nil {
		return nil, err
	}

	if len(accountsResp.Accounts) == 0 {
		return nil, fmt.Errorf("no accounts found")
	}

	// Find wallet account in requested currency
	for _, acc := range accountsResp. Accounts {
		if acc.Currency == currency && acc.Purpose == accountPurpose {
			return acc, nil
		}
	}

	return nil, fmt.Errorf("no wallet account in %s", currency)
}

// verifyPartnerBalance checks if partner has sufficient balance
func (h *PartnerHandler) verifyPartnerBalance(ctx context.Context, accountNumber string, requiredAmount float64) error {
	balanceResp, err := h. accountingClient.Client.GetBalance(ctx, &accountingpb.GetBalanceRequest{
		Identifier: &accountingpb.GetBalanceRequest_AccountNumber{
			AccountNumber: accountNumber,
		},
	})
	if err != nil {
		h.logger.Error("failed to get partner balance", zap.Error(err))
		return fmt. Errorf("failed to get partner balance")
	}

	if balanceResp.Balance. AvailableBalance < requiredAmount {
		h.logger. Warn("insufficient partner balance",
			zap. Float64("available", balanceResp.Balance. AvailableBalance),
			zap.Float64("required", requiredAmount))
		return fmt.Errorf("insufficient balance: have %.2f, need %.2f",
			balanceResp.Balance.AvailableBalance, requiredAmount)
	}

	return nil
}

// executeTransfer performs the actual transfer
func (h *PartnerHandler) executeTransfer(
	ctx context.Context,
	partner *domain.Partner,
	partnerAccount, userAccount *accountingpb.Account,
	req *CreditUser,
) (*accountingpb.TransferResponse, error) {
	description := req.Description
	if description == "" {
		description = fmt. Sprintf("Deposit from %s", partner.Name)
	}

	transferReq := &accountingpb.TransferRequest{
		FromAccountNumber:   partnerAccount.AccountNumber,
		ToAccountNumber:     userAccount.AccountNumber,
		Amount:              req.Amount,
		AccountType:         accountingpb. AccountType_ACCOUNT_TYPE_REAL,
		Description:         description,
		IdempotencyKey:      &req.TransactionRef,
		ExternalRef:         &req.TransactionRef,
		CreatedByExternalId: partner.ID,
		CreatedByType:       accountingpb. OwnerType_OWNER_TYPE_PARTNER,
		TransactionType:     accountingpb.TransactionType_TRANSACTION_TYPE_DEPOSIT,
	}

	transferResp, err := h.accountingClient.Client.Transfer(ctx, transferReq)
	if err != nil {
		h.logger.Error("transfer failed",
			zap.String("partner_id", partner.ID),
			zap.String("user_id", req.UserID),
			zap.Float64("amount", req.Amount),
			zap.Error(err))
		return nil, err
	}

	return transferResp, nil
}

// completeTransaction updates transaction with receipt
func (h *PartnerHandler) completeTransaction(ctx context.Context, txnID int64, transferResp *accountingpb.TransferResponse, extRef string) error {
	h.uc.UpdateTransactionWithReceipt(ctx, txnID, transferResp.ReceiptCode, transferResp.JournalId, "completed")
	return h.uc.UpdateTransactionCompletion(ctx, txnID, extRef, "completed")
}

// AccountBalances holds final account balances
type AccountBalances struct {
	PartnerBalance float64
	UserBalance    float64
}

// getFinalBalances retrieves final balances for both accounts
func (h *PartnerHandler) getFinalBalances(ctx context.Context, partnerAccountNumber, userAccountNumber string) *AccountBalances {
	balances := &AccountBalances{}

	// Get partner balance
	if partnerResp, err := h.accountingClient.Client.GetBalance(ctx, &accountingpb.GetBalanceRequest{
		Identifier: &accountingpb.GetBalanceRequest_AccountNumber{
			AccountNumber: partnerAccountNumber,
		},
	}); err == nil {
		balances. PartnerBalance = partnerResp.Balance.AvailableBalance
	}

	// Get user balance
	if userResp, err := h.accountingClient. Client.GetBalance(ctx, &accountingpb.GetBalanceRequest{
		Identifier: &accountingpb.GetBalanceRequest_AccountNumber{
			AccountNumber: userAccountNumber,
		},
	}); err == nil {
		balances.UserBalance = userResp.Balance. AvailableBalance
	}

	return balances
}

// publishDepositCompleted publishes deposit completed event to Redis
func (h *PartnerHandler) publishDepositCompleted(
	ctx context.Context,
	partner *domain.Partner,
	req *CreditUser,
	partnerTx *domain. PartnerTransaction,
	transferResp *accountingpb.TransferResponse,
	userBalance float64,
) {
	go func() {
		pubCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		depositEvent := &events.DepositCompletedEvent{
			TransactionRef: req.TransactionRef,
			TransactionID:  partnerTx.ID,
			PartnerID:      partner.ID,
			UserID:         req.UserID,
			Amount:         req.Amount,
			Currency:       req.Currency,
			ReceiptCode:    transferResp. ReceiptCode,
			JournalID:      transferResp.JournalId,
			FeeAmount:      transferResp.FeeAmount,
			UserBalance:    userBalance,
			PaymentMethod:  req.PaymentMethod,
			ExternalRef:    req.ExternalRef,
			Metadata:       req.Metadata,
			CompletedAt:    time.Now(),
		}

		if err := h.eventPublisher.PublishDepositCompleted(pubCtx, depositEvent); err != nil {
			h.logger.Error("failed to publish deposit completed event",
				zap.String("transaction_ref", req.TransactionRef),
				zap.Error(err))
		}
	}()
}

// publishDepositFailed publishes deposit failed event to Redis
func (h *PartnerHandler) publishDepositFailed(
	ctx context. Context,
	partner *domain. Partner,
	req *CreditUser,
	partnerTx *domain.PartnerTransaction,
	errorMsg string,
) {
	go func() {
		pubCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		failedEvent := &events.DepositFailedEvent{
			TransactionRef: req.TransactionRef,
			TransactionID:  partnerTx.ID,
			PartnerID:      partner.ID,
			UserID:         req.UserID,
			Amount:         req.Amount,
			Currency:       req.Currency,
			ErrorMessage:   errorMsg,
			FailedAt:       time.Now(),
		}

		if err := h. eventPublisher.PublishDepositFailed(pubCtx, failedEvent); err != nil {
			h.logger.Error("failed to publish deposit failed event",
				zap.String("transaction_ref", req.TransactionRef),
				zap.Error(err))
		}
	}()
}

// sendDepositWebhook sends webhook notification to partner
func (h *PartnerHandler) sendDepositWebhook(
	ctx context.Context,
	partnerID string,
	req *CreditUser,
	partnerTx *domain.PartnerTransaction,
	transferResp *accountingpb.TransferResponse,
	balances *AccountBalances,
) {
	go h.uc.SendWebhook(context.Background(), partnerID, "deposit.completed", map[string]interface{}{
		"transaction_ref": req.TransactionRef,
		"user_id":         req.UserID,
		"amount":          req.Amount,
		"currency":        req.Currency,
		"receipt_code":    transferResp.ReceiptCode,
		"journal_id":      transferResp.JournalId,
		"fee_amount":      transferResp.FeeAmount,
		"partner_balance": balances.PartnerBalance,
		"user_balance":    balances.UserBalance,
		"timestamp":       transferResp.CreatedAt. AsTime(). Unix(),
	})
}

// sendCreditResponse sends success response
func (h *PartnerHandler) sendCreditResponse(
	w http.ResponseWriter,
	req *CreditUser,
	partnerTx *domain. PartnerTransaction,
	transferResp *accountingpb.TransferResponse,
	balances *AccountBalances,
) {
	response. JSON(w, http.StatusOK, map[string]interface{}{
		"success":          true,
		"transaction_ref":  req.TransactionRef,
		"transaction_id":   partnerTx.ID,
		"receipt_code":     transferResp. ReceiptCode,
		"journal_id":       transferResp.JournalId,
		// "fee_amount":       transferResp.FeeAmount,
		// "agent_commission": transferResp.AgentCommission,
		// "partner_balance":  balances. PartnerBalance,
		// "user_balance":     balances.UserBalance,
		"created_at":       transferResp.CreatedAt. AsTime(),
	})
}

// handleCreditError handles errors and publishes failed events
func (h *PartnerHandler) handleCreditError(ctx context.Context, w http.ResponseWriter, txnID int64, errorMsg string, statusCode int) {
	// Update transaction status if we have txnID
	if txnID > 0 {
		if err := h.uc.UpdateTransactionStatus(ctx, txnID, "failed", errorMsg); err != nil {
			h.logger.Error("failed to update transaction status",
				zap.Int64("transaction_id", txnID),
				zap.Error(err))
		}

		// Get transaction details for event
		if partnerTx, err := h. uc.GetTransactionByID(ctx, txnID); err == nil {
			partner, _ := partnerMiddleware.GetPartnerFromContext(ctx)
			if partner != nil {
				// Publish failed event
				h. publishDepositFailed(ctx, partner, &CreditUser{
					UserID:         partnerTx.UserID,
					Amount:         partnerTx.Amount,
					Currency:       partnerTx.Currency,
					TransactionRef: partnerTx. TransactionRef,
				}, partnerTx, errorMsg)
			}
		}
	}

	response.Error(w, statusCode, errorMsg)
}

// validateCreditRequest validates the credit request
func (h *PartnerHandler) validateCreditRequest(req *CreditUser) error {
	if req.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if req.Amount <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}
	if req.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if req.TransactionRef == "" {
		return fmt. Errorf("transaction_ref is required")
	}
	if len(req.Currency) < 3 || len(req.Currency) > 8 {
		return fmt. Errorf("invalid currency format")
	}
	return nil
}

// handler/partner_handler.go

// logAPIActivity logs API request/response for audit and monitoring
func (h *PartnerHandler) logAPIActivity(ctx context.Context, partnerID, method, endpoint string, statusCode int, request, response interface{}) {
	// Run in background goroutine to not block the response
	go func() {
		logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Marshal response body
		var responseBody *string
		if response != nil {
			if respBytes, err := json.Marshal(response); err == nil {
				respStr := string(respBytes)
				responseBody = &respStr
			}
		}

		// ✅ Extract IP address from context
		var ipAddress *string
		if ip := partnerMiddleware.GetClientIPFromContext(ctx); ip != "" {
			ipAddress = &ip
		}

		// ✅ Extract user agent from context
		var userAgent *string
		if ua := partnerMiddleware.GetUserAgentFromContext(ctx); ua != "" {
			userAgent = &ua
		}

		// ✅ Get latency from context
		var latencyMs *int64
		latency := partnerMiddleware.GetRequestLatency(ctx)
		if latency > 0 {
			latencyMs = &latency
		}

		// ✅ Extract error message if status indicates failure
		var errorMessage *string
		if statusCode >= 400 {
			if errMsg := partnerMiddleware. GetErrorMessageFromContext(ctx); errMsg != "" {
				errorMessage = &errMsg
			} else if response != nil {
				// Try to extract error from response
				if respMap, ok := response.(map[string]interface{}); ok {
					if errMsg, exists := respMap["error"]; exists {
						errStr := fmt.Sprintf("%v", errMsg)
						errorMessage = &errStr
					}
				}
			}
		}

			// Create log entry
			// Convert response body string to map if possible, since PartnerAPILog.ResponseBody expects map[string]interface{}
			var responseBodyMap map[string]interface{}
			if responseBody != nil {
				_ = json.Unmarshal([]byte(*responseBody), &responseBodyMap)
			}
			var requestBodyMap map[string]interface{}
			if request != nil {
				b, err := json.Marshal(request)
				if err == nil {
					_ = json.Unmarshal(b, &requestBodyMap)
				}
			}

	
			log := &domain.PartnerAPILog{
				PartnerID:    partnerID,
				Endpoint:     endpoint,
				Method:       method,
				RequestBody:  requestBodyMap,
				ResponseBody: responseBodyMap,
				StatusCode:   statusCode,
				IPAddress:    ipAddress,
				UserAgent:    userAgent,
				LatencyMs:    latencyMs,
				ErrorMessage: errorMessage,
			}
	
			// Save to database via usecase
			if err := h.uc.LogAPIActivity(logCtx, log); err != nil {
				h.logger.Error("failed to log API activity",
					zap.String("partner_id", partnerID),
					zap.String("endpoint", endpoint),
					zap.Error(err))
			}
		}()
}
