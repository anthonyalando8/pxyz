package hgrpc

import (
	"fmt"
	"time"

	"accounting-service/internal/domain"
	accountingpb "x/shared/genproto/shared/accounting/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// ===============================
// ENUM CONVERSIONS
// ===============================

func convertOwnerTypeToDomain(t accountingpb.OwnerType) domain.OwnerType {
	switch t {
	case accountingpb.OwnerType_OWNER_TYPE_USER:
		return domain.OwnerTypeUser
	case accountingpb.OwnerType_OWNER_TYPE_PARTNER:
		return domain.OwnerTypePartner
	case accountingpb.OwnerType_OWNER_TYPE_SYSTEM:
		return domain.OwnerTypeSystem
	case accountingpb.OwnerType_OWNER_TYPE_ADMIN:
		return domain.OwnerTypeAdmin
	case accountingpb.OwnerType_OWNER_TYPE_AGENT:
		return domain.OwnerTypeAgent
	default:
		return domain.OwnerTypeUser
	}
}

func convertOwnerTypeToProto(t domain.OwnerType) accountingpb.OwnerType {
	switch t {
	case domain.OwnerTypeUser:
		return accountingpb.OwnerType_OWNER_TYPE_USER
	case domain.OwnerTypePartner:
		return accountingpb.OwnerType_OWNER_TYPE_PARTNER
	case domain.OwnerTypeSystem:
		return accountingpb.OwnerType_OWNER_TYPE_SYSTEM
	case domain.OwnerTypeAdmin:
		return accountingpb.OwnerType_OWNER_TYPE_ADMIN
	case domain.OwnerTypeAgent:
		return accountingpb.OwnerType_OWNER_TYPE_AGENT
	default:
		return accountingpb.OwnerType_OWNER_TYPE_UNSPECIFIED
	}
}

func convertAccountTypeToDomain(t accountingpb.AccountType) domain.AccountType {
	switch t {
	case accountingpb.AccountType_ACCOUNT_TYPE_REAL:
		return domain.AccountTypeReal
	case accountingpb.AccountType_ACCOUNT_TYPE_DEMO:
		return domain.AccountTypeDemo
	default:
		return domain.AccountTypeReal
	}
}

func convertAccountTypeToProto(t domain.AccountType) accountingpb.AccountType {
	switch t {
	case domain.AccountTypeReal:
		return accountingpb.AccountType_ACCOUNT_TYPE_REAL
	case domain.AccountTypeDemo:
		return accountingpb.AccountType_ACCOUNT_TYPE_DEMO
	default:
		return accountingpb.AccountType_ACCOUNT_TYPE_UNSPECIFIED
	}
}

func convertAccountPurposeToDomain(p accountingpb.AccountPurpose) domain.AccountPurpose {
	switch p {
	case accountingpb.AccountPurpose_ACCOUNT_PURPOSE_WALLET:
		return domain.PurposeWallet
	case accountingpb.AccountPurpose_ACCOUNT_PURPOSE_LIQUIDITY:
		return domain.PurposeLiquidity
	case accountingpb.AccountPurpose_ACCOUNT_PURPOSE_CLEARING:
		return domain.PurposeClearing
	case accountingpb.AccountPurpose_ACCOUNT_PURPOSE_FEES:
		return domain.PurposeFees
	case accountingpb.AccountPurpose_ACCOUNT_PURPOSE_ESCROW:
		return domain.PurposeEscrow
	case accountingpb.AccountPurpose_ACCOUNT_PURPOSE_SETTLEMENT:
		return domain.PurposeSettlement
	case accountingpb.AccountPurpose_ACCOUNT_PURPOSE_REVENUE:
		return domain.PurposeRevenue
	case accountingpb.AccountPurpose_ACCOUNT_PURPOSE_CONTRA:
		return domain.PurposeContra
	case accountingpb.AccountPurpose_ACCOUNT_PURPOSE_COMMISSION:
		return domain.PurposeCommission
	default:
		return domain.PurposeWallet
	}
}

func convertAccountPurposeToProto(p domain.AccountPurpose) accountingpb.AccountPurpose {
	switch p {
	case domain.PurposeWallet:
		return accountingpb.AccountPurpose_ACCOUNT_PURPOSE_WALLET
	case domain.PurposeLiquidity:
		return accountingpb.AccountPurpose_ACCOUNT_PURPOSE_LIQUIDITY
	case domain.PurposeClearing:
		return accountingpb.AccountPurpose_ACCOUNT_PURPOSE_CLEARING
	case domain.PurposeFees:
		return accountingpb.AccountPurpose_ACCOUNT_PURPOSE_FEES
	case domain.PurposeEscrow:
		return accountingpb.AccountPurpose_ACCOUNT_PURPOSE_ESCROW
	case domain.PurposeSettlement:
		return accountingpb.AccountPurpose_ACCOUNT_PURPOSE_SETTLEMENT
	case domain.PurposeRevenue:
		return accountingpb.AccountPurpose_ACCOUNT_PURPOSE_REVENUE
	case domain.PurposeContra:
		return accountingpb.AccountPurpose_ACCOUNT_PURPOSE_CONTRA
	case domain.PurposeCommission:
		return accountingpb.AccountPurpose_ACCOUNT_PURPOSE_COMMISSION
	default:
		return accountingpb.AccountPurpose_ACCOUNT_PURPOSE_UNSPECIFIED
	}
}

func convertDrCrToDomain(d accountingpb.DrCr) domain.DrCr {
	switch d {
	case accountingpb.DrCr_DR_CR_DEBIT:
		return domain.DrCrDebit
	case accountingpb.DrCr_DR_CR_CREDIT:
		return domain.DrCrCredit
	default:
		return domain.DrCrDebit
	}
}

func convertDrCrToProto(d domain.DrCr) accountingpb.DrCr {
	switch d {
	case domain.DrCrDebit:
		return accountingpb.DrCr_DR_CR_DEBIT
	case domain.DrCrCredit:
		return accountingpb.DrCr_DR_CR_CREDIT
	default:
		return accountingpb.DrCr_DR_CR_UNSPECIFIED
	}
}

func convertTransactionTypeToDomain(t accountingpb.TransactionType) domain.TransactionType {
	switch t {
	case accountingpb.TransactionType_TRANSACTION_TYPE_DEPOSIT:
		return domain.TransactionTypeDeposit
	case accountingpb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL:
		return domain.TransactionTypeWithdrawal
	case accountingpb.TransactionType_TRANSACTION_TYPE_TRANSFER:
		return domain.TransactionTypeTransfer
	case accountingpb.TransactionType_TRANSACTION_TYPE_CONVERSION:
		return domain.TransactionTypeConversion
	case accountingpb.TransactionType_TRANSACTION_TYPE_FEE:
		return domain.TransactionTypeFee
	case accountingpb.TransactionType_TRANSACTION_TYPE_COMMISSION:
		return domain.TransactionTypeCommission
	case accountingpb.TransactionType_TRANSACTION_TYPE_TRADE:
		return domain.TransactionTypeTrade
	case accountingpb.TransactionType_TRANSACTION_TYPE_ADJUSTMENT:
		return domain.TransactionTypeAdjustment
	case accountingpb.TransactionType_TRANSACTION_TYPE_REFUND:
		return domain.TransactionTypeRefund
	case accountingpb.TransactionType_TRANSACTION_TYPE_REVERSAL:
		return domain.TransactionTypeReversal
	default:
		return domain.TransactionTypeTransfer
	}
}

func convertTransactionTypeToProto(t domain.TransactionType) accountingpb.TransactionType {
	switch t {
	case domain.TransactionTypeDeposit:
		return accountingpb.TransactionType_TRANSACTION_TYPE_DEPOSIT
	case domain.TransactionTypeWithdrawal:
		return accountingpb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL
	case domain.TransactionTypeTransfer:
		return accountingpb.TransactionType_TRANSACTION_TYPE_TRANSFER
	case domain.TransactionTypeConversion:
		return accountingpb.TransactionType_TRANSACTION_TYPE_CONVERSION
	case domain.TransactionTypeFee:
		return accountingpb.TransactionType_TRANSACTION_TYPE_FEE
	case domain.TransactionTypeCommission:
		return accountingpb.TransactionType_TRANSACTION_TYPE_COMMISSION
	case domain.TransactionTypeTrade:
		return accountingpb.TransactionType_TRANSACTION_TYPE_TRADE
	case domain.TransactionTypeAdjustment:
		return accountingpb.TransactionType_TRANSACTION_TYPE_ADJUSTMENT
	case domain.TransactionTypeRefund:
		return accountingpb.TransactionType_TRANSACTION_TYPE_REFUND
	case domain.TransactionTypeReversal:
		return accountingpb.TransactionType_TRANSACTION_TYPE_REVERSAL
	default:
		return accountingpb.TransactionType_TRANSACTION_TYPE_UNSPECIFIED
	}
}

func convertTransactionStatusToProto(s string) accountingpb.TransactionStatus {
	switch s {
	case "processing":
		return accountingpb.TransactionStatus_TRANSACTION_STATUS_PROCESSING
	case "executing":
		return accountingpb.TransactionStatus_TRANSACTION_STATUS_EXECUTING
	case "completed":
		return accountingpb.TransactionStatus_TRANSACTION_STATUS_COMPLETED
	case "failed":
		return accountingpb.TransactionStatus_TRANSACTION_STATUS_FAILED
	default:
		return accountingpb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
	}
}

func convertFeeTypeToDomain(t accountingpb.FeeType) domain.FeeType {
	switch t {
	case accountingpb.FeeType_FEE_TYPE_PLATFORM:
		return domain.FeeTypePlatform
	case accountingpb.FeeType_FEE_TYPE_NETWORK:
		return domain.FeeTypeNetwork
	case accountingpb.FeeType_FEE_TYPE_CONVERSION:
		return domain.FeeTypeConversion
	case accountingpb.FeeType_FEE_TYPE_WITHDRAWAL:
		return domain.FeeTypeWithdrawal
	case accountingpb.FeeType_FEE_TYPE_AGENT_COMMISSION:
		return domain.FeeTypeAgentCommission
	default:
		return domain.FeeTypePlatform
	}
}

func convertFeeTypeToProto(t domain.FeeType) accountingpb.FeeType {
	switch t {
	case domain.FeeTypePlatform:
		return accountingpb.FeeType_FEE_TYPE_PLATFORM
	case domain.FeeTypeNetwork:
		return accountingpb.FeeType_FEE_TYPE_NETWORK
	case domain.FeeTypeConversion:
		return accountingpb.FeeType_FEE_TYPE_CONVERSION
	case domain.FeeTypeWithdrawal:
		return accountingpb.FeeType_FEE_TYPE_WITHDRAWAL
	case domain.FeeTypeAgentCommission:
		return accountingpb.FeeType_FEE_TYPE_AGENT_COMMISSION
	default:
		return accountingpb.FeeType_FEE_TYPE_UNSPECIFIED
	}
}

// ===============================
// OPTIONAL CONVERSIONS
// ===============================

func convertOptionalTransactionTypeToDomain(t *accountingpb.TransactionType) *domain.TransactionType {
	if t == nil || *t == accountingpb.TransactionType_TRANSACTION_TYPE_UNSPECIFIED {
		return nil
	}
	result := convertTransactionTypeToDomain(*t)
	return &result
}

func convertOptionalAccountTypeToDomain(t *accountingpb.AccountType) *domain.AccountType {
	if t == nil || *t == accountingpb.AccountType_ACCOUNT_TYPE_UNSPECIFIED {
		return nil
	}
	result := convertAccountTypeToDomain(*t)
	return &result
}

func convertOptionalOwnerTypeToDomain(t *accountingpb.OwnerType) *domain.OwnerType {
	if t == nil || *t == accountingpb.OwnerType_OWNER_TYPE_UNSPECIFIED {
		return nil
	}
	result := convertOwnerTypeToDomain(*t)
	return &result
}

func convertOptionalTimestamp(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}

// ===============================
// ACCOUNT CONVERSIONS
// ===============================

func convertAccountToProto(a *domain.Account) *accountingpb.Account {
	if a == nil {
		return nil
	}

	return &accountingpb.Account{
		Id:             a.ID,
		AccountNumber:  a.AccountNumber,
		OwnerType:      convertOwnerTypeToProto(a.OwnerType),
		OwnerId:        a.OwnerID,
		Currency:       a.Currency,
		Purpose:        convertAccountPurposeToProto(a.Purpose),
		AccountType:    convertAccountTypeToProto(a.AccountType),
		IsActive:       a.IsActive,
		IsLocked:       a.IsLocked,
		OverdraftLimit: a.OverdraftLimit,
		ParentAgentId:  getInt64OrZero(ptrStringToInt64(a.ParentAgentExternalID)),
		CommissionRate: getStringOrEmpty(a.CommissionRate),
		CreatedAt:      timestamppb.New(a.CreatedAt),
		UpdatedAt:      timestamppb.New(a.UpdatedAt),
	}
}

func convertAccountsToProto(accounts []*domain.Account) []*accountingpb.Account {
	result := make([]*accountingpb.Account, len(accounts))
	for i, a := range accounts {
		result[i] = convertAccountToProto(a)
	}
	return result
}

func convertCreateAccountRequestToDomain(req *accountingpb.CreateAccountRequest) *domain.CreateAccountRequest {
	if req == nil {
		return nil
	}

	var parentAgentID *string
	if req.ParentAgentId != 0 {
		str := int64ToString(req.ParentAgentId)
		parentAgentID = &str
	}

	var commissionRate *string
	if req.CommissionRate != "" {
		commissionRate = &req.CommissionRate
	}

	return &domain.CreateAccountRequest{
		OwnerType:             convertOwnerTypeToDomain(req.OwnerType),
		OwnerID:               req.OwnerId,
		Currency:              req.Currency,
		Purpose:               convertAccountPurposeToDomain(req.Purpose),
		AccountType:           convertAccountTypeToDomain(req.AccountType),
		ParentAgentExternalID: parentAgentID,
		CommissionRate:        commissionRate,
		OverdraftLimit:        req.OverdraftLimit,
		InitialBalance:        0,
	}
}

// ===============================
// BALANCE CONVERSIONS
// ===============================

func convertBalanceToProto(b *domain.Balance, account *domain.Account) *accountingpb.Balance {
	if b == nil {
		return nil
	}

	accountNumber := ""
	currency := ""
	if account != nil {
		accountNumber = account.AccountNumber
		currency = account.Currency
	}

	lastTransactionAt := b.UpdatedAt
	if lastTransactionAt.IsZero() {
		lastTransactionAt = time.Now()
	}

	return &accountingpb.Balance{
		AccountId:         b.AccountID,
		AccountNumber:     accountNumber,
		Balance:           b.Balance,
		AvailableBalance:  b.AvailableBalance,
		PendingDebit:      b.PendingDebit,
		PendingCredit:     b.PendingCredit,
		Currency:          currency,
		Version:           b.Version,
		LastTransactionAt: timestamppb.New(lastTransactionAt),
	}
}

// ===============================
// TRANSACTION CONVERSIONS
// ===============================

func convertLedgerEntryToDomain(entry *accountingpb.LedgerEntry) *domain.LedgerEntryRequest {
	if entry == nil {
		return nil
	}

	var description *string
	if entry.Description != nil {
		description = entry.Description
	}

	return &domain.LedgerEntryRequest{
		AccountNumber: entry.AccountNumber,
		Amount:        entry.Amount,
		DrCr:          convertDrCrToDomain(entry.DrCr),
		Currency:      entry.Currency,
		Description:   description,
	}
}

func convertExecuteTransactionRequestToDomain(req *accountingpb.ExecuteTransactionRequest) *domain.TransactionRequest {
	if req == nil {
		return nil
	}

	entries := make([]*domain.LedgerEntryRequest, len(req.Entries))
	for i, e := range req.Entries {
		entries[i] = convertLedgerEntryToDomain(e)
	}

	ownerType := convertOwnerTypeToDomain(req.CreatedByType)

	return &domain.TransactionRequest{
		IdempotencyKey:      req.IdempotencyKey,
		TransactionType:     convertTransactionTypeToDomain(req.TransactionType),
		AccountType:         convertAccountTypeToDomain(req.AccountType),
		ExternalRef:         req.ExternalRef,
		Description:         req.Description,
		CreatedByExternalID: &req.CreatedByExternalId,
		CreatedByType:       &ownerType,
		IPAddress:           req.IpAddress,
		UserAgent:           req.UserAgent,
		Entries:             entries,
		GenerateReceipt:     req.GenerateReceipt,
	}
}

func convertTransactionResultToProto(r *domain.TransactionResult) *accountingpb.ExecuteTransactionResponse {
	if r == nil {
		return nil
	}

	return &accountingpb.ExecuteTransactionResponse{
		ReceiptCode:      r.ReceiptCode,
		TransactionId:    r.TransactionID,
		Status:           convertTransactionStatusToProto(r.Status),
		Amount:           r.Amount,
		Currency:         r.Currency,
		Fee:              r.Fee,
		ProcessingTimeMs: r.ProcessingTime.Milliseconds(),
		CreatedAt:        timestamppb.New(r.CreatedAt),
	}
}

// ===============================
// JOURNAL CONVERSIONS
// ===============================

func convertJournalToProto(j *domain.Journal) *accountingpb.Journal {
	if j == nil {
		return nil
	}

	return &accountingpb.Journal{
		Id:                  j.ID,
		IdempotencyKey:      getStringOrEmpty(j.IdempotencyKey),
		TransactionType:     convertTransactionTypeToProto(j.TransactionType),
		AccountType:         convertAccountTypeToProto(j.AccountType),
		ExternalRef:         getStringOrEmpty(j.ExternalRef),
		Description:         getStringOrEmpty(j.Description),
		CreatedByExternalId: getStringOrEmpty(j.CreatedByExternalID),
		CreatedByType:       convertOptionalOwnerTypeToProto(j.CreatedByType),
		IpAddress:           getStringOrEmpty(j.IPAddress),
		UserAgent:           getStringOrEmpty(j.UserAgent),
		CreatedAt:           timestamppb.New(j.CreatedAt),
	}
}

func convertJournalsToProto(journals []*domain.Journal) []*accountingpb.Journal {
	result := make([]*accountingpb.Journal, len(journals))
	for i, j := range journals {
		result[i] = convertJournalToProto(j)
	}
	return result
}

// ===============================
// LEDGER CONVERSIONS
// ===============================

func convertLedgerToProto(l *domain.Ledger) *accountingpb.Ledger {
	if l == nil {
		return nil
	}

	accountNumber := ""
	if l.AccountData != nil {
		accountNumber = l.AccountData.AccountNumber
	}

	balanceAfter := float64(0)
	if l.BalanceAfter != nil {
		balanceAfter = *l.BalanceAfter
	}

	return &accountingpb.Ledger{
		Id:            l.ID,
		JournalId:     l.JournalID,
		AccountId:     l.AccountID,
		AccountNumber: accountNumber,
		Amount:        l.Amount,
		DrCr:          convertDrCrToProto(l.DrCr),
		Currency:      l.Currency,
		BalanceAfter:  balanceAfter,
		Description:   l.Description,
		ReceiptCode:   l.ReceiptCode,
		CreatedAt:     timestamppb.New(l.CreatedAt),
	}
}

func convertLedgersToProto(ledgers []*domain.Ledger) []*accountingpb.Ledger {
	result := make([]*accountingpb.Ledger, len(ledgers))
	for i, l := range ledgers {
		result[i] = convertLedgerToProto(l)
	}
	return result
}

// ===============================
// STATEMENT CONVERSIONS
// ===============================

func convertAccountStatementToProto(s *domain.AccountStatement) *accountingpb.AccountStatement {
	if s == nil {
		return nil
	}

	return &accountingpb.AccountStatement{
		AccountNumber:  s.AccountNumber,
		AccountType:    convertAccountTypeToProto(s.AccountType),
		Ledgers:        convertLedgersToProto(s.Ledgers),
		OpeningBalance: s.OpeningBalance,
		ClosingBalance: s.ClosingBalance,
		TotalDebits:    s.TotalDebits,
		TotalCredits:   s.TotalCredits,
		PeriodStart:    timestamppb.New(s.PeriodStart),
		PeriodEnd:      timestamppb.New(s.PeriodEnd),
	}
}

func convertAccountStatementsToProto(statements []*domain.AccountStatement) []*accountingpb.AccountStatement {
	result := make([]*accountingpb.AccountStatement, len(statements))
	for i, s := range statements {
		result[i] = convertAccountStatementToProto(s)
	}
	return result
}

func convertOwnerSummaryToProto(s *domain.OwnerSummary) *accountingpb.OwnerSummary {
	if s == nil {
		return nil
	}

	// Convert Balances to AccountBalance proto messages
	accountBalances := make([]*accountingpb.AccountBalance, len(s.Balances))
	for i, balance := range s.Balances {
		accountBalances[i] = &accountingpb.AccountBalance{
			AccountId:        balance.AccountID,
			AccountNumber:    balance.AccountNumber,
			Currency:         balance.Currency,
			Balance:          balance.Balance,
			AvailableBalance: balance.AvailableBalance,
		}
	}

	return &accountingpb.OwnerSummary{
		OwnerType:                 convertOwnerTypeToProto(s.OwnerType),
		OwnerId:                   s.OwnerID,
		AccountType:               convertAccountTypeToProto(s.AccountType),
		AccountBalances:           accountBalances,
		TotalBalanceUsdEquivalent: s.TotalBalance,
	}
}

// ===============================
// REPORT CONVERSIONS
// ===============================

func convertDailyReportToProto(r *domain.DailyReport) *accountingpb.DailyReport {
	if r == nil {
		return nil
	}

	return &accountingpb.DailyReport{
		OwnerType:   convertOwnerTypeToProto(r.OwnerType),
		OwnerId:     r.OwnerID,
		AccountId:   r.AccountID,
		Currency:    r.Currency,
		TotalDebit:  r.TotalDebit,
		TotalCredit: r.TotalCredit,
		Balance:     r.Balance,
		NetChange:   r.NetChange,
		Date:        timestamppb.New(r.Date),
	}
}

func convertDailyReportsToProto(reports []*domain.DailyReport) []*accountingpb.DailyReport {
	result := make([]*accountingpb.DailyReport, len(reports))
	for i, r := range reports {
		result[i] = convertDailyReportToProto(r)
	}
	return result
}

func convertTransactionSummaryToProto(s *domain.TransactionSummary) *accountingpb.TransactionSummary {
	if s == nil {
		return nil
	}

	return &accountingpb.TransactionSummary{
		TransactionType: convertTransactionTypeToProto(s.TransactionType),
		Currency:        s.Currency,
		Count:           s.Count,
		TotalAmount:     s.TotalVolume,
		MinAmount:       s.MinAmount,
		MaxAmount:       s.MaxAmount,
		AvgAmount:       s.AverageAmount,
	}
}

func convertTransactionSummariesToProto(summaries []*domain.TransactionSummary) []*accountingpb.TransactionSummary {
	result := make([]*accountingpb.TransactionSummary, len(summaries))
	for i, s := range summaries {
		result[i] = convertTransactionSummaryToProto(s)
	}
	return result
}

// ===============================
// FEE CONVERSIONS
// ===============================

func convertTransactionFeeToProto(f *domain.TransactionFee) *accountingpb.TransactionFee {
	if f == nil {
		return nil
	}

	return &accountingpb.TransactionFee{
		Id:                   f.ID,
		ReceiptCode:          f.ReceiptCode,
		FeeRuleId:            getInt64OrZero(f.FeeRuleID),
		FeeType:              convertFeeTypeToProto(f.FeeType),
		Amount:               f.Amount,
		Currency:             f.Currency,
		CollectedByAccountId: f.CollectedByAccountID,
		LedgerId:             f.LedgerID,
		AgentExternalId:      f.AgentExternalID,
		CommissionRate:       f.CommissionRate,
		CreatedAt:            timestamppb.New(f.CreatedAt),
	}
}

func convertFeesToProto(fees []*domain.TransactionFee) []*accountingpb.TransactionFee {
	result := make([]*accountingpb.TransactionFee, len(fees))
	for i, f := range fees {
		result[i] = convertTransactionFeeToProto(f)
	}
	return result
}

func convertFeeCalculationToProto(c *domain.FeeCalculation) *accountingpb.FeeCalculation {
	if c == nil {
		return nil
	}

	return &accountingpb.FeeCalculation{
		FeeType:        convertFeeTypeToProto(c.FeeType),
		Amount:         c.Amount,
		Currency:       c.Currency,
		AppliedRate:    c.AppliedRate,
		RuleId:         c.RuleID,
		CalculatedFrom: c.CalculatedFrom,
	}
}

// ===============================
// HELPER FUNCTIONS
// ===============================

func convertOptionalOwnerTypeToProto(t *domain.OwnerType) accountingpb.OwnerType {
	if t == nil {
		return accountingpb.OwnerType_OWNER_TYPE_UNSPECIFIED
	}
	return convertOwnerTypeToProto(*t)
}

func getInt64OrZero(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func getStringOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func ptrInt64(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ptrStringToInt64 converts string pointer to int64 pointer (for agent IDs)
func ptrStringToInt64(s *string) *int64 {
	if s == nil || *s == "" {
		return nil
	}
	// In real implementation, parse string to int64
	// For now, returning nil
	return nil
}

// int64ToString converts int64 to string (for agent IDs)
func int64ToString(v int64) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("%d", v)
}
