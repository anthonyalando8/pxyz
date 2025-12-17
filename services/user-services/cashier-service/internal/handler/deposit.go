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
	// Add more mobile money services as needed
}

// ✅ Service types that require bank account
var servicesRequiringBank = map[string]bool{
	"bank":          true,
	"bank_transfer": true,
	// Add more bank services as needed
}

// Handle deposit request
func (h *PaymentHandler) handleDepositRequest(ctx context.Context, client *Client, data json. RawMessage) {
	var req struct {
		Amount         float64 `json:"amount"`          // Amount in local currency (e.g., 1000 KES)
		LocalCurrency  string  `json:"local_currency"`  // Currency of amount (e.g., "KES")
		TargetCurrency *string `json:"target_currency"` // Optional: defaults to USD
		Service        string  `json:"service"`
		PartnerID      *string `json:"partner_id,omitempty"`
		AgentID        *string `json:"agent_id,omitempty"`
		//PaymentMethod  *string `json:"payment_method,omitempty"`
	}

	if err := json. Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	// ✅ Round input amount to 2 decimal places immediately
	req.Amount = roundTo2Decimals(req. Amount)

	// ✅ Validate input amount
	if req.Amount <= 0 {
		client.SendError("amount must be greater than zero")
		return
	}
	if req.Amount < 0.01 {
		client. SendError("amount must be at least 0.01")
		return
	}
	if req.Amount > 999999999999999999.99 {
		client. SendError("amount exceeds maximum allowed value")
		return
	}
	if req.LocalCurrency == "" {
		client.SendError("local_currency is required")
		return
	}
	if req.Service == "" {
		client.SendError("service is required")
		return
	}

	userIDInt, _ := strconv. ParseInt(client.UserID, 10, 64)

	// ✅ NEW: Validate user profile based on service requirements
	profile, err := h.profileFetcher.FetchProfile(ctx, "user", client.UserID)
	if err != nil {
		h.logger.Error("failed to fetch user profile",
			zap.String("user_id", client.UserID),
			zap.String("service", req.Service),
			zap.Error(err))
		client.SendError("failed to fetch user profile")
		return
	}

	// ✅ Check if service requires phone number
	var phoneNumber string
	var bankAccount string

	if servicesRequiringPhone[req. Service] {
		if profile.Phone == "" {
			h.logger. Warn("phone number required but not set",
				zap.String("user_id", client.UserID),
				zap.String("service", req.Service))
			client.SendError("phone number is required for this payment method.  Please add your phone number in profile settings.")
			return
		}
		phoneNumber = profile.Phone
		h.logger.Info("phone number validated for service",
			zap.String("user_id", client.UserID),
			zap.String("service", req.Service),
			zap.String("phone", phoneNumber))
	}

	// ✅ Check if service requires bank account
	if servicesRequiringBank[req.Service] {
		if profile.BankAccount == "" {
			h.logger.Warn("bank account required but not set",
				zap.String("user_id", client.UserID),
				zap.String("service", req. Service))
			client.SendError("bank account is required for this payment method. Please add your bank account in profile settings.")
			return
		}
		bankAccount = profile.BankAccount
		h. logger.Info("bank account validated for service",
			zap.String("user_id", client.UserID),
			zap.String("service", req.Service),
			zap.String("bank_account", bankAccount))
	}

	// Set target currency (default USD)
	targetCurrency := "USD"
	if req.TargetCurrency != nil && *req.TargetCurrency != "" {
		targetCurrency = *req.TargetCurrency
	}

	// Determine flow:  Agent or Partner
	var selectedPartner *partnersvcpb. Partner
	var selectedAgent *accountingpb.Agent
	var convertedAmount float64
	var exchangeRate float64
	var isAgentFlow bool

	// Initialize currency service
	currencyService := service.NewCurrencyService(h.partnerClient)

	if req.AgentID != nil && *req.AgentID != "" {
		// ===== AGENT FLOW =====
		isAgentFlow = true

		// Fetch agent
		agentResp, err := h.accountingClient.Client.GetAgentByID(ctx, &accountingpb.GetAgentByIDRequest{
			AgentExternalId: *req.AgentID,
			IncludeAccounts:  false,
		})
		if err != nil || agentResp. Agent == nil {
			client.SendError("agent not found")
			return
		}
		selectedAgent = agentResp.Agent

		if ! selectedAgent.IsActive {
			client.SendError("agent is not active")
			return
		}

		// For agent flow, we still need a partner for currency conversion
		partners, err := h.GetPartnersByService(ctx, req.Service)
		if err != nil || len(partners) == 0 {
			client.SendError("no partners available for currency conversion")
			return
		}
		selectedPartner = SelectRandomPartner(partners)

		// ✅ Convert currency with validation
		var convErr error
		convertedAmount, exchangeRate, convErr = currencyService.ConvertToUSDWithValidation(ctx, selectedPartner, req.Amount)
		if convErr != nil {
			h.logger. Error("currency conversion failed",
				zap.String("user_id", client.UserID),
				zap.Float64("amount", req.Amount),
				zap.String("currency", req.LocalCurrency),
				zap.Error(convErr))
			client.SendError("currency conversion failed:  " + convErr.Error())
			return
		}

	} else {
		// ===== PARTNER FLOW =====
		isAgentFlow = false

		// Select partner
		if req.PartnerID != nil && *req.PartnerID != "" {
			if err := h.ValidatePartnerService(ctx, *req.PartnerID, req.Service); err != nil {
				client.SendError(err.Error())
				return
			}
			var err error
			selectedPartner, err = h.GetPartnerByID(ctx, *req.PartnerID)
			if err != nil {
				client.SendError("partner not found")
				return
			}
		} else {
			partners, err := h.GetPartnersByService(ctx, req.Service)
			if err != nil || len(partners) == 0 {
				client.SendError("no partners available for this service")
				return
			}
			selectedPartner = SelectRandomPartner(partners)
		}

		// ✅ Convert currency with validation
		var convErr error
		convertedAmount, exchangeRate, convErr = currencyService. ConvertToUSDWithValidation(ctx, selectedPartner, req.Amount)
		if convErr != nil {
			h.logger.Error("currency conversion failed",
				zap.String("user_id", client.UserID),
				zap.Float64("amount", req.Amount),
				zap.String("currency", req. LocalCurrency),
				zap.Error(convErr))
			client.SendError("currency conversion failed: " + convErr.Error())
			return
		}
	}

	// ✅ Log conversion details
	h.logger. Info("currency conversion completed",
		zap.String("user_id", client.UserID),
		zap.Float64("original_amount", req.Amount),
		zap.String("original_currency", req.LocalCurrency),
		zap.Float64("converted_amount", convertedAmount),
		zap.String("target_currency", targetCurrency),
		zap.Float64("exchange_rate", exchangeRate),
		zap.String("partner_id", selectedPartner.Id),
		zap.String("service", req.Service),
		zap.String("phone_number", phoneNumber))

	// ✅ Create deposit request with validated amounts
	depositReq := &domain.DepositRequest{
		UserID:          userIDInt,
		PartnerID:       selectedPartner.Id,
		RequestRef:      id.GenerateTransactionID("DEP"), //fmt.Sprintf("DEP-%d-%s", userIDInt, generateID()),
		Amount:          convertedAmount,     // ✅ Already validated and rounded to 2 decimals
		Currency:        targetCurrency,
		Service:         req.Service,
		AgentExternalID: req.AgentID,
		PaymentMethod:   strToPtr(req.Service),
		Status:          domain.DepositStatusPending,
		ExpiresAt:       time.Now().Add(30 * time.Minute),
		CreatedAt:       time. Now(),
		UpdatedAt:       time.Now(),
	}

	// ✅ Store original amount and rate in metadata (also rounded)
	depositReq.SetOriginalAmount(
		roundTo2Decimals(req.Amount),       // ✅ Ensure original amount is also rounded
		req.LocalCurrency,
		roundTo2Decimals(exchangeRate),     // ✅ Round exchange rate too
	)

	// ✅ Add phone number and bank account to metadata if available
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
	if err := h.userUc.CreateDepositRequest(ctx, depositReq); err != nil {
		h.logger.Error("failed to create deposit request",
			zap. String("user_id", client. UserID),
			zap.String("request_ref", depositReq.RequestRef),
			zap.Float64("amount", convertedAmount),
			zap.Error(err))
		client.SendError("failed to create deposit request: " + err.Error())
		return
	}

	// Process based on flow
	if isAgentFlow {
		// Agent deposit:  Placeholder for agent to fulfill
		go h.sendDepositRequestToAgent(depositReq, selectedAgent, selectedPartner, phoneNumber, bankAccount)

		client.SendSuccess("deposit request sent to agent", map[string]interface{}{
			"request_ref":         depositReq.RequestRef,
			"agent_id":           selectedAgent.AgentExternalId,
			"agent_name":         selectedAgent.Name,
			"original_amount":    roundTo2Decimals(req. Amount),      // ✅ Rounded
			"original_currency":  req.LocalCurrency,
			"converted_amount":   convertedAmount,                   // ✅ Already rounded
			"target_currency":     targetCurrency,
			"exchange_rate":      roundTo2Decimals(exchangeRate),    // ✅ Rounded
			"status":             "sent_to_agent",
			"expires_at":         depositReq.ExpiresAt,
			"phone_number":       phoneNumber,
			"service":            req.Service,
		})
	} else {
		// Partner deposit: Send webhook
		go h.sendDepositWebhookToPartner(depositReq, selectedPartner, req.Amount, req.LocalCurrency, phoneNumber, bankAccount)
		h.userUc. MarkDepositSentToPartner(ctx, depositReq.RequestRef, "")

		client.SendSuccess("deposit request created", map[string]interface{}{
			"request_ref":        depositReq.RequestRef,
			"partner_id":         selectedPartner.Id,
			"partner_name":       selectedPartner.Name,
			"original_amount":    roundTo2Decimals(req.Amount),      // ✅ Rounded
			"original_currency":  req. LocalCurrency,
			"converted_amount":   convertedAmount,                   // ✅ Already rounded
			"target_currency":     targetCurrency,
			"exchange_rate":      roundTo2Decimals(exchangeRate),    // ✅ Rounded
			"status":             "sent_to_partner",
			"expires_at":         depositReq.ExpiresAt,
			"phone_number":       phoneNumber,
			"service":            req.Service,
		})
	}
}

// ✅ Add helper function to the handler file
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