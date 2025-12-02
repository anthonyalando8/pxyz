package handler

import (
	//"encoding/json"
	"context"
	"log"
	"net/http"
	domain "partner-service/internal/domain"
	"strconv"
	"time"

	// emailclient "x/shared/email"
	// smsclient "x/shared/sms"

	//"partner-service/internal/usecase"

	"x/shared/auth/middleware"

	"x/shared/response"

	//accountingclient "x/shared/common/accounting"
	authpb "x/shared/genproto/partner/authpb"
	accountingpb "x/shared/genproto/shared/accounting/v1"

	"github.com/go-chi/chi/v5"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// getPartnerContext extracts partner ID from authenticated context
func (h *PartnerHandler) getPartnerContext(r *http. Request) (partnerID string, userID string, ok bool) {
	ctx := r.Context()

	// Extract user ID
	userID, ok = ctx. Value(middleware. ContextUserID).(string)
	if !ok || userID == "" {
		return "", "", false
	}

	// Fetch partner ID from profile
	profileResp, err := h.authClient. PartnerClient.GetUserProfile(ctx, &authpb.GetUserProfileRequest{
		UserId: userID,
	})
	if err != nil || profileResp == nil || profileResp.User == nil {
		return "", "", false
	}

	partnerID = profileResp.User. PartnerId
	if partnerID == "" {
		return "", "", false
	}

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

// POST /api/partner/accounting/transactions/credit
// CreditUser allows partner to credit a user's wallet (via API key authentication)
func (h *PartnerHandler) CreditUser(w http.ResponseWriter, r *http.Request) {
	// This endpoint uses API key authentication (handled by middleware)
	// Partner context should be set by API key middleware
	partnerID, ok := r.Context().Value("partner_id").(string)
	if !ok || partnerID == "" {
		response.Error(w, http.StatusUnauthorized, "invalid API credentials")
		return
	}

	var req struct {
		UserID          string  `json:"user_id"`
		Amount          float64 `json:"amount"`
		Currency        string  `json:"currency"`
		TransactionRef  string  `json:"transaction_ref"` // Partner's reference
		Description     string  `json:"description"`
		PaymentMethod   string  `json:"payment_method,omitempty"`
		ExternalRef     string  `json:"external_ref,omitempty"`
		Metadata        map[string]interface{} `json:"metadata,omitempty"`
	}

	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validation
	if req.UserID == "" {
		response. Error(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "amount must be greater than zero")
		return
	}
	if req.Currency == "" {
		response.Error(w, http.StatusBadRequest, "currency is required")
		return
	}
	if req.TransactionRef == "" {
		response.Error(w, http.StatusBadRequest, "transaction_ref is required")
		return
	}

	// 1. Verify user exists and get their account
	userAccountsResp, err := h.accountingClient.Client.GetAccountsByOwner(r.Context(), &accountingpb.GetAccountsByOwnerRequest{
		OwnerType:   accountingpb.OwnerType_OWNER_TYPE_USER,
		OwnerId:     req.UserID,
		AccountType: accountingpb.AccountType_ACCOUNT_TYPE_REAL,
	})
	if err != nil {
		response. Error(w, http.StatusBadGateway, "failed to get user accounts: "+err.Error())
		return
	}

	if len(userAccountsResp.Accounts) == 0 {
		response.Error(w, http.StatusNotFound, "user has no accounts")
		return
	}

	// Find wallet account in the requested currency
	var targetAccount *accountingpb.Account
	for _, acc := range userAccountsResp.Accounts {
		if acc.Currency == req.Currency && acc.Purpose == accountingpb.AccountPurpose_ACCOUNT_PURPOSE_WALLET {
			targetAccount = acc
			break
		}
	}

	if targetAccount == nil {
		response.Error(w, http.StatusNotFound, "user has no wallet account in currency: "+req.Currency)
		return
	}

	// 2. Create partner transaction record
	partnerTx := &domain.PartnerTransaction{
		PartnerID:      partnerID,
		TransactionRef: req.TransactionRef,
		UserID:         req.UserID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Status:         "processing",
		PaymentMethod:  StringPtr(req.PaymentMethod),
		ExternalRef:    StringPtr(req.ExternalRef),
		Metadata:       req.Metadata,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := h.uc.CreateTransaction(r.Context(), partnerTx); err != nil {
		response.Error(w, http. StatusInternalServerError, "failed to create transaction record: "+err.Error())
		return
	}

	// 3. Credit user account via accounting service
	creditReq := &accountingpb.CreditRequest{
		AccountNumber:       targetAccount.AccountNumber,
		Amount:              req. Amount,
		Currency:            req.Currency,
		AccountType:         accountingpb. AccountType_ACCOUNT_TYPE_REAL,
		Description:         req.Description,
		ExternalRef:         &req.TransactionRef,
		CreatedByExternalId: partnerID,
		CreatedByType:       accountingpb. OwnerType_OWNER_TYPE_PARTNER,
	}

	creditResp, err := h.accountingClient.Client.Credit(r.Context(), creditReq)
	if err != nil {
		// Update transaction status to failed
		h.uc.UpdateTransactionStatus(r.Context(), partnerTx.ID, "failed", err.Error())
		response. Error(w, http.StatusBadGateway, "failed to credit user: "+err.Error())
		return
	}

	// 4. Update transaction status to completed
	if err := h.uc.UpdateTransactionStatus(r.Context(), partnerTx.ID, "completed", ""); err != nil {
		// Log error but don't fail the request as money was already credited
		log.Printf("[WARN] Failed to update transaction status for txn=%d: %v", partnerTx.ID, err)
	}

	// 5. Send webhook notification to partner (async)
	go h.sendTransactionWebhook(partnerID, "deposit. completed", map[string]interface{}{
		"transaction_ref": req.TransactionRef,
		"user_id":         req.UserID,
		"amount":          req.Amount,
		"currency":        req.Currency,
		"receipt_code":    creditResp.ReceiptCode,
		"journal_id":      creditResp.JournalId,
		"balance_after":   creditResp.BalanceAfter,
		"timestamp":       creditResp.CreatedAt.AsTime(). Unix(),
	})

	// 6. Log API activity
	h.logAPIActivity(r.Context(), partnerID, "POST", "/transactions/credit", http.StatusOK, req, creditResp)

	// Return success response
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"success":         true,
		"transaction_ref": req.TransactionRef,
		"receipt_code":    creditResp. ReceiptCode,
		"journal_id":      creditResp.JournalId,
		"balance_after":   creditResp.BalanceAfter,
		"created_at":      creditResp.CreatedAt. AsTime(),
	})
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
