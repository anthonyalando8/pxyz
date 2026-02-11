// internal/usecase/p2p_profile_usecase.go
package usecase

import (
	"context"
	"fmt"
	"time"

	"p2p-service/internal/domain"
	"p2p-service/internal/repository"
	"p2p-service/internal/pkg/utils"

	"go.uber.org/zap"
)

type P2PProfileUsecase struct {
	profileRepo *repository.P2PProfileRepository
	logger      *zap.Logger
}

func NewP2PProfileUsecase(
	profileRepo *repository.P2PProfileRepository,
	logger *zap.Logger,
) *P2PProfileUsecase {
	return &P2PProfileUsecase{
		profileRepo: profileRepo,
		logger:      logger,
	}
}

// ============================================================================
// PROFILE MANAGEMENT
// ============================================================================

// CreateProfile creates a new P2P profile
func (uc *P2PProfileUsecase) CreateProfile(
	ctx context.Context,
	req *domain.CreateProfileRequest,
) (*domain.P2PProfile, error) {

	uc.logger.Info("Creating P2P profile",
		zap.String("user_id", req.UserID))

	// Validate inputs
	if err := uc.validateCreateRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if profile already exists for this user
	existingProfile, err := uc.profileRepo.GetByUserID(ctx, req.UserID)
	if err == nil && existingProfile != nil {
		uc.logger.Warn("Profile already exists for user",
			zap.String("user_id", req.UserID),
			zap.Int64("profile_id", existingProfile.ID))
		return existingProfile, nil // Return existing profile
	}

	// Check if username is taken
	if req.Username != "" {
		existingUsername, err := uc.profileRepo.GetByUsername(ctx, req.Username)
		if err == nil && existingUsername != nil {
			return nil, fmt.Errorf("username already taken")
		}
	}

	// Create profile
	profile, err := uc.profileRepo.Create(ctx, req)
	if err != nil {
		uc.logger.Error("Failed to create profile",
			zap.String("user_id", req.UserID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create profile: %w", err)
	}

	uc.logger.Info("P2P profile created successfully",
		zap.Int64("profile_id", profile.ID),
		zap.String("user_id", profile.UserID))

	return profile, nil
}

// GetProfile retrieves a profile by ID
func (uc *P2PProfileUsecase) GetProfile(
	ctx context.Context,
	profileID int64,
) (*domain.P2PProfile, error) {

	profile, err := uc.profileRepo.GetByID(ctx, profileID)
	if err != nil {
		uc.logger.Error("Failed to get profile",
			zap.Int64("profile_id", profileID),
			zap.Error(err))
		return nil, fmt.Errorf("profile not found: %w", err)
	}

	// Update last active
	go uc.profileRepo.UpdateLastActive(context.Background(), profileID)

	return profile, nil
}

// GetProfileByUserID retrieves a profile by user ID
func (uc *P2PProfileUsecase) GetProfileByUserID(
	ctx context.Context,
	userID string,
) (*domain.P2PProfile, error) {

	profile, err := uc.profileRepo.GetByUserID(ctx, userID)
	if err != nil {
		uc.logger.Error("Failed to get profile by user ID",
			zap.String("user_id", userID),
			zap.Error(err))
		return nil, fmt.Errorf("profile not found: %w", err)
	}

	// Update last active
	go uc.profileRepo.UpdateLastActive(context.Background(), profile.ID)

	return profile, nil
}

// GetProfileByUsername retrieves a profile by username
func (uc *P2PProfileUsecase) GetProfileByUsername(
	ctx context.Context,
	username string,
) (*domain.P2PProfile, error) {

	profile, err := uc.profileRepo.GetByUsername(ctx, username)
	if err != nil {
		uc.logger.Error("Failed to get profile by username",
			zap.String("username", username),
			zap.Error(err))
		return nil, fmt.Errorf("profile not found: %w", err)
	}

	return profile, nil
}

// UpdateProfile updates a profile
func (uc *P2PProfileUsecase) UpdateProfile(
	ctx context.Context,
	profileID int64,
	req *domain.UpdateProfileRequest,
) (*domain.P2PProfile, error) {

	uc.logger.Info("Updating P2P profile",
		zap.Int64("profile_id", profileID))

	// Validate inputs
	if err := uc.validateUpdateRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if username is being changed and if it's taken
	if req.Username != nil && *req.Username != "" {
		existingUsername, err := uc.profileRepo.GetByUsername(ctx, *req.Username)
		if err == nil && existingUsername != nil && existingUsername.ID != profileID {
			return nil, fmt.Errorf("username already taken")
		}
	}

	// Update profile
	profile, err := uc.profileRepo.Update(ctx, profileID, req)
	if err != nil {
		uc.logger.Error("Failed to update profile",
			zap.Int64("profile_id", profileID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	uc.logger.Info("Profile updated successfully",
		zap.Int64("profile_id", profileID))

	return profile, nil
}

// ListProfiles retrieves profiles with filters
func (uc *P2PProfileUsecase) ListProfiles(
	ctx context.Context,
	filter *domain.ProfileFilter,
) ([]*domain.P2PProfile, int, error) {

	// Set defaults
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 20
	}

	profiles, total, err := uc.profileRepo.List(ctx, filter)
	if err != nil {
		uc.logger.Error("Failed to list profiles", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to list profiles: %w", err)
	}

	return profiles, total, nil
}

// ============================================================================
// PROFILE STATISTICS
// ============================================================================

// GetProfileStats retrieves profile statistics
func (uc *P2PProfileUsecase) GetProfileStats(
	ctx context.Context,
	profileID int64,
) (*domain.ProfileStats, error) {

	stats, err := uc.profileRepo.GetStats(ctx, profileID)
	if err != nil {
		uc.logger.Error("Failed to get profile stats",
			zap.Int64("profile_id", profileID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	return stats, nil
}

// IncrementTradeCount increments trade counters after a trade
func (uc *P2PProfileUsecase) IncrementTradeCount(
	ctx context.Context,
	profileID int64,
	completed bool,
) error {

	uc.logger.Info("Incrementing trade count",
		zap.Int64("profile_id", profileID),
		zap.Bool("completed", completed))

	// Get current stats
	stats, err := uc.profileRepo.GetStats(ctx, profileID)
	if err != nil {
		return fmt.Errorf("failed to get current stats: %w", err)
	}

	// Update counters
	stats.TotalTrades++
	if completed {
		stats.CompletedTrades++
	} else {
		stats.CancelledTrades++
	}

	// Update in database
	if err := uc.profileRepo.UpdateStats(ctx, profileID, stats); err != nil {
		uc.logger.Error("Failed to update trade count",
			zap.Int64("profile_id", profileID),
			zap.Error(err))
		return fmt.Errorf("failed to update stats: %w", err)
	}

	return nil
}

// UpdateRating updates profile rating after a review
func (uc *P2PProfileUsecase) UpdateRating(
	ctx context.Context,
	profileID int64,
	newRating int,
) error {

	uc.logger.Info("Updating profile rating",
		zap.Int64("profile_id", profileID),
		zap.Int("rating", newRating))

	if newRating < 1 || newRating > 5 {
		return fmt.Errorf("rating must be between 1 and 5")
	}

	// Get current stats
	stats, err := uc.profileRepo.GetStats(ctx, profileID)
	if err != nil {
		return fmt.Errorf("failed to get current stats: %w", err)
	}

	// Calculate new average rating
	totalRatingScore := stats.AvgRating * float64(stats.TotalReviews)
	totalRatingScore += float64(newRating)
	stats.TotalReviews++
	stats.AvgRating = totalRatingScore / float64(stats.TotalReviews)

	// Update in database
	if err := uc.profileRepo.UpdateStats(ctx, profileID, stats); err != nil {
		uc.logger.Error("Failed to update rating",
			zap.Int64("profile_id", profileID),
			zap.Error(err))
		return fmt.Errorf("failed to update rating: %w", err)
	}

	uc.logger.Info("Rating updated",
		zap.Int64("profile_id", profileID),
		zap.Float64("new_avg_rating", stats.AvgRating),
		zap.Int("total_reviews", stats.TotalReviews))

	return nil
}

// ============================================================================
// VERIFICATION & MERCHANT STATUS
// ============================================================================

// SetVerified sets verification status for a profile
func (uc *P2PProfileUsecase) SetVerified(
	ctx context.Context,
	profileID int64,
	verified bool,
) error {

	uc.logger.Info("Setting verification status",
		zap.Int64("profile_id", profileID),
		zap.Bool("verified", verified))

	if err := uc.profileRepo.SetVerified(ctx, profileID, verified); err != nil {
		uc.logger.Error("Failed to set verification status",
			zap.Int64("profile_id", profileID),
			zap.Error(err))
		return fmt.Errorf("failed to set verification: %w", err)
	}

	uc.logger.Info("Verification status updated",
		zap.Int64("profile_id", profileID),
		zap.Bool("verified", verified))

	return nil
}

// SetMerchant sets merchant status for a profile
func (uc *P2PProfileUsecase) SetMerchant(
	ctx context.Context,
	profileID int64,
	isMerchant bool,
) error {

	uc.logger.Info("Setting merchant status",
		zap.Int64("profile_id", profileID),
		zap.Bool("is_merchant", isMerchant))

	// Get profile
	profile, err := uc.profileRepo.GetByID(ctx, profileID)
	if err != nil {
		return fmt.Errorf("profile not found: %w", err)
	}

	// Check if profile is verified (requirement for merchant)
	if isMerchant && !profile.IsVerified {
		return fmt.Errorf("profile must be verified to become a merchant")
	}

	// Check minimum completed trades for merchant status
	if isMerchant && profile.CompletedTrades < 10 {
		return fmt.Errorf("profile must have at least 10 completed trades to become a merchant")
	}

	// Check minimum rating for merchant status
	if isMerchant && profile.AvgRating < 4.0 {
		return fmt.Errorf("profile must have at least 4.0 rating to become a merchant")
	}

	if err := uc.profileRepo.SetMerchant(ctx, profileID, isMerchant); err != nil {
		uc.logger.Error("Failed to set merchant status",
			zap.Int64("profile_id", profileID),
			zap.Error(err))
		return fmt.Errorf("failed to set merchant status: %w", err)
	}

	uc.logger.Info("Merchant status updated",
		zap.Int64("profile_id", profileID),
		zap.Bool("is_merchant", isMerchant))

	return nil
}

// ============================================================================
// SUSPENSION MANAGEMENT
// ============================================================================

// SuspendProfile suspends a profile
func (uc *P2PProfileUsecase) SuspendProfile(
	ctx context.Context,
	profileID int64,
	reason string,
	duration *time.Duration,
) error {

	uc.logger.Info("Suspending profile",
		zap.Int64("profile_id", profileID),
		zap.String("reason", reason))

	if reason == "" {
		return fmt.Errorf("suspension reason is required")
	}

	var until *time.Time
	if duration != nil {
		suspendUntil := time.Now().Add(*duration)
		until = &suspendUntil
	}

	if err := uc.profileRepo.Suspend(ctx, profileID, reason, until); err != nil {
		uc.logger.Error("Failed to suspend profile",
			zap.Int64("profile_id", profileID),
			zap.Error(err))
		return fmt.Errorf("failed to suspend profile: %w", err)
	}

	uc.logger.Info("Profile suspended",
		zap.Int64("profile_id", profileID),
		zap.String("reason", reason))

	// TODO: Notify user of suspension
	// go uc.notifyProfileSuspension(ctx, profileID, reason, until)

	return nil
}

// UnsuspendProfile unsuspends a profile
func (uc *P2PProfileUsecase) UnsuspendProfile(
	ctx context.Context,
	profileID int64,
) error {

	uc.logger.Info("Unsuspending profile",
		zap.Int64("profile_id", profileID))

	if err := uc.profileRepo.Unsuspend(ctx, profileID); err != nil {
		uc.logger.Error("Failed to unsuspend profile",
			zap.Int64("profile_id", profileID),
			zap.Error(err))
		return fmt.Errorf("failed to unsuspend profile: %w", err)
	}

	uc.logger.Info("Profile unsuspended",
		zap.Int64("profile_id", profileID))

	// TODO: Notify user of unsuspension
	// go uc.notifyProfileUnsuspension(ctx, profileID)

	return nil
}

// CheckSuspension checks if a suspension has expired and auto-unsuspends
func (uc *P2PProfileUsecase) CheckSuspension(
	ctx context.Context,
	profileID int64,
) error {

	profile, err := uc.profileRepo.GetByID(ctx, profileID)
	if err != nil {
		return err
	}

	// Check if suspended and if suspension has expired
	if profile.IsSuspended && profile.SuspendedUntil != nil {
		if time.Now().After(*profile.SuspendedUntil) {
			uc.logger.Info("Suspension expired, auto-unsuspending",
				zap.Int64("profile_id", profileID))
			return uc.profileRepo.Unsuspend(ctx, profileID)
		}
	}

	return nil
}

// ============================================================================
// PROFILE VALIDATION
// ============================================================================

// CanTrade checks if a profile can trade
func (uc *P2PProfileUsecase) CanTrade(
	ctx context.Context,
	profileID int64,
) (bool, string, error) {

	profile, err := uc.profileRepo.GetByID(ctx, profileID)
	if err != nil {
		return false, "Profile not found", err
	}

	// Check suspension
	if profile.IsSuspended {
		if profile.SuspendedUntil != nil {
			if time.Now().Before(*profile.SuspendedUntil) {
				return false, fmt.Sprintf("Profile suspended until %s. Reason: %s",
					profile.SuspendedUntil.Format("2006-01-02 15:04"),
					stringValue(profile.SuspensionReason)), nil
			}
			// Suspension expired, auto-unsuspend
			uc.profileRepo.Unsuspend(ctx, profileID)
		} else {
			return false, fmt.Sprintf("Profile suspended indefinitely. Reason: %s",
				stringValue(profile.SuspensionReason)), nil
		}
	}

	return true, "", nil
}

// MeetsRequirements checks if profile meets trading requirements
func (uc *P2PProfileUsecase) MeetsRequirements(
	ctx context.Context,
	profileID int64,
	minCompletionRate *int,
	requiresVerification bool,
) (bool, string, error) {

	profile, err := uc.profileRepo.GetByID(ctx, profileID)
	if err != nil {
		return false, "Profile not found", err
	}

	// Check verification requirement
	if requiresVerification && !profile.IsVerified {
		return false, "Profile must be verified for this trade", nil
	}

	// Check completion rate
	if minCompletionRate != nil && *minCompletionRate > 0 {
		completionRate := profile.CompletionRate()
		if completionRate < float64(*minCompletionRate) {
			return false, fmt.Sprintf("Completion rate (%.2f%%) is below required %.2f%%",
				completionRate, float64(*minCompletionRate)), nil
		}
	}

	return true, "", nil
}

// ============================================================================
// VALIDATION HELPERS
// ============================================================================

func (uc *P2PProfileUsecase) validateCreateRequest(req *domain.CreateProfileRequest) error {
	if req.UserID == "" {
		return fmt.Errorf("user_id is required")
	}

	if err := utils.ValidateUsername(req.Username); err != nil {
		return err
	}

	if err := utils.ValidatePhoneNumber(req.PhoneNumber); err != nil {
		return err
	}

	if err := utils.ValidateEmail(req.Email); err != nil {
		return err
	}

	return nil
}

func (uc *P2PProfileUsecase) validateUpdateRequest(req *domain.UpdateProfileRequest) error {
	if req.Username != nil {
		if err := utils.ValidateUsername(*req.Username); err != nil {
			return err
		}
	}

	if req.PhoneNumber != nil {
		if err := utils.ValidatePhoneNumber(*req.PhoneNumber); err != nil {
			return err
		}
	}

	if req.Email != nil {
		if err := utils.ValidateEmail(*req.Email); err != nil {
			return err
		}
	}

	return nil
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}