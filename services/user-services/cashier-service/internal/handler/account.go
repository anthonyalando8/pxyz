package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	accountingpb "x/shared/genproto/shared/accounting/v1"

	"go.uber.org/zap"
	//"google.golang.org/protobuf/types/known/timestamppb"
)

// ============================================================================
// ACCOUNT OPERATIONS
// ============================================================================

// Get user accounts
func (h *PaymentHandler) handleGetAccounts(ctx context.Context, client *Client) {
	accountsResp, err := h.GetAccounts(ctx, client.UserID, "user")
	if err != nil {
		client.SendError("failed to fetch accounts: " + err.Error())
		return
	}

	// Format response with balances
	var formattedAccounts []map[string]interface{}
	for _, account := range accountsResp.Accounts {
		formattedAccounts = append(formattedAccounts, map[string]interface{}{
			"id":             account.Id,
			"account_number": account.AccountNumber,
			"currency":       account.Currency,
			"purpose":        account.Purpose.String(),
			"account_type":   account.AccountType.String(),
			"is_active":      account.IsActive,
			"is_locked":      account.IsLocked,
			"created_at":     account.CreatedAt.AsTime(),
		})
	}

	client.SendSuccess("accounts retrieved", map[string]interface{}{
		"accounts": formattedAccounts,
		"count":    len(formattedAccounts),
	})
}

// Get account balance
func (h *PaymentHandler) handleGetAccountBalance(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		AccountNumber string `json:"account_number"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	// Verify account ownership
	if err := h.ValidateAccountOwnership(ctx, req.AccountNumber, client.UserID, "user"); err != nil {
		client.SendError("unauthorized: " + err.Error())
		return
	}

	balance, err := h.GetAccountBalance(ctx, req.AccountNumber)
	if err != nil {
		client.SendError("failed to get balance: " + err.Error())
		return
	}

	// The accounting service now returns decimal amounts (float64). Use them directly.
	client.SendSuccess("balance retrieved", map[string]interface{}{
		"account_number":    balance.AccountNumber,
		"balance":           balance.Balance,
		"available_balance": balance.AvailableBalance,
		"pending_debit":     balance.PendingDebit,
		"pending_credit":    balance.PendingCredit,
		"currency":          balance.Currency,
		"last_transaction":  balance.LastTransactionAt.AsTime(),
	})
}

// Supported currencies (can be moved to config/database)
var SupportedCurrencies = []string{
	"USD", "EUR", "GBP", "BTC", "ETH", "USDT", "USDC",
	"KES", "TZS", "UGX", "RWF", // East African currencies
	"ZAR", "NGN", "GHS",         // Other African currencies
}

// handleCreateAccount creates a new wallet account for user
func (h *PaymentHandler) handleCreateAccount(ctx context. Context, client *Client, data json.RawMessage) {
	var req struct {
		Currency    string `json:"currency"`
		AccountType string `json:"account_type"` // "real" or "demo" (optional, defaults to "real")
	}

	if err := json. Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	// Validate currency
	req.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))
	if req.Currency == "" {
		client.SendError("currency is required")
		return
	}

	if ! isSupportedCurrency(req. Currency) {
		client.SendError(fmt.Sprintf("currency '%s' is not supported.  Use get_supported_currencies to see available currencies", req.Currency))
		return
	}

	// Default to real account
	if req.AccountType == "" {
		req.AccountType = "real"
	}

	// Validate account type
	req.AccountType = strings.ToLower(strings.TrimSpace(req.AccountType))
	if req.AccountType != "real" && req.AccountType != "demo" {
		client.SendError("account_type must be 'real' or 'demo'")
		return
	}

	// Check if account already exists
	existingAccounts, err := h.GetAccounts(ctx, client.UserID, "user")
	if err != nil {
		client. SendError("failed to check existing accounts: " + err.Error())
		return
	}

	// Check for duplicate
	accountTypeEnum := accountingpb.AccountType_ACCOUNT_TYPE_REAL
	accountTypeStr := "REAL"
	if req.AccountType == "demo" {
		accountTypeEnum = accountingpb.AccountType_ACCOUNT_TYPE_DEMO
		accountTypeStr = "DEMO"
	}

	for _, acc := range existingAccounts. Accounts {
		if acc. Currency == req.Currency && 
		   acc.AccountType == accountTypeEnum && 
		   acc.Purpose == accountingpb.AccountPurpose_ACCOUNT_PURPOSE_WALLET {
			client.SendError(fmt.Sprintf("%s %s wallet account already exists:  %s", 
				req.Currency, accountTypeStr, acc.AccountNumber))
			return
		}
	}

	// Create the account
	createReq := &accountingpb.CreateAccountsRequest{
		Accounts: []*accountingpb.CreateAccountRequest{
			{
				OwnerType:   accountingpb.OwnerType_OWNER_TYPE_USER,
				OwnerId:     client.UserID,
				Currency:    req.Currency,
				Purpose:     accountingpb.AccountPurpose_ACCOUNT_PURPOSE_WALLET,
				AccountType: accountTypeEnum,
			},
		},
	}

	resp, err := h.accountingClient.Client.CreateAccounts(ctx, createReq)
	if err != nil {
		h.logger.Error("failed to create account",
			zap.String("user_id", client.UserID),
			zap.String("currency", req.Currency),
			zap.String("account_type", req.AccountType),
			zap.Error(err))
		client.SendError("failed to create account: " + err.Error())
		return
	}

	if resp == nil || len(resp.Accounts) == 0 {
		if len(resp.Errors) > 0 {
			client.SendError("failed to create account: " + resp.Errors[0])
		} else {
			client.SendError("failed to create account:  unknown error")
		}
		return
	}

	// Get the created account
	account := resp.Accounts[0]

	h.logger.Info("account created successfully",
		zap.String("user_id", client.UserID),
		zap.String("currency", req.Currency),
		zap.String("account_type", req.AccountType),
		zap.String("account_number", account.AccountNumber),
		zap.Int64("account_id", account. Id))

	// Send success response
	client.SendSuccess("account created successfully", map[string]interface{}{
		"account":  map[string]interface{}{
			"id":             account.Id,
			"account_number": account.AccountNumber,
			"currency":       account. Currency,
			"purpose":        account.Purpose.String(),
			"account_type":   account. AccountType.String(),
			"is_active":      account.IsActive,
			"is_locked":      account.IsLocked,
			"created_at":     account.CreatedAt.AsTime(),
		},
		"message": fmt.Sprintf("%s %s wallet account created", req.Currency, accountTypeStr),
	})
}

// handleGetSupportedCurrencies returns list of supported currencies
func (h *PaymentHandler) handleGetSupportedCurrencies(ctx context. Context, client *Client) {
	currencies := make([]map[string]interface{}, 0, len(SupportedCurrencies))
	
	for _, curr := range SupportedCurrencies {
		currencies = append(currencies, map[string]interface{}{
			"code":   curr,
			"name":   getCurrencyName(curr),
			"symbol": getCurrencySymbol(curr),
			"type":   getCurrencyType(curr),
		})
	}

	client.SendSuccess("supported currencies", map[string]interface{}{
		"currencies": currencies,
		"count":      len(currencies),
	})
}

// Helper:  Check if currency is supported
func isSupportedCurrency(currency string) bool {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	for _, supported := range SupportedCurrencies {
		if supported == currency {
			return true
		}
	}
	return false
}

// Helper: Get currency name
func getCurrencyName(code string) string {
	names := map[string]string{
		"USD":   "US Dollar",
		"EUR":  "Euro",
		"GBP":  "British Pound",
		"BTC":  "Bitcoin",
		"ETH":   "Ethereum",
		"USDT":  "Tether",
		"USDC": "USD Coin",
		"KES":  "Kenyan Shilling",
		"TZS":  "Tanzanian Shilling",
		"UGX":  "Ugandan Shilling",
		"RWF":  "Rwandan Franc",
		"ZAR":  "South African Rand",
		"NGN":  "Nigerian Naira",
		"GHS":  "Ghanaian Cedi",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return code
}

// Helper: Get currency symbol
func getCurrencySymbol(code string) string {
	symbols := map[string]string{
		"USD":   "$",
		// "EUR":   "€",
		// "GBP":  "£",
		"BTC":  "₿",
		// "ETH":   "Ξ",
		"USDT": "₮",
		// "USDC": "$",
		// "KES":  "KSh",
		// "TZS":   "TSh",
		// "UGX":  "USh",
		// "RWF":  "FRw",
		// "ZAR":  "R",
		// "NGN":  "₦",
		// "GHS":  "GH₵",
	}
	if symbol, ok := symbols[code]; ok {
		return symbol
	}
	return code
}

// Helper: Get currency type
func getCurrencyType(code string) string {
	switch code {
	case "BTC", "ETH", "USDT", "USDC":
		return "crypto"
	default:
		return "fiat"
	}
}