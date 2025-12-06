package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"accounting-service/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type AgentRepository interface {
	// Transaction management
	BeginTx(ctx context.Context) (pgx. Tx, error)

	// Agents
	CreateAgent(ctx context.Context, tx pgx.Tx, a *domain.Agent) error
	UpdateAgent(ctx context.Context, tx pgx.Tx, a *domain.Agent) error
	DeleteAgent(ctx context.Context, agentExternalID string) error
	GetAgentByAgentID(ctx context.Context, agentExternalID string) (*domain. Agent, error)
	GetAgentByUserID(ctx context. Context, userExternalID string) (*domain.Agent, error)
	ListAgents(ctx context. Context, limit, offset int) ([]*domain.Agent, error)

	// Commissions
	CreateCommission(ctx context.Context, tx pgx.Tx, c *domain.AgentCommission) (int64, error)
	ListCommissionsForAgent(ctx context.Context, agentExternalID string, limit, offset int) ([]*domain.AgentCommission, error)
	MarkCommissionPaid(ctx context.Context, tx pgx.Tx, commissionID int64, payoutReceipt string) error
}

type pgRepo struct {
	pool *pgxpool.Pool
}

func NewAgentRepository(pool *pgxpool.Pool) AgentRepository {
	return &pgRepo{pool: pool}
}

// ============================================================================
// TRANSACTION MANAGEMENT
// ============================================================================

func (p *pgRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return p.pool.Begin(ctx)
}

// ============================================================================
// AGENTS
// ============================================================================

// CreateAgent creates a new agent within a transaction
func (p *pgRepo) CreateAgent(ctx context. Context, tx pgx.Tx, a *domain.Agent) error {
	if tx == nil {
		return errors. New("transaction cannot be nil")
	}

	metaBytes := []byte("null")
	if a.Metadata != nil {
		b, _ := json.Marshal(a. Metadata)
		metaBytes = b
	}

	var commRate interface{}
	if a.CommissionRate != nil {
		commRate = a. CommissionRate.String()
	} else {
		commRate = nil
	}

	query := `
		INSERT INTO agents
		  (agent_external_id, user_external_id, service, commission_rate, relationship_type, is_active, name, metadata, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now(),now())
		ON CONFLICT (agent_external_id) DO UPDATE
		  SET user_external_id = EXCLUDED.user_external_id,
		      service = EXCLUDED. service,
		      commission_rate = EXCLUDED.commission_rate,
		      relationship_type = EXCLUDED.relationship_type,
		      is_active = EXCLUDED. is_active,
		      name = EXCLUDED.name,
		      metadata = EXCLUDED. metadata,
		      updated_at = now()
		RETURNING created_at, updated_at
	`

	// Execute within transaction and get timestamps
	err := tx.QueryRow(ctx, query,
		a.AgentExternalID,
		a.UserExternalID,
		a.Service,
		commRate,
		a. RelationshipType,
		a.IsActive,
		a. Name,
		string(metaBytes),
	).Scan(&a.CreatedAt, &a.UpdatedAt)

	return err
}

// UpdateAgent updates an existing agent within a transaction
func (p *pgRepo) UpdateAgent(ctx context.Context, tx pgx.Tx, a *domain.Agent) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	metaBytes := []byte("null")
	if a.Metadata != nil {
		b, _ := json. Marshal(a.Metadata)
		metaBytes = b
	}

	var commRate interface{}
	if a.CommissionRate != nil {
		commRate = a.CommissionRate.String()
	} else {
		commRate = nil
	}

	query := `
		UPDATE agents SET
		  user_external_id = $2,
		  service = $3,
		  commission_rate = $4,
		  relationship_type = $5,
		  is_active = $6,
		  name = $7,
		  metadata = $8::jsonb,
		  updated_at = now()
		WHERE agent_external_id = $1
		RETURNING updated_at
	`

	err := tx.QueryRow(ctx, query,
		a.AgentExternalID,
		a. UserExternalID,
		a.Service,
		commRate,
		a.RelationshipType,
		a.IsActive,
		a.Name,
		string(metaBytes),
	).Scan(&a.UpdatedAt)

	return err
}

// DeleteAgent deletes an agent (soft delete recommended in production)
func (p *pgRepo) DeleteAgent(ctx context. Context, agentExternalID string) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM agents WHERE agent_external_id = $1`, agentExternalID)
	return err
}

// GetAgentByAgentID fetches an agent by agent external ID
func (p *pgRepo) GetAgentByAgentID(ctx context.Context, agentExternalID string) (*domain. Agent, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT agent_external_id, user_external_id, service, commission_rate, relationship_type, is_active, name, metadata, created_at, updated_at
		FROM agents WHERE agent_external_id = $1
	`, agentExternalID)

	return scanAgent(row)
}

// GetAgentByUserID fetches an agent by user external ID
func (p *pgRepo) GetAgentByUserID(ctx context.Context, userExternalID string) (*domain.Agent, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT agent_external_id, user_external_id, service, commission_rate, relationship_type, is_active, name, metadata, created_at, updated_at
		FROM agents WHERE user_external_id = $1
	`, userExternalID)

	return scanAgent(row)
}

// ListAgents lists all agents with pagination
func (p *pgRepo) ListAgents(ctx context. Context, limit, offset int) ([]*domain.Agent, error) {
	rows, err := p. pool.Query(ctx, `
		SELECT agent_external_id, user_external_id, service, commission_rate, relationship_type, is_active, name, metadata, created_at, updated_at
		FROM agents
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*domain.Agent
	for rows.Next() {
		agent, err := scanAgentFromRows(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}

	return agents, rows.Err()
}

// ============================================================================
// COMMISSIONS
// ============================================================================

// CreateCommission creates a new commission within a transaction
func (p *pgRepo) CreateCommission(ctx context.Context, tx pgx. Tx, c *domain.AgentCommission) (int64, error) {
	if c == nil {
		return 0, errors.New("nil commission")
	}
	if tx == nil {
		return 0, errors.New("transaction cannot be nil")
	}

	txAmt := c.TransactionAmount. String()
	commAmt := c.CommissionAmount. String()
	commRate := c.CommissionRate. String()

	var id int64
	err := tx. QueryRow(ctx, `
		INSERT INTO agent_commissions
		  (agent_external_id, user_external_id, agent_account_id, user_account_id, receipt_code,
		   transaction_amount, commission_rate, commission_amount, currency, paid_out, created_at)
		VALUES ($1,$2,$3,$4,$5,$6::numeric,$7::numeric,$8::numeric,$9,$10,now())
		RETURNING id
	`, c.AgentExternalID, c.UserExternalID, c.AgentAccountID, c.UserAccountID, c. ReceiptCode, txAmt, commRate, commAmt, c.Currency, c.PaidOut). Scan(&id)

	if err != nil {
		// handle unique violation: return existing id for same agent+receipt if present
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			row := tx.QueryRow(ctx, `
				SELECT id FROM agent_commissions
				WHERE agent_external_id = $1 AND receipt_code = $2
				ORDER BY created_at DESC LIMIT 1
			`, c. AgentExternalID, c. ReceiptCode)
			if scanErr := row.Scan(&id); scanErr == nil {
				return id, nil
			}
		}
		return 0, err
	}
	return id, nil
}

// ListCommissionsForAgent lists commissions for a specific agent
func (p *pgRepo) ListCommissionsForAgent(ctx context.Context, agentExternalID string, limit, offset int) ([]*domain.AgentCommission, error) {
	rows, err := p.pool. Query(ctx, `
		SELECT id, agent_external_id, user_external_id, agent_account_id, user_account_id, receipt_code,
		       transaction_amount::text, commission_rate::text, commission_amount::text, currency, paid_out, payout_receipt_code, paid_out_at, created_at
		FROM agent_commissions
		WHERE agent_external_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, agentExternalID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commissions []*domain.AgentCommission
	for rows.Next() {
		var ac domain.AgentCommission
		var txAmt string
		var commAmt string
		var commRate string
		var payoutReceipt *string
		var paidOutAt *time.Time

		if err := rows.Scan(&ac.ID, &ac.AgentExternalID, &ac.UserExternalID, &ac.AgentAccountID, &ac.UserAccountID, &ac.ReceiptCode,
			&txAmt, &commRate, &commAmt, &ac.Currency, &ac. PaidOut, &payoutReceipt, &paidOutAt, &ac.CreatedAt); err != nil {
			return nil, err
		}

		ac. TransactionAmount, _ = decimal.NewFromString(txAmt)
		ac.CommissionAmount, _ = decimal.NewFromString(commAmt)
		ac.CommissionRate, _ = decimal.NewFromString(commRate)

		if payoutReceipt != nil {
			ac.PayoutReceiptCode = *payoutReceipt
		}
		ac.PaidOutAt = paidOutAt

		commissions = append(commissions, &ac)
	}

	return commissions, rows. Err()
}

// MarkCommissionPaid marks a commission as paid within a transaction
func (p *pgRepo) MarkCommissionPaid(ctx context.Context, tx pgx.Tx, commissionID int64, payoutReceipt string) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	_, err := tx.Exec(ctx, `
		UPDATE agent_commissions
		SET paid_out = true, payout_receipt_code = $2, paid_out_at = now()
		WHERE id = $1
	`, commissionID, payoutReceipt)
	return err
}

// ============================================================================
// HELPER SCAN FUNCTIONS
// ============================================================================

func scanAgent(row pgx. Row) (*domain.Agent, error) {
	var a domain.Agent
	var userID *string
	var service *string
	var commRate *string
	var name *string
	var metadata []byte

	if err := row.Scan(&a.AgentExternalID, &userID, &service, &commRate, &a.RelationshipType, &a.IsActive, &name, &metadata, &a. CreatedAt, &a.UpdatedAt); err != nil {
		if errors.Is(err, pgx. ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	a.UserExternalID = userID
	a.Service = service
	a.Name = name

	if commRate != nil {
		if d, err := decimal.NewFromString(*commRate); err == nil {
			a.CommissionRate = &d
		}
	}

	if len(metadata) > 0 && string(metadata) != "null" {
		var m map[string]interface{}
		_ = json.Unmarshal(metadata, &m)
		a.Metadata = m
	}

	return &a, nil
}

func scanAgentFromRows(rows pgx. Rows) (*domain.Agent, error) {
	var a domain.Agent
	var userID *string
	var service *string
	var commRate *string
	var name *string
	var metadata []byte

	if err := rows.Scan(&a.AgentExternalID, &userID, &service, &commRate, &a.RelationshipType, &a.IsActive, &name, &metadata, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, err
	}

	a.UserExternalID = userID
	a.Service = service
	a.Name = name

	if commRate != nil {
		if d, err := decimal. NewFromString(*commRate); err == nil {
			a.CommissionRate = &d
		}
	}

	if len(metadata) > 0 && string(metadata) != "null" {
		var m map[string]interface{}
		_ = json. Unmarshal(metadata, &m)
		a.Metadata = m
	}

	return &a, nil
}