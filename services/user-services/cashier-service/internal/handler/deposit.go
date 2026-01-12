// handler/deposit. go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"time"

	"cashier-service/internal/domain"
	"cashier-service/internal/service"
	usecase "cashier-service/internal/usecase/transaction"
	partnersvcpb "x/shared/genproto/partner/svcpb"
	accountingpb "x/shared/genproto/shared/accounting/v1"
    "x/shared/utils/id"

	"go.uber.org/zap"
)

// ✅ Service types that require phone numbers
var servicesRequiringPhone = map[string]bool{
	"mpesa":        true,
	"airtel_money": true,
	"mtn_money":    true,
	"orange_money": true,
}

// ✅ Service types that require bank account
var servicesRequiringBank = map[string]bool{
	"bank":           true,
	"bank_transfer": true,
}

// DepositRequest represents the deposit request structure
type DepositRequest struct {
	Amount         float64 `json:"amount"`
	LocalCurrency  string  `json:"local_currency"`
	TargetCurrency *string `json:"target_currency"`
	Service        string  `json:"service"`
	PartnerID      *string `json:"partner_id,omitempty"`
	AgentID        *string `json:"agent_id,omitempty"`
}

// Main handler - orchestrates the deposit flow
func (h *PaymentHandler) handleDepositRequest(ctx context.Context, client *Client, data json.RawMessage) {
	// Step 1: Parse and validate request
	req, err := h.parseDepositRequest(data)
	if err != nil {
		client.SendError(err.Error())
		return
	}

	// Step 2: Validate amount
	if err := h.validateDepositAmount(req); err != nil {
		client.SendError(err.Error())
		return
	}

	userIDInt, _ := strconv. ParseInt(client.UserID, 10, 64)

	// Step 3: Fetch and validate user profile
	phoneNumber, bankAccount, err := h.validateUserProfileForDeposit(ctx, client. UserID, req. Service)
	if err != nil {
		client.SendError(err.Error())
		return
	}

	// Step 4: Determine target currency
	targetCurrency := h.determineTargetCurrency(req)

	// Step 5: Handle agent or partner flow
	selectedPartner, selectedAgent, convertedAmount, exchangeRate, isAgentFlow, err := h.handleDepositFlow(ctx, client, req)
	if err != nil {
		client.SendError(err. Error())
		return
	}

	// Step 6: Create deposit request
	depositReq, err := h.createDepositRequest(userIDInt, req, selectedPartner, convertedAmount, exchangeRate, targetCurrency, phoneNumber, bankAccount)
	if err != nil {
		client. SendError(err.Error())
		return
	}

	// Step 7: Process based on flow
	h.processDepositFlow(client, depositReq, selectedPartner, selectedAgent, req, isAgentFlow, convertedAmount, exchangeRate, targetCurrency, phoneNumber, bankAccount)
}

// ============================================
// HELPER FUNCTIONS
// ============================================

// parseDepositRequest parses and returns the deposit request
func (h *PaymentHandler) parseDepositRequest(data json.RawMessage) (*DepositRequest, error) {
	var req DepositRequest
	if err := json. Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request format")
	}

	// Round amount to 2 decimal places
	req.Amount = roundTo2Decimals(req. Amount)

	return &req, nil
}

// validateDepositAmount validates the deposit amount
func (h *PaymentHandler) validateDepositAmount(req *DepositRequest) error {
	if req.Amount <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}
	if req.Amount < 10 {
		return fmt. Errorf("amount must be at least 10")
	}
	if req.Amount > 999999999999999999.99 {
		return fmt. Errorf("amount exceeds maximum allowed value")
	}
	if req.Service == "" {
		return fmt. Errorf("service is required")
	}
	return nil
}

// ✅ SHARED:  Can be used by both deposit and withdrawal
// validateUserProfile validates user profile and returns phone and bank account
func (h *PaymentHandler) validateUserProfile(ctx context.Context, userID, service string) (string, string, error) {
	// Fetch user profile
	profile, err := h.profileFetcher.FetchProfile(ctx, "user", userID)
	if err != nil {
		h.logger.Error("failed to fetch user profile",
			zap. String("user_id", userID),
			zap.String("service", service),
			zap.Error(err))
		return "", "", fmt.Errorf("failed to fetch user profile")
	}

	var phoneNumber, bankAccount string

	// Check phone number requirement
	if servicesRequiringPhone[service] {
		if profile.Phone == "" {
			h.logger. Warn("phone number required but not set",
				zap.String("user_id", userID),
				zap.String("service", service))
			return "", "", fmt. Errorf("phone number is required for this payment method.  Please add your phone number in profile settings")
		}
		phoneNumber = profile.Phone
		h.logger.Info("phone number validated",
			zap.String("user_id", userID),
			zap.String("service", service),
			zap.String("phone", phoneNumber))
	}

	// Check bank account requirement
	if servicesRequiringBank[service] {
		if profile.BankAccount == "" {
			h. logger.Warn("bank account required but not set",
				zap.String("user_id", userID),
				zap.String("service", service))
			return "", "", fmt.Errorf("bank account is required for this payment method. Please add your bank account in profile settings")
		}
		bankAccount = profile.BankAccount
		h.logger.Info("bank account validated",
			zap. String("user_id", userID),
			zap.String("service", service),
			zap.String("bank_account", bankAccount))
	}

	return phoneNumber, bankAccount, nil
}

// validateUserProfileForDeposit is a wrapper for deposit-specific profile validation
func (h *PaymentHandler) validateUserProfileForDeposit(ctx context.Context, userID, service string) (string, string, error) {
	return h.validateUserProfile(ctx, userID, service)
}

// determineTargetCurrency determines the target currency (defaults to USD)
func (h *PaymentHandler) determineTargetCurrency(req *DepositRequest) string {
	targetCurrency := "USD"
	if req. TargetCurrency != nil && *req.TargetCurrency != "" {
		targetCurrency = *req.TargetCurrency
	}
	return targetCurrency
}

// handleDepositFlow handles both agent and partner flows
func (h *PaymentHandler) handleDepositFlow(ctx context.Context, client *Client, req *DepositRequest) (*partnersvcpb.Partner, *accountingpb.Agent, float64, float64, bool, error) {
	currencyService := service.NewCurrencyService(h.partnerClient)

	if req.AgentID != nil && *req. AgentID != "" {
		// AGENT FLOW
		return h.handleAgentDepositFlow(ctx, client, req, currencyService)
	}

	// PARTNER FLOW
	return h.handlePartnerDepositFlow(ctx, client, req, currencyService)
}

// handleAgentDepositFlow handles the agent deposit flow
func (h *PaymentHandler) handleAgentDepositFlow(ctx context.Context, client *Client, req *DepositRequest, currencyService *service.CurrencyService) (*partnersvcpb.Partner, *accountingpb.Agent, float64, float64, bool, error) {
	// Fetch agent
	agentResp, err := h.accountingClient.Client.GetAgentByID(ctx, &accountingpb.GetAgentByIDRequest{
		AgentExternalId: *req.AgentID,
		IncludeAccounts:  false,
	})
	if err != nil || agentResp.Agent == nil {
		return nil, nil, 0, 0, false, fmt.Errorf("agent not found")
	}

	selectedAgent := agentResp.Agent
	if ! selectedAgent.IsActive {
		return nil, nil, 0, 0, false, fmt.Errorf("agent is not active")
	}

	// Get partner for currency conversion
	partners, err := h.GetPartnersByService(ctx, req.Service)
	if err != nil || len(partners) == 0 {
		return nil, nil, 0, 0, false, fmt.Errorf("no partners available for currency conversion")
	}

	selectedPartner := SelectRandomPartner(partners)
	req.LocalCurrency = selectedPartner.LocalCurrency

	// Convert currency
	convertedAmount, exchangeRate, err := currencyService. ConvertToUSDWithValidation(ctx, selectedPartner, req.Amount)
	if err != nil {
		h.logger.Error("currency conversion failed",
			zap.String("user_id", client.UserID),
			zap.Float64("amount", req.Amount),
			zap.String("currency", req.LocalCurrency),
			zap.Error(err))
		return nil, nil, 0, 0, false, fmt.Errorf("currency conversion failed: %v", err)
	}

	return selectedPartner, selectedAgent, convertedAmount, exchangeRate, true, nil
}

// handlePartnerDepositFlow handles the partner deposit flow
func (h *PaymentHandler) handlePartnerDepositFlow(ctx context.Context, client *Client, req *DepositRequest, currencyService *service.CurrencyService) (*partnersvcpb.Partner, *accountingpb.Agent, float64, float64, bool, error) {
	var selectedPartner *partnersvcpb.Partner
	var err error

	// Select partner
	if req.PartnerID != nil && *req.PartnerID != "" {
		if err := h.ValidatePartnerService(ctx, *req.PartnerID, req.Service); err != nil {
			return nil, nil, 0, 0, false, err
		}
		selectedPartner, err = h.GetPartnerByID(ctx, *req. PartnerID)
		if err != nil {
			return nil, nil, 0, 0, false, fmt.Errorf("partner not found")
		}
	} else {
		partners, err := h.GetPartnersByService(ctx, req.Service)
		if err != nil || len(partners) == 0 {
			return nil, nil, 0, 0, false, fmt.Errorf("no partners available for this service")
		}
		selectedPartner = SelectRandomPartner(partners)
	}

	req.LocalCurrency = selectedPartner.LocalCurrency

	// Convert currency
	convertedAmount, exchangeRate, err := currencyService.ConvertToUSDWithValidation(ctx, selectedPartner, req.Amount)
	if err != nil {
		h.logger.Error("currency conversion failed",
			zap.String("user_id", client.UserID),
			zap.Float64("amount", req.Amount),
			zap.String("currency", req. LocalCurrency),
			zap.Error(err))
		return nil, nil, 0, 0, false, fmt.Errorf("currency conversion failed: %v", err)
	}

	return selectedPartner, nil, convertedAmount, exchangeRate, false, nil
}

// createDepositRequest creates and saves the deposit request
func (h *PaymentHandler) createDepositRequest(
	userIDInt int64,
	req *DepositRequest,
	partner *partnersvcpb.Partner,
	convertedAmount, exchangeRate float64,
	targetCurrency, phoneNumber, bankAccount string,
) (*domain.DepositRequest, error) {
	ctx := context.Background()

	h.logger.Info("currency conversion completed",
		zap.Int64("user_id", userIDInt),
		zap.Float64("original_amount", req.Amount),
		zap.String("original_currency", req.LocalCurrency),
		zap.Float64("converted_amount", convertedAmount),
		zap.String("target_currency", targetCurrency),
		zap.Float64("exchange_rate", exchangeRate),
		zap.String("partner_id", partner.Id),
		zap.String("service", req.Service),
		zap.String("phone_number", phoneNumber))

	depositReq := &domain.DepositRequest{
		UserID:          userIDInt,
		PartnerID:       partner.Id,
		RequestRef:      id.GenerateTransactionID("DEP"),
		Amount:          convertedAmount,
		Currency:        targetCurrency,
		Service:         req.Service,
		AgentExternalID: req.AgentID,
		PaymentMethod:   strToPtr(req.Service),
		Status:          domain.DepositStatusPending,
		ExpiresAt:       time.Now().Add(30 * time.Minute),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Store original amount and rate
	depositReq.SetOriginalAmount(
		roundTo2Decimals(req.Amount),
		req.LocalCurrency,
		roundTo2Decimals(exchangeRate),
	)

	// Add phone and bank to metadata
	if depositReq. Metadata == nil {
		depositReq.Metadata = make(map[string]interface{})
	}
	if phoneNumber != "" {
		depositReq.Metadata["phone_number"] = phoneNumber
	}
	if bankAccount != "" {
		depositReq.Metadata["bank_account"] = bankAccount
	}

	// Save to database
	if err := h. userUc.CreateDepositRequest(ctx, depositReq); err != nil {
		h.logger.Error("failed to create deposit request",
			zap.Int64("user_id", userIDInt),
			zap.String("request_ref", depositReq.RequestRef),
			zap.Float64("amount", convertedAmount),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create deposit request: %v", err)
	}

	return depositReq, nil
}

// processDepositFlow processes the deposit based on the flow type
func (h *PaymentHandler) processDepositFlow(
	client *Client,
	depositReq *domain.DepositRequest,
	partner *partnersvcpb.Partner,
	agent *accountingpb.Agent,
	req *DepositRequest,
	isAgentFlow bool,
	convertedAmount, exchangeRate float64,
	targetCurrency, phoneNumber, bankAccount string,
) {
	if isAgentFlow && agent != nil {
		// Agent flow
		go h.sendDepositRequestToAgent(depositReq, agent, partner, phoneNumber, bankAccount)

		client.SendSuccess("deposit request sent to agent", map[string]interface{}{
			"request_ref":        depositReq.RequestRef,
			"agent_id":          agent.AgentExternalId,
			"agent_name":        agent.Name,
			"original_amount":   roundTo2Decimals(req. Amount),
			"original_currency": req.LocalCurrency,
			"converted_amount":  convertedAmount,
			"target_currency":   targetCurrency,
			"exchange_rate":     roundTo2Decimals(exchangeRate),
			"status":            "sent_to_agent",
			"expires_at":        depositReq.ExpiresAt,
			"phone_number":      phoneNumber,
			"service":           req.Service,
		})
	} else {
		// Partner flow
		go h.sendDepositWebhookToPartner(depositReq, partner, req.Amount, req.LocalCurrency, phoneNumber, bankAccount)
		h.userUc. MarkDepositSentToPartner(context.Background(), depositReq.RequestRef, "")

		client.SendSuccess("deposit request created", map[string]interface{}{
			"request_ref":        depositReq.RequestRef,
			"partner_id":         partner.Id,
			"partner_name":       partner.Name,
			"original_amount":    roundTo2Decimals(req.Amount),
			"original_currency":  req.LocalCurrency,
			"converted_amount":    convertedAmount,
			"target_currency":    targetCurrency,
			"exchange_rate":       roundTo2Decimals(exchangeRate),
			"status":             "sent_to_partner",
			"expires_at":         depositReq.ExpiresAt,
			"phone_number":       phoneNumber,
			"service":            req.Service,
		})
	}
}

// ============================================
// SHARED UTILITY FUNCTIONS
// ============================================

// ✅ SHARED: Round to 2 decimal places
func roundTo2Decimals(value float64) float64 {
	return math.Round(value*100) / 100
}


// ✅ UPDATED:  Placeholder for agent deposit with phone number
func (h *PaymentHandler) sendDepositRequestToAgent(
	deposit *domain.DepositRequest,
	agent *accountingpb.Agent,
	partner *partnersvcpb.Partner,
	phoneNumber string,
	bankAccount string,
) {
	ctx := context.Background()

	// TODO: Implement agent notification system (SMS, push notification, etc.)
	log.Printf("[Deposit] Deposit request %s sent to agent %s with phone %s (PLACEHOLDER)",
		deposit.RequestRef, agent.AgentExternalId, phoneNumber)

	// Mark as sent to agent
	h.userUc.UpdateDepositStatus(ctx, deposit.RequestRef, domain.DepositStatusSentToAgent, nil)

	// Store additional metadata
	if deposit.Metadata == nil {
		deposit.Metadata = make(map[string]interface{})
	}
	deposit.Metadata["agent_id"] = agent.AgentExternalId
	deposit. Metadata["agent_name"] = *agent.Name
	if phoneNumber != "" {
		deposit.Metadata["phone_number"] = phoneNumber
	}
	if bankAccount != "" {
		deposit.Metadata["bank_account"] = bankAccount
	}
}

// ✅ UPDATED: Send webhook to partner with phone number in metadata
func (h *PaymentHandler) sendDepositWebhookToPartner(
	deposit *domain.DepositRequest,
	partner *partnersvcpb.Partner,
	originalAmount float64,
	originalCurrency string,
	phoneNumber string,
	bankAccount string,
) {
	ctx := context.Background()

	// ✅ Build metadata with phone number and bank account
	metadata := map[string]string{
		"request_ref":       deposit.RequestRef,
		"original_amount":   fmt.Sprintf("%.2f", originalAmount),
		"converted_amount":  fmt.Sprintf("%.2f", deposit.Amount),
		"original_currency": originalCurrency,
		"target_currency":   deposit.Currency,
		"exchange_rate":      fmt.Sprintf("%.4f", deposit. Metadata["exchange_rate"].(float64)),
	}

	// ✅ Add phone number if available
	if phoneNumber != "" {
		metadata["phone_number"] = phoneNumber
	}

	// ✅ Add bank account if available
	if bankAccount != "" {
		metadata["bank_account"] = bankAccount
	}

	// ✅ Add account number if available
	if accountNumber, ok := deposit.Metadata["account_number"].(string); ok && accountNumber != "" {
		metadata["account_number"] = accountNumber
	}

	h.logger.Info("sending deposit webhook to partner",
		zap.String("partner_id", partner.Id),
		zap.String("request_ref", deposit.RequestRef),
		zap.String("phone_number", phoneNumber),
		zap.String("service", deposit.Service),
		zap.Any("metadata", metadata))

	// Send original amount in local currency to partner
	_, err := h.partnerClient.Client.InitiateDeposit(ctx, &partnersvcpb.InitiateDepositRequest{
		PartnerId:       partner.Id,
		TransactionRef: deposit.RequestRef,
		UserId:         fmt.Sprintf("%d", deposit.UserID),
		Amount:         deposit.Amount,
		Currency:       deposit.Currency,
		PaymentMethod:  ptrToStr(deposit.PaymentMethod),
		Metadata:       metadata,
	})

	if err != nil {
		h.logger.Error("failed to send webhook to partner",
			zap. String("partner_id", partner. Id),
			zap.String("request_ref", deposit.RequestRef),
			zap.Error(err))
		h.userUc. FailDeposit(ctx, deposit.RequestRef, err.Error())
	} else {
		h.logger.Info("deposit webhook sent successfully",
			zap.String("partner_id", partner.Id),
			zap.String("request_ref", deposit.RequestRef))
	}
}

// Get deposit status (updated to include phone number)
func (h *PaymentHandler) handleGetDepositStatus(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		RequestRef string `json:"request_ref"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	userIDInt, _ := strconv.ParseInt(client.UserID, 10, 64)

	deposit, err := h.userUc.GetDepositDetails(ctx, req.RequestRef, userIDInt)
	if err != nil {
		if err == usecase.ErrUnauthorized {
			client.SendError("unauthorized")
		} else {
			client.SendError("deposit not found")
		}
		return
	}

	// ✅ Build response with both original and converted amounts
	response := map[string]interface{}{
		"id":                      deposit.ID,
		"request_ref":             deposit.RequestRef,
		"converted_amount":        deposit.Amount,
		"target_currency":         deposit.Currency,
		"status":                  deposit.Status,
		"service":                 deposit.Service,
		"payment_method":          deposit.PaymentMethod,
		"partner_transaction_ref": deposit.PartnerTransactionRef,
		"receipt_code":            deposit.ReceiptCode,
		"error_message":           deposit.ErrorMessage,
		"expires_at":              deposit.ExpiresAt,
		"created_at":              deposit.CreatedAt,
		"completed_at":            deposit.CompletedAt,
	}

	// Add original amount if available
	if origAmount, origCurrency, rate, ok := deposit.GetOriginalAmount(); ok {
		response["original_amount"] = origAmount
		response["original_currency"] = origCurrency
		response["exchange_rate"] = rate
	}

	// ✅ Add phone number if available
	if phoneNumber, ok := deposit.Metadata["phone_number"].(string); ok && phoneNumber != "" {
		response["phone_number"] = phoneNumber
	}

	// ✅ Add bank account if available
	if bankAccount, ok := deposit. Metadata["bank_account"].(string); ok && bankAccount != "" {
		response["bank_account"] = bankAccount
	}

	// Add agent info if present
	if deposit.AgentExternalID != nil {
		response["agent_id"] = *deposit.AgentExternalID
		if agentName, ok := deposit. Metadata["agent_name"]; ok {
			response["agent_name"] = agentName
		}
	}

	client.SendSuccess("deposit status", response)
}

// Cancel deposit (unchanged)
func (h *PaymentHandler) handleCancelDeposit(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		RequestRef string `json:"request_ref"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client. SendError("invalid request format")
		return
	}

	userIDInt, _ := strconv. ParseInt(client.UserID, 10, 64)

	if err := h.userUc. CancelDeposit(ctx, req.RequestRef, userIDInt); err != nil {
		client.SendError(err.Error())
		return
	}

	client. SendSuccess("deposit cancelled", nil)
}

// Helper function