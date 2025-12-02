package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"x/shared/auth/middleware"
	accountingpb "x/shared/genproto/shared/accounting/v1"
	"x/shared/response"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// ============================================================================
// DTOs
// ============================================================================

type CreditDebitDTO struct {
	AccountNumber string  `json:"account_number"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Description   string  `json:"description"`
}

type TransferDTO struct {
	From        string  `json:"from"`
	To          string  `json:"to"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
}

type ConversionDTO struct {
	FromAccount string  `json:"from_account"`
	ToAccount   string  `json:"to_account"`
	Amount      float64 `json:"amount"`
}

type TradeDTO struct {
	AccountNumber string  `json:"account_number"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	TradeID       string  `json:"trade_id"`
	TradeType     string  `json:"trade_type"`
}

type CommissionDTO struct {
	AgentExternalID   string  `json:"agent_external_id"`
	TransactionRef    string  `json:"transaction_ref"`
	Currency          string  `json:"currency"`
	TransactionAmount float64 `json:"transaction_amount"`
	CommissionAmount  float64 `json:"commission_amount"`
	CommissionRate    string  `json:"commission_rate,omitempty"`
}

type ApprovalDTO struct {
	Approved bool   `json:"approved"`
	Reason   string `json:"reason,omitempty"`
}

type GetAccountsJSON struct {
	OwnerType   string `json:"owner_type"`
	OwnerID     string `json:"owner_id"`
	AccountType string `json:"account_type,omitempty"`
}

type AccountStatementJSON struct {
	AccountNumber string    `json:"account_number"`
	From          time.Time `json:"from"`
	To            time.Time `json:"to"`
}

type OwnerStatementJSON struct {
	OwnerType string    `json:"owner_type"`
	OwnerID   string    `json:"owner_id"`
	From      time.Time `json:"from"`
	To        time.Time `json:"to"`
}

type UpdateAccountDTO struct {
	IsActive       *bool     `json:"is_active,omitempty"`
	IsLocked       *bool     `json:"is_locked,omitempty"`
	OverdraftLimit *float64  `json:"overdraft_limit,omitempty"`  //  Changed to *float64
}

type DailyReportQuery struct {
	Date        time.Time `json:"date"`
	AccountType string    `json:"account_type"`
}

type TransactionSummaryQuery struct {
	From        time.Time `json:"from"`
	To          time.Time `json:"to"`
	AccountType string    `json:"account_type"`
}

type FeeCalculationQuery struct {
	TransactionType string  `json:"transaction_type"`
	Amount          float64 `json:"amount"`
	SourceCurrency  string  `json:"source_currency,omitempty"`
	TargetCurrency  string  `json:"target_currency,omitempty"`
	AccountType     string  `json:"account_type,omitempty"`
	OwnerType       string  `json:"owner_type,omitempty"`
}

type CommissionSummaryQuery struct {
	AgentExternalID string    `json:"agent_external_id"`
	From            time.Time `json:"from"`
	To              time.Time `json:"to"`
}

type ApprovalHistoryQuery struct {
	RequestedBy *int64     `json:"requested_by,omitempty"`
	Status      string     `json:"status,omitempty"`
	From        *time.Time `json:"from,omitempty"`
	To          *time.Time `json:"to,omitempty"`
	Limit       int32      `json:"limit"`
	Offset      int32      `json:"offset"`
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func mapOwnerType(s string) accountingpb.OwnerType {
	switch s {
	case "user":
		return accountingpb.OwnerType_OWNER_TYPE_USER
	case "partner":
		return accountingpb.OwnerType_OWNER_TYPE_PARTNER
	case "system":
		return accountingpb.OwnerType_OWNER_TYPE_SYSTEM
	case "admin":
		return accountingpb.OwnerType_OWNER_TYPE_ADMIN
	case "agent":
		return accountingpb.OwnerType_OWNER_TYPE_AGENT
	default:
		return accountingpb.OwnerType_OWNER_TYPE_UNSPECIFIED
	}
}

func mapAccountType(s string) accountingpb.AccountType {
	switch s {
	case "demo":
		return accountingpb.AccountType_ACCOUNT_TYPE_DEMO
	case "real":
		return accountingpb.AccountType_ACCOUNT_TYPE_REAL
	default:
		return accountingpb.AccountType_ACCOUNT_TYPE_REAL
	}
}

func mapTransactionType(s string) accountingpb.TransactionType {
	switch s {
	case "deposit":
		return accountingpb.TransactionType_TRANSACTION_TYPE_DEPOSIT
	case "withdrawal":
		return accountingpb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL
	case "transfer":
		return accountingpb.TransactionType_TRANSACTION_TYPE_TRANSFER
	case "conversion":
		return accountingpb.TransactionType_TRANSACTION_TYPE_CONVERSION
	case "fee":
		return accountingpb.TransactionType_TRANSACTION_TYPE_FEE
	case "commission":
		return accountingpb.TransactionType_TRANSACTION_TYPE_COMMISSION
	case "trade":
		return accountingpb.TransactionType_TRANSACTION_TYPE_TRADE
	case "adjustment":
		return accountingpb.TransactionType_TRANSACTION_TYPE_ADJUSTMENT
	case "refund":
		return accountingpb.TransactionType_TRANSACTION_TYPE_REFUND
	case "reversal":
		return accountingpb.TransactionType_TRANSACTION_TYPE_REVERSAL
	default:
		return accountingpb.TransactionType_TRANSACTION_TYPE_UNSPECIFIED
	}
}

func mapApprovalStatus(s string) accountingpb.ApprovalStatus {
	switch s {
	case "pending":
		return accountingpb.ApprovalStatus_APPROVAL_STATUS_PENDING
	case "approved":
		return accountingpb.ApprovalStatus_APPROVAL_STATUS_APPROVED
	case "rejected":
		return accountingpb. ApprovalStatus_APPROVAL_STATUS_REJECTED
	case "executed":
		return accountingpb.ApprovalStatus_APPROVAL_STATUS_EXECUTED
	case "failed":
		return accountingpb.ApprovalStatus_APPROVAL_STATUS_FAILED
	default:
		return accountingpb.ApprovalStatus_APPROVAL_STATUS_UNSPECIFIED
	}
}

func (h *AdminHandler) getAdminContext(r *http.Request) (userID string, role string, ok bool) {
	userID, ok = r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		return "", "", false
	}
	role, ok = r.Context().Value(middleware.ContextRole).(string)
	if !ok || role == "" {
		return "", "", false
	}
	return userID, role, true
}

func (h *AdminHandler) isSuperAdmin(role string) bool {
	return role == "super_admin"
}


// ============================================================================
// ACCOUNT MANAGEMENT HANDLERS
// ============================================================================

// POST /api/admin/accounts
func (h *AdminHandler) CreateAccounts(w http.ResponseWriter, r *http.Request) {
	var req accountingpb. CreateAccountsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response. Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.accountingClient.Client.CreateAccounts(r.Context(), &req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to create accounts: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// POST /api/admin/accounts/user
func (h *AdminHandler) GetUserAccounts(w http.ResponseWriter, r *http.Request) {
	var in GetAccountsJSON
	if err := json.NewDecoder(r. Body).Decode(&in); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	accountType := accountingpb.AccountType_ACCOUNT_TYPE_REAL
	if in.AccountType != "" {
		accountType = mapAccountType(in.AccountType)
	}

	req := &accountingpb.GetAccountsByOwnerRequest{
		OwnerType:   mapOwnerType(in.OwnerType),
		OwnerId:     in.OwnerID,
		AccountType: accountType,
	}

	resp, err := h.accountingClient.Client.GetAccountsByOwner(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to fetch accounts: "+err.Error())
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

// GET /api/admin/accounts/{number}/balance
func (h *AdminHandler) GetAccountBalance(w http.ResponseWriter, r *http.Request) {
	accountNumber := r.PathValue("number")
	if accountNumber == "" {
		response.Error(w, http.StatusBadRequest, "account_number required")
		return
	}

	req := &accountingpb.GetBalanceRequest{
		Identifier: &accountingpb.GetBalanceRequest_AccountNumber{
			AccountNumber: accountNumber,
		},
	}

	resp, err := h.accountingClient.Client.GetBalance(r. Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get balance: "+err.Error())
		return
	}

	response. JSON(w, http.StatusOK, resp)
}

// PUT /api/admin/accounts/{id}
func (h *AdminHandler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	accountID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid account id")
		return
	}

	var dto UpdateAccountDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req := &accountingpb.UpdateAccountRequest{
		Id: accountID,
	}

	if dto.IsActive != nil {
		req.IsActive = dto. IsActive
	}
	if dto.IsLocked != nil {
		req.IsLocked = dto.IsLocked
	}
	if dto.OverdraftLimit != nil {
		req.OverdraftLimit = dto.OverdraftLimit
	}

	resp, err := h.accountingClient.Client.UpdateAccount(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to update account: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// ============================================================================
// TRANSACTION OPERATION HANDLERS
// ============================================================================

// POST /api/admin/transactions/credit
func (h *AdminHandler) CreditAccount(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := h. getAdminContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var dto CreditDebitDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if dto.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "amount must be greater than zero")
		return
	}

	// Super admin: execute immediately
	if h.isSuperAdmin(role) {
		req := &accountingpb.CreditRequest{
			AccountNumber:       dto.AccountNumber,
			Amount:              dto. Amount,
			Currency:            dto.Currency,
			AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
			Description:         dto.Description,
			CreatedByExternalId: userID,
			CreatedByType:       accountingpb. OwnerType_OWNER_TYPE_ADMIN,
		}

		resp, err := h.accountingClient.Client.Credit(r.Context(), req)
		if err != nil {
			response.Error(w, http. StatusBadGateway, "failed to credit account: "+err. Error())
			return
		}

		response.JSON(w, http.StatusOK, resp)
		return
	}

	// Regular admin: create approval request
	userIDInt, _ := strconv.ParseInt(userID, 10, 64)
	req := &accountingpb.CreateTransactionApprovalRequest{
		RequestedBy:     userIDInt,
		TransactionType: accountingpb.TransactionType_TRANSACTION_TYPE_DEPOSIT,
		AccountNumber:   dto.AccountNumber,
		Amount:          dto.Amount,
		Currency:        dto.Currency,
		Description:     dto. Description,
	}

	resp, err := h.accountingClient.Client.CreateTransactionApproval(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to create approval request: "+err.Error())
		return
	}

	response. JSON(w, http.StatusAccepted, resp)
}

// POST /api/admin/transactions/debit
func (h *AdminHandler) DebitAccount(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := h.getAdminContext(r)
	if !ok {
		response. Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var dto CreditDebitDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if dto.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "amount must be greater than zero")
		return
	}

	// Super admin: execute immediately
	if h.isSuperAdmin(role) {
		req := &accountingpb.DebitRequest{
			AccountNumber:       dto.AccountNumber,
			Amount:              dto.Amount,
			Currency:            dto.Currency,
			AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
			Description:         dto.Description,
			CreatedByExternalId: userID,
			CreatedByType:       accountingpb.OwnerType_OWNER_TYPE_ADMIN,
		}

		resp, err := h.accountingClient.Client.Debit(r.Context(), req)
		if err != nil {
			response.Error(w, http.StatusBadGateway, "failed to debit account: "+err. Error())
			return
		}

		response.JSON(w, http.StatusOK, resp)
		return
	}

	// Regular admin: create approval request
	userIDInt, _ := strconv.ParseInt(userID, 10, 64)
	req := &accountingpb. CreateTransactionApprovalRequest{
		RequestedBy:     userIDInt,
		TransactionType: accountingpb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL,
		AccountNumber:   dto.AccountNumber,
		Amount:          dto.Amount,
		Currency:        dto.Currency,
		Description:     dto.Description,
	}

	resp, err := h.accountingClient.Client. CreateTransactionApproval(r.Context(), req)
	if err != nil {
		response. Error(w, http.StatusBadGateway, "failed to create approval request: "+err.Error())
		return
	}

	response.JSON(w, http.StatusAccepted, resp)
}

// POST /api/admin/transactions/transfer
func (h *AdminHandler) TransferFunds(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := h.getAdminContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var dto TransferDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if dto.From == dto.To {
		response.Error(w, http.StatusBadRequest, "from and to accounts must be different")
		return
	}
	if dto.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "amount must be greater than zero")
		return
	}

	req := &accountingpb.TransferRequest{
		FromAccountNumber:   dto.From,
		ToAccountNumber:     dto.To,
		Amount:              dto.Amount,
		AccountType:         accountingpb. AccountType_ACCOUNT_TYPE_REAL,
		Description:         dto.Description,
		CreatedByExternalId: userID,
		CreatedByType:       accountingpb.OwnerType_OWNER_TYPE_ADMIN,
	}

	resp, err := h.accountingClient.Client.Transfer(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to transfer: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// POST /api/admin/transactions/convert
func (h *AdminHandler) ConvertAndTransfer(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := h. getAdminContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var dto ConversionDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if dto.FromAccount == dto.ToAccount {
		response.Error(w, http. StatusBadRequest, "from and to accounts must be different")
		return
	}
	if dto.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "amount must be greater than zero")
		return
	}

	req := &accountingpb.ConversionRequest{
		FromAccountNumber:   dto.FromAccount,
		ToAccountNumber:     dto. ToAccount,
		Amount:              dto.Amount,
		AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		CreatedByExternalId: userID,
		CreatedByType:       accountingpb.OwnerType_OWNER_TYPE_ADMIN,
	}

	resp, err := h.accountingClient.Client.ConvertAndTransfer(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to convert and transfer: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// POST /api/admin/transactions/trade/win
func (h *AdminHandler) ProcessTradeWin(w http. ResponseWriter, r *http.Request) {
	userID, _, ok := h.getAdminContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var dto TradeDTO
	if err := json.NewDecoder(r.Body). Decode(&dto); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if dto.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "amount must be greater than zero")
		return
	}

	req := &accountingpb.TradeRequest{
		AccountNumber:       dto.AccountNumber,
		Amount:              dto.Amount,
		Currency:            dto.Currency,
		AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		TradeId:             dto.TradeID,
		TradeType:           dto.TradeType,
		CreatedByExternalId: userID,
		CreatedByType:       accountingpb.OwnerType_OWNER_TYPE_ADMIN,
	}

	resp, err := h.accountingClient.Client.ProcessTradeWin(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to process trade win: "+err.Error())
		return
	}

	response. JSON(w, http.StatusOK, resp)
}

// POST /api/admin/transactions/trade/loss
func (h *AdminHandler) ProcessTradeLoss(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := h.getAdminContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var dto TradeDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if dto.Amount <= 0 {
		response. Error(w, http.StatusBadRequest, "amount must be greater than zero")
		return
	}

	req := &accountingpb.TradeRequest{
		AccountNumber:       dto.AccountNumber,
		Amount:              dto. Amount,
		Currency:            dto.Currency,
		AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		TradeId:             dto.TradeID,
		TradeType:           dto.TradeType,
		CreatedByExternalId: userID,
		CreatedByType:       accountingpb.OwnerType_OWNER_TYPE_ADMIN,
	}

	resp, err := h.accountingClient. Client.ProcessTradeLoss(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to process trade loss: "+err. Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// POST /api/admin/transactions/commission
func (h *AdminHandler) ProcessAgentCommission(w http.ResponseWriter, r *http.Request) {
	var dto CommissionDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		response. Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if dto.CommissionAmount <= 0 {
		response.Error(w, http.StatusBadRequest, "commission amount must be greater than zero")
		return
	}

	req := &accountingpb.AgentCommissionRequest{
		AgentExternalId:   dto.AgentExternalID,
		TransactionRef:    dto.TransactionRef,
		Currency:          dto.Currency,
		TransactionAmount: dto.TransactionAmount,
		CommissionAmount:  dto.CommissionAmount,
	}

	if dto.CommissionRate != "" {
		req.CommissionRate = &dto.CommissionRate
	}

	resp, err := h.accountingClient.Client.ProcessAgentCommission(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to process commission: "+err.Error())
		return
	}

	response. JSON(w, http.StatusOK, resp)
}

// ============================================================================
// APPROVAL MANAGEMENT HANDLERS
// ============================================================================

// GET /api/admin/approvals/pending
func (h *AdminHandler) GetPendingApprovals(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query(). Get("offset")

	var limit, offset int32 = 50, 0
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

	req := &accountingpb. GetPendingApprovalsRequest{
		Limit:  &limit,
		Offset: &offset,
	}

	resp, err := h.accountingClient.Client.GetPendingApprovals(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get pending approvals: "+err.Error())
		return
	}

	response. JSON(w, http.StatusOK, resp)
}

// POST /api/admin/approvals/{id}/approve
func (h *AdminHandler) ApproveTransaction(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := h.getAdminContext(r)
	if ! ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if !h.isSuperAdmin(role) {
		response.Error(w, http.StatusForbidden, "only super admin can approve transactions")
		return
	}

	requestID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request_id")
		return
	}

	var dto ApprovalDTO
	if err := json.NewDecoder(r.Body). Decode(&dto); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	approverID, _ := strconv.ParseInt(userID, 10, 64)
	req := &accountingpb.ApproveTransactionRequest{
		RequestId:  requestID,
		ApprovedBy: approverID,
		Approved:   dto.Approved,
	}

	if dto.Reason != "" {
		req. Reason = &dto.Reason
	}

	resp, err := h.accountingClient.Client. ApproveTransaction(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to approve transaction: "+err.Error())
		return
	}

	response. JSON(w, http.StatusOK, resp)
}

// GET /api/admin/approvals/history
func (h *AdminHandler) GetApprovalHistory(w http.ResponseWriter, r *http.Request) {
	var query ApprovalHistoryQuery
	
	// Parse query parameters
	if reqBy := r.URL.Query().Get("requested_by"); reqBy != "" {
		if id, err := strconv.ParseInt(reqBy, 10, 64); err == nil {
			query.RequestedBy = &id
		}
	}
	
	query.Status = r.URL.Query().Get("status")
	
	if fromStr := r.URL.Query(). Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			query.From = &t
		}
	}
	
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			query.To = &t
		}
	}
	
	query.Limit = 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.ParseInt(limitStr, 10, 32); err == nil {
			query. Limit = int32(l)
		}
	}
	
	query.Offset = 0
	if offsetStr := r.URL. Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.ParseInt(offsetStr, 10, 32); err == nil {
			query.Offset = int32(o)
		}
	}

	req := &accountingpb.GetApprovalHistoryRequest{
		Limit:  query.Limit,
		Offset: query.Offset,
	}

	if query.RequestedBy != nil {
		req.RequestedBy = query.RequestedBy
	}
	if query.Status != "" {
		status := mapApprovalStatus(query.Status)
		req.Status = &status
	}
	if query.From != nil {
		req.From = timestamppb.New(*query.From)
	}
	if query.To != nil {
		req.To = timestamppb. New(*query.To)
	}

	resp, err := h.accountingClient.Client.GetApprovalHistory(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get approval history: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// ============================================================================
// STATEMENT & REPORT HANDLERS
// ============================================================================

// POST /api/admin/statements/account
func (h *AdminHandler) GetAccountStatement(w http.ResponseWriter, r *http.Request) {
	var in AccountStatementJSON
	if err := json.NewDecoder(r.Body). Decode(&in); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req := &accountingpb.GetAccountStatementRequest{
		AccountNumber: in.AccountNumber,
		AccountType:   accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		From:          timestamppb.New(in. From),
		To:            timestamppb.New(in. To),
	}

	resp, err := h.accountingClient.Client.GetAccountStatement(r.Context(), req)
	if err != nil {
		response. Error(w, http.StatusBadGateway, "failed to fetch account statement: "+err.Error())
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

// POST /api/admin/statements/owner
func (h *AdminHandler) GetOwnerStatement(w http. ResponseWriter, r *http.Request) {
	var in OwnerStatementJSON
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response. Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req := &accountingpb. GetOwnerStatementRequest{
		OwnerType:   mapOwnerType(in.OwnerType),
		OwnerId:     in. OwnerID,
		AccountType: accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		From:        timestamppb.New(in. From),
		To:          timestamppb.New(in. To),
	}

	resp, err := h.accountingClient.Client.GetOwnerStatement(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to request owner statement: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// GET /api/admin/statements/summary/{owner_type}/{owner_id}
func (h *AdminHandler) GetOwnerSummary(w http.ResponseWriter, r *http.Request) {
	ownerType := r.PathValue("owner_type")
	ownerID := r.PathValue("owner_id")

	if ownerType == "" || ownerID == "" {
		response. Error(w, http.StatusBadRequest, "owner_type and owner_id required")
		return
	}

	req := &accountingpb.GetOwnerSummaryRequest{
		OwnerType:   mapOwnerType(ownerType),
		OwnerId:     ownerID,
		AccountType: accountingpb.AccountType_ACCOUNT_TYPE_REAL,
	}

	resp, err := h. accountingClient.Client.GetOwnerSummary(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get owner summary: "+err.Error())
		return
	}

	response.JSON(w, http. StatusOK, resp)
}

// ============================================================================
// TRANSACTION QUERY HANDLERS
// ============================================================================

// GET /api/admin/transactions/{receipt}
func (h *AdminHandler) GetTransactionByReceipt(w http.ResponseWriter, r *http.Request) {
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

	response.JSON(w, http.StatusOK, resp)
}

// GET /api/admin/ledgers/account/{number}
func (h *AdminHandler) GetAccountLedgers(w http.ResponseWriter, r *http.Request) {
	accountNumber := r.PathValue("number")
	if accountNumber == "" {
		response.Error(w, http.StatusBadRequest, "account_number required")
		return
	}

	limitStr := r.URL.Query(). Get("limit")
	offsetStr := r.URL.Query().Get("offset")

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

	req := &accountingpb.ListLedgersByAccountRequest{
		AccountNumber: accountNumber,
		AccountType:   accountingpb. AccountType_ACCOUNT_TYPE_REAL,
		Limit:         limit,
		Offset:        offset,
	}

	resp, err := h.accountingClient. Client.ListLedgersByAccount(r.Context(), req)
	if err != nil {
		response.Error(w, http. StatusBadGateway, "failed to get ledgers: "+err.Error())
		return
	}

	response.JSON(w, http. StatusOK, resp)
}

// GET /api/admin/ledgers/journal/{id}
func (h *AdminHandler) GetJournalLedgers(w http.ResponseWriter, r *http.Request) {
	journalIDStr := r.PathValue("id")
	journalID, err := strconv. ParseInt(journalIDStr, 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid journal_id")
		return
	}

	req := &accountingpb.ListLedgersByJournalRequest{
		JournalId: journalID,
	}

	resp, err := h.accountingClient.Client.ListLedgersByJournal(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get journal ledgers: "+err.Error())
		return
	}

	response.JSON(w, http. StatusOK, resp)
}

// ============================================================================
// REPORTS & ANALYTICS HANDLERS
// ============================================================================

// GET /api/admin/reports/daily
func (h *AdminHandler) GenerateDailyReport(w http.ResponseWriter, r *http. Request) {
	dateStr := r.URL.Query(). Get("date")
	accountTypeStr := r.URL.Query(). Get("account_type")

	if dateStr == "" {
		response.Error(w, http. StatusBadRequest, "date parameter required (RFC3339 format)")
		return
	}

	date, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid date format (use RFC3339)")
		return
	}

	accountType := accountingpb.AccountType_ACCOUNT_TYPE_REAL
	if accountTypeStr != "" {
		accountType = mapAccountType(accountTypeStr)
	}

	req := &accountingpb.GenerateDailyReportRequest{
		Date:        timestamppb.New(date),
		AccountType: accountType,
	}

	resp, err := h.accountingClient.Client.GenerateDailyReport(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to generate daily report: "+err. Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// GET /api/admin/reports/transaction-summary
func (h *AdminHandler) GetTransactionSummary(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query(). Get("from")
	toStr := r.URL.Query().Get("to")
	accountTypeStr := r.URL.Query().Get("account_type")

	if fromStr == "" || toStr == "" {
		response.Error(w, http.StatusBadRequest, "from and to parameters required (RFC3339 format)")
		return
	}

	from, err := time.Parse(time. RFC3339, fromStr)
	if err != nil {
		response.Error(w, http. StatusBadRequest, "invalid from date format")
		return
	}

	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid to date format")
		return
	}

	accountType := accountingpb.AccountType_ACCOUNT_TYPE_REAL
	if accountTypeStr != "" {
		accountType = mapAccountType(accountTypeStr)
	}

	req := &accountingpb.GetTransactionSummaryRequest{
		AccountType: accountType,
		From:        timestamppb.New(from),
		To:          timestamppb.New(to),
	}

	resp, err := h.accountingClient. Client.GetTransactionSummary(r.Context(), req)
	if err != nil {
		response.Error(w, http. StatusBadGateway, "failed to get transaction summary: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// GET /api/admin/reports/system-holdings
func (h *AdminHandler) GetSystemHoldings(w http.ResponseWriter, r *http. Request) {
	accountTypeStr := r.URL.Query().Get("account_type")

	accountType := accountingpb.AccountType_ACCOUNT_TYPE_REAL
	if accountTypeStr != "" {
		accountType = mapAccountType(accountTypeStr)
	}

	req := &accountingpb.GetSystemHoldingsRequest{
		AccountType: accountType,
	}

	resp, err := h.accountingClient.Client.GetSystemHoldings(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get system holdings: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// ============================================================================
// FEE MANAGEMENT HANDLERS
// ============================================================================

// GET /api/admin/fees/calculate
func (h *AdminHandler) CalculateFee(w http.ResponseWriter, r *http.Request) {
	transactionType := r.URL.Query().Get("transaction_type")
	amountStr := r.URL.Query().Get("amount")
	sourceCurrency := r.URL.Query().Get("source_currency")
	targetCurrency := r.URL.Query().Get("target_currency")
	accountTypeStr := r.URL.Query().Get("account_type")
	ownerTypeStr := r.URL. Query().Get("owner_type")

	if transactionType == "" || amountStr == "" {
		response.Error(w, http.StatusBadRequest, "transaction_type and amount required")
		return
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid amount")
		return
	}

	req := &accountingpb. CalculateFeeRequest{
		TransactionType: mapTransactionType(transactionType),
		Amount:          amount,
	}

	if sourceCurrency != "" {
		req.SourceCurrency = &sourceCurrency
	}
	if targetCurrency != "" {
		req.TargetCurrency = &targetCurrency
	}
	if accountTypeStr != "" {
		accountType := mapAccountType(accountTypeStr)
		req.AccountType = &accountType
	}
	if ownerTypeStr != "" {
		ownerType := mapOwnerType(ownerTypeStr)
		req.OwnerType = &ownerType
	}

	resp, err := h.accountingClient.Client. CalculateFee(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to calculate fee: "+err.Error())
		return
	}

	response. JSON(w, http.StatusOK, resp)
}

// GET /api/admin/fees/receipt/{receipt}
func (h *AdminHandler) GetFeesByReceipt(w http.ResponseWriter, r *http.Request) {
	receiptCode := r.PathValue("receipt")
	if receiptCode == "" {
		response.Error(w, http.StatusBadRequest, "receipt_code required")
		return
	}

	req := &accountingpb.GetFeesByReceiptRequest{
		ReceiptCode: receiptCode,
	}

	resp, err := h.accountingClient.Client. GetFeesByReceipt(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get fees: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// GET /api/admin/fees/commission/{agent_id}
func (h *AdminHandler) GetAgentCommissionSummary(w http.ResponseWriter, r *http.Request) {
	agentID := r. PathValue("agent_id")
	fromStr := r.URL.Query(). Get("from")
	toStr := r.URL.Query().Get("to")

	if agentID == "" || fromStr == "" || toStr == "" {
		response.Error(w, http.StatusBadRequest, "agent_id, from, and to required")
		return
	}

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid from date format")
		return
	}

	to, err := time.Parse(time. RFC3339, toStr)
	if err != nil {
		response.Error(w, http. StatusBadRequest, "invalid to date format")
		return
	}

	req := &accountingpb.GetAgentCommissionSummaryRequest{
		AgentExternalId: agentID,
		From:            timestamppb.New(from),
		To:              timestamppb.New(to),
	}

	resp, err := h.accountingClient. Client.GetAgentCommissionSummary(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to get commission summary: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}