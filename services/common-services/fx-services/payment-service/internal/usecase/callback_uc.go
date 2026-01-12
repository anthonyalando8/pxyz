// internal/usecase/callback_usecase.go
package usecase

import (
    "context"
    "encoding/json"
    "fmt"

    "payment-service/internal/domain"
    "payment-service/internal/provider/mpesa"
    "payment-service/internal/repository"
    "payment-service/pkg/client"

    "go.uber.org/zap"
)

type CallbackUsecase struct {
    paymentRepo         repository.PaymentRepository
    providerTxRepo      repository.  ProviderTransactionRepository
    mpesaProvider       *mpesa.MpesaProvider
    partnerClient       *client. PartnerClient
    partnerCreditClient *client.  PartnerCreditClient
    partnerDebitClient  *client. PartnerDebitClient  // ✅ NEW
    logger              *zap. Logger
}

func NewCallbackUsecase(
    paymentRepo repository.PaymentRepository,
    providerTxRepo repository. ProviderTransactionRepository,
    mpesaProvider *mpesa.MpesaProvider,
    partnerClient *client. PartnerClient,
    partnerCreditClient *client.PartnerCreditClient,
    partnerDebitClient *client. PartnerDebitClient,  // ✅ NEW
    logger *zap.Logger,
) *CallbackUsecase {
    return &CallbackUsecase{
        paymentRepo:           paymentRepo,
        providerTxRepo:      providerTxRepo,
        mpesaProvider:       mpesaProvider,
        partnerClient:       partnerClient,
        partnerCreditClient: partnerCreditClient,
        partnerDebitClient:  partnerDebitClient,  // ✅ NEW
        logger:              logger,
    }
}

// ProcessMpesaSTKCallback processes M-Pesa STK callback
func (uc *CallbackUsecase) ProcessMpesaSTKCallback(ctx context.Context, paymentRef string, payload []byte) error {
    uc.logger.Info("received M-Pesa STK callback",
        zap.String("payment_ref", paymentRef),
        zap.Int("payload_size", len(payload)))

    // Parse callback
    callbackResult, err := uc. mpesaProvider.ParseSTKCallback(payload)
    if err != nil {
        uc. logger.Error("failed to parse M-Pesa STK callback",
            zap.String("payment_ref", paymentRef),
            zap.Error(err))
        return fmt.Errorf("failed to parse callback: %w", err)
    }

    uc.logger.Info("M-Pesa STK callback parsed",
        zap.String("payment_ref", paymentRef),
        zap.String("checkout_request_id", callbackResult.CheckoutRequestID),
        zap.String("result_code", callbackResult.ResultCode),
        zap.Bool("success", callbackResult.Success),
        zap.String("mpesa_receipt", callbackResult.  ProviderTxID))

    // Get payment
    payment, err := uc.  paymentRepo.GetByPaymentRef(ctx, paymentRef)
    if err != nil {
        uc.logger.  Error("payment not found for callback",
            zap.String("payment_ref", paymentRef),
            zap.Error(err))
        return fmt.  Errorf("payment not found:  %w", err)
    }

    // Get provider transaction
    providerTx, err := uc.providerTxRepo.GetByCheckoutRequestID(ctx, callbackResult.CheckoutRequestID)
    if err != nil {
        uc.logger.Error("provider transaction not found",
            zap.String("payment_ref", paymentRef),
            zap.String("checkout_request_id", callbackResult.  CheckoutRequestID),
            zap.Error(err))
        return fmt.Errorf("provider transaction not found: %w", err)
    }

    // Update callback data
    callbackData := make(map[string]interface{})
    callbackJSON, _ := json.Marshal(callbackResult)
    _ = json.Unmarshal(callbackJSON, &callbackData)

    var providerRef *string
    if callbackResult.  ProviderTxID != "" {
        providerRef = &callbackResult. ProviderTxID
    }

    if err := uc.paymentRepo.UpdateCallback(ctx, payment.ID, callbackData, providerRef); err != nil {
        uc.  logger.Error("failed to update callback data",
            zap.String("payment_ref", paymentRef),
            zap.Error(err))
    }

    // Update status based on result
    if callbackResult.Success {
        uc.  logger.Info("M-Pesa payment successful, preparing to credit user",
            zap.  String("payment_ref", paymentRef),
            zap.String("mpesa_receipt", callbackResult.  ProviderTxID),
            zap.Float64("local_amount", callbackResult.Amount),
            zap.Float64("usd_amount", payment.Amount))

        // Update payment to completed
        if err := uc.paymentRepo.UpdateStatus(ctx, payment.ID, domain.  PaymentStatusCompleted); err != nil {
            uc.logger.Error("failed to update payment status to completed",
                zap.String("payment_ref", paymentRef),
                zap.Error(err))
        }

        // Update provider transaction
        if err := uc.providerTxRepo. UpdateStatus(ctx, providerTx.ID, domain.TxStatusCompleted,
            callbackResult.ResultCode, callbackResult.ResultDescription); err != nil {
            uc.logger.Error("failed to update provider transaction status",
                zap.Int64("provider_tx_id", providerTx.ID),
                zap.Error(err))
        }

        // ✅ Credit user on partner system with USD amount
        go uc. creditUserOnPartner(payment, callbackResult)

        // Notify partner about payment status
        go uc.notifyPartner(payment, callbackResult)

    } else {
        uc.  logger.Warn("M-Pesa payment failed",
            zap.String("payment_ref", paymentRef),
            zap.String("result_code", callbackResult.  ResultCode),
            zap.String("result_description", callbackResult.ResultDescription))

        // Update payment to failed
        if err := uc.paymentRepo.  UpdateStatus(ctx, payment.ID, domain.PaymentStatusFailed); err != nil {
            uc.logger.Error("failed to update payment status to failed",
                zap.String("payment_ref", paymentRef),
                zap.Error(err))
        }

        // Set error message
        if err := uc.paymentRepo.  SetError(ctx, payment.ID, callbackResult.ResultDescription); err != nil {
            uc.logger.Error("failed to set error message",
                zap.String("payment_ref", paymentRef),
                zap.Error(err))
        }

        // Update provider transaction
        if err := uc.providerTxRepo.  UpdateStatus(ctx, providerTx.ID, domain.TxStatusFailed,
            callbackResult.ResultCode, callbackResult.ResultDescription); err != nil {
            uc.logger.Error("failed to update provider transaction status",
                zap.Int64("provider_tx_id", providerTx.ID),
                zap.Error(err))
        }

        // Still notify partner about failure
        go uc.notifyPartner(payment, callbackResult)
    }

    return nil
}

// creditUserOnPartner credits user account on partner system (for deposits)
func (uc *CallbackUsecase) creditUserOnPartner(payment *domain.  Payment, callbackResult *mpesa.CallbackResult) {
    ctx := context.Background()

    uc.logger.Info("crediting user on partner system",
        zap.String("payment_ref", payment.PaymentRef),
        zap.String("user_id", payment.UserID),
        zap.Float64("amount", payment.Amount),
        zap.String("currency", payment.Currency),
        zap.String("mpesa_receipt", callbackResult.  ProviderTxID))

    // Build credit request with USD amount
    creditReq := &client.CreditUserRequest{
        UserID:         payment.UserID,
        Amount:         payment.Amount, // ✅ USD amount (converted)
        Currency:       payment.Currency, // ✅ USD
        TransactionRef: payment.  PartnerTxRef,
        Description:    fmt.Sprintf("Deposit via M-Pesa - %s", payment.  PartnerTxRef),
        ExternalRef:    callbackResult.  ProviderTxID, // ✅ M-Pesa transaction code
    }

    // Call partner credit API
    response, err := uc.partnerCreditClient.CreditUser(ctx,string(payment.Provider), creditReq)
    if err != nil {
        uc.logger.Error("failed to credit user on partner system",
            zap.String("payment_ref", payment.PaymentRef),
            zap.String("user_id", payment.UserID),
            zap.Error(err))
        
        // Mark payment as needing manual review
        _ = uc.paymentRepo.  SetError(ctx, payment.ID, fmt.Sprintf("Credit failed: %v", err))
        return
    }

    uc.logger.  Info("user credited successfully on partner system",
        zap.String("payment_ref", payment.PaymentRef),
        zap.String("user_id", payment.UserID),
        zap.String("receipt_code", response.ReceiptCode))
}

// notifyPartner notifies partner about payment status (notification, not credit)
func (uc *CallbackUsecase) notifyPartner(payment *domain. Payment, callbackResult *mpesa.CallbackResult) {
    ctx := context.Background()

    uc.logger.Info("notifying partner about payment status",
        zap.String("payment_ref", payment.PaymentRef),
        zap.String("partner_id", payment.PartnerID),
        zap.String("status", string(payment.Status)))

    notification := &client.  PartnerNotification{
        PaymentRef:        payment.PaymentRef,
        PartnerTxRef:      payment.PartnerTxRef,
        Status:            string(payment.Status),
        Amount:            payment.Amount,
        Currency:          payment.Currency,
        ProviderReference: callbackResult.ProviderTxID,
        ResultCode:        callbackResult.ResultCode,
        ResultDescription: callbackResult.ResultDescription,
    }

    err := uc.partnerClient.  SendNotification(ctx, string(payment.Provider), notification)
    if err != nil {
        uc.logger.Error("failed to notify partner",
            zap.String("payment_ref", payment.PaymentRef),
            zap.String("partner_id", payment.PartnerID),
            zap.Error(err))
        return
    }

    if err := uc.paymentRepo. MarkPartnerNotified(ctx, payment.ID); err != nil {
        uc.  logger.Error("failed to mark partner as notified",
            zap. String("payment_ref", payment. PaymentRef),
            zap.Error(err))
    } else {
        uc.logger. Info("partner notified successfully",
            zap.  String("payment_ref", payment.  PaymentRef),
            zap.String("partner_id", payment.PartnerID))
    }
}

// ProcessMpesaB2CCallback processes M-Pesa B2C callback (for withdrawals)
func (uc *CallbackUsecase) ProcessMpesaB2CCallback(ctx context.Context, paymentRef string, payload []byte) error {
    uc.logger.  Info("received M-Pesa B2C callback",
        zap.String("payment_ref", paymentRef),
        zap.Int("payload_size", len(payload)))

    // Parse callback
    callbackResult, err := uc. mpesaProvider.  ParseB2CCallback(payload)
    if err != nil {
        uc.logger.Error("failed to parse M-Pesa B2C callback",
            zap.String("payment_ref", paymentRef),
            zap.Error(err))
        return fmt.Errorf("failed to parse callback: %w", err)
    }

    uc.  logger. Info("M-Pesa B2C callback parsed",
        zap.String("payment_ref", paymentRef),
        zap.String("conversation_id", callbackResult.ConversationID),
        zap.String("result_code", callbackResult.ResultCode),
        zap.Bool("success", callbackResult.Success),
        zap.String("mpesa_receipt", callbackResult.ProviderTxID))

    // Get payment
    payment, err := uc. paymentRepo.GetByPaymentRef(ctx, paymentRef)
    if err != nil {
        uc.logger. Error("payment not found for B2C callback",
            zap. String("payment_ref", paymentRef),
            zap.Error(err))
        return fmt. Errorf("payment not found:  %w", err)
    }

    // Update callback data
    callbackData := make(map[string]interface{})
    callbackJSON, _ := json.Marshal(callbackResult)
    _ = json.Unmarshal(callbackJSON, &callbackData)

    var providerRef *string
    if callbackResult.ProviderTxID != "" {
        providerRef = &callbackResult.  ProviderTxID
    }

    if err := uc.paymentRepo.UpdateCallback(ctx, payment.ID, callbackData, providerRef); err != nil {
        uc.logger.Error("failed to update B2C callback data",
            zap.String("payment_ref", paymentRef),
            zap.Error(err))
    }

    // Update status
    if callbackResult.Success {
        uc.logger.Info("M-Pesa B2C withdrawal successful",
            zap.String("payment_ref", paymentRef),
            zap.String("mpesa_receipt", callbackResult.ProviderTxID),
            zap.Float64("amount", callbackResult.Amount))

        if err := uc.paymentRepo.UpdateStatus(ctx, payment.  ID, domain.PaymentStatusCompleted); err != nil {
            uc.logger.Error("failed to update B2C payment status",
                zap.String("payment_ref", paymentRef),
                zap.Error(err))
        }

        // ✅ NEW: Debit user on partner system (complete withdrawal)
        go uc.debitUserOnPartner(payment, callbackResult)

        go uc.notifyPartner(payment, callbackResult)

    } else {
        uc.  logger.Warn("M-Pesa B2C withdrawal failed",
            zap.String("payment_ref", paymentRef),
            zap.String("result_code", callbackResult.  ResultCode),
            zap.String("result_description", callbackResult.ResultDescription))

        if err := uc.paymentRepo.UpdateStatus(ctx, payment.  ID, domain.PaymentStatusFailed); err != nil {
            uc.logger.Error("failed to update B2C payment status",
                zap.String("payment_ref", paymentRef),
                zap.Error(err))
        }

        if err := uc.paymentRepo.SetError(ctx, payment. ID, callbackResult.  ResultDescription); err != nil {
            uc.logger.Error("failed to set B2C error message",
                zap.String("payment_ref", paymentRef),
                zap.Error(err))
        }

        go uc.notifyPartner(payment, callbackResult)
    }

    return nil
}

// ProcessMpesaB2BCallback processes M-Pesa B2B callback (for bank withdrawals)
func (uc *CallbackUsecase) ProcessMpesaB2BCallback(ctx context.Context, paymentRef string, payload []byte) error {
	uc.logger.Info("received M-Pesa B2B callback",
		zap.String("payment_ref", paymentRef),
		zap.Int("payload_size", len(payload)),
		zap.String("raw_payload", string(payload))) // ✅ Log full payload for debugging

	// ✅ Use flexible parser
	callbackResult, err := uc. mpesaProvider.ParseB2BCallbackFlexible(payload)
	if err != nil {
		uc.logger.Error("failed to parse M-Pesa B2B callback",
			zap.String("payment_ref", paymentRef),
			zap.String("payload", string(payload)),
			zap.Error(err))
		return fmt.Errorf("failed to parse callback: %w", err)
	}

	uc.logger.Info("M-Pesa B2B callback parsed",
		zap.String("payment_ref", paymentRef),
		zap.  String("conversation_id", callbackResult.ConversationID),
		zap.String("result_code", callbackResult.ResultCode),
		zap.Bool("success", callbackResult.Success),
		zap.String("transaction_id", callbackResult.TransactionID),
		zap.Float64("amount", callbackResult. Amount),
		zap.Any("raw_data", callbackResult.RawData))

	// Get payment
	payment, err := uc. paymentRepo.GetByPaymentRef(ctx, paymentRef)
	if err != nil {
		uc.logger. Error("payment not found for B2B callback",
			zap.String("payment_ref", paymentRef),
			zap.Error(err))
		return fmt.Errorf("payment not found:   %w", err)
	}

	// Update callback data
	callbackData := make(map[string]interface{})
	callbackJSON, _ := json.Marshal(callbackResult)
	_ = json.Unmarshal(callbackJSON, &callbackData)

	var providerRef *string
	if callbackResult.ProviderTxID != "" {
		providerRef = &callbackResult. ProviderTxID
	}

	if err := uc.paymentRepo.UpdateCallback(ctx, payment. ID, callbackData, providerRef); err != nil {
		uc. logger.Error("failed to update B2B callback data",
			zap.String("payment_ref", paymentRef),
			zap.Error(err))
	}

	// Update status based on result
	if callbackResult.Success {
		uc.logger. Info("M-Pesa B2B bank withdrawal successful",
			zap.String("payment_ref", paymentRef),
			zap.String("transaction_id", callbackResult.TransactionID),
			zap.Float64("amount", callbackResult.Amount))

		// Update payment to completed
		if err := uc.  paymentRepo.UpdateStatus(ctx, payment. ID, domain.PaymentStatusCompleted); err != nil {
			uc.logger.Error("failed to update payment status to completed",
				zap.String("payment_ref", paymentRef),
				zap.Error(err))
		}

		// Notify partner about withdrawal completion
		go uc.debitUserOnPartner(payment, callbackResult)
		go uc.notifyPartner(payment, callbackResult)

	} else {
		uc. logger. Warn("M-Pesa B2B bank withdrawal failed",
			zap.String("payment_ref", paymentRef),
			zap.String("result_code", callbackResult.ResultCode),
			zap.String("result_description", callbackResult.ResultDescription))

		// Update payment to failed
		if err := uc. paymentRepo.  UpdateStatus(ctx, payment.ID, domain.PaymentStatusFailed); err != nil {
			uc.logger.Error("failed to update payment status to failed",
				zap.String("payment_ref", paymentRef),
				zap.Error(err))
		}

		// Set error message
		if err := uc.paymentRepo. SetError(ctx, payment.ID, callbackResult.ResultDescription); err != nil {
			uc.logger.Error("failed to set error message",
				zap.String("payment_ref", paymentRef),
				zap.Error(err))
		}

		// Still notify partner about failure
		go uc.notifyPartner(payment, callbackResult)
	}

	return nil
}

// ✅ NEW: debitUserOnPartner completes withdrawal on partner system
func (uc *CallbackUsecase) debitUserOnPartner(payment *domain.Payment, callbackResult *mpesa.CallbackResult) {
    ctx := context.Background()

    uc.logger.Info("completing withdrawal on partner system",
        zap.String("payment_ref", payment.PaymentRef),
        zap.String("user_id", payment.UserID),
        zap.Float64("amount", payment.Amount),
        zap.String("currency", payment.Currency),
        zap.String("mpesa_receipt", callbackResult.ProviderTxID))

    // Parse metadata to get original amount info
    var metadata map[string]interface{}
    if payment.Metadata != nil {
        _ = json.Unmarshal(payment.Metadata, &metadata)
    }

    // Build debit request with USD amount
    debitReq := &client.DebitUserRequest{
        UserID:         payment.UserID,
        Amount:         payment.Amount,     // ✅ USD amount
        Currency:       payment.Currency,   // ✅ USD
        TransactionRef: payment. PartnerTxRef,
        Description:    fmt.Sprintf("Withdrawal via M-Pesa - %s", payment.PartnerTxRef),
        ExternalRef:    callbackResult. ProviderTxID, // ✅ M-Pesa transaction code
        PaymentMethod:  "mpesa",
        Metadata:  map[string]interface{}{
            "payment_ref":        payment.PaymentRef,
            "mpesa_receipt":      callbackResult.ProviderTxID,
            "result_code":       callbackResult. ResultCode,
            "result_desc":       callbackResult.ResultDescription,
            //"transaction_id":    callbackResult.TransactionID,
            "original_amount":   getMetadataString(metadata, "original_amount"),
            "original_currency": getMetadataString(metadata, "original_currency"),
            "exchange_rate":     getMetadataString(metadata, "exchange_rate"),
        },
    }

    // Call partner debit API
    response, err := uc.partnerDebitClient.DebitUser(ctx, string(payment.Provider), debitReq)
    if err != nil {
        uc.logger.Error("failed to complete withdrawal on partner system",
            zap.String("payment_ref", payment.PaymentRef),
            zap.String("user_id", payment.UserID),
            zap.Error(err))

        // Mark payment as needing manual review
        _ = uc. paymentRepo.SetError(ctx, payment.ID, fmt. Sprintf("Debit failed: %v", err))
        return
    }

    uc.logger.Info("withdrawal completed successfully on partner system",
        zap.String("payment_ref", payment.PaymentRef),
        zap.String("user_id", payment. UserID),
        zap.String("transaction_ref", response.TransactionRef),
        zap.String("external_ref", response.ExternalRef),
        zap.String("status", response.Status))
}

// ✅ Helper to get metadata string value
func getMetadataString(metadata map[string]interface{}, key string) string {
    if metadata == nil {
        return ""
    }
    if val, ok := metadata[key].(string); ok {
        return val
    }
    return ""
}