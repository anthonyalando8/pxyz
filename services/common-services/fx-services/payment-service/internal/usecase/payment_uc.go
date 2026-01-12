// internal/usecase/payment_usecase.go
package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"payment-service/config"
	"payment-service/internal/domain"
	"payment-service/internal/provider/mpesa"
	"payment-service/internal/repository"

	"go.uber.org/zap"
)

type PaymentUsecase struct {
    paymentRepo    repository.PaymentRepository
    providerTxRepo repository.ProviderTransactionRepository
    mpesaProvider  *mpesa.MpesaProvider
    config         *config.Config
    logger         *zap.Logger
}

func NewPaymentUsecase(
    paymentRepo repository.PaymentRepository,
    providerTxRepo repository.ProviderTransactionRepository,
    mpesaProvider *mpesa.MpesaProvider,
    cfg *config. Config,
    logger *zap.Logger,
) *PaymentUsecase {
    return &PaymentUsecase{
        paymentRepo:    paymentRepo,
        providerTxRepo: providerTxRepo,
        mpesaProvider:  mpesaProvider,
        config:         cfg,
        logger:         logger,
    }
}

// InitiateDeposit initiates a deposit (called from partner webhook)
func (uc *PaymentUsecase) InitiateDeposit(ctx context.Context, req *domain. DepositRequest) (*domain.Payment, error) {
    // Validate request
    if err := req.Validate(); err != nil {
        uc.logger.Error("deposit validation failed",
            zap.String("transaction_ref", req.TransactionRef),
            zap.String("partner_id", req.PartnerID),
            zap.Error(err))
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    uc.logger.Info("initiating deposit",
        zap.String("transaction_ref", req.TransactionRef),
        zap.String("partner_id", req. PartnerID),
        zap.String("provider", string(req.Provider)),
        zap.Float64("amount", req.Amount),
        zap.String("currency", req.Currency),
        zap.String("user_id", req.UserID))

    // Check for duplicate (idempotency)
    existing, err := uc.paymentRepo.GetByPartnerTxRef(ctx, req.PartnerID, req.TransactionRef)
    if err == nil && existing != nil {
        uc.logger.Info("duplicate deposit request detected, returning existing payment",
            zap.String("transaction_ref", req.TransactionRef),
            zap.String("payment_ref", existing.PaymentRef),
            zap.String("status", string(existing.Status)))
        return existing, nil // Return existing payment (idempotent)
    }

    // Use transaction ref as payment ref (no internal generation)
    paymentRef := req.TransactionRef

    // Create payment record
    payment := &domain.Payment{
        PaymentRef:    paymentRef,
        PartnerID:     req. PartnerID,
        PartnerTxRef:  req.TransactionRef,
        Provider:      req.Provider,
        PaymentType:   domain.PaymentTypeDeposit,
        Amount:        req.Amount,
        Currency:      req.Currency,
        UserID:        req. UserID,
        AccountNumber:  &req.AccountNumber,
        PhoneNumber:   &req. PhoneNumber,
        Status:         domain.PaymentStatusPending,
        Description:   &req.Description,
    }

    // Add metadata
    if req. Metadata != nil {
        metadataJSON, _ := json. Marshal(req.Metadata)
        payment.Metadata = metadataJSON
    }

    // Save to database
    if err := uc.paymentRepo.Create(ctx, payment); err != nil {
        uc.logger. Error("failed to create payment",
            zap.String("transaction_ref", req.TransactionRef),
            zap.String("partner_id", req.PartnerID),
            zap.Error(err))
        return nil, fmt.Errorf("failed to create payment: %w", err)
    }

    uc.logger.Info("payment record created successfully",
        zap. Int64("payment_id", payment. ID),
        zap.String("payment_ref", payment.PaymentRef),
        zap.String("provider", string(payment.Provider)))

    // Process based on provider
    switch req.Provider {
    case domain.ProviderMpesa:
        go uc. processMpesaDeposit(payment)
    case domain.ProviderBank:
        go uc.processBankDeposit(payment)
    default:
        uc.logger.Error("unsupported payment provider",
            zap.String("provider", string(req.Provider)),
            zap.String("payment_ref", paymentRef))
        return nil, fmt.Errorf("unsupported provider: %s", req. Provider)
    }

    return payment, nil
}

// processMpesaDeposit processes M-Pesa deposit via STK Push
func (uc *PaymentUsecase) processMpesaDeposit(payment *domain.Payment) {
    ctx := context.Background()

    uc.logger.Info("processing M-Pesa deposit",
        zap.Int64("payment_id", payment.ID),
        zap.String("payment_ref", payment.PaymentRef),
        zap.String("phone_number", *payment.PhoneNumber))

    // Parse metadata to get original amount
    // Parse metadata
var metadata domain.DepositWebhookMetadata
if payment.Metadata != nil {
    metadataBytes, err := json.Marshal(payment.Metadata)
    if err != nil {
        uc.logger.Error("failed to marshal payment metadata", zap.Error(err))
    } else if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
        uc.logger.Error("failed to unmarshal payment metadata", zap.Error(err))
    }
}

// Parse original amount (KES) from metadata
	var stkAmount float64
	if metadata.OriginalAmount != "" {
		parsedAmount, err := strconv.ParseFloat(metadata.OriginalAmount, 64)
		if err != nil || parsedAmount <= 0 {
			uc.logger.Error("invalid original_amount in metadata",
				zap.String("payment_ref", payment.PaymentRef),
				zap.String("original_amount", metadata.OriginalAmount),
				zap.Error(err),
			)
		} else {
			stkAmount = parsedAmount
		}
	}

	// Fallback (last resort)
	if stkAmount <= 0 {
		uc.logger.Warn("falling back to payment amount for STK push",
			zap.String("payment_ref", payment.PaymentRef),
			zap.Float64("fallback_amount", payment.Amount),
		)
		stkAmount = payment.Amount
	}


    uc.logger.Info("initiating STK push with original amount",
        zap.String("payment_ref", payment.PaymentRef),
        zap.Float64("stk_amount", stkAmount),
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

    // Build callback URL
    callbackURL := fmt.Sprintf("%s/api/v1/callbacks/mpesa/stk/%s", uc.config.BaseCallbackURL, payment.PaymentRef)

    uc.logger.Debug("initiating M-Pesa STK push",
        zap.String("payment_ref", payment.PaymentRef),
        zap.Float64("amount", stkAmount),
        zap.String("callback_url", callbackURL))

    // ✅ Initiate STK Push with original local currency amount
    response, err := uc.mpesaProvider.InitiateSTKPush(
        ctx,
        *payment.PhoneNumber,
        payment. PartnerTxRef,
        stkAmount, // ✅ Use original amount (KES, not USD)
        callbackURL,
    )

    if err != nil {
        uc.logger.Error("M-Pesa STK push failed",
            zap.Int64("payment_id", payment. ID),
            zap.String("payment_ref", payment.PaymentRef),
            zap.Error(err))
        _ = uc.paymentRepo. SetError(ctx, payment.ID, err. Error())
        return
    }

    uc.logger.Info("M-Pesa STK push initiated",
        zap.String("payment_ref", payment.PaymentRef),
        zap.String("checkout_request_id", response.CheckoutRequestID),
        zap.String("response_code", response.ResponseCode),
        zap.String("customer_message", response.CustomerMessage))

    // Create provider transaction record
    requestPayload, _ := json.Marshal(map[string]interface{}{
        "phone_number":        *payment.PhoneNumber,
        "amount":            stkAmount, // Original amount
        "converted_amount":  payment.Amount, // USD amount
        "original_currency": metadata.OriginalCurrency,
        "target_currency":   payment.Currency,
        "callback_url":      callbackURL,
        "account_reference": payment. PartnerTxRef,
    })

    responsePayload, _ := json.Marshal(response)

    providerTx := &domain.ProviderTransaction{
        PaymentID:         payment.ID,
        Provider:          domain.ProviderMpesa,
        TransactionType:   "stk_push",
        RequestPayload:    requestPayload,
        ResponsePayload:   responsePayload,
        CheckoutRequestID: &response.CheckoutRequestID,
        Status:            domain.TxStatusSent,
    }

    if response.ResponseCode == "0" {
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
        uc.logger. Warn("M-Pesa STK push rejected",
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

// processBankDeposit processes bank deposit
func (uc *PaymentUsecase) processBankDeposit(payment *domain.Payment) {
    ctx := context.Background()

    uc.logger.Info("processing bank deposit",
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

    // TODO: Implement bank deposit processing
    uc.logger. Warn("bank deposit processing not yet implemented",
        zap.String("payment_ref", payment.PaymentRef))
}