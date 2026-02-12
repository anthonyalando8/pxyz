// internal/repository/p2p_profile_repository.go
package repository

import (
	"context"
	//"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"p2p-service/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type P2PProfileRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewP2PProfileRepository(pool *pgxpool.Pool, logger *zap.Logger) *P2PProfileRepository {
	return &P2PProfileRepository{
		pool:   pool,
		logger: logger,
	}
}

// ============================================================================
// CREATE
// ============================================================================

// Create creates a new P2P profile
func (r *P2PProfileRepository) Create(ctx context.Context, req *domain.CreateProfileRequest) (*domain.P2PProfile, error) {
	query := `
		INSERT INTO p2p_profiles (
			user_id, username, phone_number, email, profile_picture_url,
			preferred_currency, preferred_payment_methods, auto_reply_message,
			has_consent, consented_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING 
			id, user_id, username, phone_number, email, profile_picture_url,
			total_trades, completed_trades, cancelled_trades, avg_rating, total_reviews,
			is_verified, is_merchant, is_suspended, suspension_reason, suspended_until,
			preferred_currency, preferred_payment_methods, auto_reply_message,
			has_consent, consented_at,
			metadata, last_active_at, joined_at, created_at, updated_at
	`

	var preferredPaymentMethodsJSON []byte
	if len(req.PreferredPaymentMethods) > 0 {
		preferredPaymentMethodsJSON, _ = json.Marshal(req.PreferredPaymentMethods)
	}

	profile := &domain.P2PProfile{}
	err := r.pool.QueryRow(ctx, query,
		req.UserID,
		nullString(req.Username),
		nullString(req.PhoneNumber),
		nullString(req.Email),
		nullString(req.ProfilePictureURL),
		nullString(req.PreferredCurrency),
		preferredPaymentMethodsJSON,
		nullString(req.AutoReplyMessage),
		req.HasConsent,
		time.Now(),
	).Scan(
		&profile.ID,
		&profile.UserID,
		&profile.Username,
		&profile.PhoneNumber,
		&profile.Email,
		&profile.ProfilePictureURL,
		&profile.TotalTrades,
		&profile.CompletedTrades,
		&profile.CancelledTrades,
		&profile.AvgRating,
		&profile.TotalReviews,
		&profile.IsVerified,
		&profile.IsMerchant,
		&profile.IsSuspended,
		&profile.SuspensionReason,
		&profile.SuspendedUntil,
		&profile.PreferredCurrency,
		&profile.PreferredPaymentMethods,
		&profile.AutoReplyMessage,
		&profile.HasConsent,
		&profile.ConsentedAt,
		&profile.Metadata,
		&profile.LastActiveAt,
		&profile.JoinedAt,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)

	if err != nil {
		r.logger.Error("Failed to create P2P profile",
			zap.String("user_id", req.UserID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create profile: %w", err)
	}

	r.logger.Info("P2P profile created",
		zap.Int64("profile_id", profile.ID),
		zap.String("user_id", profile.UserID))

	return profile, nil
}

// ============================================================================
// READ
// ============================================================================

// GetByID retrieves a profile by ID
func (r *P2PProfileRepository) GetByID(ctx context.Context, id int64) (*domain.P2PProfile, error) {
	query := `
		SELECT 
			id, user_id, username, phone_number, email, profile_picture_url,
			total_trades, completed_trades, cancelled_trades, avg_rating, total_reviews,
			is_verified, is_merchant, is_suspended, suspension_reason, suspended_until,
			preferred_currency, preferred_payment_methods, auto_reply_message, has_consent, consented_at,
			metadata, last_active_at, joined_at, created_at, updated_at
		FROM p2p_profiles
		WHERE id = $1
	`

	profile := &domain.P2PProfile{}
	err := r.scanProfile(r.pool.QueryRow(ctx, query, id), profile)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("profile not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	return profile, nil
}

// GetByUserID retrieves a profile by user ID
func (r *P2PProfileRepository) GetByUserID(ctx context.Context, userID string) (*domain.P2PProfile, error) {
	query := `
		SELECT 
			id, user_id, username, phone_number, email, profile_picture_url,
			total_trades, completed_trades, cancelled_trades, avg_rating, total_reviews,
			is_verified, is_merchant, is_suspended, suspension_reason, suspended_until,
			preferred_currency, preferred_payment_methods, auto_reply_message, has_consent, consented_at,
			metadata, last_active_at, joined_at, created_at, updated_at
		FROM p2p_profiles
		WHERE user_id = $1
	`

	profile := &domain.P2PProfile{}
	err := r.scanProfile(r.pool.QueryRow(ctx, query, userID), profile)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("profile not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	return profile, nil
}

// GetByUsername retrieves a profile by username
func (r *P2PProfileRepository) GetByUsername(ctx context.Context, username string) (*domain.P2PProfile, error) {
	query := `
		SELECT 
			id, user_id, username, phone_number, email, profile_picture_url,
			total_trades, completed_trades, cancelled_trades, avg_rating, total_reviews,
			is_verified, is_merchant, is_suspended, suspension_reason, suspended_until,
			preferred_currency, preferred_payment_methods, auto_reply_message, has_consent, consented_at,
			metadata, last_active_at, joined_at, created_at, updated_at
		FROM p2p_profiles
		WHERE username = $1
	`

	profile := &domain.P2PProfile{}
	err := r.scanProfile(r.pool.QueryRow(ctx, query, username), profile)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("profile not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	return profile, nil
}

// List retrieves profiles with filters
func (r *P2PProfileRepository) List(ctx context.Context, filter *domain.ProfileFilter) ([]*domain.P2PProfile, int, error) {
	// Build WHERE clause
	whereConditions := []string{}
	args := []interface{}{}
	argPos := 1

	if filter.IsVerified != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("is_verified = $%d", argPos))
		args = append(args, *filter.IsVerified)
		argPos++
	}

	if filter.IsMerchant != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("is_merchant = $%d", argPos))
		args = append(args, *filter.IsMerchant)
		argPos++
	}

	if filter.IsSuspended != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("is_suspended = $%d", argPos))
		args = append(args, *filter.IsSuspended)
		argPos++
	}

	if filter.MinRating != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("avg_rating >= $%d", argPos))
		args = append(args, *filter.MinRating)
		argPos++
	}

	if filter.MinTrades != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("completed_trades >= $%d", argPos))
		args = append(args, *filter.MinTrades)
		argPos++
	}

	if filter.Search != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("username ILIKE $%d", argPos))
		args = append(args, "%"+filter.Search+"%")
		argPos++
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + strings.Join(whereConditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM p2p_profiles %s", whereClause)
	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count profiles: %w", err)
	}

	// Get profiles
	query := fmt.Sprintf(`
		SELECT 
			id, user_id, username, phone_number, email, profile_picture_url,
			total_trades, completed_trades, cancelled_trades, avg_rating, total_reviews,
			is_verified, is_merchant, is_suspended, suspension_reason, suspended_until,
			preferred_currency, preferred_payment_methods, auto_reply_message, has_consent, consented_at,
			metadata, last_active_at, joined_at, created_at, updated_at
		FROM p2p_profiles
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argPos, argPos+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list profiles: %w", err)
	}
	defer rows.Close()

	profiles := []*domain.P2PProfile{}
	for rows.Next() {
		profile := &domain.P2PProfile{}
		if err := r.scanProfile(rows, profile); err != nil {
			return nil, 0, err
		}
		profiles = append(profiles, profile)
	}

	return profiles, total, nil
}

// ============================================================================
// UPDATE
// ============================================================================

// Update updates a profile
func (r *P2PProfileRepository) Update(ctx context.Context, id int64, req *domain.UpdateProfileRequest) (*domain.P2PProfile, error) {
	updates := []string{}
	args := []interface{}{}
	argPos := 1

	if req.Username != nil {
		updates = append(updates, fmt.Sprintf("username = $%d", argPos))
		args = append(args, *req.Username)
		argPos++
	}

	if req.PhoneNumber != nil {
		updates = append(updates, fmt.Sprintf("phone_number = $%d", argPos))
		args = append(args, *req.PhoneNumber)
		argPos++
	}

	if req.Email != nil {
		updates = append(updates, fmt.Sprintf("email = $%d", argPos))
		args = append(args, *req.Email)
		argPos++
	}

	if req.ProfilePictureURL != nil {
		updates = append(updates, fmt.Sprintf("profile_picture_url = $%d", argPos))
		args = append(args, *req.ProfilePictureURL)
		argPos++
	}

	if req.PreferredCurrency != nil {
		updates = append(updates, fmt.Sprintf("preferred_currency = $%d", argPos))
		args = append(args, *req.PreferredCurrency)
		argPos++
	}

	if req.PreferredPaymentMethods != nil {
		jsonData, _ := json.Marshal(req.PreferredPaymentMethods)
		updates = append(updates, fmt.Sprintf("preferred_payment_methods = $%d", argPos))
		args = append(args, jsonData)
		argPos++
	}

	if req.AutoReplyMessage != nil {
		updates = append(updates, fmt.Sprintf("auto_reply_message = $%d", argPos))
		args = append(args, *req.AutoReplyMessage)
		argPos++
	}

	if len(updates) == 0 {
		return r.GetByID(ctx, id)
	}

	args = append(args, id)
	query := fmt.Sprintf(`
		UPDATE p2p_profiles
		SET %s, updated_at = NOW()
		WHERE id = $%d
		RETURNING 
			id, user_id, username, phone_number, email, profile_picture_url,
			total_trades, completed_trades, cancelled_trades, avg_rating, total_reviews,
			is_verified, is_merchant, is_suspended, suspension_reason, suspended_until,
			preferred_currency, preferred_payment_methods, auto_reply_message, has_consent, consented_at,
			metadata, last_active_at, joined_at, created_at, updated_at
	`, strings.Join(updates, ", "), argPos)

	profile := &domain.P2PProfile{}
	err := r.scanProfile(r.pool.QueryRow(ctx, query, args...), profile)

	if err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	r.logger.Info("Profile updated",
		zap.Int64("profile_id", id))

	return profile, nil
}

// UpdateStats updates trading statistics
func (r *P2PProfileRepository) UpdateStats(ctx context.Context, profileID int64, stats *domain.ProfileStats) error {
	query := `
		UPDATE p2p_profiles
		SET 
			total_trades = $1,
			completed_trades = $2,
			cancelled_trades = $3,
			avg_rating = $4,
			total_reviews = $5,
			updated_at = NOW()
		WHERE id = $6
	`

	result, err := r.pool.Exec(ctx, query,
		stats.TotalTrades,
		stats.CompletedTrades,
		stats.CancelledTrades,
		stats.AvgRating,
		stats.TotalReviews,
		profileID,
	)

	if err != nil {
		return fmt.Errorf("failed to update stats: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("profile not found")
	}

	return nil
}

// UpdateLastActive updates last active timestamp
func (r *P2PProfileRepository) UpdateLastActive(ctx context.Context, profileID int64) error {
	query := `
		UPDATE p2p_profiles
		SET last_active_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, profileID)
	return err
}

// ============================================================================
// SUSPENSION
// ============================================================================

// Suspend suspends a profile
func (r *P2PProfileRepository) Suspend(ctx context.Context, profileID int64, reason string, until *time.Time) error {
	query := `
		UPDATE p2p_profiles
		SET 
			is_suspended = true,
			suspension_reason = $1,
			suspended_until = $2,
			updated_at = NOW()
		WHERE id = $3
	`

	result, err := r.pool.Exec(ctx, query, reason, until, profileID)
	if err != nil {
		return fmt.Errorf("failed to suspend profile: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("profile not found")
	}

	r.logger.Info("Profile suspended",
		zap.Int64("profile_id", profileID),
		zap.String("reason", reason))

	return nil
}

// Unsuspend unsuspends a profile
func (r *P2PProfileRepository) Unsuspend(ctx context.Context, profileID int64) error {
	query := `
		UPDATE p2p_profiles
		SET 
			is_suspended = false,
			suspension_reason = NULL,
			suspended_until = NULL,
			updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, profileID)
	if err != nil {
		return fmt.Errorf("failed to unsuspend profile: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("profile not found")
	}

	r.logger.Info("Profile unsuspended",
		zap.Int64("profile_id", profileID))

	return nil
}

// ============================================================================
// VERIFICATION
// ============================================================================

// SetVerified sets verification status
func (r *P2PProfileRepository) SetVerified(ctx context.Context, profileID int64, verified bool) error {
	query := `
		UPDATE p2p_profiles
		SET is_verified = $1, updated_at = NOW()
		WHERE id = $2
	`

	result, err := r.pool.Exec(ctx, query, verified, profileID)
	if err != nil {
		return fmt.Errorf("failed to update verification: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("profile not found")
	}

	return nil
}

// SetMerchant sets merchant status
func (r *P2PProfileRepository) SetMerchant(ctx context.Context, profileID int64, isMerchant bool) error {
	query := `
		UPDATE p2p_profiles
		SET is_merchant = $1, updated_at = NOW()
		WHERE id = $2
	`

	result, err := r.pool.Exec(ctx, query, isMerchant, profileID)
	if err != nil {
		return fmt.Errorf("failed to update merchant status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("profile not found")
	}

	return nil
}

// ============================================================================
// STATISTICS
// ============================================================================

// GetStats retrieves profile statistics
func (r *P2PProfileRepository) GetStats(ctx context.Context, profileID int64) (*domain.ProfileStats, error) {
	query := `
		SELECT 
			total_trades,
			completed_trades,
			cancelled_trades,
			avg_rating,
			total_reviews
		FROM p2p_profiles
		WHERE id = $1
	`

	stats := &domain.ProfileStats{}
	err := r.pool.QueryRow(ctx, query, profileID).Scan(
		&stats.TotalTrades,
		&stats.CompletedTrades,
		&stats.CancelledTrades,
		&stats.AvgRating,
		&stats.TotalReviews,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	// Calculate completion rate
	if stats.TotalTrades > 0 {
		stats.CompletionRate = (float64(stats.CompletedTrades) / float64(stats.TotalTrades)) * 100
	}

	return stats, nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func (r *P2PProfileRepository) scanProfile(row pgx.Row, profile *domain.P2PProfile) error {
	return row.Scan(
		&profile.ID,
		&profile.UserID,
		&profile.Username,
		&profile.PhoneNumber,
		&profile.Email,
		&profile.ProfilePictureURL,
		&profile.TotalTrades,
		&profile.CompletedTrades,
		&profile.CancelledTrades,
		&profile.AvgRating,
		&profile.TotalReviews,
		&profile.IsVerified,
		&profile.IsMerchant,
		&profile.IsSuspended,
		&profile.SuspensionReason,
		&profile.SuspendedUntil,
		&profile.PreferredCurrency,
		&profile.PreferredPaymentMethods,
		&profile.AutoReplyMessage,
		&profile.HasConsent,
		&profile.ConsentedAt,
		&profile.Metadata,
		&profile.LastActiveAt,
		&profile.JoinedAt,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}