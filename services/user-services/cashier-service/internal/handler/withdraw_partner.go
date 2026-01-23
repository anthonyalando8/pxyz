// handler/withdrawal_partner.go
package handler

import (
	"context"
	"fmt"
	"log"
	"time"

	"cashier-service/internal/domain"
	convsvc "cashier-service/internal/service"
	accountingpb "x/shared/genproto/shared/accounting/v1"
	partnersvcpb "x/shared/genproto/partner/svcpb"
	"x/shared/utils/id"

	"go.uber.org/zap"
)

// buildPartnerWithdrawalContext builds context for partner withdrawal
func (h *PaymentHandler) buildPartnerWithdrawalContext(ctx context.Context, wctx *WithdrawalContext) (*WithdrawalContext, error) {
	req := wctx.Request

	// Get service
	service := "mpesa" // default
	if req.Service != nil {
		service = *req.Service
	}

	// Get user profile for phone/bank
	phone, bank, err := h.validateUserProfileForWithdrawal(ctx, wctx.UserID, req)
	if err != nil {
		return nil, err
	}

	// Get partners for service
	partners, err := h.GetPartnersByService(ctx, service)
	if err != nil || len(partners) == 0 {
		return nil, fmt.Errorf("no partners available for service: %s", service)
	}

	selectedPartner := SelectRandomPartner(partners)

	// ✅ Extract local currency from partner
	localCurrency := selectedPartner.LocalCurrency
	if localCurrency == "" {
		return nil, fmt.Errorf("partner has no local currency configured")
	}

	// ✅ Convert to USD using partner's currency
	currencyService := convsvc.NewCurrencyService(h.partnerClient)
	amountInUSD, exchangeRate, err := currencyService.ConvertToUSDWithValidation(ctx, selectedPartner, req.Amount)
	if err != nil {
		h.logger.Error("currency conversion failed",
			zap.String("user_id", wctx.UserID),
			zap.Float64("amount", req.Amount),
			zap.String("currency", localCurrency),
			zap.Error(err))
		return nil, fmt.Errorf("currency conversion failed: %v", err)
	}

	wctx.AmountInUSD = amountInUSD
	wctx.ExchangeRate = exchangeRate
	wctx.Partner = selectedPartner
	wctx.PhoneNumber = phone
	wctx.BankAccount = bank

	// ✅ Update request with partner's currency
	req.LocalCurrency = localCurrency

	// Get user USD account
	userAccount, err := h.GetAccountByCurrency(ctx, wctx.UserID, "user", "USD",nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get user account: %v", err)
	}
	wctx.UserAccount = userAccount

	// Validate balance
	balanceResp, err := h.accountingClient.Client.GetBalance(ctx, &accountingpb.GetBalanceRequest{
		Identifier: &accountingpb.GetBalanceRequest_AccountNumber{
			AccountNumber: userAccount,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check balance: %v", err)
	}

	if balanceResp.Balance.AvailableBalance < wctx.AmountInUSD {
		return nil, fmt.Errorf("insufficient balance: need %.2f USD (%.2f %s), have %.2f USD",
			wctx.AmountInUSD, req.Amount, localCurrency, balanceResp.Balance.AvailableBalance)
	}

	h.logger.Info("partner withdrawal context built",
		zap.String("partner_id", selectedPartner.Id),
		zap.String("partner_name", selectedPartner.Name),
		zap.String("local_currency", localCurrency),
		zap.Float64("amount_local", req.Amount),
		zap.Float64("amount_usd", wctx.AmountInUSD),
		zap.Float64("exchange_rate", exchangeRate))

	return wctx, nil
}

// processPartnerWithdrawal processes partner withdrawal
func (h *PaymentHandler) processPartnerWithdrawal(ctx context.Context, client *Client, wctx *WithdrawalContext) {
	req := wctx.Request

	// Create withdrawal request
	withdrawalReq := &domain.WithdrawalRequest{
		UserID:      wctx.UserIDInt,
		RequestRef:  id.GenerateTransactionID("WD-PT"),
		Amount:      wctx.AmountInUSD,
		Currency:    "USD",
		Destination: req.Destination,
		Service:     req.Service,
		PartnerID:   &wctx.Partner.Id,
		Status:      domain.WithdrawalStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Store original amount and metadata
	withdrawalReq.SetOriginalAmount(req.Amount, req.LocalCurrency, wctx.ExchangeRate)
	if withdrawalReq.Metadata == nil {
		withdrawalReq.Metadata = make(map[string]interface{})
	}
	if wctx.PhoneNumber != "" {
		withdrawalReq.Metadata["phone_number"] = wctx.PhoneNumber
	}
	if wctx.BankAccount != "" {
		withdrawalReq.Metadata["bank_account"] = wctx.BankAccount
	}
	withdrawalReq.Metadata["withdrawal_type"] = "partner"
	withdrawalReq.Metadata["partner_id"] = wctx.Partner.Id
	withdrawalReq.Metadata["partner_name"] = wctx.Partner.Name
	withdrawalReq.Metadata["exchange_rate"] = wctx.ExchangeRate

	// Save to database
	if err := h.userUc.CreateWithdrawalRequest(ctx, withdrawalReq); err != nil {
		h.logger.Error("failed to create withdrawal request", zap.Error(err))
		client.SendError(fmt.Sprintf("failed to create withdrawal request: %v", err))
		return
	}

	// Process asynchronously
	go h.executePartnerWithdrawal(withdrawalReq, wctx)

	// Send success response
	client.SendSuccess("partner withdrawal request created", map[string]interface{}{
		"request_ref":     withdrawalReq.RequestRef,
		"amount_usd":      wctx.AmountInUSD,
		"amount_local":    req.Amount,
		"local_currency":  req.LocalCurrency,
		"exchange_rate":   wctx.ExchangeRate,
		"withdrawal_type": "partner",
		"partner_id":      wctx.Partner.Id,
		"partner_name":    wctx.Partner.Name,
		"status":          "processing",
	})
}

// executePartnerWithdrawal executes the partner withdrawal
func (h *PaymentHandler) executePartnerWithdrawal(withdrawal *domain.WithdrawalRequest, wctx *WithdrawalContext) {
	ctx := context.Background()

	if err := h.userUc.MarkWithdrawalProcessing(ctx, withdrawal.RequestRef); err != nil {
		log.Printf("[PartnerWithdrawal] Failed to mark as processing: %v", err)
		return
	}

	// Step 1: Get partner's USD account
	partnerAccount, err := h.GetAccountByCurrency(ctx, wctx.Partner.Id, "partner", "USD",nil)
	if err != nil {
		errMsg := fmt.Sprintf("partner account not found: %v", err)
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

	// Step 2: Transfer from user to partner (in USD)
	transferReq := &accountingpb.TransferRequest{
		FromAccountNumber:   wctx.UserAccount,
		ToAccountNumber:     partnerAccount,
		Amount:              withdrawal.Amount, // USD amount
		AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		Description:         fmt.Sprintf("Withdrawal %.2f %s to %s via partner %s",
			wctx.Request.Amount, wctx.Request.LocalCurrency, withdrawal.Destination, wctx.Partner.Name),
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
                "error": "%s"
            }
        }`, withdrawal.RequestRef, errMsg)))
		return
	}

	// Step 3: Build metadata for partner webhook
	metadata := map[string]string{
		"request_ref":       withdrawal.RequestRef,
		"destination":       withdrawal.Destination,
		"original_amount":   fmt.Sprintf("%.2f", wctx.Request.Amount),
		"converted_amount":  fmt.Sprintf("%.2f", withdrawal.Amount),
		"original_currency": wctx.Request.LocalCurrency,
		"target_currency":   "USD",
		"exchange_rate":     fmt.Sprintf("%.4f", wctx.ExchangeRate),
		"receipt_code":      resp.ReceiptCode,
		"journal_id":        fmt.Sprintf("%d", resp.JournalId),
	}

	if wctx.PhoneNumber != "" {
		metadata["phone_number"] = wctx.PhoneNumber
	}
	if wctx.BankAccount != "" {
		metadata["bank_account"] = wctx.BankAccount
	}

	h.logger.Info("sending withdrawal webhook to partner",
		zap.String("partner_id", wctx.Partner.Id),
		zap.String("request_ref", withdrawal.RequestRef),
		zap.Any("metadata", metadata))

	// Step 4: Send to partner for payout
	partnerResp, err := h.partnerClient.Client.InitiateWithdrawal(ctx, &partnersvcpb.InitiateWithdrawalRequest{
		PartnerId:      wctx.Partner.Id,
		TransactionRef: withdrawal.RequestRef,
		UserId:         fmt.Sprintf("%d", withdrawal.UserID),
		Amount:         withdrawal.Amount, // USD amount
		Currency:       "USD",
		PaymentMethod:  getPaymentMethod(withdrawal.Service),
		ExternalRef:    resp.ReceiptCode,
		Metadata:       metadata,
	})

	if err != nil {
		log.Printf("[PartnerWithdrawal] Failed to send to partner %s: %v", wctx.Partner.Id, err)

		// Money already transferred to partner
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

	// Step 5: Mark as sent to partner
	h.userUc.UpdateWithdrawalStatus(ctx, withdrawal.RequestRef, "sent_to_partner", nil)

	if withdrawal.Metadata == nil {
		withdrawal.Metadata = make(map[string]interface{})
	}
	withdrawal.Metadata["partner_transaction_id"] = partnerResp.TransactionId
	withdrawal.Metadata["partner_transaction_ref"] = partnerResp.TransactionRef
	withdrawal.Metadata["receipt_code"] = resp.ReceiptCode
	withdrawal.Metadata["journal_id"] = resp.JournalId
	withdrawal.Metadata["sent_to_partner_at"] = time.Now()

	// Notify user
	h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
        "type": "withdrawal_sent_to_partner",
        "data": {
            "request_ref": "%s",
            "receipt_code": "%s",
            "partner_id": "%s",
            "partner_name": "%s",
            "partner_transaction_id": %d,
            "partner_transaction_ref": "%s",
            "amount_usd": %.2f,
            "amount_local": %.2f,
            "local_currency": "%s",
            "fee_amount": %.2f,
            "status": "sent_to_partner"
        }
    }`, withdrawal.RequestRef, resp.ReceiptCode, wctx.Partner.Id, wctx.Partner.Name,
		partnerResp.TransactionId, partnerResp.TransactionRef,
		withdrawal.Amount, wctx.Request.Amount, wctx.Request.LocalCurrency, resp.FeeAmount)))
}