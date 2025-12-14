// handler/deposit.go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"cashier-service/internal/domain"
	"cashier-service/internal/service"
	usecase "cashier-service/internal/usecase/transaction"
	partnersvcpb "x/shared/genproto/partner/svcpb"
	accountingpb "x/shared/genproto/shared/accounting/v1"
)

// Handle deposit request
func (h *PaymentHandler) handleDepositRequest(ctx context.Context, client *Client, data json.RawMessage) {
    var req struct {
        Amount         float64 `json:"amount"`          // Amount in local currency (e.g., 1000 KES)
        LocalCurrency  string  `json:"local_currency"`  // Currency of amount (e.g., "KES")
        TargetCurrency *string `json:"target_currency"` // Optional:  defaults to USD
        Service        string  `json:"service"`
        PartnerID      *string `json:"partner_id,omitempty"`
        AgentID        *string `json:"agent_id,omitempty"`       // ✅ NEW
        PaymentMethod  *string `json:"payment_method,omitempty"`
    }

    if err := json. Unmarshal(data, &req); err != nil {
        client.SendError("invalid request format")
        return
    }

    // Validate
    if req.Amount <= 0 {
        client.SendError("amount must be greater than zero")
        return
    }
    if req.LocalCurrency == "" {
        client.SendError("local_currency is required")
        return
    }

    userIDInt, _ := strconv.ParseInt(client.UserID, 10, 64)

    // Set target currency (default USD)
    targetCurrency := "USD"
    if req. TargetCurrency != nil && *req.TargetCurrency != "" {
        targetCurrency = *req.TargetCurrency
    }

    // ✅ Determine flow:  Agent or Partner
    var selectedPartner *partnersvcpb. Partner
    var selectedAgent *accountingpb.Agent
    var convertedAmount float64
    var exchangeRate float64
    var isAgentFlow bool

    if req. AgentID != nil && *req.AgentID != "" {
        // ===== AGENT FLOW =====
        isAgentFlow = true

        // Fetch agent
        agentResp, err := h.accountingClient.Client.GetAgentByID(ctx, &accountingpb.GetAgentByIDRequest{
            AgentExternalId: *req.AgentID,
            IncludeAccounts:  false,
        })
        if err != nil || agentResp.Agent == nil {
            client.SendError("agent not found")
            return
        }
        selectedAgent = agentResp.Agent

        if ! selectedAgent.IsActive {
            client.SendError("agent is not active")
            return
        }

        // ✅ For agent flow, we still need a partner for currency conversion
        // Get partner by service
        partners, err := h.GetPartnersByService(ctx, req.Service)
        if err != nil || len(partners) == 0 {
            client.SendError("no partners available for currency conversion")
            return
        }
        selectedPartner = SelectRandomPartner(partners)

        // ✅ Convert currency using partner rate
        currencyService := service.NewCurrencyService(h.partnerClient)
        convertedAmount, exchangeRate = currencyService.ConvertToUSD(ctx, selectedPartner, req.Amount)

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

        // ✅ Convert currency using partner rate
        currencyService := service.NewCurrencyService(h.partnerClient)
        convertedAmount, exchangeRate = currencyService.ConvertToUSD(ctx, selectedPartner, req.Amount)
    }

    // ✅ Create deposit request
    depositReq := &domain.DepositRequest{
        UserID:          userIDInt,
        PartnerID:        selectedPartner.Id,
        RequestRef:      fmt.Sprintf("DEP-%d-%s", userIDInt, generateID()),
        Amount:          convertedAmount,     // Store converted amount (USD)
        Currency:        targetCurrency,      // Store target currency (USD)
        Service:         req.Service,
        AgentExternalID: req.AgentID,         // ✅ Store agent ID if provided
        PaymentMethod:   req.PaymentMethod,
        Status:          domain.DepositStatusPending,
        ExpiresAt:       time.Now().Add(30 * time.Minute),
        CreatedAt:       time.Now(),
        UpdatedAt:       time.Now(),
    }

    // ✅ Store original amount and rate in metadata
    depositReq.SetOriginalAmount(req.Amount, req.LocalCurrency, exchangeRate)

    // Save to database
    if err := h. userUc.CreateDepositRequest(ctx, depositReq); err != nil {
        client.SendError("failed to create deposit request:  " + err.Error())
        return
    }

    // ✅ Process based on flow
    if isAgentFlow {
        // Agent deposit:  Placeholder for agent to fulfill
        go h.sendDepositRequestToAgent(depositReq, selectedAgent, selectedPartner)
        
        client.SendSuccess("deposit request sent to agent", map[string]interface{}{
            "request_ref":       depositReq.RequestRef,
            "agent_id":          selectedAgent.AgentExternalId,
            "agent_name":        selectedAgent.Name,
            "original_amount":   req.Amount,
            "original_currency": req.LocalCurrency,
            "converted_amount":  convertedAmount,
            "target_currency":   targetCurrency,
            "exchange_rate":     exchangeRate,
            "status":            "sent_to_agent",
            "expires_at":        depositReq.ExpiresAt,
        })
    } else {
        // Partner deposit: Send webhook
        go h.sendDepositWebhookToPartner(depositReq, selectedPartner, req.Amount, req.LocalCurrency)
        h.userUc. MarkDepositSentToPartner(ctx, depositReq.RequestRef, "")

        client.SendSuccess("deposit request created", map[string]interface{}{
            "request_ref":       depositReq.RequestRef,
            "partner_id":        selectedPartner.Id,
            "partner_name":      selectedPartner. Name,
            "original_amount":   req.Amount,
            "original_currency": req.LocalCurrency,
            "converted_amount":  convertedAmount,
            "target_currency":   targetCurrency,
            "exchange_rate":     exchangeRate,
            "status":            "sent_to_partner",
            "expires_at":        depositReq. ExpiresAt,
        })
    }
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
        Amount:         originalAmount,      // ✅ Send original local amount
        Currency:       originalCurrency,    // ✅ Send local currency
        PaymentMethod:  ptrToStr(deposit.PaymentMethod),
        Metadata:  map[string]string{
            "request_ref":       deposit.RequestRef,
            "converted_amount":  fmt.Sprintf("%.2f", deposit.Amount),
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