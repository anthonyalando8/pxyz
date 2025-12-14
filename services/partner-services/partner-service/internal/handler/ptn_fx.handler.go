package handler

import (
	//"encoding/json"
	"context"
	"fmt"
	"log"
	"net/http"
	domain "partner-service/internal/domain"
	"strconv"
	"time"

	// emailclient "x/shared/email"
	// smsclient "x/shared/sms"

	//"partner-service/internal/usecase"
	partnerMiddleware "partner-service/pkg/auth"

	"x/shared/auth/middleware"
	"partner-service/internal/events"

	"x/shared/response"

	//accountingclient "x/shared/common/accounting"
	authpb "x/shared/genproto/partner/authpb"
	accountingpb "x/shared/genproto/shared/accounting/v1"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// getPartnerContext extracts partner ID from authenticated context
func (h *PartnerHandler) getPartnerContext(r *http.Request) (partnerID string, userID string, ok bool) {
	ctx := r.Context()

	// ✅ Method 1: Try to get partner from API key authentication (fast path)
	// This is set by the RequireAPIKey middleware
	partner, hasPartner := partnerMiddleware.GetPartnerFromContext(ctx)
	if hasPartner && partner != nil && partner.ID != "" {
		h.logger.Debug("partner authenticated via API key",
			zap.String("partner_id", partner.ID),
			zap.String("partner_name", partner.Name))
		
		// For API key auth, we might not have a user ID
		// Extract from request body or use a system user ID
		return partner.ID, "", true
	}

	// ✅ Method 2: Try to get partner via user profile (slower path)
	// This is set by the JWT/session authentication middleware
	userID, hasUser := ctx.Value(middleware. ContextUserID).(string)
	if ! hasUser || userID == "" {
		h.logger. Warn("no authentication found - neither API key nor user session")
		return "", "", false
	}

	h.logger.Debug("attempting to fetch partner via user profile",
		zap. String("user_id", userID))

	// Fetch partner ID from user profile
	profileResp, err := h. authClient. PartnerClient.GetUserProfile(ctx, &authpb. GetUserProfileRequest{
		UserId: userID,
	})
	if err != nil {
		h.logger.Warn("failed to get user profile",
			zap.String("user_id", userID),
			zap.Error(err))
		return "", "", false
	}

	if profileResp == nil || profileResp.User == nil {
		h.logger.Warn("user profile not found",
			zap.String("user_id", userID))
		return "", "", false
	}

	partnerID = profileResp.User.PartnerId
	if partnerID == "" {
		h.logger.Warn("user has no associated partner",
			zap.String("user_id", userID))
		return "", "", false
	}

	h.logger.Debug("partner authenticated via user profile",
		zap. String("partner_id", partnerID),
		zap.String("user_id", userID))

	return partnerID, userID, true
}
// ============================================================================
// ACCOUNT MANAGEMENT ENDPOINTS
// ============================================================================

// GET /api/partner/accounting/accounts
// GetUserAccounts returns all accounts for the partner
func (h *PartnerHandler) GetUserAccounts(w http.ResponseWriter, r *http.Request) {
	partnerID, _, ok := h.getPartnerContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized or partner not linked")
		return
	}

	req := &accountingpb.GetAccountsByOwnerRequest{
		OwnerType:   accountingpb.OwnerType_OWNER_TYPE_PARTNER,
		OwnerId:     partnerID,
		AccountType: accountingpb.AccountType_ACCOUNT_TYPE_REAL,
	}

	resp, err := h.accountingClient.Client.GetAccountsByOwner(r.Context(), req)
	if err != nil {
		response. Error(w, http.StatusBadGateway, "failed to fetch accounts: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// GET /api/partner/accounting/accounts/{number}/balance
// GetAccountBalance retrieves balance for a specific account
func (h *PartnerHandler) GetAccountBalance(w http.ResponseWriter, r *http.Request) {
	partnerID, _, ok := h.getPartnerContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized or partner not linked")
		return
	}

	accountNumber := r.PathValue("number")
	if accountNumber == "" {
		response.Error(w, http.StatusBadRequest, "account_number required")
		return
	}

	// Verify account belongs to this partner
	accountResp, err := h.accountingClient.Client.GetAccount(r.Context(), &accountingpb.GetAccountRequest{
		Identifier: &accountingpb.GetAccountRequest_AccountNumber{
			AccountNumber: accountNumber,
		},
	})
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get account: "+err.Error())
		return
	}

	if accountResp.Account. OwnerId != partnerID {
		response.Error(w, http.StatusForbidden, "account does not belong to this partner")
		return
	}

	// Get balance
	balanceReq := &accountingpb.GetBalanceRequest{
		Identifier: &accountingpb. GetBalanceRequest_AccountNumber{
			AccountNumber: accountNumber,
		},
	}

	balanceResp, err := h. accountingClient.Client.GetBalance(r.Context(), balanceReq)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get balance: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, balanceResp)
}

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
	if err := h. completeTransaction(ctx, partnerTx. ID, transferResp); err != nil {
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
func (h *PartnerHandler) completeTransaction(ctx context.Context, txnID int64, transferResp *accountingpb.TransferResponse) error {
	return h.uc.UpdateTransactionWithReceipt(ctx, txnID, transferResp.ReceiptCode, transferResp.JournalId, "completed")
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
// ============================================================================
// STATEMENT & REPORT ENDPOINTS
// ============================================================================

// POST /api/partner/accounting/statements/account
// GetAccountStatement fetches statement for a specific account
func (h *PartnerHandler) GetAccountStatement(w http.ResponseWriter, r *http.Request) {
	partnerID, _, ok := h.getPartnerContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized or partner not linked")
		return
	}

	var in struct {
		AccountNumber string    `json:"account_number"`
		From          time.Time `json:"from"`
		To            time.Time `json:"to"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Verify account belongs to this partner
	accountResp, err := h.accountingClient.Client.GetAccount(r.Context(), &accountingpb.GetAccountRequest{
		Identifier: &accountingpb.GetAccountRequest_AccountNumber{
			AccountNumber: in.AccountNumber,
		},
	})
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get account: "+err.Error())
		return
	}

	if accountResp. Account.OwnerId != partnerID {
		response.Error(w, http.StatusForbidden, "account does not belong to this partner")
		return
	}

	req := &accountingpb.GetAccountStatementRequest{
		AccountNumber: in.AccountNumber,
		AccountType:   accountingpb. AccountType_ACCOUNT_TYPE_REAL,
		From:          timestamppb.New(in. From),
		To:            timestamppb.New(in.To),
	}

	resp, err := h.accountingClient.Client.GetAccountStatement(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to fetch account statement: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// POST /api/partner/accounting/statements/owner
// GetOwnerStatement fetches all account statements for the partner
func (h *PartnerHandler) GetOwnerStatement(w http.ResponseWriter, r *http.Request) {
	partnerID, _, ok := h. getPartnerContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized or partner not linked")
		return
	}

	var in struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req := &accountingpb.GetOwnerStatementRequest{
		OwnerType:   accountingpb.OwnerType_OWNER_TYPE_PARTNER,
		OwnerId:     partnerID,
		AccountType: accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		From:        timestamppb.New(in. From),
		To:          timestamppb.New(in. To),
	}

	resp, err := h.accountingClient.Client.GetOwnerStatement(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to request owner statement: "+err.Error())
		return
	}

	response.JSON(w, http. StatusOK, resp)
}

// GET /api/partner/accounting/summary
// GetOwnerSummary fetches consolidated balance summary for the partner
func (h *PartnerHandler) GetOwnerSummary(w http. ResponseWriter, r *http.Request) {
	partnerID, _, ok := h.getPartnerContext(r)
	if ! ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized or partner not linked")
		return
	}

	req := &accountingpb.GetOwnerSummaryRequest{
		OwnerType:   accountingpb.OwnerType_OWNER_TYPE_PARTNER,
		OwnerId:     partnerID,
		AccountType: accountingpb.AccountType_ACCOUNT_TYPE_REAL,
	}

	resp, err := h.accountingClient. Client.GetOwnerSummary(r.Context(), req)
	if err != nil {
		response.Error(w, http. StatusBadGateway, "failed to get owner summary: "+err.Error())
		return
	}

	response.JSON(w, http. StatusOK, resp)
}

// ============================================================================
// TRANSACTION QUERY ENDPOINTS
// ============================================================================

// GET /api/partner/accounting/transactions/{receipt}
// GetTransactionByReceipt retrieves transaction details by receipt code
func (h *PartnerHandler) GetTransactionByReceipt(w http.ResponseWriter, r *http.Request) {
	partnerID, _, ok := h.getPartnerContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized or partner not linked")
		return
	}

	receiptCode := r.PathValue("receipt")
	if receiptCode == "" {
		response.Error(w, http.StatusBadRequest, "receipt_code required")
		return
	}

	req := &accountingpb.GetTransactionByReceiptRequest{
		ReceiptCode: receiptCode,
	}

	resp, err := h.accountingClient.Client.GetTransactionByReceipt(r.Context(), req)
	if err != nil {
		response. Error(w, http.StatusBadGateway, "failed to get transaction: "+err.Error())
		return
	}

	// Verify transaction belongs to this partner (check journal created_by)
	if resp.Journal.CreatedByExternalId != partnerID {
		response.Error(w, http.StatusForbidden, "transaction does not belong to this partner")
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// GET /api/partner/accounting/ledgers/account/{number}
// GetAccountLedgers retrieves ledger entries for a specific account
func (h *PartnerHandler) GetAccountLedgers(w http.ResponseWriter, r *http. Request) {
	partnerID, _, ok := h.getPartnerContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized or partner not linked")
		return
	}

	accountNumber := r.PathValue("number")
	if accountNumber == "" {
		response.Error(w, http.StatusBadRequest, "account_number required")
		return
	}

	// Verify account belongs to this partner
	accountResp, err := h.accountingClient.Client.GetAccount(r.Context(), &accountingpb.GetAccountRequest{
		Identifier: &accountingpb.GetAccountRequest_AccountNumber{
			AccountNumber: accountNumber,
		},
	})
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get account: "+err.Error())
		return
	}

	if accountResp. Account.OwnerId != partnerID {
		response.Error(w, http.StatusForbidden, "account does not belong to this partner")
		return
	}

	// Parse query parameters
	limitStr := r.URL.Query(). Get("limit")
	offsetStr := r.URL.Query(). Get("offset")

	var limit, offset int32 = 100, 0
	if limitStr != "" {
		if l, err := strconv.ParseInt(limitStr, 10, 32); err == nil {
			limit = int32(l)
		}
	}
	if offsetStr != "" {
		if o, err := strconv.ParseInt(offsetStr, 10, 32); err == nil {
			offset = int32(o)
		}
	}

	req := &accountingpb. ListLedgersByAccountRequest{
		AccountNumber: accountNumber,
		AccountType:   accountingpb. AccountType_ACCOUNT_TYPE_REAL,
		Limit:         limit,
		Offset:        offset,
	}

	resp, err := h.accountingClient.Client.ListLedgersByAccount(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get ledgers: "+err.Error())
		return
	}

	response.JSON(w, http. StatusOK, resp)
}

// ============================================================================
// PARTNER TRANSACTION MANAGEMENT
// ============================================================================

// GET /api/partner/accounting/transactions
// ListPartnerTransactions returns all partner-initiated transactions
func (h *PartnerHandler) ListPartnerTransactions(w http.ResponseWriter, r *http.Request) {
	partnerID, _, ok := h.getPartnerContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized or partner not linked")
		return
	}

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r. URL.Query().Get("offset")
	status := r.URL.Query().Get("status")

	var limit, offset int = 20, 0
	if limitStr != "" {
		if l, err := strconv. Atoi(limitStr); err == nil {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv. Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	var statusFilter *string
	if status != "" {
		statusFilter = &status
	}

	transactions, total, err := h.uc. ListTransactions(r.Context(), partnerID, limit, offset, statusFilter, nil, nil)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list transactions: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"transactions": transactions,
		"total":        total,
		"limit":        limit,
		"offset":       offset,
	})
}

// GET /api/partner/accounting/transactions/ref/{ref}
// GetTransactionByRef retrieves partner transaction by reference
func (h *PartnerHandler) GetTransactionByRef(w http.ResponseWriter, r *http.Request) {
	partnerID, _, ok := h.getPartnerContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized or partner not linked")
		return
	}

	transactionRef := r.PathValue("ref")
	if transactionRef == "" {
		response.Error(w, http.StatusBadRequest, "transaction_ref required")
		return
	}

	transaction, err := h.uc. GetTransactionStatus(r.Context(), partnerID, transactionRef)
	if err != nil {
		response. Error(w, http.StatusNotFound, "transaction not found: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, transaction)
}

// ============================================================================
// HELPER METHODS
// ============================================================================

func (h *PartnerHandler) sendTransactionWebhook(partnerID, eventType string, payload map[string]interface{}) {
	if err := h.uc.SendWebhook(context.Background(), partnerID, eventType, payload); err != nil {
		log.Printf("[ERROR] Failed to send webhook for partner=%s, event=%s: %v", partnerID, eventType, err)
	}
}

func (h *PartnerHandler) logAPIActivity(ctx context.Context, partnerID, method, endpoint string, statusCode int, request, response interface{}) {
	// Implement API logging logic
	// This would call your usecase method to log to partner_api_logs table
}

// ==================== TRANSACTION MANAGEMENT ====================

// InitiateDeposit creates a new deposit transaction (partner-initiated)
func (h *PartnerHandler) InitiateDeposit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req struct {
		TransactionRef string                 `json:"transaction_ref"`
		UserID         string                 `json:"user_id"`
		Amount         float64                `json:"amount"`
		Currency       string                 `json:"currency"`
		PaymentMethod  string                 `json:"payment_method"`
		ExternalRef    string                 `json:"external_ref"`
		Metadata       map[string]interface{} `json:"metadata"`
	}

	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validation
	if req.TransactionRef == "" {
		response.Error(w, http.StatusBadRequest, "transaction_ref is required")
		return
	}
	if req.UserID == "" {
		response.Error(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "amount must be greater than 0")
		return
	}
	if req.Currency == "" {
		response.Error(w, http.StatusBadRequest, "currency is required")
		return
	}

	txn := &domain.PartnerTransaction{
		PartnerID:      partnerID,
		TransactionRef: req.TransactionRef,
		UserID:         req.UserID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Metadata:       req.Metadata,
	}

	if req.PaymentMethod != "" {
		txn.PaymentMethod = &req.PaymentMethod
	}
	if req.ExternalRef != "" {
		txn.ExternalRef = &req.ExternalRef
	}

	if err := h.uc.InitiateDeposit(ctx, txn); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"transaction_id":  txn.ID,
		"transaction_ref": txn.TransactionRef,
		"status":          txn.Status,
		"message":         "Deposit initiated successfully",
	})
}

// GetTransactionStatus retrieves transaction by reference
func (h *PartnerHandler) GetTransactionStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	transactionRef := chi.URLParam(r, "ref")
	if transactionRef == "" {
		response.Error(w, http.StatusBadRequest, "transaction reference is required")
		return
	}

	txn, err := h.uc.GetTransactionStatus(ctx, partnerID, transactionRef)
	if err != nil {
		response.Error(w, http.StatusNotFound, "transaction not found")
		return
	}

	response.JSON(w, http.StatusOK, txn)
}

// ListTransactions returns paginated list of transactions
func (h *PartnerHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	status := r.URL.Query().Get("status")

	limit := 20
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	var statusFilter *string
	if status != "" {
		statusFilter = &status
	}

	txns, total, err := h.uc.ListTransactions(ctx, partnerID, limit, offset, statusFilter, nil, nil)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch transactions: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"transactions": txns,
		"total_count":  total,
		"limit":        limit,
		"offset":       offset,
	})
}

// GetTransactionsByDateRange returns transactions within a date range
func (h *PartnerHandler) GetTransactionsByDateRange(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req struct {
		From   time.Time `json:"from"`
		To     time.Time `json:"to"`
		Status string    `json:"status"`
		Limit  int       `json:"limit"`
		Offset int       `json:"offset"`
	}

	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	var statusFilter *string
	if req.Status != "" {
		statusFilter = &req.Status
	}

	txns, total, err := h.uc.ListTransactions(ctx, partnerID, req.Limit, req.Offset, statusFilter, &req.From, &req.To)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch transactions: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"transactions": txns,
		"total_count":  total,
		"from":         req.From,
		"to":           req.To,
		"limit":        req.Limit,
		"offset":       req.Offset,
	})
}
