package hgrpc

import (
	"context"
	"errors"
	"fmt"


	log "github.com/sirupsen/logrus"

	"accounting-service/internal/domain"

	xerrors "x/shared/utils/errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)
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
	// ===============================
	// NOT FOUND ERRORS
	// ===============================
	case errors.Is(err, xerrors.ErrNotFound),
		errors.Is(err, xerrors.ErrAccountNotFound),
		errors.Is(err, xerrors.ErrTransactionNotFound),
		errors.Is(err, xerrors.ErrLedgerNotFound),
		errors.Is(err, xerrors. ErrJournalNotFound),
		errors.Is(err, xerrors.ErrReceiptNotFound),
		errors.Is(err, xerrors.ErrBalanceNotFound),
		errors.Is(err, xerrors.ErrFeeRuleNotFound),
		errors.Is(err, xerrors.ErrAgentNotFound),
		errors.Is(err, xerrors.ErrSystemAccountNotFound),
		errors.Is(err, xerrors.ErrStatementNotFound):
		logger.WithField("grpc_code", codes.NotFound).Warn("resource not found")
		return status.Error(codes.NotFound, err.Error())

	// ===============================
	// INSUFFICIENT BALANCE/FUNDS
	// ===============================
	case errors.Is(err, xerrors.ErrInsufficientBalance),
		errors.Is(err, xerrors.ErrInsufficientAvailable),
		errors.Is(err, xerrors.ErrInsufficientFunds),
		errors.Is(err, xerrors.ErrOverdraftLimitExceeded),
		errors.Is(err, xerrors.ErrSystemBalanceInsufficient):
		logger.WithField("grpc_code", codes. FailedPrecondition).Warn("insufficient balance for transaction")
		return status.Error(codes. FailedPrecondition, err.Error())

	// ===============================
	// PERMISSION DENIED (Account State)
	// ===============================
	case errors.Is(err, xerrors.ErrAccountLocked):
		logger.WithField("grpc_code", codes.PermissionDenied).Warn("attempted operation on locked account")
		return status.Error(codes.PermissionDenied, "account is locked")

	case errors.Is(err, xerrors.ErrAccountInactive):
		logger.WithField("grpc_code", codes.PermissionDenied).Warn("attempted operation on inactive account")
		return status.Error(codes.PermissionDenied, "account is inactive")

	case errors.Is(err, xerrors.ErrDemoAccountRestricted),
		errors.Is(err, xerrors.ErrDemoDepositNotAllowed),
		errors.Is(err, xerrors. ErrDemoWithdrawalNotAllowed),
		errors.Is(err, xerrors.ErrDemoTransferNotAllowed):
		logger.WithField("grpc_code", codes. PermissionDenied).Warn("operation not allowed for demo account")
		return status.Error(codes.PermissionDenied, err.Error())

	// ===============================
	// ALREADY EXISTS (Duplicates)
	// ===============================
	case errors.Is(err, xerrors.ErrDuplicateIdempotencyKey):
		logger.WithField("grpc_code", codes.AlreadyExists).Warn("duplicate idempotency key detected")
		return status.Error(codes.AlreadyExists, "duplicate idempotency key")

	case errors.Is(err, xerrors.ErrDuplicateAccount),
		errors.Is(err, xerrors.ErrDuplicateReceipt),
		errors.Is(err, xerrors.ErrTransactionAlreadyProcessed):
		logger.WithField("grpc_code", codes.AlreadyExists).Warn("duplicate resource detected")
		return status.Error(codes.AlreadyExists, err.Error())

	// ===============================
	// INVALID ARGUMENT (Validation)
	// ===============================
	case errors.Is(err, xerrors.ErrInvalidInput),
		errors.Is(err, xerrors.ErrInvalidRequest),
		errors.Is(err, xerrors.ErrInvalidAccountNumber),
		errors.Is(err, xerrors.ErrInvalidAccountType),
		errors.Is(err, xerrors.ErrInvalidAccountPurpose),
		errors.Is(err, xerrors.ErrInvalidOwnerType),
		errors.Is(err, xerrors.ErrInvalidTransaction),
		errors.Is(err, xerrors.ErrInvalidTransactionType),
		errors.Is(err, xerrors.ErrInvalidTransactionAmount),
		errors.Is(err, xerrors.ErrInvalidLedgerEntry),
		errors.Is(err, xerrors.ErrInvalidJournal),
		errors.Is(err, xerrors.ErrInvalidDrCr),
		errors.Is(err, xerrors.ErrInvalidCurrency),
		errors.Is(err, xerrors.ErrInvalidCurrencyFormat),
		errors.Is(err, xerrors.ErrInvalidAmount),
		errors.Is(err, xerrors.ErrInvalidFeeType),
		errors.Is(err, xerrors.ErrInvalidFeeAmount),
		errors.Is(err, xerrors.ErrInvalidReceiptCode),
		errors.Is(err, xerrors.ErrInvalidCommissionRate),
		errors.Is(err, xerrors.ErrInvalidLimit),
		errors.Is(err, xerrors.ErrInvalidOffset),
		errors.Is(err, xerrors.ErrInvalidDateRange),
		errors.Is(err, xerrors.ErrInvalidFilter),
		errors.Is(err, xerrors. ErrInvalidStatementPeriod),
		errors.Is(err, xerrors. ErrRequiredFieldMissing):
		logger.WithField("grpc_code", codes.InvalidArgument).Warn("invalid input provided")
		return status.Error(codes.InvalidArgument, err. Error())

	// ===============================
	// FAILED PRECONDITION (Business Logic)
	// ===============================
	case errors.Is(err, xerrors.ErrCurrencyNotSupported),
		errors.Is(err, xerrors. ErrCurrencyMismatch),
		errors.Is(err, xerrors.ErrMultipleCurrencies),
		errors.Is(err, xerrors.ErrLedgerNotBalanced),
		errors.Is(err, xerrors.ErrInsufficientEntries),
		errors.Is(err, xerrors.ErrNegativeBalance),
		errors.Is(err, xerrors.ErrTransactionPending),
		errors.Is(err, xerrors.ErrFeeRuleExpired),
		errors.Is(err, xerrors.ErrFeeRuleInactive),
		errors.Is(err, xerrors.ErrReceiptExpired),
		errors.Is(err, xerrors.ErrCommissionNotApplicable),
		errors.Is(err, xerrors.ErrInvalidSystemOperation):
		logger.WithField("grpc_code", codes. FailedPrecondition).Warn("business logic constraint violation")
		return status.Error(codes.FailedPrecondition, err.Error())

	// ===============================
	// ABORTED (Concurrency/Retry)
	// ===============================
	case errors.Is(err, xerrors.ErrConcurrentModification),
		errors.Is(err, xerrors.ErrOptimisticLockFailed),
		errors.Is(err, xerrors.ErrVersionMismatch),
		errors.Is(err, xerrors.ErrDeadlockDetected):
		logger.WithField("grpc_code", codes.Aborted).Warn("concurrent modification detected, client should retry")
		return status.Error(codes.Aborted, "concurrent modification, please retry")

	// ===============================
	// UNAVAILABLE (Service Failures)
	// ===============================
	case errors.Is(err, xerrors.ErrTransactionFailed),
		errors.Is(err, xerrors.ErrFeeCalculationFailed),
		errors.Is(err, xerrors.ErrStatementGenerationFailed):
		logger.WithField("grpc_code", codes.Unavailable).Error("service operation failed")
		return status.Error(codes.Unavailable, err.Error())

	// ===============================
	// CONTEXT ERRORS
	// ===============================
	case errors.Is(err, context. DeadlineExceeded):
		logger.WithField("grpc_code", codes.DeadlineExceeded).Error("request deadline exceeded")
		return status.Error(codes.DeadlineExceeded, "request timeout")

	case errors.Is(err, context.Canceled):
		logger.WithField("grpc_code", codes.Canceled).Info("request canceled by client")
		return status.Error(codes.Canceled, "request canceled")

	// ===============================
	// DEFAULT:  INTERNAL SERVER ERROR
	// ===============================
	default:
		// Log detailed error for debugging
		logger.WithFields(log.Fields{
			"grpc_code": codes.Internal,
			"error_detail": fmt.Sprintf("%+v", err),
		}).Error("unhandled error - internal server error")
		
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