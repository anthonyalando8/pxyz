package hgrpc

import (
	"context"

	"time"

	//log "github.com/sirupsen/logrus"

	"accounting-service/internal/domain"
	"accounting-service/internal/usecase"
	accountingpb "x/shared/genproto/shared/accounting/v1"
	//xerrors "x/shared/utils/errors"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	//"google.golang.org/protobuf/types/known/timestamppb"
)

// handler/grpc_accounting. go

type AccountingHandler struct {
    accountingpb.UnimplementedAccountingServiceServer

    // Usecases
    accountUC   *usecase.AccountUsecase
    txUC        *usecase.TransactionUsecase
    statementUC *usecase.StatementUsecase
    journalUC   *usecase. JournalUsecase
    ledgerUC    *usecase. LedgerUsecase
    feeUC       *usecase. TransactionFeeUsecase
    feeRuleUC   *usecase.TransactionFeeRuleUsecase
    agentUC     usecase. AgentUsecase
    approvalUC  *usecase. TransactionApprovalUsecase  // ✅ NEW

    // Infrastructure
    redisClient *redis.Client
}

func NewAccountingHandler(
    accountUC *usecase. AccountUsecase,
    txUC *usecase.TransactionUsecase,
    statementUC *usecase.StatementUsecase,
    journalUC *usecase.JournalUsecase,
    ledgerUC *usecase.LedgerUsecase,
    feeUC *usecase. TransactionFeeUsecase,
    feeRuleUC *usecase.TransactionFeeRuleUsecase,
    agentUC usecase. AgentUsecase,
    approvalUC *usecase. TransactionApprovalUsecase,  // ✅ NEW
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
        agentUC:     agentUC,
        approvalUC:  approvalUC,  // ✅ NEW
        redisClient: redisClient,
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




