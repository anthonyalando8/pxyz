package hgrpc

import (
	"context"


	"accounting-service/internal/domain"
	accountingpb "x/shared/genproto/shared/accounting/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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