package repository

import (
	"context"
	"errors"
	"fmt"
	"partner-service/internal/domain"
	"time"
	"x/shared/utils/id"

	"github.com/jackc/pgx/v5"
)

// CreatePartner inserts a new partner and returns the inserted record.
// CreatePartner inserts a new partner with API credentials
func (r *PartnerRepo) CreatePartner(ctx context.Context, partner *domain.Partner) error {
	// Generate unique partner ID if not set
	if partner.ID == "" {
		partner.ID = id.GenerateID("PTN")
	}
	if partner.Status == "" {
		partner.Status = "active"
	}

	query := `
		INSERT INTO partners (
			id, name, country, contact_email, contact_phone, status, service, currency,
			api_key, api_secret_hash, is_api_enabled, api_rate_limit,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	return r.db.QueryRow(ctx, query,
		partner.ID,
		partner.Name,
		partner.Country,
		partner.ContactEmail,
		partner.ContactPhone,
		partner.Status,
		partner.Service,
		partner.Currency,
		partner.APIKey,
		partner.APISecretHash,
		partner.IsAPIEnabled,
		partner.APIRateLimit,
	).Scan(&partner.CreatedAt, &partner.UpdatedAt)
}

// GetPartnerByID fetches a partner by id with all fields including API credentials
func (r *PartnerRepo) GetPartnerByID(ctx context.Context, id string) (*domain.Partner, error) {
	query := `
		SELECT id, name, country, contact_email, contact_phone, status, service, currency,
		       api_key, api_secret_hash, webhook_url, webhook_secret, callback_url,
		       is_api_enabled, api_rate_limit, allowed_ips, metadata,
		       created_at, updated_at
		FROM partners
		WHERE id = $1
	`
	
	var p domain.Partner
	err := r.db.QueryRow(ctx, query, id).Scan(
		&p.ID,
		&p.Name,
		&p.Country,
		&p.ContactEmail,
		&p.ContactPhone,
		&p.Status,
		&p.Service,
		&p.Currency,
		&p.APIKey,
		&p.APISecretHash,
		&p.WebhookURL,
		&p.WebhookSecret,
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

// UpdatePartner updates partner info
func (r *PartnerRepo) UpdatePartner(ctx context.Context, partner *domain.Partner) error {
	query := `
		UPDATE partners
		SET name=$1, country=$2, contact_email=$3, contact_phone=$4, 
		    status=$5, service=$6, currency=$7, updated_at=NOW()
		WHERE id=$8
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
		partner.ID,
	).Scan(&partner.UpdatedAt)
}

// GetAllPartners fetches all partners with API credentials info
func (r *PartnerRepo) GetAllPartners(ctx context.Context) ([]*domain.Partner, error) {
	query := `
		SELECT id, name, country, contact_email, contact_phone, status, service, currency,
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
			&p.Service,
			&p.Currency,
			&p.APIKey,
			&p.IsAPIEnabled,
			&p.APIRateLimit,
			&p.CreatedAt,
			&p.UpdatedAt,
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
			SELECT id, name, country, contact_email, contact_phone, status, service, currency,
			       api_key, is_api_enabled, api_rate_limit,
			       created_at, updated_at
			FROM partners
			ORDER BY name ASC
		`
		rows, err = r.db.Query(ctx, query)
		if err != nil {
			return nil, err
		}
	} else {
		query := `
			SELECT id, name, country, contact_email, contact_phone, status, service, currency,
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
			&p.Country,
			&p.ContactEmail,
			&p.ContactPhone,
			&p.Status,
			&p.Service,
			&p.Currency,
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

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return partners, nil
}

// GetPartnersByService fetches all partners providing a specific service
func (r *PartnerRepo) GetPartnersByService(ctx context.Context, service string) ([]*domain.Partner, error) {
	query := `
		SELECT id, name, country, contact_email, contact_phone, status, service, currency,
		       api_key, is_api_enabled, api_rate_limit,
		       created_at, updated_at
		FROM partners
		WHERE service = $1
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
			&p.ContactEmail,
			&p.ContactPhone,
			&p.Status,
			&p.Service,
			&p.Currency,
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

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return partners, nil
}

// DeletePartner deletes a partner
func (r *PartnerRepo) DeletePartner(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM partners WHERE id=$1`, id)
	return err
}

// repository/partner_repository.go - ADD THESE METHODS

func (r *PartnerRepo) GenerateAPICredentials(ctx context.Context, partnerID, apiKey, apiSecretHash string) error {
	query := `
		UPDATE partners
		SET api_key = $1,
		    api_secret_hash = $2,
		    is_api_enabled = true,
		    updated_at = NOW()
		WHERE id = $3
	`
	_, err := r.db.Exec(ctx, query, apiKey, apiSecretHash, partnerID)
	return err
}

func (r *PartnerRepo) RevokeAPICredentials(ctx context.Context, partnerID string) error {
	query := `
		UPDATE partners
		SET api_key = NULL,
		    api_secret_hash = NULL,
		    is_api_enabled = false,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, partnerID)
	return err
}

func (r *PartnerRepo) GetPartnerByAPIKey(ctx context.Context, apiKey string) (*domain.Partner, error) {
	query := `
		SELECT id, name, country, contact_email, contact_phone, status, service, currency,
		       api_key, api_secret_hash, webhook_url, webhook_secret, callback_url,
		       is_api_enabled, api_rate_limit, allowed_ips, metadata, created_at, updated_at
		FROM partners
		WHERE api_key = $1 AND is_api_enabled = true
	`
	_ = query
	// Implement scanning logic similar to GetPartnerByID
	// Return *domain.Partner
	return nil, nil // placeholder
}

func (r *PartnerRepo) UpdateWebhookConfig(ctx context.Context, partnerID, webhookURL, webhookSecret, callbackURL string) error {
	query := `
		UPDATE partners
		SET webhook_url = $1,
		    webhook_secret = $2,
		    callback_url = $3,
		    updated_at = NOW()
		WHERE id = $4
	`
	_, err := r.db.Exec(ctx, query, webhookURL, webhookSecret, callbackURL, partnerID)
	return err
}

func (r *PartnerRepo) CreateTransaction(ctx context.Context, txn *domain.PartnerTransaction) error {
	query := `
		INSERT INTO partner_transactions 
		(partner_id, transaction_ref, user_id, amount, currency, status, payment_method, external_ref, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		txn.PartnerID, txn.TransactionRef, txn.UserID, txn.Amount, txn.Currency,
		txn.Status, txn.PaymentMethod, txn.ExternalRef, txn.Metadata,
	).Scan(&txn.ID, &txn.CreatedAt, &txn.UpdatedAt)
}

func (r *PartnerRepo) GetTransactionByRef(ctx context.Context, partnerID, transactionRef string) (*domain.PartnerTransaction, error) {
	query := `
		SELECT id, partner_id, transaction_ref, user_id, amount, currency, status,
		       payment_method, external_ref, metadata, processed_at, created_at, updated_at
		FROM partner_transactions
		WHERE partner_id = $1 AND transaction_ref = $2
	`
	_ = query
	// Implement scanning
	return nil, nil // placeholder
}

func (r *PartnerRepo) ListTransactions(ctx context.Context, partnerID string, limit, offset int, status *string) ([]domain.PartnerTransaction, int64, error) {
	// Implement with optional status filter
	return nil, 0, nil // placeholder
}

func (r *PartnerRepo) LogAPIRequest(ctx context.Context, log *domain.PartnerAPILog) error {
	query := `
		INSERT INTO partner_api_logs
		(partner_id, endpoint, method, request_body, response_body, status_code, ip_address, user_agent, latency_ms, error_message, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
	`
	_, err := r.db.Exec(ctx, query,
		log.PartnerID, log.Endpoint, log.Method, log.RequestBody, log.ResponseBody,
		log.StatusCode, log.IPAddress, log.UserAgent, log.LatencyMs, log.ErrorMessage,
	)
	return err
}

// repository/partner_repository.go - ADD THESE

func (r *PartnerRepo) UpdateAPISettings(ctx context.Context, partnerID string, isEnabled bool, rateLimit int, allowedIPs []string) error {
	query := `
		UPDATE partners
		SET is_api_enabled = $1,
		    api_rate_limit = $2,
		    allowed_ips = $3,
		    updated_at = NOW()
		WHERE id = $4
	`
	_, err := r.db.Exec(ctx, query, isEnabled, rateLimit, allowedIPs, partnerID)
	return err
}

func (r *PartnerRepo) CreateWebhook(ctx context.Context, webhook *domain.PartnerWebhook) error {
	query := `
		INSERT INTO partner_webhooks
		(partner_id, event_type, payload, status, attempts, max_attempts, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		webhook.PartnerID, webhook.EventType, webhook.Payload,
		webhook.Status, webhook.Attempts, webhook.MaxAttempts,
	).Scan(&webhook.ID, &webhook.CreatedAt, &webhook.UpdatedAt)
}

func (r *PartnerRepo) GetWebhookByID(ctx context.Context, webhookID int64) (*domain.PartnerWebhook, error) {
	query := `
		SELECT id, partner_id, event_type, payload, status, attempts, max_attempts,
		       last_attempt_at, next_retry_at, response_status, response_body, error_message,
		       created_at, updated_at
		FROM partner_webhooks
		WHERE id = $1
	`
	_ = query
	// Implement scanning
	return nil, nil // placeholder
}

func (r *PartnerRepo) UpdateWebhookStatus(ctx context.Context, webhookID int64, status string, statusCode int, responseBody, errorMsg string) error {
	query := `
		UPDATE partner_webhooks
		SET status = $1,
		    attempts = attempts + 1,
		    last_attempt_at = NOW(),
		    response_status = $2,
		    response_body = $3,
		    error_message = $4,
		    updated_at = NOW()
		WHERE id = $5
	`
	_, err := r.db.Exec(ctx, query, status, statusCode, responseBody, errorMsg, webhookID)
	return err
}

func (r *PartnerRepo) ListWebhookLogs(ctx context.Context, partnerID string, limit, offset int) ([]domain.PartnerWebhook, int64, error) {
	// Implement pagination
	return nil, 0, nil // placeholder
}

// repository/partner_repository.go - ADD THIS METHOD

func (r *PartnerRepo) ResetWebhookForRetry(ctx context.Context, webhookID int64) error {
	query := `
		UPDATE partner_webhooks
		SET status = 'retrying',
		    next_retry_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		  AND attempts < max_attempts
	`
	
	cmdTag, err := r.db.Exec(ctx, query, webhookID)
	if err != nil {
		return fmt.Errorf("failed to reset webhook: %w", err)
	}
	
	if cmdTag.RowsAffected() == 0 {
		return errors.New("webhook not found or max attempts reached")
	}
	
	return nil
}

// repository/partner_repository.go - ADD THESE METHODS

func (r *PartnerRepo) GetAPILogs(ctx context.Context, partnerID string, limit, offset int, endpointFilter *string) ([]domain.PartnerAPILog, int64, error) {
	// Count total
	countQuery := `
		SELECT COUNT(*) 
		FROM partner_api_logs 
		WHERE partner_id = $1
	`
	args := []interface{}{partnerID}
	
	if endpointFilter != nil && *endpointFilter != "" {
		countQuery += " AND endpoint = $2"
		args = append(args, *endpointFilter)
	}
	
	var total int64
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count API logs: %w", err)
	}

	// Fetch logs
	query := `
		SELECT id, partner_id, endpoint, method, request_body, response_body,
		       status_code, ip_address, user_agent, latency_ms, error_message, created_at
		FROM partner_api_logs
		WHERE partner_id = $1
	`
	
	queryArgs := []interface{}{partnerID}
	argPos := 2
	
	if endpointFilter != nil && *endpointFilter != "" {
		query += fmt.Sprintf(" AND endpoint = $%d", argPos)
		queryArgs = append(queryArgs, *endpointFilter)
		argPos++
	}
	
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argPos, argPos+1)
	queryArgs = append(queryArgs, limit, offset)

	rows, err := r.db.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch API logs: %w", err)
	}
	defer rows.Close()

	var logs []domain.PartnerAPILog
	for rows.Next() {
		var log domain.PartnerAPILog
		err := rows.Scan(
			&log.ID, &log.PartnerID, &log.Endpoint, &log.Method,
			&log.RequestBody, &log.ResponseBody, &log.StatusCode,
			&log.IPAddress, &log.UserAgent, &log.LatencyMs,
			&log.ErrorMessage, &log.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan API log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, total, nil
}

func (r *PartnerRepo) GetAPIUsageStats(ctx context.Context, partnerID string, from, to time.Time) (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(*) as total_requests,
			COUNT(*) FILTER (WHERE status_code >= 200 AND status_code < 300) as successful_requests,
			COUNT(*) FILTER (WHERE status_code >= 400 OR error_message IS NOT NULL) as failed_requests,
			AVG(latency_ms)::INTEGER as average_latency_ms,
			json_object_agg(endpoint, endpoint_count) as requests_by_endpoint
		FROM (
			SELECT 
				endpoint,
				status_code,
				error_message,
				latency_ms,
				COUNT(*) OVER (PARTITION BY endpoint) as endpoint_count
			FROM partner_api_logs
			WHERE partner_id = $1
			  AND created_at >= $2
			  AND created_at <= $3
		) subquery
	`

	var (
		totalRequests      int64
		successfulRequests int64
		failedRequests     int64
		averageLatencyMs   int
		requestsByEndpoint map[string]interface{}
	)

	err := r.db.QueryRow(ctx, query, partnerID, from, to).Scan(
		&totalRequests,
		&successfulRequests,
		&failedRequests,
		&averageLatencyMs,
		&requestsByEndpoint,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get API usage stats: %w", err)
	}

	// Get daily breakdown
	dailyQuery := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as request_count
		FROM partner_api_logs
		WHERE partner_id = $1
		  AND created_at >= $2
		  AND created_at <= $3
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`

	rows, err := r.db.Query(ctx, dailyQuery, partnerID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily stats: %w", err)
	}
	defer rows.Close()

	var requestsByDay []map[string]interface{}
	for rows.Next() {
		var date time.Time
		var count int64
		if err := rows.Scan(&date, &count); err != nil {
			return nil, fmt.Errorf("failed to scan daily stats: %w", err)
		}
		requestsByDay = append(requestsByDay, map[string]interface{}{
			"date":  date.Format("2006-01-02"),
			"count": count,
		})
	}

	return map[string]interface{}{
		"total_requests":       totalRequests,
		"successful_requests":  successfulRequests,
		"failed_requests":      failedRequests,
		"average_latency_ms":   averageLatencyMs,
		"requests_by_endpoint": requestsByEndpoint,
		"requests_by_day":      requestsByDay,
	}, nil
}

func (r *PartnerRepo) StreamAllPartners(
    ctx context.Context,
    batchSize int,
    callback func(*domain.Partner) error,
) error {

    query := `
        SELECT id, name, country, contact_email, contact_phone,
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
            &p.LocalCurrency,
            &p.Rate,
            &p.InverseRate,
            &p.CommissionRate,
            &p.APIKey,
            &p.IsAPIEnabled,
            &p.APIRateLimit,
            &p.WebhookURL,
            &p.CallbackURL,
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
            count = 0 // resets after each batch
        }
    }

    return rows.Err()
}
