package hgrpc

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"accounting-service/internal/domain"
	"accounting-service/internal/usecase"
	accountingpb "x/shared/genproto/shared/accounting/v1"
	xerrors "x/shared/utils/errors"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AccountingHandler struct {
	accountingpb.UnimplementedAccountingServiceServer

	// Usecases
	accountUC   *usecase.AccountUsecase
	txUC        *usecase.TransactionUsecase
	statementUC *usecase.StatementUsecase
	journalUC   *usecase.JournalUsecase
	ledgerUC    *usecase.LedgerUsecase
	feeUC       *usecase.TransactionFeeUsecase
	feeRuleUC   *usecase.TransactionFeeRuleUsecase
	agentUC usecase.AgentUsecase


	// Infrastructure
	redisClient *redis.Client
}

func NewAccountingHandler(
	accountUC *usecase.AccountUsecase,
	txUC *usecase.TransactionUsecase,
	statementUC *usecase.StatementUsecase,
	journalUC *usecase.JournalUsecase,
	ledgerUC *usecase.LedgerUsecase,
	feeUC *usecase.TransactionFeeUsecase,
	feeRuleUC *usecase.TransactionFeeRuleUsecase,
	agentUC usecase.AgentUsecase,
	redisClient *redis.Client,

) *AccountingHandler {
	return &AccountingHandler{
		accountUC:   accountUC,
		txUC:        txUC,
		statementUC: statementUC,
		journalUC:   journalUC,
		ledgerUC:    ledgerUC,
		feeUC:       feeUC,
		feeRuleUC:   feeRuleUC,
		redisClient: redisClient,
		agentUC:    agentUC,
	}
}

// ===============================
// ACCOUNT MANAGEMENT
// ===============================

func (h *AccountingHandler) CreateAccount(
	ctx context.Context,
	req *accountingpb.CreateAccountRequest,
) (*accountingpb.CreateAccountResponse, error) {
	// Begin transaction
	tx, err := h.accountUC.BeginTx(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	// Convert and create
	domainReq := convertCreateAccountRequestToDomain(req)
	account, err := h.accountUC.CreateAccount(ctx, domainReq, tx)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	// Commit
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Internal, "failed to commit transaction")
	}

	return &accountingpb.CreateAccountResponse{
		Account: convertAccountToProto(account),
	}, nil
}

func (h *AccountingHandler) CreateAccounts(
	ctx context.Context,
	req *accountingpb.CreateAccountsRequest,
) (*accountingpb.CreateAccountsResponse, error) {
	if len(req.Accounts) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one account required")
	}

	// Begin transaction
	tx, err := h.accountUC.BeginTx(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	// Convert requests
	createRequests := make([]*domain.CreateAccountRequest, len(req.Accounts))
	for i, protoReq := range req.Accounts {
		createRequests[i] = convertCreateAccountRequestToDomain(protoReq)
	}

	// Create accounts (batch) - returns only error map
	errs := h.accountUC.CreateAccounts(ctx, createRequests, tx)

	// Determine which accounts succeeded
	successfulAccounts := make([]*domain.Account, 0)
	for i, createReq := range createRequests {
		if _, hasError := errs[i]; !hasError {
			// Account was created successfully
			// You may want to fetch it back or convert from createReq
			account := &domain.Account{
				OwnerType:   createReq.OwnerType,
				OwnerID:     createReq.OwnerID,
				Currency:    createReq.Currency,
				Purpose:     createReq.Purpose,
				AccountType: createReq.AccountType,
				// IsActive:              createReq.IsActive,
				// IsLocked:              createReq.IsLocked,
				OverdraftLimit:        createReq.OverdraftLimit,
				ParentAgentExternalID: createReq.ParentAgentExternalID,
				CommissionRate:        createReq.CommissionRate,
				//AccountNumber:         createReq.AccountNumber,
			}
			successfulAccounts = append(successfulAccounts, account)
		}
	}

	// Commit if at least one succeeded
	if len(successfulAccounts) > 0 {
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return nil, status.Error(codes.Internal, "failed to commit transaction")
		}
	} else {
		// All failed, rollback
		return nil, status.Error(codes.Internal, "all accounts failed to create")
	}

	// Build response
	response := &accountingpb.CreateAccountsResponse{
		Accounts: convertAccountsToProto(successfulAccounts),
		Errors:   make(map[int32]string),
	}

	for idx, err := range errs {
		response.Errors[int32(idx)] = err.Error()
	}

	return response, nil
}

func (h *AccountingHandler) GetAccount(
	ctx context.Context,
	req *accountingpb.GetAccountRequest,
) (*accountingpb.GetAccountResponse, error) {
	var account *domain.Account
	var err error

	switch id := req.Identifier.(type) {
	case *accountingpb.GetAccountRequest_Id:
		account, err = h.accountUC.GetByID(ctx, id.Id)
	case *accountingpb.GetAccountRequest_AccountNumber:
		account, err = h.accountUC.GetByAccountNumber(ctx, id.AccountNumber)
	default:
		return nil, status.Error(codes.InvalidArgument, "identifier is required")
	}

	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.GetAccountResponse{
		Account: convertAccountToProto(account),
	}, nil
}

func (h *AccountingHandler) GetAccountsByOwner(
	ctx context.Context,
	req *accountingpb.GetAccountsByOwnerRequest,
) (*accountingpb.GetAccountsByOwnerResponse, error) {
	if req.OwnerId == "" {
		return nil, status.Error(codes.InvalidArgument, "owner_id is required")
	}

	ownerType := convertOwnerTypeToDomain(req.OwnerType)
	accountType := convertAccountTypeToDomain(req.AccountType)

	accounts, err := h.accountUC.GetByOwner(ctx, ownerType, req.OwnerId, accountType)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.GetAccountsByOwnerResponse{
		Accounts: convertAccountsToProto(accounts),
	}, nil
}

func (h *AccountingHandler) GetOrCreateUserAccounts(
	ctx context.Context,
	req *accountingpb.GetOrCreateUserAccountsRequest,
) (*accountingpb.GetOrCreateUserAccountsResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	accountType := convertAccountTypeToDomain(req.AccountType)
	ownerType := convertOwnerTypeToDomain(req.OwnerType)

	// Begin transaction
	tx, err := h.accountUC.BeginTx(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	accounts, err := h.accountUC.GetOrCreateUserAccounts(ctx, ownerType, req.UserId, accountType, tx)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	// Commit
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Internal, "failed to commit transaction")
	}

	return &accountingpb.GetOrCreateUserAccountsResponse{
		Accounts: convertAccountsToProto(accounts),
	}, nil
}

func (h *AccountingHandler) UpdateAccount(
	ctx context.Context,
	req *accountingpb.UpdateAccountRequest,
) (*accountingpb.UpdateAccountResponse, error) {
	// Get existing account
	account, err := h.accountUC.GetByID(ctx, req.Id)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	// Update fields
	if req.IsActive != nil {
		account.IsActive = *req.IsActive
	}
	if req.IsLocked != nil {
		account.IsLocked = *req.IsLocked
	}
	if req.OverdraftLimit != nil {
		account.OverdraftLimit = *req.OverdraftLimit
	}

	// Begin transaction
	tx, err := h.accountUC.BeginTx(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	// Update
	if err := h.accountUC.UpdateAccount(ctx, account, tx); err != nil {
		return nil, handleUsecaseError(err)
	}

	// Commit
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Internal, "failed to commit transaction")
	}

	return &accountingpb.UpdateAccountResponse{
		Account: convertAccountToProto(account),
	}, nil
}

func (h *AccountingHandler) GetBalance(
	ctx context.Context,
	req *accountingpb.GetBalanceRequest,
) (*accountingpb.GetBalanceResponse, error) {
	var balance *domain.Balance
	var account *domain.Account
	var err error

	switch id := req.Identifier.(type) {
	case *accountingpb.GetBalanceRequest_AccountId:
		balance, err = h.statementUC.GetCachedBalance(ctx, id.AccountId)
		if err == nil && balance != nil {
			account, _ = h.accountUC.GetByID(ctx, id.AccountId)
		}
	case *accountingpb.GetBalanceRequest_AccountNumber:
		account, err = h.accountUC.GetByAccountNumber(ctx, id.AccountNumber)
		if err == nil && account != nil {
			balance, err = h.statementUC.GetCachedBalance(ctx, account.ID)
		}
	default:
		return nil, status.Error(codes.InvalidArgument, "identifier is required")
	}

	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.GetBalanceResponse{
		Balance: convertBalanceToProto(balance, account),
	}, nil
}

func (h *AccountingHandler) BatchGetBalances(
	ctx context.Context,
	req *accountingpb.BatchGetBalancesRequest,
) (*accountingpb.BatchGetBalancesResponse, error) {
	if len(req.AccountNumbers) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one account_number required")
	}

	response := &accountingpb.BatchGetBalancesResponse{
		Balances: make([]*accountingpb.Balance, 0, len(req.AccountNumbers)),
		Errors:   make(map[int32]string),
	}

	for i, accountNumber := range req.AccountNumbers {
		account, err := h.accountUC.GetByAccountNumber(ctx, accountNumber)
		if err != nil {
			response.Errors[int32(i)] = err.Error()
			continue
		}

		balance, err := h.statementUC.GetCachedBalance(ctx, account.ID)
		if err != nil {
			response.Errors[int32(i)] = err.Error()
			continue
		}

		response.Balances = append(response.Balances, convertBalanceToProto(balance, account))
	}

	return response, nil
}

// ===============================
// TRANSACTION EXECUTION
// ===============================

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

func (h *AccountingHandler) GetTransactionStatus(
	ctx context.Context,
	req *accountingpb.GetTransactionStatusRequest,
) (*accountingpb.GetTransactionStatusResponse, error) {
	if req.ReceiptCode == "" {
		return nil, status.Error(codes.InvalidArgument, "receipt_code is required")
	}

	status, err := h.txUC.GetTransactionStatus(ctx, req.ReceiptCode)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	response := &accountingpb.GetTransactionStatusResponse{
		ReceiptCode: status.ReceiptCode,
		Status:      convertTransactionStatusToProto(status.Status),
		StartedAt:   timestamppb.New(status.StartedAt),
	}

	if status.ErrorMessage != "" {
		response.ErrorMessage = &status.ErrorMessage
	}

	if !status.UpdatedAt.IsZero() {
		response.CompletedAt = timestamppb.New(status.UpdatedAt)
	}

	return response, nil
}

func (h *AccountingHandler) GetTransactionByReceipt(
	ctx context.Context,
	req *accountingpb.GetTransactionByReceiptRequest,
) (*accountingpb.GetTransactionByReceiptResponse, error) {
	if req.ReceiptCode == "" {
		return nil, status.Error(codes.InvalidArgument, "receipt_code is required")
	}

	// Get journals by receipt
	journals, err := h.journalUC.GetByExternalRef(ctx, req.ReceiptCode)
	if err != nil || len(journals) == 0 {
		return nil, handleUsecaseError(err)
	}

	journal := journals[0] // Take first journal

	// Get ledgers
	ledgers, err := h.ledgerUC.ListByJournal(ctx, journal.ID)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	// Get fees
	fees, err := h.feeUC.GetByReceipt(ctx, req.ReceiptCode)
	if err != nil {
		fees = []*domain.TransactionFee{} // Empty on error
	}

	return &accountingpb.GetTransactionByReceiptResponse{
		Journal: convertJournalToProto(journal),
		Ledgers: convertLedgersToProto(ledgers),
		Fees:    convertFeesToProto(fees),
	}, nil
}

// ===============================
// JOURNAL & LEDGER QUERIES
// ===============================

func (h *AccountingHandler) GetJournal(
	ctx context.Context,
	req *accountingpb.GetJournalRequest,
) (*accountingpb.GetJournalResponse, error) {
	journal, err := h.journalUC.GetByID(ctx, req.Id)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.GetJournalResponse{
		Journal: convertJournalToProto(journal),
	}, nil
}

func (h *AccountingHandler) ListJournals(
	ctx context.Context,
	req *accountingpb.ListJournalsRequest,
) (*accountingpb.ListJournalsResponse, error) {
	filter := &domain.JournalFilter{
		TransactionType: convertOptionalTransactionTypeToDomain(req.TransactionType),
		AccountType:     convertOptionalAccountTypeToDomain(req.AccountType),
		ExternalRef:     req.ExternalRef,
		CreatedByID:     req.CreatedByExternalId,
		StartDate:       convertOptionalTimestamp(req.From),
		EndDate:         convertOptionalTimestamp(req.To),
		Limit:           int(req.Limit),
		Offset:          int(req.Offset),
	}

	journals, total, err := h.journalUC.List(ctx, filter)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.ListJournalsResponse{
		Journals: convertJournalsToProto(journals),
		Total:    int32(total),
	}, nil
}

func (h *AccountingHandler) ListLedgersByJournal(
	ctx context.Context,
	req *accountingpb.ListLedgersByJournalRequest,
) (*accountingpb.ListLedgersByJournalResponse, error) {
	ledgers, err := h.ledgerUC.ListByJournal(ctx, req.JournalId)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.ListLedgersByJournalResponse{
		Ledgers: convertLedgersToProto(ledgers),
	}, nil
}

func (h *AccountingHandler) ListLedgersByAccount(
	ctx context.Context,
	req *accountingpb.ListLedgersByAccountRequest,
) (*accountingpb.ListLedgersByAccountResponse, error) {
	if req.AccountNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "account_number is required")
	}

	accountType := convertAccountTypeToDomain(req.AccountType)
	from := convertOptionalTimestamp(req.From)
	to := convertOptionalTimestamp(req.To)
	limit := int(req.Limit)
	offset := int(req.Offset)

	ledgers, total, err := h.ledgerUC.ListByAccount(ctx, req.AccountNumber, accountType, from, to, limit, offset)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.ListLedgersByAccountResponse{
		Ledgers: convertLedgersToProto(ledgers),
		Total:   int32(total),
	}, nil
}

// ===============================
// STATEMENTS & REPORTING
// ===============================

func (h *AccountingHandler) GetAccountStatement(
	ctx context.Context,
	req *accountingpb.GetAccountStatementRequest,
) (*accountingpb.GetAccountStatementResponse, error) {
	if req.AccountNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "account_number is required")
	}

	accountType := convertAccountTypeToDomain(req.AccountType)
	from := req.From.AsTime()
	to := req.To.AsTime()

	stmt, err := h.statementUC.GetAccountStatement(ctx, req.AccountNumber, accountType, from, to)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.GetAccountStatementResponse{
		Statement: convertAccountStatementToProto(stmt),
	}, nil
}

func (h *AccountingHandler) GetOwnerStatement(
	ctx context.Context,
	req *accountingpb.GetOwnerStatementRequest,
) (*accountingpb.GetOwnerStatementResponse, error) {
	if req.OwnerId == "" {
		return nil, status.Error(codes.InvalidArgument, "owner_id is required")
	}

	ownerType := convertOwnerTypeToDomain(req.OwnerType)
	accountType := convertAccountTypeToDomain(req.AccountType)
	from := req.From.AsTime()
	to := req.To.AsTime()

	statements, err := h.statementUC.GetOwnerStatement(ctx, ownerType, req.OwnerId, accountType, from, to)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.GetOwnerStatementResponse{
		Statements: convertAccountStatementsToProto(statements),
	}, nil
}

func (h *AccountingHandler) GetOwnerSummary(
	ctx context.Context,
	req *accountingpb.GetOwnerSummaryRequest,
) (*accountingpb.GetOwnerSummaryResponse, error) {
	if req.OwnerId == "" {
		return nil, status.Error(codes.InvalidArgument, "owner_id is required")
	}

	ownerType := convertOwnerTypeToDomain(req.OwnerType)
	accountType := convertAccountTypeToDomain(req.AccountType)

	summary, err := h.statementUC.GetOwnerSummary(ctx, ownerType, req.OwnerId, accountType)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.GetOwnerSummaryResponse{
		Summary: convertOwnerSummaryToProto(summary),
	}, nil
}

func (h *AccountingHandler) GenerateDailyReport(
	ctx context.Context,
	req *accountingpb.GenerateDailyReportRequest,
) (*accountingpb.GenerateDailyReportResponse, error) {
	date := req.Date.AsTime()
	accountType := convertAccountTypeToDomain(req.AccountType)

	reports, err := h.statementUC.GenerateDailyReport(ctx, date, accountType)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.GenerateDailyReportResponse{
		Reports: convertDailyReportsToProto(reports),
	}, nil
}

func (h *AccountingHandler) GetTransactionSummary(
	ctx context.Context,
	req *accountingpb.GetTransactionSummaryRequest,
) (*accountingpb.GetTransactionSummaryResponse, error) {
	accountType := convertAccountTypeToDomain(req.AccountType)
	from := req.From.AsTime()
	to := req.To.AsTime()

	summaries, err := h.statementUC.GetTransactionSummary(ctx, accountType, from, to)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.GetTransactionSummaryResponse{
		Summaries: convertTransactionSummariesToProto(summaries),
	}, nil
}

func (h *AccountingHandler) GetSystemHoldings(
	ctx context.Context,
	req *accountingpb.GetSystemHoldingsRequest,
) (*accountingpb.GetSystemHoldingsResponse, error) {
	accountType := convertAccountTypeToDomain(req.AccountType)

	holdings, err := h.statementUC.GetSystemHoldings(ctx, accountType)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	return &accountingpb.GetSystemHoldingsResponse{
		Holdings: holdings,
	}, nil
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
// STREAMING
// ===============================

func (h *AccountingHandler) StreamTransactionEvents(
	req *accountingpb.StreamTransactionEventsRequest,
	stream accountingpb.AccountingService_StreamTransactionEventsServer,
) error {
	// TODO: Implement streaming with Kafka or Redis pub/sub
	return status.Error(codes.Unimplemented, "streaming not yet implemented")
}

// ===============================
// HEALTH & MONITORING
// ===============================

func (h *AccountingHandler) HealthCheck(
	ctx context.Context,
	req *accountingpb.HealthCheckRequest,
) (*accountingpb.HealthCheckResponse, error) {
	components := make(map[string]string)

	// Check Redis
	if err := h.redisClient.Ping(ctx).Err(); err != nil {
		components["redis"] = "unhealthy"
	} else {
		components["redis"] = "healthy"
	}

	// Determine overall status
	overallStatus := "healthy"
	for _, status := range components {
		if status == "unhealthy" {
			overallStatus = "degraded"
			break
		}
	}

	return &accountingpb.HealthCheckResponse{
		Status:     overallStatus,
		Components: components,
		Timestamp:  time.Now().Unix(),
	}, nil
}

// ===============================
// ERROR HANDLING
// ===============================

func handleUsecaseError(err error) error {
	if err == nil {
		return nil
	}

	// Create a base logger with the original error and function context
	logger := log.WithFields(log.Fields{
		"function":   "handleUsecaseError",
		"error":      err.Error(),
		"error_type": fmt.Sprintf("%T", err),
	})

	switch {
	case errors.Is(err, xerrors.ErrNotFound):
		logger.WithField("grpc_code", codes.NotFound).Warn("resource not found")
		return status.Error(codes.NotFound, err.Error())

	case errors.Is(err, xerrors.ErrInsufficientBalance):
		logger.WithField("grpc_code", codes.FailedPrecondition).Warn("insufficient balance for transaction")
		return status.Error(codes.FailedPrecondition, err.Error())

	case errors.Is(err, xerrors.ErrAccountLocked):
		logger.WithField("grpc_code", codes.PermissionDenied).Warn("attempted operation on locked account")
		return status.Error(codes.PermissionDenied, "account is locked")

	case errors.Is(err, xerrors.ErrAccountInactive):
		logger.WithField("grpc_code", codes.PermissionDenied).Warn("attempted operation on inactive account")
		return status.Error(codes.PermissionDenied, "account is inactive")

	case errors.Is(err, xerrors.ErrDuplicateIdempotencyKey):
		logger.WithField("grpc_code", codes.AlreadyExists).Warn("duplicate idempotency key detected")
		return status.Error(codes.AlreadyExists, "duplicate idempotency key")

	case errors.Is(err, xerrors.ErrConcurrentModification):
		logger.WithField("grpc_code", codes.Aborted).Warn("concurrent modification detected, client should retry")
		return status.Error(codes.Aborted, "concurrent modification, retry")

	case errors.Is(err, context.DeadlineExceeded):
		logger.WithField("grpc_code", codes.DeadlineExceeded).Error("request deadline exceeded")
		return status.Error(codes.DeadlineExceeded, "request timeout")

	case errors.Is(err, context.Canceled):
		logger.WithField("grpc_code", codes.Canceled).Info("request canceled by client")
		return status.Error(codes.Canceled, "request canceled")

	default:
		logger.WithField("grpc_code", codes.Internal).Error("unhandled error - internal server error")
		return status.Error(codes.Internal, "internal server error")
	}
}

// ===============================
// HELPER FUNCTIONS
// ===============================

func ptrOwnerType(t domain.OwnerType) *domain.OwnerType {
	return &t
}

func ptrAccountType(t domain.AccountType) *domain.AccountType {
	return &t
}

//additional helper functions

func (h *AccountingHandler) Credit(
	ctx context.Context,
	req *accountingpb.CreditRequest,
) (*accountingpb.CreditResponse, error) {
	// Validate
	if req.AccountNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "account_number is required")
	}
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}
	if req.Currency == "" {
		return nil, status.Error(codes.InvalidArgument, "currency is required")
	}
	transactionType := domain.TransactionTypeTransfer
	if req.TransactionType != 0 {
		transactionType = convertTransactionTypeToDomain(req.TransactionType)
	}

	// Convert to domain
	domainReq := &domain.CreditRequest{
		AccountNumber:       req.AccountNumber,
		Amount:              req.Amount,
		Currency:            req.Currency,
		AccountType:         convertAccountTypeToDomain(req.AccountType),
		Description:         req.Description,
		IdempotencyKey:      req.IdempotencyKey,
		ExternalRef:         req.ExternalRef,
		CreatedByExternalID: req.CreatedByExternalId,
		CreatedByType:       convertOwnerTypeToDomain(req.CreatedByType),
		TransactionType:    transactionType,
	}

	// Execute
	aggregate, err := h.txUC.Credit(ctx, domainReq)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	// Get balance after
	var balanceAfter float64
	if len(aggregate.Ledgers) > 0 {
		for _, ledger := range aggregate.Ledgers {
			if ledger.DrCr == domain.DrCrCredit && ledger.BalanceAfter != nil {
				balanceAfter = *ledger.BalanceAfter
				break
			}
		}
	}

	receiptCode := ""
	if aggregate.Journal.ExternalRef != nil {
		receiptCode = *aggregate.Journal.ExternalRef
	}

	return &accountingpb.CreditResponse{
		JournalId:    aggregate.Journal.ID,
		ReceiptCode:  receiptCode,
		BalanceAfter: balanceAfter,
		CreatedAt:    timestamppb.New(aggregate.Journal.CreatedAt),
	}, nil
}

// Debit removes money from account (user → system)
func (h *AccountingHandler) Debit(
	ctx context.Context,
	req *accountingpb.DebitRequest,
) (*accountingpb.DebitResponse, error) {
	// Validate
	if req.AccountNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "account_number is required")
	}
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}
	if req.Currency == "" {
		return nil, status.Error(codes.InvalidArgument, "currency is required")
	}
	transactionType := domain.TransactionTypeTransfer
	if req.TransactionType != 0 {
		transactionType = convertTransactionTypeToDomain(req.TransactionType)
	}

	// Convert to domain
	domainReq := &domain.DebitRequest{
		AccountNumber:       req.AccountNumber,
		Amount:              req.Amount,
		Currency:            req.Currency,
		AccountType:         convertAccountTypeToDomain(req.AccountType),
		Description:         req.Description,
		IdempotencyKey:      req.IdempotencyKey,
		ExternalRef:         req.ExternalRef,
		CreatedByExternalID: req.CreatedByExternalId,
		CreatedByType:       convertOwnerTypeToDomain(req.CreatedByType),
		TransactionType:    transactionType,
	}

	// Execute
	aggregate, err := h.txUC.Debit(ctx, domainReq)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	// Get balance after
	var balanceAfter float64
	if len(aggregate.Ledgers) > 0 {
		for _, ledger := range aggregate.Ledgers {
			if ledger.DrCr == domain.DrCrDebit && ledger.BalanceAfter != nil {
				balanceAfter = *ledger.BalanceAfter
				break
			}
		}
	}

	receiptCode := ""
	if aggregate.Journal.ExternalRef != nil {
		receiptCode = *aggregate.Journal.ExternalRef
	}

	return &accountingpb.DebitResponse{
		JournalId:    aggregate.Journal.ID,
		ReceiptCode:  receiptCode,
		BalanceAfter: balanceAfter,
		CreatedAt:    timestamppb.New(aggregate.Journal.CreatedAt),
	}, nil
}

// Transfer moves money between accounts (P2P)
func (h *AccountingHandler) Transfer(
	ctx context.Context,
	req *accountingpb.TransferRequest,
) (*accountingpb.TransferResponse, error) {
	// Validate
	if req.FromAccountNumber == "" || req.ToAccountNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "from_account_number and to_account_number are required")
	}
	if req.FromAccountNumber == req.ToAccountNumber {
		return nil, status.Error(codes.InvalidArgument, "cannot transfer to same account")
	}
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}

	transactionType := domain.TransactionTypeTransfer
	if req.TransactionType != 0 {
		transactionType = convertTransactionTypeToDomain(req.TransactionType)
	}

	// Convert to domain
	domainReq := &domain.TransferRequest{
		FromAccountNumber:   req.FromAccountNumber,
		ToAccountNumber:     req.ToAccountNumber,
		Amount:              req.Amount,
		AccountType:         convertAccountTypeToDomain(req.AccountType),
		Description:         req.Description,
		IdempotencyKey:      req.IdempotencyKey,
		ExternalRef:         req.ExternalRef,
		CreatedByExternalID: req.CreatedByExternalId,
		CreatedByType:       convertOwnerTypeToDomain(req.CreatedByType),
		AgentExternalID:     req.AgentExternalId,
		TransactionType: transactionType,
	}

	// Execute
	aggregate, err := h.txUC.Transfer(ctx, domainReq)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	receiptCode := ""
	if aggregate.Journal.ExternalRef != nil {
		receiptCode = *aggregate.Journal.ExternalRef
	}

	// Get fees if available
	var feeAmount float64
	var agentCommission float64
	if len(aggregate.Fees) > 0 {
		for _, fee := range aggregate.Fees {
			if fee.FeeType == domain.FeeTypePlatform {
				feeAmount = fee.Amount
			}
			if fee.FeeType == domain.FeeTypeAgentCommission {
				agentCommission = fee.Amount
			}
		}
	}

	return &accountingpb.TransferResponse{
		JournalId:       aggregate.Journal.ID,
		ReceiptCode:     receiptCode,
		FeeAmount:       feeAmount,
		AgentCommission: agentCommission,
		CreatedAt:       timestamppb.New(aggregate.Journal.CreatedAt),
	}, nil
}

// ConvertAndTransfer performs currency conversion
func (h *AccountingHandler) ConvertAndTransfer(
	ctx context.Context,
	req *accountingpb.ConversionRequest,
) (*accountingpb.ConversionResponse, error) {
	// Validate
	if req.FromAccountNumber == "" || req.ToAccountNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "from_account_number and to_account_number are required")
	}
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}

	// Convert to domain
	domainReq := &domain.ConversionRequest{
		FromAccountNumber:   req.FromAccountNumber,
		ToAccountNumber:     req.ToAccountNumber,
		Amount:              req.Amount,
		AccountType:         convertAccountTypeToDomain(req.AccountType),
		IdempotencyKey:      req.IdempotencyKey,
		ExternalRef:         req.ExternalRef,
		CreatedByExternalID: req.CreatedByExternalId,
		CreatedByType:       convertOwnerTypeToDomain(req.CreatedByType),
		AgentExternalID:     req.AgentExternalId,
	}

	// Execute
	aggregate, err := h.txUC.ConvertAndTransfer(ctx, domainReq)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	receiptCode := ""
	if aggregate.Journal.ExternalRef != nil {
		receiptCode = *aggregate.Journal.ExternalRef
	}

	// Extract metadata from ledgers
	var sourceCurrency, destCurrency string
	var sourceAmount, convertedAmount float64
	var fxRate string
	var fxRateID int64
	var feeAmount float64

	if len(aggregate.Ledgers) >= 2 {
		// Source ledger (debit)
		sourceCurrency = aggregate.Ledgers[0].Currency
		sourceAmount = aggregate.Ledgers[0].Amount

		// Destination ledger (credit)
		destCurrency = aggregate.Ledgers[1].Currency
		convertedAmount = aggregate.Ledgers[1].Amount
		if aggregate.Ledgers[0].Metadata != nil {
			// Use reflection to safely iterate metadata map without assuming key/value types.
			metaVal := reflect.ValueOf(aggregate.Ledgers[0].Metadata)
			if metaVal.IsValid() && metaVal.Kind() == reflect.Map {
				for _, k := range metaVal.MapKeys() {
					v := metaVal.MapIndex(k)
					keyStr := fmt.Sprint(k.Interface())
					// Match common key names and extract values robustly
					switch keyStr {
					case "fx_rate", "fxRate", "rate":
						fxRate = fmt.Sprint(v.Interface())
					case "fx_rate_id", "fxRateId", "fx_rateid", "fxRateID":
						idStr := fmt.Sprint(v.Interface())
						if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
							fxRateID = id
						}
					}
				}
			}
		}
	}

	// Get fees
	if len(aggregate.Fees) > 0 {
		for _, fee := range aggregate.Fees {
			if fee.FeeType == domain.FeeTypePlatform || fee.FeeType == domain.FeeTypeConversion {
				feeAmount = fee.Amount
				break
			}
		}
	}

	return &accountingpb.ConversionResponse{
		JournalId:       aggregate.Journal.ID,
		ReceiptCode:     receiptCode,
		SourceCurrency:  sourceCurrency,
		DestCurrency:    destCurrency,
		SourceAmount:    sourceAmount,
		ConvertedAmount: convertedAmount,
		FxRate:          fxRate,
		FxRateId:        fxRateID,
		FeeAmount:       feeAmount,
		CreatedAt:       timestamppb.New(aggregate.Journal.CreatedAt),
	}, nil
}

// ProcessTradeWin credits account for trade win
func (h *AccountingHandler) ProcessTradeWin(
	ctx context.Context,
	req *accountingpb.TradeRequest,
) (*accountingpb.TradeResponse, error) {
	// Validate
	if req.AccountNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "account_number is required")
	}
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}
	if req.TradeId == "" {
		return nil, status.Error(codes.InvalidArgument, "trade_id is required")
	}

	// Convert to domain
	domainReq := &domain.TradeRequest{
		AccountNumber:       req.AccountNumber,
		Amount:              req.Amount,
		Currency:            req.Currency,
		AccountType:         convertAccountTypeToDomain(req.AccountType),
		TradeID:             req.TradeId,
		TradeType:           req.TradeType,
		IdempotencyKey:      req.IdempotencyKey,
		CreatedByExternalID: req.CreatedByExternalId,
		CreatedByType:       convertOwnerTypeToDomain(req.CreatedByType),
	}

	// Execute
	aggregate, err := h.txUC.ProcessTradeWin(ctx, domainReq)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	receiptCode := ""
	if aggregate.Journal.ExternalRef != nil {
		receiptCode = *aggregate.Journal.ExternalRef
	}

	// Get balance after
	var balanceAfter float64
	if len(aggregate.Ledgers) > 0 {
		for _, ledger := range aggregate.Ledgers {
			if ledger.DrCr == domain.DrCrCredit && ledger.BalanceAfter != nil {
				balanceAfter = *ledger.BalanceAfter
				break
			}
		}
	}

	return &accountingpb.TradeResponse{
		JournalId:    aggregate.Journal.ID,
		ReceiptCode:  receiptCode,
		TradeId:      req.TradeId,
		TradeResult:  "win",
		BalanceAfter: balanceAfter,
		CreatedAt:    timestamppb.New(aggregate.Journal.CreatedAt),
	}, nil
}

// ProcessTradeLoss debits account for trade loss
func (h *AccountingHandler) ProcessTradeLoss(
	ctx context.Context,
	req *accountingpb.TradeRequest,
) (*accountingpb.TradeResponse, error) {
	// Validate
	if req.AccountNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "account_number is required")
	}
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}
	if req.TradeId == "" {
		return nil, status.Error(codes.InvalidArgument, "trade_id is required")
	}

	// Convert to domain
	domainReq := &domain.TradeRequest{
		AccountNumber:       req.AccountNumber,
		Amount:              req.Amount,
		Currency:            req.Currency,
		AccountType:         convertAccountTypeToDomain(req.AccountType),
		TradeID:             req.TradeId,
		TradeType:           req.TradeType,
		IdempotencyKey:      req.IdempotencyKey,
		CreatedByExternalID: req.CreatedByExternalId,
		CreatedByType:       convertOwnerTypeToDomain(req.CreatedByType),
	}

	// Execute
	aggregate, err := h.txUC.ProcessTradeLoss(ctx, domainReq)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	receiptCode := ""
	if aggregate.Journal.ExternalRef != nil {
		receiptCode = *aggregate.Journal.ExternalRef
	}

	// Get balance after
	var balanceAfter float64
	if len(aggregate.Ledgers) > 0 {
		for _, ledger := range aggregate.Ledgers {
			if ledger.DrCr == domain.DrCrDebit && ledger.BalanceAfter != nil {
				balanceAfter = *ledger.BalanceAfter
				break
			}
		}
	}

	return &accountingpb.TradeResponse{
		JournalId:    aggregate.Journal.ID,
		ReceiptCode:  receiptCode,
		TradeId:      req.TradeId,
		TradeResult:  "loss",
		BalanceAfter: balanceAfter,
		CreatedAt:    timestamppb.New(aggregate.Journal.CreatedAt),
	}, nil
}

// ProcessAgentCommission pays commission to agent
func (h *AccountingHandler) ProcessAgentCommission(
	ctx context.Context,
	req *accountingpb.AgentCommissionRequest,
) (*accountingpb.AgentCommissionResponse, error) {
	// Validate
	if req.AgentExternalId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_external_id is required")
	}
	if req.TransactionRef == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_ref is required")
	}
	if req.CommissionAmount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "commission_amount must be positive")
	}

	// Convert to domain
	domainReq := &domain.AgentCommissionRequest{
		AgentExternalID:   req.AgentExternalId,
		TransactionRef:    req.TransactionRef,
		Currency:          req.Currency,
		TransactionAmount: req.TransactionAmount,
		CommissionAmount:  req.CommissionAmount,
		CommissionRate:    req.CommissionRate,
		IdempotencyKey:    req.IdempotencyKey,
	}

	// Execute
	aggregate, err := h.txUC.ProcessAgentCommission(ctx, domainReq)
	if err != nil {
		return nil, handleUsecaseError(err)
	}

	receiptCode := ""
	if aggregate.Journal.ExternalRef != nil {
		receiptCode = *aggregate.Journal.ExternalRef
	}

	return &accountingpb.AgentCommissionResponse{
		JournalId:        aggregate.Journal.ID,
		ReceiptCode:      receiptCode,
		AgentExternalId:  req.AgentExternalId,
		CommissionAmount: req.CommissionAmount,
		CreatedAt:        timestamppb.New(aggregate.Journal.CreatedAt),
	}, nil
}
