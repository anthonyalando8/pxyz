package hgrpc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"accounting-service/internal/domain"
	"accounting-service/internal/usecase"
	accountingpb "x/shared/genproto/shared/accounting/accountingpb"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type AccountingGRPCHandler struct {
	accountingpb.UnimplementedAccountingServiceServer
	accountUC   *usecase.AccountUsecase
	ledgerUC    *usecase.LedgerUsecase
	statementUC *usecase.StatementUsecase
	redisClient *redis.Client
}

// NewAccountingGRPCHandler initializes the handler with optional Redis client
func NewAccountingGRPCHandler(
	accountUC *usecase.AccountUsecase,
	ledgerUC *usecase.LedgerUsecase,
	statementUC *usecase.StatementUsecase,
	redisClient *redis.Client,
) *AccountingGRPCHandler {
	return &AccountingGRPCHandler{
		accountUC:   accountUC,
		ledgerUC:    ledgerUC,
		statementUC: statementUC,
		redisClient: redisClient,
	}
}


// ===============================
// Accounts
// ===============================
func (h *AccountingGRPCHandler) CreateAccounts(ctx context.Context, req *accountingpb.CreateAccountRequest) (*accountingpb.CreateAccountResponse, error) {
	// Start a transaction
	tx, err := h.accountUC.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Convert proto accounts to domain accounts
	domainAccounts := make([]*domain.Account, len(req.Accounts))
	for i, acc := range req.Accounts {
		domainAccounts[i] = &domain.Account{
			OwnerType:   strings.ToLower(acc.OwnerType.String()),
			OwnerID:     acc.OwnerId,
			Currency:    acc.Currency,
			Purpose:     acc.Purpose,
			AccountType: acc.AccountType,
			IsActive:    acc.IsActive,
		}
	}

	// Call usecase batch creation
	errMap := h.accountUC.CreateAccounts(ctx, domainAccounts, tx)

	// Prepare response
	resp := &accountingpb.CreateAccountResponse{
		Accounts: []*accountingpb.Account{},
		Errors:   map[int32]string{},
	}

	for i, a := range domainAccounts {
		if err, exists := errMap[i]; exists {
			resp.Errors[int32(i)] = err.Error()
		} else {
			resp.Accounts = append(resp.Accounts, &accountingpb.Account{
				Id:          a.ID,
				OwnerType:   req.Accounts[i].OwnerType,
				OwnerId:     a.OwnerID,
				Currency:    a.Currency,
				Purpose:     a.Purpose,
				AccountType: a.AccountType,
				IsActive:    a.IsActive,
			})
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return resp, nil
}


func (h *AccountingGRPCHandler) GetUserAccounts(ctx context.Context, req *accountingpb.GetAccountsRequest) (*accountingpb.GetAccountsResponse, error) {
	tx, err := h.accountUC.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	accounts, err := h.accountUC.GetByOwner(ctx, strings.ToLower(req.OwnerType.String()), req.OwnerId, tx)
	if err != nil {
		return nil, err
	}

	resp := &accountingpb.GetAccountsResponse{
		Accounts: []*accountingpb.Account{},
	}

	for _, a := range accounts {
		resp.Accounts = append(resp.Accounts, &accountingpb.Account{
			Id:          a.ID,
			AccountNumber: a.AccountNumber,
			OwnerType:   req.OwnerType,
			OwnerId:     a.OwnerID,
			Currency:    a.Currency,
			Purpose:     a.Purpose,
			AccountType: a.AccountType,
			IsActive:    a.IsActive,
		})
	}
	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return resp, nil
}

// ===============================
// Transactions
// ===============================
func (h *AccountingGRPCHandler) PostTransaction(
	ctx context.Context,
	req *accountingpb.CreateTransactionRequest,
) (*accountingpb.CreateTransactionResponse, error) {
	// Map proto → domain journal
	journal := &domain.Journal{
		ExternalRef:   req.ExternalRef,
		Description:   req.Description,
		CreatedBy:     req.CreatedByUser,
		CreatedByType: strings.ToLower(req.CreatedByType.String()),
		CreatedAt:     time.Now(),
	}

	// Convert proto entries to domain postings
	var postings []*domain.Posting
	var drAmount float64
	var drCurrency string

	for _, e := range req.Entries {
		posting := &domain.Posting{
			AccountID: e.AccountId,
			DrCr:      e.DrCr.String(),
			Amount:    e.Amount,
			Currency:  e.Currency,
			AccountData: &domain.Account{
				AccountNumber: e.AccountNumber,
			},
		}

		// Capture DR posting details for transactionAmount and currency
		if posting.DrCr == "DR" {
			drAmount = posting.Amount
			drCurrency = posting.Currency
		}

		postings = append(postings, posting)
	}

	if drAmount == 0 || drCurrency == "" {
		return nil, errors.New("no DR posting found to determine transaction amount and currency")
	}

	// Delegate transaction creation to the usecase
	ledger, err := h.ledgerUC.CreateTransactionMulti(
		ctx,
		req.TransactionType,
		drAmount,
		drCurrency,
		journal,
		postings,
		nil, // no existing DB transaction
	)
	if err != nil {
		return nil, err
	}

	return &accountingpb.CreateTransactionResponse{
		ExternalRef: ledger.Journal.ExternalRef,
	}, nil
}

// ===============================
// Statements
// ===============================
func (h *AccountingGRPCHandler) GetAccountStatement(
    ctx context.Context,
    req *accountingpb.AccountStatementRequest,
) (*accountingpb.AccountStatement, error) {

    stmt, err := h.statementUC.GetAccountStatement(ctx, req.AccountNumber, req.From.AsTime(), req.To.AsTime())
    if err != nil {
        return nil, err
    }

    pbs := make([]*accountingpb.Posting, 0, len(stmt.Postings))
    for _, p := range stmt.Postings {
        receiptCode := string("")
        if p.ReceiptCode != nil {
            receiptCode = *p.ReceiptCode
        }

        pbs = append(pbs, &accountingpb.Posting{
            Id:        p.ID,
            JournalId: p.JournalID,
            AccountId: p.AccountID,
            Amount:    p.Amount,
            DrCr:      accountingpb.DrCr(accountingpb.DrCr_value[p.DrCr]),
            Currency:  p.Currency,
            CreatedAt: timestamppb.New(p.CreatedAt),
            ReceiptCode: receiptCode,
        })
    }

    return &accountingpb.AccountStatement{
        AccountId: stmt.AccountID,
		AccountNumber: stmt.AccountNumber,
        Postings:  pbs,
        Balance:   stmt.Balance,
    }, nil
}



func (h *AccountingGRPCHandler) GetOwnerStatement(req *accountingpb.OwnerStatementRequest, stream accountingpb.AccountingService_GetOwnerStatementServer) error {
	ctx := stream.Context()
	tx, err := h.statementUC.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	stmts, err := h.statementUC.GetOwnerStatement(ctx, strings.ToLower(req.OwnerType.String()), req.OwnerId, req.From.AsTime(), req.To.AsTime(), h.accountUC, tx)
	if err != nil {
		return err
	}

	for _, stmt := range stmts {
		pbs := []*accountingpb.Posting{}
		for _, p := range stmt.Postings {
			pbs = append(pbs, &accountingpb.Posting{
				Id:        p.ID,
				JournalId: p.JournalID,
				AccountId: p.AccountID,
				Amount:    p.Amount,
				DrCr:      accountingpb.DrCr(accountingpb.DrCr_value[p.DrCr]),
				Currency:  p.Currency,
				ReceiptCode: func() string {
					if p.ReceiptCode != nil {
						return *p.ReceiptCode
					}
					return ""
				}(),
				CreatedAt: timestamppb.New(p.CreatedAt),
			})
		}

		if err := stream.Send(&accountingpb.AccountStatement{
			AccountId: stmt.AccountID,
			AccountNumber: stmt.AccountNumber,
			Postings:  pbs,
			Balance:   stmt.Balance,
		}); err != nil {
			return err
		}
	}
	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

// ===============================
// Journal
// ===============================
func (h *AccountingGRPCHandler) GetJournalPostings(req *accountingpb.JournalPostingsRequest, stream accountingpb.AccountingService_GetJournalPostingsServer) error {
	postings, err := h.statementUC.GetJournalPostings(stream.Context(), req.JournalId)
	if err != nil {
		return err
	}

	for _, p := range postings {
		if err := stream.Send(&accountingpb.Posting{
			Id:        p.ID,
			JournalId: p.JournalID,
			AccountId: p.AccountID,
			Amount:    p.Amount,
			DrCr:      accountingpb.DrCr(accountingpb.DrCr_value[p.DrCr]),
			Currency:  p.Currency,
			ReceiptCode: func() string {
				if p.ReceiptCode != nil {
					return *p.ReceiptCode
				}
				return ""
			}(),
			CreatedAt: timestamppb.New(p.CreatedAt),
		}); err != nil {
			return err
		}
	}
	return nil
}

// ===============================
// Reports
// ===============================
func (h *AccountingGRPCHandler) GenerateDailyReport(req *accountingpb.DailyReportRequest, stream accountingpb.AccountingService_GenerateDailyReportServer) error {
	reports, err := h.statementUC.GenerateDailyReport(stream.Context(), req.Date.AsTime())
	if err != nil {
		return err
	}

	for _, r := range reports {
		if err := stream.Send(&accountingpb.DailyReport{
			OwnerType:   accountingpb.OwnerType(accountingpb.OwnerType_value[r.OwnerType]),
			OwnerId:     r.OwnerID,
			AccountId:   r.AccountID,
			Currency:    r.Currency,
			TotalDebit:  r.TotalDebit,
			TotalCredit: r.TotalCredit,
			Balance:     r.Balance,
			NetChange:   r.NetChange,
			Date:        timestamppb.New(r.Date),
		}); err != nil {
			return err
		}
	}
	return nil
}
