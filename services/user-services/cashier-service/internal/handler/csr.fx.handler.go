package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	partnersvcpb "x/shared/genproto/partner/svcpb"
	accountingpb "x/shared/genproto/shared/accounting/accountingpb"

	//"google.golang.org/protobuf/types/known/timestamppb"

	"x/shared/auth/middleware"
	"x/shared/response"
)

func (h *PaymentHandler) DepositHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Amount   float64 `json:"amount"`
		Method   string  `json:"method"`
		Service  string  `json:"service"`  // service to pick partner from
		Currency string  `json:"currency"` // user wallet currency
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	// For now hardcoding, later you can extend
	req.Currency = "USD"        // user wallet currency
	depositCurrency := "KES"    // partner account currency

	// Get authenticated user
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	// Get both accounts concurrently
	partnerAccount, userAccount, err := h.GetPartnerAndUserAccounts(r.Context(), req.Service, req.Currency, userID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "your request has been declined: "+err.Error())
		return
	}

	// Parse userID to int64 for gRPC
	userIDInt, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	// Construct accounting transaction (User → Partner)
	grpcReq := &accountingpb.CreateTransactionRequest{
		Description:   fmt.Sprintf("Deposit by user %s via %s", userID, req.Method),
		CreatedByType: accountingpb.OwnerType_USER,
		CreatedByUser: userIDInt,
		DepositCurrency: depositCurrency,
		Entries: []*accountingpb.TransactionEntry{
			{
				AccountNumber: userAccount,
				DrCr:          accountingpb.DrCr_CR, // debit user
				Amount:        req.Amount,
				Currency:      req.Currency,
			},
			{
				AccountNumber: partnerAccount,
				DrCr:          accountingpb.DrCr_DR, // credit partner
				Amount:        req.Amount,
				Currency:      req.Currency,
			},
		},
	}

	// Call gRPC to post transaction
	resp, err := h.accountingClient.Client.PostTransaction(r.Context(), grpcReq)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to post transaction: "+err.Error())
		return
	}

	// Success response
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message":         "deposit successful",
		"user_account":    userAccount,
		"partner_account": partnerAccount,
		"transaction":     resp,
	})
}
func (h *PaymentHandler) WithdrawHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Amount   float64 `json:"amount"`
		Method   string  `json:"method"`
		Service  string  `json:"service"`  // service to pick partner from
		Currency string  `json:"currency"` // user wallet currency
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	// For now, assume user withdraws from USD wallet → partner KES account
	req.Currency = "USD"     // user wallet currency
	withdrawCurrency := "KES" // partner account currency

	// Get authenticated user
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	// Get both accounts concurrently
	partnerAccount, userAccount, err := h.GetPartnerAndUserAccounts(r.Context(), req.Service, req.Currency, userID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "your request has been declined: "+err.Error())
		return
	}

	// Parse userID to int64 for gRPC
	userIDInt, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	// Construct accounting transaction (User → Partner)
	grpcReq := &accountingpb.CreateTransactionRequest{
		Description:   fmt.Sprintf("Withdrawal by user %s via %s", userID, req.Method),
		CreatedByType: accountingpb.OwnerType_USER,
		CreatedByUser: userIDInt,
		DepositCurrency: withdrawCurrency,
		Entries: []*accountingpb.TransactionEntry{
			{
				AccountNumber: userAccount,
				DrCr:          accountingpb.DrCr_DR, // Debit user wallet (money leaves)
				Amount:        req.Amount,
				Currency:      req.Currency,
			},
			{
				AccountNumber: partnerAccount,
				DrCr:          accountingpb.DrCr_CR, // Credit partner account (money arrives in M-Pesa)
				Amount:        req.Amount,
				Currency:      req.Currency,
			},
		},
	}

	// Call gRPC to post transaction
	resp, err := h.accountingClient.Client.PostTransaction(r.Context(), grpcReq)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to post transaction: "+err.Error())
		return
	}

	// Success response
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message":         "withdrawal successful",
		"user_account":    userAccount,
		"partner_account": partnerAccount,
		"transaction":     resp,
	})
}



func mapOwnerType(s string) accountingpb.OwnerType {
	switch s {
	case "user":
		return accountingpb.OwnerType_USER
	case "partner":
		return accountingpb.OwnerType_PARTNER
	case "system":
		return accountingpb.OwnerType_SYSTEM
	case "admin":
		return accountingpb.OwnerType_ADMIN
	default:
		return accountingpb.OwnerType_OWNER_TYPE_UNSPECIFIED
	}
}
// GetPartnersByService fetches partners offering a specific service
// GetPartnersByService fetches partners by service
// GetPartnersByService fetches partners offering a specific service
func (h *PaymentHandler) GetPartnersByService(ctx context.Context, service string) ([]*partnersvcpb.Partner, error) {
	if service == "" {
		return nil, fmt.Errorf("service cannot be empty")
	}

	req := &partnersvcpb.GetPartnersByServiceRequest{
		Service: service,
	}

	res, err := h.partnerClient.Client.GetPartnersByService(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch partners by service '%s': %w", service, err)
	}

	return res.Partners, nil
}

func (h *PaymentHandler) GetUserAccountsHandler(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from context
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	// Default owner type = user
	ownerType := "user"

	// Call helper
	accountsResp, err := h.GetAccounts(r.Context(), userID, ownerType)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to fetch accounts: "+err.Error())
		return
	}

	// Success
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message":  "accounts retrieved successfully",
		"accounts": accountsResp.Accounts,
	})
}


// GetAccounts fetches accounts for a given owner ID and ownerType
func (h *PaymentHandler) GetAccounts(ctx context.Context, ownerID, ownerType string) (*accountingpb.GetAccountsResponse, error) {
	if ownerID == "" || ownerType == "" {
		return nil, fmt.Errorf("ownerID and ownerType cannot be empty")
	}

	req := &accountingpb.GetAccountsRequest{
		OwnerType: mapOwnerType(ownerType),
		OwnerId:   ownerID,
	}

	resp, err := h.accountingClient.Client.GetUserAccounts(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch accounts for ownerID=%s, ownerType=%s: %w", ownerID, ownerType, err)
	}

	return resp, nil
}

// GetAccountByCurrency fetches a single account number for a given ownerID, ownerType, and currency
func (h *PaymentHandler) GetAccountByCurrency(ctx context.Context, ownerID, ownerType, currency string) (string, error) {
	if ownerID == "" || ownerType == "" || currency == "" {
		return "", fmt.Errorf("ownerID, ownerType, and currency cannot be empty")
	}

	resp, err := h.GetAccounts(ctx, ownerID, ownerType)
	if err != nil {
		return "", err
	}

	for _, acct := range resp.Accounts {
		if acct.Currency == currency {
			return acct.AccountNumber, nil
		}
	}

	return "", fmt.Errorf("no account found for ownerID=%s, ownerType=%s with currency=%s", ownerID, ownerType, currency)
}


// Concurrent helper: fetch random partner and their account + user account
func (h *PaymentHandler) GetPartnerAndUserAccounts(ctx context.Context, service, currency, userID string) (partnerAccount, userAccount string, err error) {
	if service == "" || currency == "" || userID == "" {
		return "", "", fmt.Errorf("service, currency, and userID cannot be empty")
	}

	var wg sync.WaitGroup
	var partnerErr, userErr error
	var partners []*partnersvcpb.Partner
	var selectedPartner *partnersvcpb.Partner

	wg.Add(2)

	// Fetch partners concurrently
	go func() {
		defer wg.Done()
		partners, partnerErr = h.GetPartnersByService(ctx, service)
		if partnerErr != nil || len(partners) == 0 {
			if partnerErr == nil {
				partnerErr = fmt.Errorf("no partners found for service %s", service)
			}
			return
		}
		rand.Seed(time.Now().UnixNano())
		selectedPartner = partners[rand.Intn(len(partners))]
	}()

	// Fetch user account concurrently
	go func() {
		defer wg.Done()
		userAccount, userErr = h.GetAccountByCurrency(ctx, userID, "user", currency)
	}()

	wg.Wait()

	if partnerErr != nil {
		return "", "", fmt.Errorf("partner error: %w", partnerErr)
	}
	if userErr != nil {
		return "", "", fmt.Errorf("user account error: %w", userErr)
	}

	// Fetch selected partner's account
	partnerAccount, err = h.GetAccountByCurrency(ctx, selectedPartner.Id, "partner", currency)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch partner account: %w", err)
	}

	return partnerAccount, userAccount, nil
}
