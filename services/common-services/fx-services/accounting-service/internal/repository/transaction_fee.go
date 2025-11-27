package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"accounting-service/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	xerrors "x/shared/utils/errors"
)

type TransactionFeeRepository interface {
	// Basic CRUD
	Create(ctx context.Context, tx pgx.Tx, fee *domain.TransactionFee) error
	CreateBatch(ctx context.Context, tx pgx.Tx, fees []*domain.TransactionFee) map[int]error
	GetByID(ctx context.Context, id int64) (*domain.TransactionFee, error)
	
	// Query operations
	GetByReceipt(ctx context.Context, receiptCode string) ([]*domain.TransactionFee, error)
	GetByFeeRule(ctx context.Context, feeRuleID int64, limit int) ([]*domain.TransactionFee, error)
	GetByAgent(ctx context.Context, agentExternalID string, from, to time.Time) ([]*domain.TransactionFee, error)
	
	// Statistics
	GetTotalFeesByType(ctx context.Context, feeType domain.FeeType, from, to time.Time) (int64, error)
	GetAgentCommissionSummary(ctx context.Context, agentExternalID string, from, to time.Time) (map[string]int64, error) // Currency -> Amount
	
	// Transaction management
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type transactionFeeRepo struct {
	db *pgxpool.Pool
}

func NewTransactionFeeRepo(db *pgxpool.Pool) TransactionFeeRepository {
	return &transactionFeeRepo{db: db}
}

func (r *transactionFeeRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}

// ===============================
// BASIC CRUD
// ===============================

// Create inserts a new transaction fee
func (r *transactionFeeRepo) Create(ctx context.Context, tx pgx.Tx, fee *domain.TransactionFee) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	// Validate currency code length
	if len(fee.Currency) > 8 {
		return errors.New("currency code must be 8 characters or less")
	}

	// Validate agent commission
	if fee.FeeType == domain.FeeTypeAgentCommission && fee.AgentExternalID == nil {
		return errors.New("agent_external_id required for agent_commission fee type")
	}

	query := `
		INSERT INTO transaction_fees (
			receipt_code, fee_rule_id, fee_type, amount, currency,
			collected_by_account_id, ledger_id, agent_external_id, commission_rate,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`

	now := time.Now()
	err := tx.QueryRow(ctx, query,
		fee.ReceiptCode,
		fee.FeeRuleID,
		fee.FeeType,
		fee.Amount,
		fee.Currency,
		fee.CollectedByAccountID,
		fee.LedgerID,
		fee.AgentExternalID,
		fee.CommissionRate,
		now,
	).Scan(&fee.ID, &fee.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create transaction fee: %w", err)
	}

	return nil
}

// CreateBatch creates multiple transaction fees (bulk insert)
func (r *transactionFeeRepo) CreateBatch(ctx context.Context, tx pgx.Tx, fees []*domain.TransactionFee) map[int]error {
	if tx == nil {
		return map[int]error{0: errors.New("transaction cannot be nil")}
	}

	if len(fees) == 0 {
		return nil
	}

	errs := make(map[int]error)
	now := time.Now()

	batch := &pgx.Batch{}
	query := `
		INSERT INTO transaction_fees (
			receipt_code, fee_rule_id, fee_type, amount, currency,
			collected_by_account_id, ledger_id, agent_external_id, commission_rate,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`

	validFees := make([]*domain.TransactionFee, 0, len(fees))
	indexMap := make(map[int]int)

	for i, fee := range fees {
		// Validate currency code
		if len(fee.Currency) > 8 {
			errs[i] = errors.New("currency code must be 8 characters or less")
			continue
		}

		// Validate agent commission
		if fee.FeeType == domain.FeeTypeAgentCommission && fee.AgentExternalID == nil {
			errs[i] = errors.New("agent_external_id required for agent_commission fee type")
			continue
		}

		batch.Queue(query,
			fee.ReceiptCode,
			fee.FeeRuleID,
			fee.FeeType,
			fee.Amount,
			fee.Currency,
			fee.CollectedByAccountID,
			fee.LedgerID,
			fee.AgentExternalID,
			fee.CommissionRate,
			now,
		)

		indexMap[len(validFees)] = i
		validFees = append(validFees, fee)
	}

	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	for batchIdx := 0; batchIdx < len(validFees); batchIdx++ {
		originalIdx := indexMap[batchIdx]
		fee := validFees[batchIdx]

		err := br.QueryRow().Scan(&fee.ID, &fee.CreatedAt)
		if err != nil {
			errs[originalIdx] = fmt.Errorf("failed to create transaction fee: %w", err)
		}
	}

	return errs
}

// GetByID fetches a transaction fee by ID
func (r *transactionFeeRepo) GetByID(ctx context.Context, id int64) (*domain.TransactionFee, error) {
	query := `
		SELECT 
			id, receipt_code, fee_rule_id, fee_type, amount, currency,
			collected_by_account_id, ledger_id, agent_external_id, commission_rate,
			created_at
		FROM transaction_fees
		WHERE id = $1
	`

	var fee domain.TransactionFee
	err := r.db.QueryRow(ctx, query, id).Scan(
		&fee.ID,
		&fee.ReceiptCode,
		&fee.FeeRuleID,
		&fee.FeeType,
		&fee.Amount,
		&fee.Currency,
		&fee.CollectedByAccountID,
		&fee.LedgerID,
		&fee.AgentExternalID,
		&fee.CommissionRate,
		&fee.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get transaction fee: %w", err)
	}

	return &fee, nil
}

// ===============================
// QUERY OPERATIONS
// ===============================

// GetByReceipt fetches all fees for a specific receipt (uses idx_fees_receipt)
func (r *transactionFeeRepo) GetByReceipt(ctx context.Context, receiptCode string) ([]*domain.TransactionFee, error) {
	query := `
		SELECT 
			id, receipt_code, fee_rule_id, fee_type, amount, currency,
			collected_by_account_id, ledger_id, agent_external_id, commission_rate,
			created_at
		FROM transaction_fees
		WHERE receipt_code = $1
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(ctx, query, receiptCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get fees by receipt: %w", err)
	}
	defer rows.Close()

	var fees []*domain.TransactionFee
	for rows.Next() {
		var fee domain.TransactionFee
		err := rows.Scan(
			&fee.ID,
			&fee.ReceiptCode,
			&fee.FeeRuleID,
			&fee.FeeType,
			&fee.Amount,
			&fee.Currency,
			&fee.CollectedByAccountID,
			&fee.LedgerID,
			&fee.AgentExternalID,
			&fee.CommissionRate,
			&fee.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction fee: %w", err)
		}
		fees = append(fees, &fee)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating fee rows: %w", err)
	}

	return fees, nil
}

// GetByFeeRule fetches all fees that used a specific rule
func (r *transactionFeeRepo) GetByFeeRule(ctx context.Context, feeRuleID int64, limit int) ([]*domain.TransactionFee, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT 
			id, receipt_code, fee_rule_id, fee_type, amount, currency,
			collected_by_account_id, ledger_id, agent_external_id, commission_rate,
			created_at
		FROM transaction_fees
		WHERE fee_rule_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, feeRuleID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get fees by rule: %w", err)
	}
	defer rows.Close()

	var fees []*domain.TransactionFee
	for rows.Next() {
		var fee domain.TransactionFee
		err := rows.Scan(
			&fee.ID,
			&fee.ReceiptCode,
			&fee.FeeRuleID,
			&fee.FeeType,
			&fee.Amount,
			&fee.Currency,
			&fee.CollectedByAccountID,
			&fee.LedgerID,
			&fee.AgentExternalID,
			&fee.CommissionRate,
			&fee.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction fee: %w", err)
		}
		fees = append(fees, &fee)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating fee rows: %w", err)
	}

	return fees, nil
}

// GetByAgent fetches all fees for a specific agent in a date range (uses idx_fees_agent)
func (r *transactionFeeRepo) GetByAgent(ctx context.Context, agentExternalID string, from, to time.Time) ([]*domain.TransactionFee, error) {
	query := `
		SELECT 
			id, receipt_code, fee_rule_id, fee_type, amount, currency,
			collected_by_account_id, ledger_id, agent_external_id, commission_rate,
			created_at
		FROM transaction_fees
		WHERE agent_external_id = $1
		  AND created_at >= $2
		  AND created_at <= $3
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, agentExternalID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get fees by agent: %w", err)
	}
	defer rows.Close()

	var fees []*domain.TransactionFee
	for rows.Next() {
		var fee domain.TransactionFee
		err := rows.Scan(
			&fee.ID,
			&fee.ReceiptCode,
			&fee.FeeRuleID,
			&fee.FeeType,
			&fee.Amount,
			&fee.Currency,
			&fee.CollectedByAccountID,
			&fee.LedgerID,
			&fee.AgentExternalID,
			&fee.CommissionRate,
			&fee.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction fee: %w", err)
		}
		fees = append(fees, &fee)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating fee rows: %w", err)
	}

	return fees, nil
}

// ===============================
// STATISTICS
// ===============================

// GetTotalFeesByType returns total fees collected by fee type in a date range
func (r *transactionFeeRepo) GetTotalFeesByType(ctx context.Context, feeType domain.FeeType, from, to time.Time) (int64, error) {
	query := `
		SELECT COALESCE(SUM(amount), 0) AS total
		FROM transaction_fees
		WHERE fee_type = $1
		  AND created_at >= $2
		  AND created_at <= $3
	`

	var total int64
	err := r.db.QueryRow(ctx, query, feeType, from, to).Scan(&total)
	
	if err != nil {
		return 0, fmt.Errorf("failed to get total fees by type: %w", err)
	}

	return total, nil
}

// GetAgentCommissionSummary returns agent commission totals grouped by currency
func (r *transactionFeeRepo) GetAgentCommissionSummary(ctx context.Context, agentExternalID string, from, to time.Time) (map[string]int64, error) {
	query := `
		SELECT 
			currency,
			SUM(amount) AS total_commission
		FROM transaction_fees
		WHERE agent_external_id = $1
		  AND fee_type = 'agent_commission'
		  AND created_at >= $2
		  AND created_at <= $3
		GROUP BY currency
		ORDER BY total_commission DESC
	`

	rows, err := r.db.Query(ctx, query, agentExternalID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent commission summary: %w", err)
	}
	defer rows.Close()

	summary := make(map[string]int64)
	for rows.Next() {
		var currency string
		var total int64
		
		err := rows.Scan(&currency, &total)
		if err != nil {
			return nil, fmt.Errorf("failed to scan commission summary: %w", err)
		}

		summary[currency] = total
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating commission summary rows: %w", err)
	}

	return summary, nil
}