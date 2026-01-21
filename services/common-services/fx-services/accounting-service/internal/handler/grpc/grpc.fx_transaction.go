package hgrpc

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"accounting-service/internal/domain"
	//"accounting-service/internal/usecase"
	accountingpb "x/shared/genproto/shared/accounting/v1"
	//xerrors "x/shared/utils/errors"

	//"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *AccountingHandler) ExecuteTransaction(
	ctx context.Context,
	req *accountingpb.ExecuteTransactionRequest,
) (*accountingpb.ExecuteTransactionResponse, error) {
	// Validate
	if len(req.Entries) < 2 {
		return nil, status.Error(codes.InvalidArgument, "at least 2 entries required")
	}

	// Convert
	domainReq := convertExecuteTransactionRequestToDomain(req)

	// Execute (async)
	result, err := h.txUC.ExecuteTransaction(ctx, domainReq)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return convertTransactionResultToProto(result), nil
}

func (h *AccountingHandler) ExecuteTransactionSync(
	ctx context.Context,
	req *accountingpb.ExecuteTransactionSyncRequest,
) (*accountingpb.ExecuteTransactionSyncResponse, error) {
	// Validate
	if len(req.Entries) < 2 {
		return nil, status.Error(codes.InvalidArgument, "at least 2 entries required")
	}

	// Convert (reuse ExecuteTransactionRequest conversion)
	domainReq := &domain.TransactionRequest{
		IdempotencyKey:      req.IdempotencyKey,
		TransactionType:     convertTransactionTypeToDomain(req.TransactionType),
		AccountType:         convertAccountTypeToDomain(req.AccountType),
		ExternalRef:         req.ExternalRef,
		Description:         req.Description,
		CreatedByExternalID: &req.CreatedByExternalId,
		CreatedByType:       ptrOwnerType(convertOwnerTypeToDomain(req.CreatedByType)),
		IPAddress:           req.IpAddress,
		UserAgent:           req.UserAgent,
		GenerateReceipt:     req.GenerateReceipt,
	}

	// Convert entries
	domainReq.Entries = make([]*domain.LedgerEntryRequest, len(req.Entries))
	for i, e := range req.Entries {
		domainReq.Entries[i] = convertLedgerEntryToDomain(e)
	}

	// Execute (sync)
	result, err := h.txUC.ExecuteTransactionSync(ctx, domainReq)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.ExecuteTransactionSyncResponse{
		ReceiptCode:      result.ReceiptCode,
		TransactionId:    result.TransactionID,
		Status:           convertTransactionStatusToProto(result.Status),
		Amount:           result.Amount,
		Currency:         result.Currency,
		Fee:              result.Fee,
		ProcessingTimeMs: result.ProcessingTime.Milliseconds(),
		CreatedAt:        timestamppb.New(result.CreatedAt),
	}, nil
}

func (h *AccountingHandler) BatchExecuteTransactions(
	ctx context.Context,
	req *accountingpb.BatchExecuteTransactionsRequest,
) (*accountingpb.BatchExecuteTransactionsResponse, error) {
	if len(req.Transactions) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one transaction required")
	}

	response := &accountingpb.BatchExecuteTransactionsResponse{
		Results: make([]*accountingpb.ExecuteTransactionResponse, 0, len(req.Transactions)),
		Errors:  make(map[int32]string),
	}

	for i, txReq := range req.Transactions {
		domainReq := convertExecuteTransactionRequestToDomain(txReq)

		result, err := h.txUC.ExecuteTransaction(ctx, domainReq)
		if err != nil {
			response.Errors[int32(i)] = err.Error()
			if req.FailOnFirstError {
				break
			}
			continue
		}

		response.Results = append(response.Results, convertTransactionResultToProto(result))
	}

	return response, nil
}



// ===============================
// FEE MANAGEMENT
// ===============================

func (h *AccountingHandler) CalculateFee(
	ctx context.Context,
	req *accountingpb.CalculateFeeRequest,
) (*accountingpb.CalculateFeeResponse, error) {
	txType := convertTransactionTypeToDomain(req.TransactionType)
	sourceCurrency := getStringOrEmpty(req.SourceCurrency)
	targetCurrency := getStringOrEmpty(req.TargetCurrency)

	var accountType domain.AccountType
	if req.AccountType != nil {
		accountType = convertAccountTypeToDomain(*req.AccountType)
	} else {
		accountType = domain.AccountTypeReal
	}

	var ownerType domain.OwnerType
	if req.OwnerType != nil {
		ownerType = convertOwnerTypeToDomain(*req.OwnerType)
	} else {
		ownerType = domain.OwnerTypeUser
	}

	calculation, err := h.feeUC.CalculateFee(
		ctx,
		txType,
		req.Amount,
		ptrString(sourceCurrency),
		ptrString(targetCurrency),
		ptrAccountType(accountType),
		ptrOwnerType(ownerType),
		req.ToAddress,
	)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.CalculateFeeResponse{
		Calculation: convertFeeCalculationToProto(calculation),
	}, nil
}

func (h *AccountingHandler) GetFeesByReceipt(
	ctx context.Context,
	req *accountingpb.GetFeesByReceiptRequest,
) (*accountingpb.GetFeesByReceiptResponse, error) {
	if req.ReceiptCode == "" {
		return nil, status.Error(codes.InvalidArgument, "receipt_code is required")
	}

	fees, err := h.feeUC.GetByReceipt(ctx, req.ReceiptCode)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.GetFeesByReceiptResponse{
		Fees: convertFeesToProto(fees),
	}, nil
}

func (h *AccountingHandler) GetAgentCommissionSummary(
	ctx context.Context,
	req *accountingpb.GetAgentCommissionSummaryRequest,
) (*accountingpb.GetAgentCommissionSummaryResponse, error) {
	if req.AgentExternalId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_external_id is required")
	}

	from := req.From.AsTime()
	to := req.To.AsTime()

	summary, err := h.feeUC.GetAgentCommissionSummary(ctx, req.AgentExternalId, from, to)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.GetAgentCommissionSummaryResponse{
		Commissions: summary,
	}, nil
}

// ===============================
// CREDIT
// ===============================

func (h *AccountingHandler) Credit(
	ctx context.Context,
	req *accountingpb.CreditRequest,
) (*accountingpb.CreditResponse, error) {
	// Validate
	if err := validateBasicTransaction(req. AccountNumber, req.Amount); err != nil {
		return nil, err
	}

	// Convert to domain
	domainReq := &domain.CreditRequest{
		AccountNumber:        req.AccountNumber,
		Amount:              req.Amount,
		AccountType:         convertAccountTypeToDomain(req. AccountType),
		Description:         req.Description,
		IdempotencyKey:      req.IdempotencyKey,
		ExternalRef:         req.ExternalRef,
		CreatedByExternalID: req.CreatedByExternalId,
		CreatedByType:       convertOwnerTypeToDomain(req.CreatedByType),
		TransactionType:     getTransactionType(req.TransactionType, domain.TransactionTypeTransfer),
		ToAddress: req.ToAddress,
	}

	// Execute
	aggregate, err := h.txUC.Credit(ctx, domainReq)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	// Extract receipt and balance
	receiptCode := extractReceiptCode(aggregate)
	balanceAfter := extractBalanceAfter(aggregate, domain.DrCrCredit)

	return &accountingpb.CreditResponse{
		JournalId:    aggregate.Journal.ID,
		ReceiptCode:  receiptCode,
		BalanceAfter: balanceAfter,
		CreatedAt:    timestamppb.New(aggregate.Journal.CreatedAt),
		PayableAmount: aggregate.PayableAmount,
	}, nil
}

// ===============================
// DEBIT
// ===============================

func (h *AccountingHandler) Debit(
	ctx context.Context,
	req *accountingpb.DebitRequest,
) (*accountingpb.DebitResponse, error) {
	// Validate
	if err := validateBasicTransaction(req.AccountNumber, req.Amount); err != nil {
		return nil, err
	}

	// Convert to domain
	domainReq := &domain.DebitRequest{
		AccountNumber:       req.AccountNumber,
		Amount:              req. Amount,
		AccountType:         convertAccountTypeToDomain(req.AccountType),
		Description:         req.Description,
		IdempotencyKey:      req. IdempotencyKey,
		ExternalRef:         req.ExternalRef,
		CreatedByExternalID: req.CreatedByExternalId,
		CreatedByType:       convertOwnerTypeToDomain(req.CreatedByType),
		TransactionType:     getTransactionType(req.TransactionType, domain. TransactionTypeTransfer),
		ToAddress: req.ToAddress,
	}

	// Execute
	aggregate, err := h.txUC.Debit(ctx, domainReq)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	// Extract receipt and balance
	receiptCode := extractReceiptCode(aggregate)
	balanceAfter := extractBalanceAfter(aggregate, domain. DrCrDebit)

	return &accountingpb.DebitResponse{
		JournalId:    aggregate.Journal.ID,
		ReceiptCode:   receiptCode,
		BalanceAfter: balanceAfter,
		CreatedAt:     timestamppb.New(aggregate.Journal.CreatedAt),
		PayableAmount: aggregate.PayableAmount,
	}, nil
}

// ===============================
// TRANSFER
// ===============================

func (h *AccountingHandler) Transfer(
	ctx context.Context,
	req *accountingpb. TransferRequest,
) (*accountingpb.TransferResponse, error) {
	// Validate
	if err := validateTransfer(req.FromAccountNumber, req.ToAccountNumber, req.Amount); err != nil {
		return nil, err
	}

	// Convert to domain
	domainReq := &domain.TransferRequest{
		FromAccountNumber:    req.FromAccountNumber,
		ToAccountNumber:     req. ToAccountNumber,
		Amount:               req.Amount,
		AccountType:         convertAccountTypeToDomain(req.AccountType),
		Description:         req.Description,
		IdempotencyKey:       req.IdempotencyKey,
		ExternalRef:         req.ExternalRef,
		CreatedByExternalID: req.CreatedByExternalId,
		CreatedByType:       convertOwnerTypeToDomain(req.CreatedByType),
		AgentExternalID:     req. AgentExternalId,
		TransactionType:     getTransactionType(req.TransactionType, domain.TransactionTypeTransfer),
		ToAddress: req.ToAddress,
	}

	// Execute
	aggregate, err := h.txUC.Transfer(ctx, domainReq)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	// Extract data
	receiptCode := extractReceiptCode(aggregate)
	feeAmount, agentCommission := extractFees(aggregate)

	return &accountingpb.TransferResponse{
		JournalId:       aggregate.Journal.ID,
		ReceiptCode:     receiptCode,
		FeeAmount:        feeAmount,
		AgentCommission: agentCommission,
		CreatedAt:       timestamppb.New(aggregate.Journal.CreatedAt),
		PayableAmount: aggregate.PayableAmount,
	}, nil
}

// ===============================
// CONVERT AND TRANSFER
// ===============================

func (h *AccountingHandler) ConvertAndTransfer(
	ctx context.Context,
	req *accountingpb.ConversionRequest,
) (*accountingpb.ConversionResponse, error) {
	// Validate
	if err := validateTransfer(req.FromAccountNumber, req.ToAccountNumber, req. Amount); err != nil {
		return nil, err
	}

	// Convert to domain
	domainReq := &domain.ConversionRequest{
		FromAccountNumber:   req.FromAccountNumber,
		ToAccountNumber:     req.ToAccountNumber,
		Amount:              req.Amount,
		AccountType:         convertAccountTypeToDomain(req.AccountType),
		IdempotencyKey:      req. IdempotencyKey,
		ExternalRef:         req.ExternalRef,
		CreatedByExternalID: req.CreatedByExternalId,
		CreatedByType:       convertOwnerTypeToDomain(req.CreatedByType),
		AgentExternalID:     req. AgentExternalId,
		ToAddress: req.ToAddress,
	}

	// Execute
	aggregate, err := h.txUC.ConvertAndTransfer(ctx, domainReq)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	// Extract conversion details
	receiptCode := extractReceiptCode(aggregate)
	conversionData := extractConversionData(aggregate)

	return &accountingpb.ConversionResponse{
		JournalId:       aggregate.Journal.ID,
		ReceiptCode:     receiptCode,
		SourceCurrency:  conversionData.SourceCurrency,
		DestCurrency:    conversionData.DestCurrency,
		SourceAmount:    conversionData.SourceAmount,
		ConvertedAmount: conversionData.ConvertedAmount,
		FxRate:          conversionData.FxRate,
		FxRateId:        conversionData.FxRateID,
		FeeAmount:       conversionData.FeeAmount,
		CreatedAt:       timestamppb.New(aggregate.Journal.CreatedAt),
		PayableAmount: aggregate.PayableAmount,
	}, nil
}

// ===============================
// TRADE WIN
// ===============================

func (h *AccountingHandler) ProcessTradeWin(
	ctx context.Context,
	req *accountingpb.TradeRequest,
) (*accountingpb.TradeResponse, error) {
	// Validate
	if err := validateTrade(req.AccountNumber, req.Amount, req.TradeId); err != nil {
		return nil, err
	}

	// Convert and execute
	aggregate, err := h.executeTrade(ctx, req, h.txUC. ProcessTradeWin)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return buildTradeResponse(aggregate, req.TradeId, "win", domain.DrCrCredit), nil
}

// ===============================
// TRADE LOSS
// ===============================

func (h *AccountingHandler) ProcessTradeLoss(
	ctx context.Context,
	req *accountingpb.TradeRequest,
) (*accountingpb.TradeResponse, error) {
	// Validate
	if err := validateTrade(req.AccountNumber, req.Amount, req.TradeId); err != nil {
		return nil, err
	}

	// Convert and execute
	aggregate, err := h.executeTrade(ctx, req, h.txUC.ProcessTradeLoss)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return buildTradeResponse(aggregate, req.TradeId, "loss", domain.DrCrDebit), nil
}

// ===============================
// AGENT COMMISSION
// ===============================

func (h *AccountingHandler) ProcessAgentCommission(
	ctx context.Context,
	req *accountingpb.AgentCommissionRequest,
) (*accountingpb.AgentCommissionResponse, error) {
	// Validate
	if err := validateAgentCommission(req); err != nil {
		return nil, err
	}

	// Convert to domain
	domainReq := &domain.AgentCommissionRequest{
		AgentExternalID:   req.AgentExternalId,
		TransactionRef:     req.TransactionRef,
		Currency:          req.Currency,
		TransactionAmount: req. TransactionAmount,
		CommissionAmount:  req.CommissionAmount,
		CommissionRate:    req.CommissionRate,
		IdempotencyKey:    req.IdempotencyKey,
	}

	// Execute
	aggregate, err := h.txUC.ProcessAgentCommission(ctx, domainReq)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.AgentCommissionResponse{
		JournalId:        aggregate.Journal.ID,
		ReceiptCode:       extractReceiptCode(aggregate),
		AgentExternalId:   req.AgentExternalId,
		CommissionAmount: req.CommissionAmount,
		CreatedAt:        timestamppb.New(aggregate.Journal.CreatedAt),
	}, nil
}

// ===============================
// VALIDATION HELPERS
// ===============================

func validateBasicTransaction(accountNumber string, amount float64) error {
	if accountNumber == "" {
		return status.Error(codes. InvalidArgument, "account_number is required")
	}
	if amount <= 0 {
		return status.Error(codes. InvalidArgument, "amount must be positive")
	}
	return nil
}

func validateTransfer(fromAccount, toAccount string, amount float64) error {
	if fromAccount == "" || toAccount == "" {
		return status.Error(codes.InvalidArgument, "from_account_number and to_account_number are required")
	}
	if fromAccount == toAccount {
		return status.Error(codes.InvalidArgument, "cannot transfer to same account")
	}
	if amount <= 0 {
		return status.Error(codes.InvalidArgument, "amount must be positive")
	}
	return nil
}

func validateTrade(accountNumber string, amount float64, tradeID string) error {
	if err := validateBasicTransaction(accountNumber, amount); err != nil {
		return err
	}
	if tradeID == "" {
		return status.Error(codes. InvalidArgument, "trade_id is required")
	}
	return nil
}

func validateAgentCommission(req *accountingpb.AgentCommissionRequest) error {
	if req.AgentExternalId == "" {
		return status.Error(codes.InvalidArgument, "agent_external_id is required")
	}
	if req.TransactionRef == "" {
		return status.Error(codes.InvalidArgument, "transaction_ref is required")
	}
	if req. CommissionAmount <= 0 {
		return status.Error(codes.InvalidArgument, "commission_amount must be positive")
	}
	return nil
}

// ===============================
// EXTRACTION HELPERS
// ===============================

// extractReceiptCode extracts receipt code with fallback
func extractReceiptCode(aggregate *domain.LedgerAggregate) string {
	if aggregate == nil {
		return ""
	}

	// Try ledgers first (primary source)
	for _, ledger := range aggregate. Ledgers {
		if ledger. ReceiptCode != nil && *ledger.ReceiptCode != "" {
			return *ledger.ReceiptCode
		}
	}

	// Fallback to journal external_ref
	if aggregate.Journal. ExternalRef != nil {
		return *aggregate.Journal. ExternalRef
	}

	return ""
}

// extractBalanceAfter extracts balance for specific debit/credit side
func extractBalanceAfter(aggregate *domain.LedgerAggregate, drCr domain.DrCr) float64 {
	if aggregate == nil || len(aggregate.Ledgers) == 0 {
		return 0
	}

	for _, ledger := range aggregate. Ledgers {
		if ledger.DrCr == drCr && ledger.BalanceAfter != nil {
			return *ledger.BalanceAfter
		}
	}

	return 0
}

// extractFees extracts platform fee and agent commission
func extractFees(aggregate *domain.LedgerAggregate) (platformFee, agentCommission float64) {
	if aggregate == nil || len(aggregate.Fees) == 0 {
		return 0, 0
	}

	for _, fee := range aggregate.Fees {
		switch fee.FeeType {
		case domain. FeeTypePlatform:
			platformFee = fee.Amount
		case domain.FeeTypeAgentCommission:
			agentCommission = fee.Amount
		}
	}

	return platformFee, agentCommission
}

// ConversionData holds extracted conversion details
type ConversionData struct {
	SourceCurrency  string
	DestCurrency    string
	SourceAmount    float64
	ConvertedAmount float64
	FxRate          string
	FxRateID        int64
	FeeAmount       float64
}

// extractConversionData extracts all conversion-related data
func extractConversionData(aggregate *domain.LedgerAggregate) ConversionData {
	data := ConversionData{}

	if aggregate == nil || len(aggregate. Ledgers) < 2 {
		return data
	}

	// Extract from ledgers
	for _, ledger := range aggregate.Ledgers {
		if ledger.DrCr == domain.DrCrDebit {
			data.SourceCurrency = ledger.Currency
			data.SourceAmount = ledger.Amount
			data. FxRate, data.FxRateID = extractMetadata(ledger.Metadata)
		} else {
			data.DestCurrency = ledger.Currency
			data.ConvertedAmount = ledger.Amount
		}
	}

	// Extract fee
	for _, fee := range aggregate.Fees {
		if fee.FeeType == domain.FeeTypePlatform || fee.FeeType == domain. FeeTypeConversion {
			data.FeeAmount = fee.Amount
			break
		}
	}

	return data
}

// extractMetadata safely extracts fx_rate and fx_rate_id from metadata
func extractMetadata(metadata interface{}) (fxRate string, fxRateID int64) {
	if metadata == nil {
		return "", 0
	}

	metaVal := reflect.ValueOf(metadata)
	if ! metaVal.IsValid() || metaVal.Kind() != reflect.Map {
		return "", 0
	}

	for _, k := range metaVal.MapKeys() {
		v := metaVal.MapIndex(k)
		keyStr := fmt.Sprint(k. Interface())

		switch keyStr {
		case "fx_rate", "fxRate", "rate":
			fxRate = fmt.Sprint(v.Interface())
		case "fx_rate_id", "fxRateId", "fx_rateid", "fxRateID": 
			if idStr := fmt.Sprint(v.Interface()); idStr != "" {
				if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
					fxRateID = id
				}
			}
		}
	}

	return fxRate, fxRateID
}

// ===============================
// TRADE HELPERS
// ===============================

// executeTrade is a helper for trade operations
func (h *AccountingHandler) executeTrade(
	ctx context.Context,
	req *accountingpb.TradeRequest,
	fn func(context.Context, *domain.TradeRequest) (*domain.LedgerAggregate, error),
) (*domain.LedgerAggregate, error) {
	domainReq := &domain.TradeRequest{
		AccountNumber:        req.AccountNumber,
		Amount:              req.Amount,
		AccountType:         convertAccountTypeToDomain(req.AccountType),
		TradeID:             req.TradeId,
		TradeType:           req.TradeType,
		IdempotencyKey:      req. IdempotencyKey,
		CreatedByExternalID: req. CreatedByExternalId,
		CreatedByType:       convertOwnerTypeToDomain(req.CreatedByType),
	}

	return fn(ctx, domainReq)
}

// buildTradeResponse builds trade response from aggregate
func buildTradeResponse(aggregate *domain.LedgerAggregate, tradeID, result string, drCr domain.DrCr) *accountingpb.TradeResponse {
	return &accountingpb.TradeResponse{
		JournalId:    aggregate.Journal.ID,
		ReceiptCode:  extractReceiptCode(aggregate),
		TradeId:      tradeID,
		TradeResult:  result,
		BalanceAfter: extractBalanceAfter(aggregate, drCr),
		CreatedAt:    timestamppb.New(aggregate.Journal.CreatedAt),
	}
}

// ===============================
// CONVERSION HELPERS
// ===============================

func getTransactionType(protoType accountingpb.TransactionType, defaultType domain.TransactionType) domain.TransactionType {
	if protoType != 0 {
		return convertTransactionTypeToDomain(protoType)
	}
	return defaultType
}