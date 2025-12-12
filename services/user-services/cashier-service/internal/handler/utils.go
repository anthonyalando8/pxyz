package handler

import (
    accountingpb "x/shared/genproto/shared/accounting/v1"
)
// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func mapTransactionType(s string) accountingpb.TransactionType {
	switch s {
	case "deposit":
		return accountingpb.TransactionType_TRANSACTION_TYPE_DEPOSIT
	case "withdrawal":
		return accountingpb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL
	case "transfer":
		return accountingpb.TransactionType_TRANSACTION_TYPE_TRANSFER
	case "conversion":
		return accountingpb.TransactionType_TRANSACTION_TYPE_CONVERSION
	default:
		return accountingpb.TransactionType_TRANSACTION_TYPE_UNSPECIFIED
	}
}
