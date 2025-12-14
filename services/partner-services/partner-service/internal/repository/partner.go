package repository

import (
	"context"
	"errors"
	"fmt"
	"partner-service/internal/domain"
	//"strconv"
	"x/shared/utils/id"

	"github.com/jackc/pgx/v5"
)

// CreatePartner inserts a new partner with required fields validation
func (r *PartnerRepo) CreatePartner(ctx context.Context, partner *domain.Partner) error {
	// ✅ Validate required fields
	if err := validatePartnerRequired(partner); err != nil {
		return err
	}

	// Generate unique partner ID if not set
	if partner.ID == "" {
		partner.ID = id.GenerateID("PTN")
	}
	
	// Set defaults
	if partner.Status == "" {
		partner.Status = domain. PartnerStatusActive
	}
	if partner.APIRateLimit == 0 {
		partner.APIRateLimit = 1000
	}

	query := `
		INSERT INTO partners (
			id, name, country, contact_email, contact_phone, status, service, currency,
			local_currency, rate, commission_rate,
			api_key, api_secret_hash, is_api_enabled, api_rate_limit,
			webhook_url, webhook_secret, callback_url, allowed_ips, metadata,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	return r.db.QueryRow(ctx, query,
		partner.ID,
		partner.Name,
		partner. Country,
		partner.ContactEmail,
		partner.ContactPhone,
		partner.Status,
		partner.Service,
		partner. Currency,
		partner.LocalCurrency,    // ✅ Required
		partner.Rate,             // ✅ Required
		partner. CommissionRate,
		partner.APIKey,
		partner.APISecretHash,
		partner.IsAPIEnabled,
		partner.APIRateLimit,
		partner.WebhookURL,
		partner.WebhookSecret,
		partner.CallbackURL,
		partner.AllowedIPs,
		partner. Metadata,
	).Scan(&partner.CreatedAt, &partner.UpdatedAt)
}

// validatePartnerRequired validates required fields for partner creation
func validatePartnerRequired(partner *domain.Partner) error {
	if partner.Name == "" {
		return errors.New("partner name is required")
	}
	if partner.LocalCurrency == "" {
		return errors.New("local_currency is required")
	}
	
	if partner. Rate <= 0 {
		return errors.New("rate must be greater than 0")
	}
	if partner.Currency == "" {
		return errors. New("currency is required")
	}
	if partner.Service == "" {
		return errors.New("service is required")
	}
	
	// Validate currency format (3-letter code)
	if len(partner.LocalCurrency) != 3 {
		return errors.New("local_currency must be a 3-letter code (e.g., KES, USD)")
	}
	if len(partner.Currency) < 3 || len(partner.Currency) > 8 {
		return errors.New("currency must be 3-8 characters")
	}
	// Validate rate precision
	if partner. Rate > 999999999.99999999 {
		return errors.New("rate exceeds maximum allowed value")
	}
	
	return nil
}

// GetPartnerByID fetches a partner by id with all fields
func (r *PartnerRepo) GetPartnerByID(ctx context.Context, id string) (*domain.Partner, error) {
	query := `
		SELECT 
			id, name, country, contact_email, contact_phone, status, service, currency,
			local_currency, rate, inverse_rate, commission_rate,
			api_key, api_secret_hash, webhook_url, webhook_secret, callback_url,
			is_api_enabled, api_rate_limit, allowed_ips, metadata,
			created_at, updated_at
		FROM partners
		WHERE id = $1
	`
	
	var p domain.Partner
	err := r.db. QueryRow(ctx, query, id).Scan(
		&p.ID,
		&p. Name,
		&p.Country,
		&p.ContactEmail,
		&p.ContactPhone,
		&p.Status,
		&p.Service,
		&p.Currency,
		&p.LocalCurrency,     // ✅ Added
		&p.Rate,              // ✅ Added
		&p.InverseRate,       // ✅ Added (computed column)
		&p.CommissionRate,    // ✅ Added
		&p.APIKey,
		&p.APISecretHash,
		&p.WebhookURL,
		&p. WebhookSecret,
		&p.CallbackURL,
		&p.IsAPIEnabled,
		&p.APIRateLimit,
		&p.AllowedIPs,
		&p.Metadata,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("partner not found: %s", id)
		}
		return nil, err
	}
	return &p, nil
}

// UpdatePartner updates partner info including rate and currency
func (r *PartnerRepo) UpdatePartner(ctx context.Context, partner *domain.Partner) error {
	// ✅ Validate before update
	if partner.LocalCurrency != "" && len(partner.LocalCurrency) != 3 {
		return errors.New("local_currency must be a 3-letter code")
	}
	
	if partner. Rate < 0 {
		return errors.New("rate cannot be negative")
	}

	query := `
		UPDATE partners
		SET 
			name = $1, 
			country = $2, 
			contact_email = $3, 
			contact_phone = $4, 
			status = $5, 
			service = $6, 
			currency = $7,
			local_currency = $8,
			rate = $9,
			commission_rate = $10,
			updated_at = NOW()
		WHERE id = $11
		RETURNING updated_at
	`
	
	return r.db.QueryRow(ctx, query,
		partner.Name,
		partner.Country,
		partner.ContactEmail,
		partner.ContactPhone,
		partner.Status,
		partner.Service,
		partner.Currency,
		partner.LocalCurrency,  // ✅ Added
		partner.Rate,           // ✅ Added
		partner.CommissionRate, // ✅ Added
		partner.ID,
	).Scan(&partner.UpdatedAt)
}

// GetAllPartners fetches all partners with complete info
func (r *PartnerRepo) GetAllPartners(ctx context.Context) ([]*domain.Partner, error) {
	query := `
		SELECT 
			id, name, country, contact_email, contact_phone, status, service, currency,
			local_currency, rate, inverse_rate, commission_rate,
			api_key, is_api_enabled, api_rate_limit,
			created_at, updated_at
		FROM partners
		ORDER BY name ASC
	`
	
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partners []*domain.Partner
	for rows.Next() {
		var p domain.Partner
		err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Country,
			&p.ContactEmail,
			&p.ContactPhone,
			&p.Status,
			&p. Service,
			&p.Currency,
			&p.LocalCurrency,   // ✅ Added
			&p.Rate,            // ✅ Added
			&p.InverseRate,     // ✅ Added
			&p.CommissionRate,  // ✅ Added
			&p.APIKey,
			&p.IsAPIEnabled,
			&p.APIRateLimit,
			&p.CreatedAt,
			&p. UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		partners = append(partners, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return partners, nil
}

// GetPartnersByIDs fetches partners by a list of IDs
func (r *PartnerRepo) GetPartnersByIDs(ctx context.Context, partnerIDs []string) ([]*domain.Partner, error) {
	var (
		rows pgx.Rows
		err  error
	)

	if len(partnerIDs) == 0 {
		query := `
			SELECT 
				id, name, country, contact_email, contact_phone, status, service, currency,
				local_currency, rate, inverse_rate, commission_rate,
				api_key, is_api_enabled, api_rate_limit,
				created_at, updated_at
			FROM partners
			ORDER BY name ASC
		`
		rows, err = r.db. Query(ctx, query)
		if err != nil {
			return nil, err
		}
	} else {
		query := `
			SELECT 
				id, name, country, contact_email, contact_phone, status, service, currency,
				local_currency, rate, inverse_rate, commission_rate,
				api_key, is_api_enabled, api_rate_limit,
				created_at, updated_at
			FROM partners
			WHERE id = ANY($1)
			ORDER BY name ASC
		`
		rows, err = r.db.Query(ctx, query, partnerIDs)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	var partners []*domain.Partner
	for rows.Next() {
		var p domain.Partner
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p. Country,
			&p.ContactEmail,
			&p.ContactPhone,
			&p.Status,
			&p.Service,
			&p.Currency,
			&p.LocalCurrency,   // ✅ Added
			&p.Rate,            // ✅ Added
			&p.InverseRate,     // ✅ Added
			&p.CommissionRate,  // ✅ Added
			&p.APIKey,
			&p.IsAPIEnabled,
			&p.APIRateLimit,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		partners = append(partners, &p)
	}

	if err := rows. Err(); err != nil {
		return nil, err
	}

	return partners, nil
}

// GetPartnersByService fetches all partners providing a specific service
func (r *PartnerRepo) GetPartnersByService(ctx context.Context, service string) ([]*domain.Partner, error) {
	query := `
		SELECT 
			id, name, country, contact_email, contact_phone, status, service, currency,
			local_currency, rate, inverse_rate, commission_rate,
			api_key, is_api_enabled, api_rate_limit,
			created_at, updated_at
		FROM partners
		WHERE service = $1 AND status = 'active'
		ORDER BY name ASC
	`
	
	rows, err := r.db.Query(ctx, query, service)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partners []*domain.Partner
	for rows.Next() {
		var p domain.Partner
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Country,
			&p. ContactEmail,
			&p. ContactPhone,
			&p. Status,
			&p.Service,
			&p.Currency,
			&p.LocalCurrency,   // ✅ Added
			&p.Rate,            // ✅ Added
			&p.InverseRate,     // ✅ Added
			&p.CommissionRate,  // ✅ Added
			&p.APIKey,
			&p.IsAPIEnabled,
			&p.APIRateLimit,
			&p.CreatedAt,
			&p. UpdatedAt,
		); err != nil {
			return nil, err
		}
		partners = append(partners, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return partners, nil
}

// ✅ NEW: GetPartnersByCurrency fetches partners by currency
func (r *PartnerRepo) GetPartnersByCurrency(ctx context.Context, currency string) ([]*domain.Partner, error) {
	query := `
		SELECT 
			id, name, country, contact_email, contact_phone, status, service, currency,
			local_currency, rate, inverse_rate, commission_rate,
			api_key, is_api_enabled, api_rate_limit,
			created_at, updated_at
		FROM partners
		WHERE currency = $1 AND status = 'active'
		ORDER BY rate ASC
	`
	
	rows, err := r.db. Query(ctx, query, currency)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partners []*domain.Partner
	for rows.Next() {
		var p domain.Partner
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Country,
			&p.ContactEmail,
			&p.ContactPhone,
			&p.Status,
			&p.Service,
			&p.Currency,
			&p. LocalCurrency,
			&p.Rate,
			&p.InverseRate,
			&p.CommissionRate,
			&p.APIKey,
			&p.IsAPIEnabled,
			&p.APIRateLimit,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		partners = append(partners, &p)
	}

	return partners, rows.Err()
}

// ✅ NEW: UpdatePartnerRate updates only the rate
func (r *PartnerRepo) UpdatePartnerRate(ctx context.Context, partnerID string, newRate float64) error {
	if newRate <= 0 {
		return errors.New("rate must be greater than 0")
	}

	query := `
		UPDATE partners
		SET rate = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING updated_at
	`
	
	var updatedAt interface{}
	err := r.db.QueryRow(ctx, query, newRate, partnerID).Scan(&updatedAt)
	if err != nil {
		if errors.Is(err, pgx. ErrNoRows) {
			return fmt.Errorf("partner not found: %s", partnerID)
		}
		return err
	}
	
	return nil
}

// DeletePartner deletes a partner
func (r *PartnerRepo) DeletePartner(ctx context. Context, id string) error {
	result, err := r.db. Exec(ctx, `DELETE FROM partners WHERE id=$1`, id)
	if err != nil {
		return err
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("partner not found: %s", id)
	}
	
	return nil
}

// StreamAllPartners streams all partners in batches
func (r *PartnerRepo) StreamAllPartners(
	ctx context.Context,
	batchSize int,
	callback func(*domain.Partner) error,
) error {
	query := `
		SELECT 
			id, name, country, contact_email, contact_phone,
			status, service, currency, local_currency,
			rate, inverse_rate, commission_rate,
			api_key, is_api_enabled, api_rate_limit,
			webhook_url, callback_url,
			created_at, updated_at
		FROM partners
		ORDER BY id
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	count := 0

	for rows.Next() {
		var p domain.Partner

		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Country,
			&p.ContactEmail,
			&p.ContactPhone,
			&p.Status,
			&p.Service,
			&p.Currency,
			&p. LocalCurrency,
			&p.Rate,
			&p. InverseRate,
			&p.CommissionRate,
			&p.APIKey,
			&p.IsAPIEnabled,
			&p.APIRateLimit,
			&p.WebhookURL,
			&p. CallbackURL,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return err
		}

		// Respect context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := callback(&p); err != nil {
			return err
		}

		count++
		if batchSize > 0 && count >= batchSize {
			count = 0
		}
	}

	return rows.Err()
}