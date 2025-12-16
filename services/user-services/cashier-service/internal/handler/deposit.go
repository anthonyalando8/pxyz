// handler/deposit.go
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

	"go.uber.org/zap"
)

// Handle deposit request
func (h *PaymentHandler) handleDepositRequest(ctx context.Context, client *Client, data json. RawMessage) {
    var req struct {
        Amount         float64 `json:"amount"`          // Amount in local currency (e. g., 1000 KES)
        LocalCurrency  string  `json:"local_currency"`  // Currency of amount (e. g., "KES")
        TargetCurrency *string `json:"target_currency"` // Optional:  defaults to USD
        Service        string  `json:"service"`
        PartnerID      *string `json:"partner_id,omitempty"`
        AgentID        *string `json:"agent_id,omitempty"`
        PaymentMethod  *string `json:"payment_method,omitempty"`
    }

    if err := json. Unmarshal(data, &req); err != nil {
        client.SendError("invalid request format")
        return
    }

    // ✅ Round input amount to 2 decimal places immediately
    req.Amount = roundTo2Decimals(req.Amount)

    // ✅ Validate input amount
    if req.Amount <= 0 {
        client.SendError("amount must be greater than zero")
        return
    }
    if req.Amount < 0.01 {
        client.SendError("amount must be at least 0.01")
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

    userIDInt, _ := strconv. ParseInt(client.UserID, 10, 64)

    // Set target currency (default USD)
    targetCurrency := "USD"
    if req.TargetCurrency != nil && *req. TargetCurrency != "" {
        targetCurrency = *req. TargetCurrency
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
        agentResp, err := h. accountingClient.Client.GetAgentByID(ctx, &accountingpb.GetAgentByIDRequest{
            AgentExternalId: *req.AgentID,
            IncludeAccounts:  false,
        })
        if err != nil || agentResp.Agent == nil {
            client.SendError("agent not found")
            return
        }
        selectedAgent = agentResp. Agent

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
            h.logger.Error("currency conversion failed",
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
        if req. PartnerID != nil && *req.PartnerID != "" {
            if err := h. ValidatePartnerService(ctx, *req.PartnerID, req.Service); err != nil {
                client.SendError(err.Error())
                return
            }
            var err error
            selectedPartner, err = h. GetPartnerByID(ctx, *req.PartnerID)
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
        convertedAmount, exchangeRate, convErr = currencyService.ConvertToUSDWithValidation(ctx, selectedPartner, req. Amount)
        if convErr != nil {
            h.logger. Error("currency conversion failed",
                zap.String("user_id", client.UserID),
                zap.Float64("amount", req.Amount),
                zap.String("currency", req.LocalCurrency),
                zap. Error(convErr))
            client.SendError("currency conversion failed: " + convErr.Error())
            return
        }
    }

    // ✅ Log conversion details
    h.logger.Info("currency conversion completed",
        zap.String("user_id", client.UserID),
        zap.Float64("original_amount", req.Amount),
        zap.String("original_currency", req.LocalCurrency),
        zap.Float64("converted_amount", convertedAmount),
        zap.String("target_currency", targetCurrency),
        zap.Float64("exchange_rate", exchangeRate),
        zap.String("partner_id", selectedPartner.Id))

    // ✅ Create deposit request with validated amounts
    depositReq := &domain.DepositRequest{
        UserID:          userIDInt,
        PartnerID:       selectedPartner. Id,
        RequestRef:      fmt.Sprintf("DEP-%d-%s", userIDInt, generateID()),
        Amount:          convertedAmount,     // ✅ Already validated and rounded to 2 decimals
        Currency:        targetCurrency,      
        Service:         req.Service,
        AgentExternalID: req.AgentID,         
        PaymentMethod:   req. PaymentMethod,
        Status:          domain.DepositStatusPending,
        ExpiresAt:       time.Now().Add(30 * time.Minute),
        CreatedAt:       time.Now(),
        UpdatedAt:       time.Now(),
    }

    // ✅ Store original amount and rate in metadata (also rounded)
    depositReq.SetOriginalAmount(
        roundTo2Decimals(req. Amount),  // ✅ Ensure original amount is also rounded
        req.LocalCurrency, 
        roundTo2Decimals(exchangeRate), // ✅ Round exchange rate too
    )

    // Save to database
    if err := h.userUc.CreateDepositRequest(ctx, depositReq); err != nil {
        h.logger. Error("failed to create deposit request",
            zap.String("user_id", client.UserID),
            zap.String("request_ref", depositReq.RequestRef),
            zap.Float64("amount", convertedAmount),
            zap.Error(err))
        client.SendError("failed to create deposit request:  " + err.Error())
        return
    }

    // Process based on flow
    if isAgentFlow {
        // Agent deposit:  Placeholder for agent to fulfill
        go h.sendDepositRequestToAgent(depositReq, selectedAgent, selectedPartner)
        
        client.SendSuccess("deposit request sent to agent", map[string]interface{}{
            "request_ref":        depositReq.RequestRef,
            "agent_id":          selectedAgent.AgentExternalId,
            "agent_name":        selectedAgent.Name,
            "original_amount":   roundTo2Decimals(req. Amount),      // ✅ Rounded
            "original_currency": req. LocalCurrency,
            "converted_amount":  convertedAmount,                    // ✅ Already rounded
            "target_currency":    targetCurrency,
            "exchange_rate":     roundTo2Decimals(exchangeRate),    // ✅ Rounded
            "status":            "sent_to_agent",
            "expires_at":        depositReq. ExpiresAt,
        })
    } else {
        // Partner deposit: Send webhook
        go h.sendDepositWebhookToPartner(depositReq, selectedPartner, req.Amount, req.LocalCurrency)
        h.userUc. MarkDepositSentToPartner(ctx, depositReq.RequestRef, "")

        client.SendSuccess("deposit request created", map[string]interface{}{
            "request_ref":       depositReq.RequestRef,
            "partner_id":        selectedPartner.Id,
            "partner_name":      selectedPartner.Name,
            "original_amount":   roundTo2Decimals(req.Amount),      // ✅ Rounded
            "original_currency": req.LocalCurrency,
            "converted_amount":  convertedAmount,                    // ✅ Already rounded
            "target_currency":   targetCurrency,
            "exchange_rate":     roundTo2Decimals(exchangeRate),    // ✅ Rounded
            "status":            "sent_to_partner",
            "expires_at":        depositReq. ExpiresAt,
        })
    }
}

// ✅ Add helper function to the handler file
func roundTo2Decimals(value float64) float64 {
    return math.Round(value*100) / 100
}
// ✅ NEW: Placeholder for agent deposit
func (h *PaymentHandler) sendDepositRequestToAgent(
    deposit *domain.DepositRequest,
    agent *accountingpb.Agent,
    partner *partnersvcpb.Partner,
) {
    ctx := context.Background()

    // TODO: Implement agent notification system (SMS, push notification, etc.)
    log.Printf("[Deposit] Deposit request %s sent to agent %s (PLACEHOLDER)", 
        deposit.RequestRef, agent.AgentExternalId)

    // Mark as sent to agent
    h.userUc.UpdateDepositStatus(ctx, deposit.RequestRef, domain.DepositStatusSentToAgent, nil)

    // Store additional metadata
    if deposit. Metadata == nil {
        deposit.Metadata = make(map[string]interface{})
    }
    deposit.Metadata["agent_id"] = agent.AgentExternalId
    deposit.Metadata["agent_name"] = *agent.Name
}

// ✅ UPDATED: Send webhook to partner with original amount
func (h *PaymentHandler) sendDepositWebhookToPartner(
    deposit *domain. DepositRequest,
    partner *partnersvcpb.Partner,
    originalAmount float64,
    originalCurrency string,
) {
    ctx := context. Background()

    // Send original amount in local currency to partner
    _, err := h.partnerClient.Client.InitiateDeposit(ctx, &partnersvcpb.InitiateDepositRequest{
        PartnerId:       partner.Id,
        TransactionRef: deposit.RequestRef,
        UserId:         fmt.Sprintf("%d", deposit. UserID),
        Amount:         deposit.Amount, //originalAmount,      // ✅ Send original local amount
        Currency:       deposit.Currency,//originalCurrency,    // ✅ Send local currency
        PaymentMethod:  ptrToStr(deposit.PaymentMethod),
        Metadata:  map[string]string{
            "request_ref":       deposit.RequestRef,
            "original_amount":  fmt.Sprintf("%.2f", originalAmount),
            "converted_amount":  fmt.Sprintf("%.2f", deposit.Amount),
            "original_currency": originalCurrency,
            "target_currency":   deposit.Currency,
            "exchange_rate":     fmt.Sprintf("%.4f", deposit. Metadata["exchange_rate"].(float64)),
        },
    })

    if err != nil {
        log.Printf("[Deposit] Failed to send webhook to partner %s: %v", partner.Id, err)
        h.userUc. FailDeposit(ctx, deposit.RequestRef, err.Error())
    }
}

// Get deposit status (updated to include original amount)
func (h *PaymentHandler) handleGetDepositStatus(ctx context.Context, client *Client, data json. RawMessage) {
    var req struct {
        RequestRef string `json:"request_ref"`
    }

    if err := json.Unmarshal(data, &req); err != nil {
        client.SendError("invalid request format")
        return
    }

    userIDInt, _ := strconv. ParseInt(client.UserID, 10, 64)
    
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
        "request_ref":              deposit.RequestRef,
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
        client.SendError("invalid request format")
        return
    }

    userIDInt, _ := strconv.ParseInt(client.UserID, 10, 64)

    if err := h.userUc. CancelDeposit(ctx, req.RequestRef, userIDInt); err != nil {
        client.SendError(err.Error())
        return
    }

    client. SendSuccess("deposit cancelled", nil)
}

// Helper function
func generateID() string {
    // Use your snowflake ID generator
    return fmt.Sprintf("%d", time.Now().UnixNano())
}