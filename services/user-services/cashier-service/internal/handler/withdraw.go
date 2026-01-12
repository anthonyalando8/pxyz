// handler/withdrawal.go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"cashier-service/internal/domain"
	convsvc"cashier-service/internal/service"
	partnersvcpb "x/shared/genproto/partner/svcpb"
	accountingpb "x/shared/genproto/shared/accounting/v1"
    "x/shared/utils/id"

	"go.uber.org/zap"
)

// WithdrawalRequest represents the withdrawal request structure
type WithdrawalRequest struct {
	Amount            float64 `json:"amount"`
	LocalCurrency     string  `json:"local_currency"`
	Service           *string `json:"service,omitempty"`
	AgentID           *string `json:"agent_id,omitempty"`
	Destination       string  `json:"destination"`
	VerificationToken string  `json:"verification_token"`
	Consent           bool    `json:"consent"`
}

// WithdrawalContext holds all the data needed for withdrawal processing
type WithdrawalContext struct {
	Request         *WithdrawalRequest
	UserIDInt       int64
	Service         string
	PhoneNumber     string
	BankAccount     string
	SelectedPartner *partnersvcpb.Partner
	AmountInUSD     float64
	ExchangeRate    float64
	Agent           *accountingpb.Agent
	AgentAccount    *accountingpb.Account
	IsAgentFlow     bool
	UserAccount     string
}

// Main handler - orchestrates the withdrawal flow
func (h *PaymentHandler) handleWithdrawRequest(ctx context.Context, client *Client, data json.RawMessage) {
	// Step 1: Parse request
	req, err := h.parseWithdrawalRequest(data)
	if err != nil {
		client.SendError(err.Error())
		return
	}

	// Step 2: Validate verification token
	if err := h.validateWithdrawalVerification(ctx, client, req); err != nil {
		client.SendError(err. Error())
		return
	}

	// Step 3: Validate request data
	if err := h.validateWithdrawalRequest(req); err != nil {
		client.SendError(err.Error())
		return
	}

	userIDInt, _ := strconv. ParseInt(client.UserID, 10, 64)

	// Step 4: Fetch and validate user profile
	phoneNumber, bankAccount, err := h.validateUserProfileForWithdrawal(ctx, client. UserID, req)
	if err != nil {
		client. SendError(err.Error())
		return
	}

	// Step 5: Select partner and convert currency
	selectedPartner, amountInUSD, exchangeRate, err := h.selectPartnerAndConvert(ctx, client. UserID, req)
	if err != nil {
		client.SendError(err. Error())
		return
	}

	// Step 6: Handle agent flow (if applicable)
	agent, agentAccount, isAgentFlow, err := h. handleAgentFlow(ctx, client, req, userIDInt)
	if err != nil {
		client.SendError(err. Error())
		return
	}

	// Step 7: Get user account and check balance
	userAccount, err := h.validateUserBalance(ctx, client.UserID, amountInUSD, req.Amount, req.LocalCurrency)
	if err != nil {
		client.SendError(err.Error())
		return
	}

	// Step 8: Create withdrawal request
	withdrawalReq, err := h.createWithdrawalRequest(userIDInt, req, selectedPartner, amountInUSD, exchangeRate, phoneNumber, bankAccount)
	if err != nil {
		client.SendError(err. Error())
		return
	}

	// Step 9: Process withdrawal based on flow
	h.processWithdrawalFlow(client, withdrawalReq, userAccount, req, selectedPartner, agent, agentAccount, isAgentFlow, amountInUSD, exchangeRate, phoneNumber, bankAccount)
}

// ============================================
// HELPER FUNCTIONS
// ============================================

// parseWithdrawalRequest parses and returns the withdrawal request
func (h *PaymentHandler) parseWithdrawalRequest(data json.RawMessage) (*WithdrawalRequest, error) {
	var req WithdrawalRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request format")
	}
	
	// Round amount to 2 decimal places
	req.Amount = roundTo2Decimals(req.Amount)
	
	return &req, nil
}

// validateWithdrawalVerification validates the verification token
func (h *PaymentHandler) validateWithdrawalVerification(ctx context.Context, client *Client, req *WithdrawalRequest) error {
	if req.VerificationToken == "" {
		return fmt.Errorf("verification_token is required.  Please complete verification first")
	}

	if ! req. Consent {
		return fmt. Errorf("you have to consent before processing a withdrawal")
	}

	tokenUserID, err := h.validateVerificationToken(ctx, req. VerificationToken, VerificationPurposeWithdrawal)
	if err != nil {
		h.logger. Warn("invalid verification token",
			zap.String("user_id", client.UserID),
			zap.Error(err))
		return fmt.Errorf("invalid or expired verification token.  Please verify again")
	}

	if tokenUserID != client.UserID {
		h.logger.Warn("token user mismatch",
			zap. String("client_user_id", client.UserID),
			zap.String("token_user_id", tokenUserID))
		return fmt.Errorf("verification token does not belong to you")
	}

	return nil
}

// validateWithdrawalRequest validates the withdrawal request data
func (h *PaymentHandler) validateWithdrawalRequest(req *WithdrawalRequest) error {
	if req.Amount <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}
	if req.Amount < 10 {
		return fmt. Errorf("amount must be at least 10")
	}
	if req.Amount > 999999999999999999.99 {
		return fmt. Errorf("amount exceeds maximum allowed value")
	}
	if req. Destination == "" {
		return fmt.Errorf("destination is required")
	}
	return nil
}

// validateUserProfileForWithdrawal fetches profile and validates payment method requirements
// If destination is provided, it takes priority over profile phone/bank account
func (h *PaymentHandler) validateUserProfileForWithdrawal(ctx context.Context, userID string, req *WithdrawalRequest) (string, string, error) {
	// Fetch user profile
	profile, err := h. profileFetcher.FetchProfile(ctx, "user", userID)
	if err != nil {
		h.logger.Error("failed to fetch user profile",
			zap.String("user_id", userID),
			zap.Error(err))
		return "", "", fmt.Errorf("failed to fetch user profile")
	}

	// Determine service
	service := "mpesa" // default
	if req.Service != nil {
		service = *req.Service
	}

	var phoneNumber, bankAccount string

	// Check phone number requirement
	if servicesRequiringPhone[service] {
		// ✅ Priority: Use provided destination, then fall back to profile phone
		if req.Destination != "" {
			phoneNumber = req.Destination
			h.logger.Info("using provided destination for withdrawal",
				zap.String("user_id", userID),
				zap.String("service", service),
				zap.String("destination", phoneNumber))
		} else if profile.Phone != "" {
			phoneNumber = profile.Phone
			h.logger.Info("using profile phone number for withdrawal",
				zap.String("user_id", userID),
				zap.String("service", service),
				zap.String("phone", phoneNumber))
		} else {
			h.logger.Warn("phone number required but not provided",
				zap.String("user_id", userID),
				zap.String("service", service))
			return "", "", fmt. Errorf("phone number is required for this payment method. Please provide a destination or add your phone number in profile settings")
		}
	}

	// Check bank account requirement
	if servicesRequiringBank[service] {
		// ✅ Priority: Use provided destination, then fall back to profile bank account
		if req.Destination != "" {
			bankAccount = req.Destination
			h.logger.Info("using provided destination for withdrawal",
				zap.String("user_id", userID),
				zap.String("service", service),
				zap.String("destination", bankAccount))
		} else if profile.BankAccount != "" {
			bankAccount = profile.BankAccount
			h.logger.Info("using profile bank account for withdrawal",
				zap.String("user_id", userID),
				zap.String("service", service),
				zap.String("bank_account", bankAccount))
		} else {
			h.logger.Warn("bank account required but not provided",
				zap.String("user_id", userID),
				zap.String("service", service))
			return "", "", fmt. Errorf("bank account is required for this payment method. Please provide a destination or add your bank account in profile settings")
		}
	}

	return phoneNumber, bankAccount, nil
}

// selectPartnerAndConvert selects a partner and converts currency
func (h *PaymentHandler) selectPartnerAndConvert(ctx context.Context, userID string, req *WithdrawalRequest) (*partnersvcpb.Partner, float64, float64, error) {
	// Determine service
	service := "mpesa" // default
	if req.Service != nil {
		service = *req.Service
	}

	// Get partners for service
	partners, err := h. GetPartnersByService(ctx, service)
	if err != nil || len(partners) == 0 {
		return nil, 0, 0, fmt.Errorf("no partners available for currency conversion")
	}

	selectedPartner := SelectRandomPartner(partners)
	req.LocalCurrency = selectedPartner.LocalCurrency

	// Convert to USD
	currencyService := convsvc.NewCurrencyService(h.partnerClient)
	amountInUSD, exchangeRate, err := currencyService.ConvertToUSDWithValidation(ctx, selectedPartner, req.Amount)
	if err != nil {
		h.logger.Error("currency conversion failed",
			zap. String("user_id", userID),
			zap.Float64("amount", req.Amount),
			zap.String("currency", req.LocalCurrency),
			zap.Error(err))
		return nil, 0, 0, fmt.Errorf("currency conversion failed: %v", err)
	}

	return selectedPartner, amountInUSD, exchangeRate, nil
}

// handleAgentFlow handles agent-specific logic if agent ID is provided
func (h *PaymentHandler) handleAgentFlow(ctx context.Context, client *Client, req *WithdrawalRequest, userIDInt int64) (*accountingpb.Agent, *accountingpb.Account, bool, error) {
	if req.AgentID == nil || *req.AgentID == "" {
		return nil, nil, false, nil // Not agent flow
	}

	// Fetch agent with accounts
	agentResp, err := h.accountingClient.Client.GetAgentByID(ctx, &accountingpb.GetAgentByIDRequest{
		AgentExternalId: *req.AgentID,
		IncludeAccounts:  true,
	})
	if err != nil {
		h. logger.Error("failed to get agent",
			zap.String("agent_id", *req. AgentID),
			zap.Error(err))
		return nil, nil, false, fmt. Errorf("invalid agent:  %v", err)
	}

	if agentResp.Agent == nil || ! agentResp.Agent.IsActive {
		return nil, nil, false, fmt.Errorf("agent not found or inactive")
	}

	agent := agentResp.Agent

	// Find agent's USD account
	var agentAccount *accountingpb.Account
	for _, acc := range agent.Accounts {
		if acc.Currency == "USD" &&
			(acc.Purpose == accountingpb.AccountPurpose_ACCOUNT_PURPOSE_WALLET ||
				acc.Purpose == accountingpb.AccountPurpose_ACCOUNT_PURPOSE_COMMISSION) &&
			acc.IsActive && ! acc.IsLocked {
			agentAccount = acc
			break
		}
	}

	if agentAccount == nil {
		return nil, nil, false, fmt.Errorf("agent does not have an active USD account")
	}

	h.logger.Info("withdrawal to agent",
		zap. Int64("user_id", userIDInt),
		zap.String("agent_id", agent.AgentExternalId),
		zap.String("agent_account", agentAccount.AccountNumber))

	return agent, agentAccount, true, nil
}

// validateUserBalance gets user account and checks if they have sufficient balance
func (h *PaymentHandler) validateUserBalance(ctx context.Context, userID string, amountInUSD, amountLocal float64, localCurrency string) (string, error) {
	// Get user USD account
	userAccount, err := h.GetAccountByCurrency(ctx, userID, "user", "USD")
	if err != nil {
		h.logger.Error("failed to get user account",
			zap. String("user_id", userID),
			zap.Error(err))
		return "", fmt.Errorf("failed to get user account: %v", err)
	}

	// Check balance
	balanceResp, err := h. accountingClient.Client.GetBalance(ctx, &accountingpb.GetBalanceRequest{
		Identifier: &accountingpb.GetBalanceRequest_AccountNumber{
			AccountNumber: userAccount,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to check balance: %v", err)
	}

	if balanceResp.Balance. AvailableBalance < amountInUSD {
		return "", fmt.Errorf("insufficient balance: need %.2f USD (%.2f %s), have %.2f USD",
			amountInUSD, amountLocal, localCurrency, balanceResp.Balance.AvailableBalance)
	}

	return userAccount, nil
}

// createWithdrawalRequest creates and saves the withdrawal request
func (h *PaymentHandler) createWithdrawalRequest(userIDInt int64, req *WithdrawalRequest, partner *partnersvcpb.Partner, amountInUSD, exchangeRate float64, phoneNumber, bankAccount string) (*domain.WithdrawalRequest, error) {
	ctx := context.Background()

	withdrawalReq := &domain.WithdrawalRequest{
		UserID:          userIDInt,
		RequestRef:      id.GenerateTransactionID("WD"),
		Amount:          amountInUSD,
		Currency:        "USD",
		Destination:     req.Destination,
		Service:         req.Service,
		AgentExternalID: req.AgentID,
		PartnerID:       &partner.Id,
		Status:          domain.WithdrawalStatusPending,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Store original amount and rate
	withdrawalReq.SetOriginalAmount(req.Amount, req.LocalCurrency, exchangeRate)

	// Add phone and bank to metadata
	if withdrawalReq.Metadata == nil {
		withdrawalReq. Metadata = make(map[string]interface{})
	}
	if phoneNumber != "" {
		withdrawalReq.Metadata["phone_number"] = phoneNumber
	}
	if bankAccount != "" {
		withdrawalReq. Metadata["bank_account"] = bankAccount
	}

	// Save to database
	if err := h. userUc.CreateWithdrawalRequest(ctx, withdrawalReq); err != nil {
		h.logger.Error("failed to create withdrawal request",
			zap.Int64("user_id", userIDInt),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create withdrawal request: %v", err)
	}

	h.logger.Info("withdrawal request created",
		zap.String("request_ref", withdrawalReq.RequestRef),
		zap.Int64("user_id", userIDInt),
		zap.Float64("amount_usd", amountInUSD),
		zap.Float64("amount_local", req. Amount),
		zap.String("currency_local", req.LocalCurrency),
		zap.String("phone_number", phoneNumber))

	return withdrawalReq, nil
}

// processWithdrawalFlow processes the withdrawal based on the flow type
func (h *PaymentHandler) processWithdrawalFlow(
	client *Client,
	withdrawalReq *domain.WithdrawalRequest,
	userAccount string,
	req *WithdrawalRequest,
	partner *partnersvcpb.Partner,
	agent *accountingpb.Agent,
	agentAccount *accountingpb.Account,
	isAgentFlow bool,
	amountInUSD, exchangeRate float64,
	phoneNumber, bankAccount string,
) {
	if isAgentFlow && agent != nil {
		// Agent flow
		go h.processWithdrawalToAgent(withdrawalReq, userAccount, agentAccount. AccountNumber, agent, req. Amount, req.LocalCurrency, phoneNumber, bankAccount)

		client.SendSuccess("withdrawal request created and being processed", map[string]interface{}{
			"request_ref":      withdrawalReq.RequestRef,
			"amount_usd":       amountInUSD,
			"amount_local":     req.Amount,
			"local_currency":   req.LocalCurrency,
			"exchange_rate":    exchangeRate,
			"destination":      req.Destination,
			"withdrawal_type":  "agent_assisted",
			"agent_id":         agent.AgentExternalId,
			"agent_name":       agent.Name,
			"status":           "processing",
			"phone_number":     phoneNumber,
		})
	} else if req.Service != nil && *req.Service != "" {
		// Partner flow
		go h.processWithdrawalViaPartner(withdrawalReq, userAccount, partner, req.Amount, req.LocalCurrency, phoneNumber, bankAccount)

		client.SendSuccess("withdrawal request created and being processed", map[string]interface{}{
			"request_ref":      withdrawalReq.RequestRef,
			"amount_usd":       amountInUSD,
			"amount_local":     req.Amount,
			"local_currency":   req.LocalCurrency,
			"exchange_rate":    exchangeRate,
			"destination":      req.Destination,
			"withdrawal_type":  "partner",
			"partner_id":       partner.Id,
			"partner_name":     partner.Name,
			"status":           "processing",
			"phone_number":      phoneNumber,
		})
	} else {
		// Direct flow
		go h.processWithdrawal(withdrawalReq, userAccount, req. Amount, req.LocalCurrency)

		client.SendSuccess("withdrawal request created and being processed", map[string]interface{}{
			"request_ref":      withdrawalReq.RequestRef,
			"amount_usd":       amountInUSD,
			"amount_local":     req.Amount,
			"local_currency":   req.LocalCurrency,
			"exchange_rate":    exchangeRate,
			"destination":      req.Destination,
			"withdrawal_type":  "direct",
			"status":           "processing",
		})
	}
}

// ✅ UPDATED: processWithdrawalToAgent with original amount tracking
// ✅ UPDATED: Pass phone and bank account to processing functions
func (h *PaymentHandler) processWithdrawalToAgent(
	withdrawal *domain.WithdrawalRequest,
	userAccount string,
	agentAccount string,
	agent *accountingpb.Agent,
	originalAmount float64,
	originalCurrency string,
	phoneNumber string,
	bankAccount string,
) {
	ctx := context.Background()

	if err := h.userUc.MarkWithdrawalProcessing(ctx, withdrawal.RequestRef); err != nil {
		log.Printf("[Withdrawal] Failed to mark as processing: %v", err)
		return
	}

	// Execute transfer (in USD)
	transferReq := &accountingpb.TransferRequest{
		FromAccountNumber:    userAccount,
		ToAccountNumber:     agentAccount,
		Amount:              withdrawal.Amount, // USD amount
		AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		Description:         fmt.Sprintf("Withdrawal %.2f %s to %s via agent %s",
			originalAmount, originalCurrency, withdrawal.Destination, *agent.Name),
		ExternalRef:         &withdrawal.RequestRef,
		CreatedByExternalId: fmt.Sprintf("%d", withdrawal.UserID),
		CreatedByType:       accountingpb.OwnerType_OWNER_TYPE_USER,
		AgentExternalId:     &agent.AgentExternalId,
		TransactionType:     accountingpb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL,
	}

	resp, err := h.accountingClient.Client.Transfer(ctx, transferReq)
	if err != nil {
		errMsg := err.Error()
		h.userUc.FailWithdrawal(ctx, withdrawal.RequestRef, errMsg)

		h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
            "type": "withdrawal_failed",
            "data": {
                "request_ref": "%s",
                "error":   "%s"
            }
        }`, withdrawal.RequestRef, errMsg)))
		return
	}

	if err := h.userUc.UpdateWithdrawalWithReceipt(ctx, withdrawal.ID, resp.ReceiptCode, resp.JournalId, true); err != nil {
		log.Printf("[Withdrawal] Failed to mark as completed: %v", err)
		return
	}

	h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
        "type": "withdrawal_completed",
        "data": {
            "request_ref": "%s",
            "receipt_code": "%s",
            "agent_id": "%s",
            "agent_name": "%s",
            "amount_usd":   %.2f,
            "amount_local": %.2f,
            "local_currency": "%s",
            "fee_amount": %.2f,
            "agent_commission": %.2f
        }
    }`, withdrawal.RequestRef, resp.ReceiptCode, agent.AgentExternalId, *agent.Name,
		withdrawal.Amount, originalAmount, originalCurrency, resp.FeeAmount, resp.AgentCommission)))
}
// ✅ UPDATED: processWithdrawal with original amount tracking
func (h *PaymentHandler) processWithdrawal(
    withdrawal *domain.WithdrawalRequest,
    userAccount string,
    originalAmount float64,
    originalCurrency string,
) {
    ctx := context.Background()

    if err := h.userUc.MarkWithdrawalProcessing(ctx, withdrawal.RequestRef); err != nil {
        log.Printf("[Withdrawal] Failed to mark as processing: %v", err)
        return
    }

    // Execute debit (in USD)
    debitReq := &accountingpb.DebitRequest{
        AccountNumber:        userAccount,
        Amount:              withdrawal.Amount, // USD amount
        //Currency:            "USD",
        AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
        Description:         fmt.Sprintf("Withdrawal %.2f %s to %s", 
            originalAmount, originalCurrency, withdrawal.Destination),
        ExternalRef:          &withdrawal.RequestRef,
        CreatedByExternalId:  fmt.Sprintf("%d", withdrawal.UserID),
        CreatedByType:       accountingpb.OwnerType_OWNER_TYPE_USER,
    }

    if withdrawal.Service != nil {
        debitReq.Description = fmt.Sprintf("Withdrawal %.2f %s to %s via %s", 
            originalAmount, originalCurrency, withdrawal.Destination, *withdrawal.Service)
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
		log.Printf("[Withdrawal] Failed to mark as completed: %v", err)
		return
	}

    h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
        "type": "withdrawal_completed",
        "data": {
            "request_ref": "%s",
            "receipt_code": "%s",
            "amount_usd": %.2f,
            "amount_local": %.2f,
            "local_currency": "%s",
            "balance_after": %.2f
        }
    }`, withdrawal.RequestRef, resp.ReceiptCode, withdrawal.Amount, originalAmount, originalCurrency, resp.BalanceAfter)))
}

// processWithdrawalViaPartner - Debit user and send to partner
// CORRECTED: processWithdrawalViaPartner - Transfer from user to partner account
// processWithdrawalViaPartner - Transfer from user to partner account
func (h *PaymentHandler) processWithdrawalViaPartner(
	withdrawal *domain.WithdrawalRequest,
	userAccount string,
	partner *partnersvcpb.Partner,
	originalAmount float64,
	originalCurrency string,
	phoneNumber string,
	bankAccount string,
) {
	ctx := context.Background()

	if err := h.userUc.MarkWithdrawalProcessing(ctx, withdrawal.RequestRef); err != nil {
		log.Printf("[Withdrawal] Failed to mark as processing: %v", err)
		return
	}

	// ✅ Step 1: Get partner's account
	partnerAccount, err := h.GetAccountByCurrency(ctx, partner.Id, "partner", "USD")
	if err != nil {
		errMsg := fmt.Sprintf("partner account not found: %v", err)
		h.userUc.FailWithdrawal(ctx, withdrawal.RequestRef, errMsg)

		h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
            "type": "withdrawal_failed",
            "data": {
                "request_ref": "%s",
                "error":  "%s"
            }
        }`, withdrawal.RequestRef, errMsg)))
		return
	}

	// ✅ Step 2: Transfer from user account to partner account (in USD)
	transferReq := &accountingpb.TransferRequest{
		FromAccountNumber:    userAccount,
		ToAccountNumber:     partnerAccount,
		Amount:              withdrawal.Amount, // USD amount
		AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		Description:         fmt.Sprintf("Withdrawal %.2f %s to %s via partner %s",
			originalAmount, originalCurrency, withdrawal.Destination, partner.Name),
		ExternalRef:         &withdrawal.RequestRef,
		CreatedByExternalId: fmt.Sprintf("%d", withdrawal.UserID),
		CreatedByType:       accountingpb.OwnerType_OWNER_TYPE_USER,
		TransactionType:     accountingpb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL,
	}

	resp, err := h.accountingClient.Client.Transfer(ctx, transferReq)
	if err != nil {
		errMsg := err.Error()
		h.userUc.FailWithdrawal(ctx, withdrawal.RequestRef, errMsg)

		h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
            "type": "withdrawal_failed",
            "data": {
                "request_ref": "%s",
                "error":  "%s"
            }
        }`, withdrawal.RequestRef, errMsg)))
		return
	}

	// ✅ Get exchange rate from withdrawal metadata
	var exchangeRate float64
	if rate, ok := withdrawal.Metadata["exchange_rate"].(float64); ok {
		exchangeRate = rate
	}

	// ✅ Build metadata with phone number and bank account
	metadata := map[string]string{
		"request_ref":       withdrawal.RequestRef,
		"destination":       withdrawal.Destination,
		"original_amount":   fmt.Sprintf("%.2f", originalAmount),      // ✅ REQUIRED
		"converted_amount":  fmt.Sprintf("%.2f", withdrawal.Amount),   // ✅ REQUIRED
		"original_currency": originalCurrency,                         // ✅ REQUIRED
		"target_currency":   "USD",                                    // ✅ REQUIRED
		"exchange_rate":     fmt.Sprintf("%.4f", exchangeRate),        // ✅ REQUIRED
		"receipt_code":      resp.ReceiptCode,
		"journal_id":        fmt.Sprintf("%d", resp.JournalId),
	}

	// ✅ Add phone number if available
	if phoneNumber != "" {
		metadata["phone_number"] = phoneNumber
	}

	// ✅ Add bank account if available
	if bankAccount != "" {
		metadata["bank_account"] = bankAccount
	}

	h.logger.Info("sending withdrawal webhook to partner",
		zap.String("partner_id", partner.Id),
		zap.String("request_ref", withdrawal.RequestRef),
		zap.String("phone_number", phoneNumber),
		zap.Any("metadata", metadata))

	// ✅ Step 3: Send withdrawal request to partner (for actual payout to user)
	partnerResp, err := h.partnerClient.Client.InitiateWithdrawal(ctx, &partnersvcpb.InitiateWithdrawalRequest{
		PartnerId:       partner.Id,
		TransactionRef: withdrawal.RequestRef,
		UserId:         fmt.Sprintf("%d", withdrawal.UserID),
		Amount:         withdrawal.Amount, // ✅ Send USD amount
		Currency:       "USD",             // ✅ Send USD currency
		PaymentMethod:  getPaymentMethod(withdrawal.Service),
		ExternalRef:    resp.ReceiptCode,
		Metadata:       metadata,
	})

	if err != nil {
		log.Printf("[Withdrawal] Failed to send to partner %s: %v", partner.Id, err)

		// ⚠️ Money already transferred to partner
		h.userUc.UpdateWithdrawalStatus(ctx, withdrawal.RequestRef, "sent_to_partner", nil)

		if withdrawal.Metadata == nil {
			withdrawal.Metadata = make(map[string]interface{})
		}
		withdrawal.Metadata["receipt_code"] = resp.ReceiptCode
		withdrawal.Metadata["journal_id"] = resp.JournalId
		withdrawal.Metadata["partner_notification_failed"] = true
		withdrawal.Metadata["partner_notification_error"] = err.Error()

		h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
            "type": "withdrawal_processing",
            "data": {
                "request_ref": "%s",
                "receipt_code": "%s",
                "message": "Withdrawal sent to partner for processing"
            }
        }`, withdrawal.RequestRef, resp.ReceiptCode)))
		return
	}

    h.userUc.UpdateWithdrawalWithReceipt(ctx, withdrawal.ID, resp.ReceiptCode, resp.JournalId, false)

	// ✅ Step 4: Mark as sent to partner
	h.userUc.UpdateWithdrawalStatus(ctx, withdrawal.RequestRef, "sent_to_partner", nil)

	// Store partner transaction reference
	if withdrawal.Metadata == nil {
		withdrawal.Metadata = make(map[string]interface{})
	}
	withdrawal.Metadata["partner_id"] = partner.Id
	withdrawal.Metadata["partner_name"] = partner.Name
	withdrawal.Metadata["partner_transaction_id"] = partnerResp.TransactionId
	withdrawal.Metadata["partner_transaction_ref"] = partnerResp.TransactionRef
	withdrawal.Metadata["receipt_code"] = resp.ReceiptCode
	withdrawal.Metadata["journal_id"] = resp.JournalId
	withdrawal.Metadata["sent_to_partner_at"] = time.Now()

	// ✅ Notify user
	h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
        "type": "withdrawal_sent_to_partner",
        "data": {
            "request_ref": "%s",
            "receipt_code": "%s",
            "partner_id":   "%s",
            "partner_name": "%s",
            "partner_transaction_id": %d,
            "partner_transaction_ref": "%s",
            "amount_usd": %.2f,
            "amount_local":   %.2f,
            "local_currency": "%s",
            "fee_amount": %.2f,
            "status": "sent_to_partner"
        }
    }`, withdrawal.RequestRef, resp.ReceiptCode, partner.Id, partner.Name,
		partnerResp.TransactionId, partnerResp.TransactionRef,
		withdrawal.Amount, originalAmount, originalCurrency, resp.FeeAmount)))
}

// ✅ Helper functions
func getPaymentMethod(service *string) string {
    if service != nil {
        return *service
    }
    return "default"
}

func getExchangeRate(metadata map[string]interface{}) string {
    if metadata == nil {
        return "0"
    }
    if rate, ok := metadata["exchange_rate"].(float64); ok {
        return fmt.Sprintf("%.4f", rate)
    }
    return "0"
}