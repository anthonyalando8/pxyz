package usecase

import (
	"context"
	"errors"
	"fmt"
	xerrors "x/shared/utils/errors"
	"x/shared/utils/id"

	"accounting-service/internal/domain"
	"accounting-service/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

var (
	ErrAgentNotFound     = errors.New("agent not found")
	ErrInvalidAgentData  = errors.New("invalid agent data")
	ErrInvalidPagination = errors.New("invalid pagination parameters")
)

type AgentUsecase interface {
	// Agent operations
	CreateAgent(ctx context.Context, req CreateAgentRequest) (*domain.Agent, error)
	UpdateAgent(ctx context.Context, agentExternalID string, req UpdateAgentRequest) (*domain.Agent, error)
	DeleteAgent(ctx context.Context, agentExternalID string) error
	GetAgentByID(ctx context.Context, agentExternalID string, includeAccounts bool) (*domain.Agent, error)
	GetAgentByUserID(ctx context.Context, userExternalID string, includeAccounts bool) (*domain.Agent, error)
	ListAgents(ctx context.Context, limit, offset int, includeAccounts bool) ([]*domain.Agent, error)
	
	// ✅ NEW:  Filtering methods
	ListAgentsByCountry(ctx context.Context, countryCode string, limit, offset int, includeAccounts bool) ([]*domain.Agent, error)
	ListAgentsByPaymentMethod(ctx context.Context, paymentMethod string, limit, offset int, includeAccounts bool) ([]*domain.Agent, error)
	ListAgentsWithFilters(ctx context.Context, filters AgentFilters, includeAccounts bool) ([]*domain.Agent, error)

	// Commission operations
	ListCommissionsForAgent(ctx context.Context, agentExternalID string, limit, offset int) ([]*domain.AgentCommission, error)
}

type agentUsecase struct {
	repo      repository.AgentRepository
	accountUC *AccountUsecase
	sf        *id.Snowflake
}

func NewAgentUsecase(repo repository.AgentRepository, accountUC *AccountUsecase, sf *id.Snowflake) AgentUsecase {
	return &agentUsecase{
		repo:      repo,
		accountUC: accountUC,
		sf:        sf,
	}
}

// Request/Response types

type CreateAgentRequest struct {
	UserExternalID           *string                `json:"user_external_id,omitempty"`
	Service                  *string                `json:"service,omitempty"`
	CommissionRate           *string                `json:"commission_rate,omitempty"` // decimal as string
	RelationshipType         string                 `json:"relationship_type"`
	IsActive                 bool                   `json:"is_active"`
	Name                     *string                `json:"name,omitempty"`
	Metadata                 map[string]interface{} `json:"metadata,omitempty"`
	
	// ✅ NEW FIELDS
	CommissionRateForDeposit *string         `json:"commission_rate_for_deposit,omitempty"`
	PaymentMethod            *string         `json:"payment_method,omitempty"`
	Location                 map[string]bool `json:"location,omitempty"` // {"KE": true, "UG": false}
	Status                   *string         `json:"status,omitempty"`   // "active", "inactive", "deleted"
}

type UpdateAgentRequest struct {
	UserExternalID           *string                `json:"user_external_id,omitempty"`
	Service                  *string                `json:"service,omitempty"`
	CommissionRate           *string                `json:"commission_rate,omitempty"` // decimal as string
	RelationshipType         *string                `json:"relationship_type,omitempty"`
	IsActive                 *bool                  `json:"is_active,omitempty"`
	Name                     *string                `json:"name,omitempty"`
	Metadata                 map[string]interface{} `json:"metadata,omitempty"`
	
	// ✅ NEW FIELDS
	CommissionRateForDeposit *string         `json:"commission_rate_for_deposit,omitempty"`
	PaymentMethod            *string         `json:"payment_method,omitempty"`
	Location                 map[string]bool `json:"location,omitempty"`
	Status                   *string         `json:"status,omitempty"`
}

// ✅ NEW:  Filter struct for listing agents
type AgentFilters struct {
	CountryCode      *string `json:"country_code,omitempty"`
	PaymentMethod    *string `json:"payment_method,omitempty"`
	Status           *string `json:"status,omitempty"`
	RelationshipType *string `json:"relationship_type,omitempty"`
	Limit            int     `json:"limit"`
	Offset           int     `json:"offset"`
}

// ----------------- Agent Operations -----------------

func (u *agentUsecase) CreateAgent(ctx context.Context, req CreateAgentRequest) (*domain.Agent, error) {
	// Validate request
	if err := validateCreateAgentRequest(req); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidAgentData, err)
	}

	if req.UserExternalID == nil || *req.UserExternalID == "" {
		return nil, fmt.Errorf("%w: user_external_id is required", ErrInvalidAgentData)
	}

	// Begin transaction
	tx, err := u.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Generate external ID
	agentExternalID := u.sf.Generate()

	// Parse commission rate
	var commissionRate *decimal.Decimal
	if req.CommissionRate != nil {
		rate, err := decimal.NewFromString(*req.CommissionRate)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid commission rate format", ErrInvalidAgentData)
		}
		commissionRate = &rate
	}

	// ✅ Parse commission rate for deposit
	var commissionRateForDeposit *decimal.Decimal
	if req.CommissionRateForDeposit != nil {
		rate, err := decimal.NewFromString(*req.CommissionRateForDeposit)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid commission rate for deposit format", ErrInvalidAgentData)
		}
		commissionRateForDeposit = &rate
	}

	// ✅ Set default status if not provided
	status := domain.AgentStatusActive
	if req.Status != nil {
		status = domain.AgentStatus(*req.Status)
	}

	agent := &domain.Agent{
		AgentExternalID:          agentExternalID,
		UserExternalID:           req.UserExternalID,
		Service:                  req.Service,
		CommissionRate:           commissionRate,
		RelationshipType:         req.RelationshipType,
		IsActive:                 req.IsActive,
		Name:                     req.Name,
		Metadata:                 req.Metadata,
		CommissionRateForDeposit: commissionRateForDeposit,
		PaymentMethod:            req.PaymentMethod,
		Location:                 req.Location,
		Status:                   status,
	}

	// Create agent within transaction
	if err := u.repo.CreateAgent(ctx, tx, agent); err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Convert user accounts to agent accounts
	if err := u.convertUserAccountsToAgentAccounts(ctx, *req.UserExternalID, agentExternalID, commissionRate, tx); err != nil {
		return nil, fmt.Errorf("failed to convert accounts: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Fetch the created agent with accounts
	createdAgent, err := u.GetAgentByID(ctx, agentExternalID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created agent: %w", err)
	}

	return createdAgent, nil
}

func (u *agentUsecase) convertUserAccountsToAgentAccounts(
	ctx context.Context,
	userExternalID string,
	agentExternalID string,
	commissionRate *decimal.Decimal,
	tx pgx.Tx,
) error {
	// Fetch user's existing accounts
	userAccounts, err := u.accountUC.GetByOwner(ctx, domain.OwnerTypeUser, userExternalID, domain.AccountTypeReal)
	if err != nil {
		if errors.Is(err, xerrors.ErrNotFound) {
			return nil // No accounts to convert
		}
		return fmt.Errorf("failed to fetch user accounts: %w", err)
	}

	if len(userAccounts) == 0 {
		return nil
	}

	// Only convert wallet accounts to commission accounts
	accountsToUpdate := make([]*domain.Account, 0)

	for _, account := range userAccounts {
		// Only convert wallet accounts
		account.ParentAgentExternalID = nullableStr(agentExternalID)
		if account.Purpose == domain.PurposeWallet {
			account.OwnerType = domain.OwnerTypeUser

			if commissionRate != nil {
				rateStr := commissionRate.String()
				account.CommissionRate = &rateStr
			}

			accountsToUpdate = append(accountsToUpdate, account)
		}
	}

	// Update accounts if any need conversion
	if len(accountsToUpdate) > 0 {
		updateErrs := u.accountUC.UpdateAccounts(ctx, accountsToUpdate, tx)

		if len(updateErrs) > 0 {
			// Return first error encountered
			for _, err := range updateErrs {
				return fmt.Errorf("failed to convert account: %w", err)
			}
		}
	}

	return nil
}

func (u *agentUsecase) UpdateAgent(ctx context.Context, agentExternalID string, req UpdateAgentRequest) (*domain.Agent, error) {
	// Validate agent exists
	existingAgent, err := u.repo.GetAgentByAgentID(ctx, agentExternalID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agent: %w", err)
	}
	if existingAgent == nil {
		return nil, ErrAgentNotFound
	}

	// Begin transaction
	tx, err := u.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Apply updates
	if req.UserExternalID != nil {
		existingAgent.UserExternalID = req.UserExternalID
	}
	if req.Service != nil {
		existingAgent.Service = req.Service
	}
	if req.CommissionRate != nil {
		rate, err := decimal.NewFromString(*req.CommissionRate)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid commission rate format", ErrInvalidAgentData)
		}
		existingAgent.CommissionRate = &rate
	}
	// ✅ NEW: Update commission rate for deposit
	if req.CommissionRateForDeposit != nil {
		rate, err := decimal.NewFromString(*req.CommissionRateForDeposit)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid commission rate for deposit format", ErrInvalidAgentData)
		}
		existingAgent.CommissionRateForDeposit = &rate
	}
	if req.RelationshipType != nil {
		existingAgent.RelationshipType = *req.RelationshipType
	}
	if req.IsActive != nil {
		existingAgent.IsActive = *req.IsActive
	}
	if req.Name != nil {
		existingAgent.Name = req.Name
	}
	if req.Metadata != nil {
		existingAgent.Metadata = req.Metadata
	}
	// ✅ NEW:  Update payment method
	if req.PaymentMethod != nil {
		existingAgent.PaymentMethod = req.PaymentMethod
	}
	// ✅ NEW:  Update location
	if req.Location != nil {
		existingAgent.Location = req.Location
	}
	// ✅ NEW: Update status
	if req.Status != nil {
		existingAgent.Status = domain.AgentStatus(*req.Status)
	}

	// Update agent within transaction
	if err := u.repo.UpdateAgent(ctx, tx, existingAgent); err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Fetch updated agent
	updatedAgent, err := u.repo.GetAgentByAgentID(ctx, agentExternalID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated agent: %w", err)
	}

	return updatedAgent, nil
}

// ✅ DeleteAgent - Now performs soft delete (sets status to 'deleted')
func (u *agentUsecase) DeleteAgent(ctx context.Context, agentExternalID string) error {
	// Validate agent exists
	existingAgent, err := u.repo.GetAgentByAgentID(ctx, agentExternalID)
	if err != nil {
		return fmt.Errorf("failed to fetch agent: %w", err)
	}
	if existingAgent == nil {
		return ErrAgentNotFound
	}

	// Soft delete (sets status to 'deleted')
	if err := u.repo.DeleteAgent(ctx, agentExternalID); err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	return nil
}

func (u *agentUsecase) GetAgentByID(ctx context.Context, agentExternalID string, includeAccounts bool) (*domain.Agent, error) {
	agent, err := u.repo.GetAgentByAgentID(ctx, agentExternalID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agent: %w", err)
	}
	if agent == nil {
		return nil, ErrAgentNotFound
	}

	// Fetch accounts if requested
	if includeAccounts && agent.UserExternalID != nil {
		accounts, err := u.accountUC.GetByOwner(ctx, domain.OwnerTypeUser, *agent.UserExternalID, domain.AccountTypeReal)
		if err == nil {
			agent.Accounts = accounts
		}
	}

	return agent, nil
}

func (u *agentUsecase) GetAgentByUserID(ctx context.Context, userExternalID string, includeAccounts bool) (*domain.Agent, error) {
	agent, err := u.repo.GetAgentByUserID(ctx, userExternalID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agent by user ID: %w", err)
	}
	if agent == nil {
		return nil, ErrAgentNotFound
	}

	// Fetch accounts if requested
	if includeAccounts && agent.UserExternalID != nil {
		accounts, err := u.accountUC.GetByOwner(ctx, domain.OwnerTypeUser, *agent.UserExternalID, domain.AccountTypeReal)
		if err == nil {
			agent.Accounts = accounts
		}
	}

	return agent, nil
}

func (u *agentUsecase) ListAgents(ctx context.Context, limit, offset int, includeAccounts bool) ([]*domain.Agent, error) {
	// Validate pagination
	if limit <= 0 || limit > 100 {
		return nil, fmt.Errorf("%w: limit must be between 1 and 100", ErrInvalidPagination)
	}
	if offset < 0 {
		return nil, fmt.Errorf("%w: offset must be non-negative", ErrInvalidPagination)
	}

	agents, err := u.repo.ListAgents(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	// Fetch accounts for each agent if requested
	if includeAccounts {
		u.populateAgentAccounts(ctx, agents)
	}

	return agents, nil
}

// ✅ NEW: ListAgentsByCountry
func (u *agentUsecase) ListAgentsByCountry(ctx context.Context, countryCode string, limit, offset int, includeAccounts bool) ([]*domain.Agent, error) {
	// Validate pagination
	if limit <= 0 || limit > 100 {
		return nil, fmt.Errorf("%w: limit must be between 1 and 100", ErrInvalidPagination)
	}
	if offset < 0 {
		return nil, fmt.Errorf("%w: offset must be non-negative", ErrInvalidPagination)
	}

	if countryCode == "" {
		return nil, fmt.Errorf("%w: country code is required", ErrInvalidAgentData)
	}

	agents, err := u.repo.ListAgentsByCountry(ctx, countryCode, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents by country: %w", err)
	}

	// Fetch accounts for each agent if requested
	if includeAccounts {
		u.populateAgentAccounts(ctx, agents)
	}

	return agents, nil
}

// ✅ NEW: ListAgentsByPaymentMethod
func (u *agentUsecase) ListAgentsByPaymentMethod(ctx context.Context, paymentMethod string, limit, offset int, includeAccounts bool) ([]*domain.Agent, error) {
	// Validate pagination
	if limit <= 0 || limit > 100 {
		return nil, fmt.Errorf("%w: limit must be between 1 and 100", ErrInvalidPagination)
	}
	if offset < 0 {
		return nil, fmt.Errorf("%w: offset must be non-negative", ErrInvalidPagination)
	}

	if paymentMethod == "" {
		return nil, fmt.Errorf("%w: payment method is required", ErrInvalidAgentData)
	}

	agents, err := u.repo.ListAgentsByPaymentMethod(ctx, paymentMethod, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents by payment method: %w", err)
	}

	// Fetch accounts for each agent if requested
	if includeAccounts {
		u.populateAgentAccounts(ctx, agents)
	}

	return agents, nil
}

// ✅ NEW:  ListAgentsWithFilters - dynamic filtering
func (u *agentUsecase) ListAgentsWithFilters(ctx context.Context, filters AgentFilters, includeAccounts bool) ([]*domain.Agent, error) {
	// Validate pagination
	if filters.Limit <= 0 || filters.Limit > 100 {
		return nil, fmt.Errorf("%w: limit must be between 1 and 100", ErrInvalidPagination)
	}
	if filters.Offset < 0 {
		return nil, fmt.Errorf("%w: offset must be non-negative", ErrInvalidPagination)
	}

	// Convert to repository filters
	repoFilters := repository.AgentFilters{
		CountryCode:       filters.CountryCode,
		PaymentMethod:    filters.PaymentMethod,
		RelationshipType: filters.RelationshipType,
		Limit:            filters.Limit,
		Offset:           filters.Offset,
	}

	// Convert status string to domain type
	if filters.Status != nil {
		status := domain.AgentStatus(*filters.Status)
		repoFilters.Status = &status
	}

	agents, err := u.repo.ListAgentsWithFilters(ctx, repoFilters)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents with filters: %w", err)
	}

	// Fetch accounts for each agent if requested
	if includeAccounts {
		u.populateAgentAccounts(ctx, agents)
	}

	return agents, nil
}

// ✅ Helper function to populate accounts for multiple agents
func (u *agentUsecase) populateAgentAccounts(ctx context.Context, agents []*domain.Agent) {
	for _, agent := range agents {
		if agent.UserExternalID != nil {
			accounts, err := u.accountUC.GetByOwner(ctx, domain.OwnerTypeAgent, *agent.UserExternalID, domain.AccountTypeReal)
			if err == nil {
				agent.Accounts = accounts
			}
			// Continue even if one agent's accounts fail
		}
	}
}

// ----------------- Commission Operations -----------------

func (u *agentUsecase) ListCommissionsForAgent(ctx context.Context, agentExternalID string, limit, offset int) ([]*domain.AgentCommission, error) {
	// Validate agent exists
	agent, err := u.repo.GetAgentByAgentID(ctx, agentExternalID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agent: %w", err)
	}
	if agent == nil {
		return nil, ErrAgentNotFound
	}

	// Validate pagination
	if limit <= 0 || limit > 100 {
		return nil, fmt.Errorf("%w: limit must be between 1 and 100", ErrInvalidPagination)
	}
	if offset < 0 {
		return nil, fmt.Errorf("%w: offset must be non-negative", ErrInvalidPagination)
	}

	commissions, err := u.repo.ListCommissionsForAgent(ctx, agentExternalID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list commissions:  %w", err)
	}

	return commissions, nil
}

// ----------------- Validation Helpers -----------------

func validateCreateAgentRequest(req CreateAgentRequest) error {
	if req.RelationshipType == "" {
		return errors.New("relationship_type is required")
	}

	// Validate commission rate if provided
	if req.CommissionRate != nil {
		rate, err := decimal.NewFromString(*req.CommissionRate)
		if err != nil {
			return errors.New("invalid commission rate format")
		}
		if rate.IsNegative() || rate.GreaterThan(decimal.NewFromInt(100)) {
			return errors.New("commission rate must be between 0 and 100")
		}
	}

	// ✅ Validate commission rate for deposit if provided
	if req.CommissionRateForDeposit != nil {
		rate, err := decimal.NewFromString(*req.CommissionRateForDeposit)
		if err != nil {
			return errors.New("invalid commission rate for deposit format")
		}
		if rate.IsNegative() || rate.GreaterThan(decimal.NewFromInt(100)) {
			return errors.New("commission rate for deposit must be between 0 and 100")
		}
	}

	// ✅ Validate status if provided
	if req.Status != nil {
		status := domain.AgentStatus(*req.Status)
		if status != domain.AgentStatusActive && status != domain.AgentStatusInactive && status != domain.AgentStatusDeleted {
			return errors.New("invalid status:  must be 'active', 'inactive', or 'deleted'")
		}
	}

	return nil
}

