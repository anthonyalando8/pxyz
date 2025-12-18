// internal/usecase/payment_usecase.go

package usecase
import (
	"context"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"

	"payment-service/internal/domain"
)

// InitiateWithdrawal initiates a withdrawal (called from partner webhook)
func (uc *PaymentUsecase) InitiateWithdrawal(ctx context. Context, req *domain.WithdrawalRequest) (*domain.Payment, error) {
    // Validate request
    if err := req. Validate(); err != nil {
        uc.logger.Error("withdrawal validation failed",
            zap.String("transaction_ref", req.TransactionRef),
            zap.String("partner_id", req. PartnerID),
            zap.Error(err))
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    uc.logger.Info("initiating withdrawal",
        zap.String("transaction_ref", req.TransactionRef),
        zap.String("partner_id", req. PartnerID),
        zap.String("provider", string(req.Provider)),
        zap.Float64("amount", req.Amount),
        zap.String("currency", req.Currency),
        zap.String("user_id", req.UserID))

    // Check for duplicate (idempotency)
    existing, err := uc.paymentRepo.GetByPartnerTxRef(ctx, req. PartnerID, req.TransactionRef)
    if err == nil && existing != nil {
        uc.logger.Info("duplicate withdrawal request detected, returning existing payment",
            zap.String("transaction_ref", req.TransactionRef),
            zap.String("payment_ref", existing.PaymentRef),
            zap.String("status", string(existing.Status)))
        return existing, nil
    }

    // Use transaction ref as payment ref
    paymentRef := req.TransactionRef

    // Create payment record
    payment := &domain.Payment{
        PaymentRef:    paymentRef,
        PartnerID:     req.PartnerID,
        PartnerTxRef:  req. TransactionRef,
        Provider:      req.Provider,
        PaymentType:   domain.PaymentTypeWithdrawal,
        Amount:         req.Amount,
        Currency:      req.Currency,
        UserID:        req.UserID,
        AccountNumber: &req.AccountNumber,
        PhoneNumber:   &req.PhoneNumber,
        Status:        domain.PaymentStatusPending,
        Description:   &req.Description,
    }

    // Add metadata
    if req.Metadata != nil {
        metadataJSON, _ := json.Marshal(req. Metadata)
        payment.Metadata = metadataJSON
    }

    // Save to database
    if err := uc.paymentRepo.Create(ctx, payment); err != nil {
        uc.logger.Error("failed to create withdrawal payment",
            zap.String("transaction_ref", req.TransactionRef),
            zap.String("partner_id", req.PartnerID),
            zap.Error(err))
        return nil, fmt. Errorf("failed to create payment:  %w", err)
    }

    uc.logger.Info("withdrawal payment record created successfully",
        zap.Int64("payment_id", payment. ID),
        zap.String("payment_ref", payment.PaymentRef),
        zap.String("provider", string(payment.Provider)))

    // Process based on provider
    switch req.Provider {
    case domain.ProviderMpesa:
        go uc.processMpesaWithdrawal(payment)
    case domain.ProviderBank:
        go uc.processBankWithdrawal(payment)
    default:
        uc.logger.Error("unsupported payment provider",
            zap.String("provider", string(req.Provider)),
            zap.String("payment_ref", paymentRef))
        return nil, fmt.Errorf("unsupported provider: %s", req.Provider)
    }

    return payment, nil
}

// processMpesaWithdrawal processes M-Pesa withdrawal via B2C
func (uc *PaymentUsecase) processMpesaWithdrawal(payment *domain.Payment) {
    ctx := context.Background()

    uc.logger.Info("processing M-Pesa withdrawal",
        zap.Int64("payment_id", payment. ID),
        zap.String("payment_ref", payment.PaymentRef),
        zap.String("phone_number", *payment.PhoneNumber),
        zap.Float64("amount", payment.Amount))

    // Parse metadata to get original amount
    var metadata domain.WithdrawalWebhookMetadata
    if payment.Metadata != nil {
        metadataBytes, _ := json.Marshal(payment.Metadata)
        _ = json.Unmarshal(metadataBytes, &metadata)
    }

    // ✅ Use original amount (local currency) for B2C
    b2cAmount := metadata.OriginalAmount
    if b2cAmount == 0 {
        uc.logger.Error("original amount not found in metadata, using payment amount",
            zap.String("payment_ref", payment.PaymentRef))
        b2cAmount = payment.Amount
    }

    uc.logger.Info("initiating B2C with original amount",
        zap.String("payment_ref", payment.PaymentRef),
        zap.Float64("b2c_amount", b2cAmount),
        zap.String("original_currency", metadata.OriginalCurrency),
        zap.Float64("converted_amount", payment.Amount),
        zap.String("target_currency", payment.Currency))

    // Update status to processing
    if err := uc.paymentRepo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusProcessing); err != nil {
        uc. logger.Error("failed to update payment status to processing",
            zap.Int64("payment_id", payment. ID),
            zap.String("payment_ref", payment.PaymentRef),
            zap.Error(err))
        return
    }

    // Build callback URLs
    resultURL := fmt.Sprintf("%s/api/v1/callbacks/mpesa/b2c/%s", uc. config.CallbackURL, payment.PaymentRef)
    timeoutURL := fmt.Sprintf("%s/api/v1/callbacks/mpesa/b2c/timeout/%s", uc.config.CallbackURL, payment.PaymentRef)

    uc.logger.Debug("initiating M-Pesa B2C",
        zap.String("payment_ref", payment.PaymentRef),
        zap.Float64("amount", b2cAmount),
        zap.String("result_url", resultURL))

    // ✅ Initiate B2C with original local currency amount
    response, err := uc.mpesaProvider.InitiateB2C(
        ctx,
        *payment.PhoneNumber,
        payment. PartnerTxRef,
        b2cAmount, // ✅ Use original amount (KES, not USD)
        resultURL,
        timeoutURL,
    )

    if err != nil {
        uc.logger.Error("M-Pesa B2C initiation failed",
            zap. Int64("payment_id", payment. ID),
            zap.String("payment_ref", payment.PaymentRef),
            zap.Error(err))
        _ = uc.paymentRepo. SetError(ctx, payment.ID, err. Error())
        return
    }

    uc.logger.Info("M-Pesa B2C initiated",
        zap.String("payment_ref", payment.PaymentRef),
        zap.String("conversation_id", response.ConversationID),
        zap.String("response_code", response.ResponseCode))

    // Create provider transaction record
    requestPayload, _ := json.Marshal(map[string]interface{}{
        "phone_number":        *payment.PhoneNumber,
        "amount":            b2cAmount, // Original amount
        "converted_amount":  payment.Amount, // USD amount
        "original_currency":  metadata.OriginalCurrency,
        "target_currency":   payment.Currency,
        "result_url":        resultURL,
        "timeout_url":       timeoutURL,
        "account_reference": payment. PartnerTxRef,
    })

    responsePayload, _ := json.Marshal(response)

    providerTx := &domain.ProviderTransaction{
        PaymentID:       payment.ID,
        Provider:        domain.ProviderMpesa,
        TransactionType: "b2c",
        RequestPayload:  requestPayload,
        ResponsePayload: responsePayload,
        Status:          domain.TxStatusSent,
    }

    if response.ResponseCode == "0" {
        conversationID := response.ConversationID
        providerTx. ProviderTxID = &conversationID

        if err := uc.providerTxRepo.Create(ctx, providerTx); err != nil {
            uc.logger.Error("failed to create provider transaction",
                zap.Int64("payment_id", payment.ID),
                zap.String("payment_ref", payment.PaymentRef),
                zap.Error(err))
        } else {
            uc.logger.Info("provider transaction created successfully",
                zap.Int64("provider_tx_id", providerTx.ID),
                zap.String("payment_ref", payment.PaymentRef))
        }
    } else {
        uc.logger. Warn("M-Pesa B2C rejected",
            zap.String("payment_ref", payment.PaymentRef),
            zap.String("response_code", response. ResponseCode),
            zap.String("response_description", response.ResponseDescription))

        providerTx.Status = domain. TxStatusFailed
        providerTx.ResultCode = &response.ResponseCode
        providerTx.ResultDescription = &response.ResponseDescription
        _ = uc.providerTxRepo.Create(ctx, providerTx)
        _ = uc.paymentRepo. SetError(ctx, payment.ID, response.ResponseDescription)
    }
}

// processBankWithdrawal processes bank withdrawal
func (uc *PaymentUsecase) processBankWithdrawal(payment *domain.Payment) {
    ctx := context. Background()

    uc.logger.Info("processing bank withdrawal",
        zap.Int64("payment_id", payment.ID),
        zap.String("payment_ref", payment.PaymentRef),
        zap.Float64("amount", payment.Amount))

    // Update status to processing
    if err := uc.paymentRepo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusProcessing); err != nil {
        uc.logger.Error("failed to update payment status",
            zap.Int64("payment_id", payment.ID),
            zap.Error(err))
        return
    }

    // TODO:  Implement bank withdrawal processing
    uc.logger. Warn("bank withdrawal processing not yet implemented",
        zap.String("payment_ref", payment.PaymentRef))
}