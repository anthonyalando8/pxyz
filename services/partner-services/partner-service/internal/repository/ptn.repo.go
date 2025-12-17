package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"partner-service/internal/domain"
	"time"
	xerrors "x/shared/utils/errors"
	//"x/shared/utils/id"

	"github.com/jackc/pgx/v5"
)


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
		       local_currency, rate, inverse_rate, commission_rate,
		       api_key, api_secret_hash, webhook_url, webhook_secret, callback_url,
		       is_api_enabled, api_rate_limit, allowed_ips, metadata, created_at, updated_at
		FROM partners
		WHERE api_key = $1 AND is_api_enabled = true
	`

	var partner domain.Partner
	var (
		apiKeyPtr        *string
		apiSecretHash    *string
		webhookURL       *string
		webhookSecret    *string
		callbackURL      *string
		allowedIPsJSON   []byte
		metadataJSON     []byte
		localCurrency    *string
		rate             *float64
		inverseRate      *float64
	)

	err := r.db.QueryRow(ctx, query, apiKey). Scan(
		&partner. ID,
		&partner.Name,
		&partner.Country,
		&partner.ContactEmail,
		&partner.ContactPhone,
		&partner.Status,
		&partner.Service,
		&partner.Currency,
		&localCurrency,
		&rate,
		&inverseRate,
		&partner.CommissionRate,
		&apiKeyPtr,
		&apiSecretHash,
		&webhookURL,
		&webhookSecret,
		&callbackURL,
		&partner.IsAPIEnabled,
		&partner.APIRateLimit,
		&allowedIPsJSON,
		&metadataJSON,
		&partner.CreatedAt,
		&partner.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("partner not found or API access disabled: %w", xerrors.ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get partner by API key: %w", err)
	}

	// Assign optional fields
	partner.APIKey = apiKeyPtr
	partner.APISecretHash = apiSecretHash
	partner. WebhookURL = webhookURL
	partner.WebhookSecret = webhookSecret
	partner. CallbackURL = callbackURL

	if localCurrency != nil {
		partner.LocalCurrency = *localCurrency
	}
	if rate != nil {
		partner.Rate = *rate
	}
	if inverseRate != nil {
		partner.InverseRate = *inverseRate
	}

	// Parse allowed IPs
	if len(allowedIPsJSON) > 0 {
		if err := json.Unmarshal(allowedIPsJSON, &partner.AllowedIPs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal allowed IPs: %w", err)
		}
	}

	// Parse metadata
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &partner.Metadata); err != nil {
			return nil, fmt. Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &partner, nil
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


// end new methods for partner transactions

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

// repository/partner_repo.go

func (r *PartnerRepo) CreateWebhook(ctx context.Context, webhook *domain.PartnerWebhook) error {
	query := `
		INSERT INTO partner_webhooks
		(partner_id, event_type, payload, status, attempts, max_attempts, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		webhook.PartnerID, 
		webhook.EventType, 
		webhook.Payload,
		webhook.Status, 
		webhook.Attempts, 
		webhook.MaxAttempts,
	).Scan(&webhook.ID, &webhook.CreatedAt, &webhook.UpdatedAt)
}

func (r *PartnerRepo) GetWebhookByID(ctx context.Context, webhookID int64) (*domain.PartnerWebhook, error) {
	query := `
		SELECT 
			id, partner_id, event_type, payload, status, attempts, max_attempts,
			last_attempt_at, next_retry_at, response_status, response_body, error_message,
			created_at, updated_at
		FROM partner_webhooks
		WHERE id = $1
	`
	
	webhook := &domain.PartnerWebhook{}
	
	err := r.db.QueryRow(ctx, query, webhookID).Scan(
		&webhook.ID,
		&webhook.PartnerID,
		&webhook. EventType,
		&webhook. Payload,
		&webhook. Status,
		&webhook. Attempts,
		&webhook.MaxAttempts,
		&webhook.LastAttemptAt,
		&webhook.NextRetryAt,
		&webhook.ResponseStatus,
		&webhook.ResponseBody,
		&webhook.ErrorMessage,
		&webhook.CreatedAt,
		&webhook.UpdatedAt,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("webhook not found: %d", webhookID)
		}
		return nil, fmt.Errorf("failed to scan webhook: %w", err)
	}
	
	return webhook, nil
}

func (r *PartnerRepo) UpdateWebhookStatus(ctx context.Context, webhookID int64, status string, statusCode int, responseBody, errorMsg string) error {
	query := `
		UPDATE partner_webhooks
		SET 
			status = $1,
			attempts = attempts + 1,
			last_attempt_at = NOW(),
			response_status = $2,
			response_body = $3,
			error_message = $4,
			updated_at = NOW()
		WHERE id = $5
	`
	
	result, err := r.db. Exec(ctx, query, status, statusCode, responseBody, errorMsg, webhookID)
	if err != nil {
		return fmt.Errorf("failed to update webhook status: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("webhook not found: %d", webhookID)
	}
	
	return nil
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

