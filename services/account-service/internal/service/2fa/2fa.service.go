package _2faservice

import (
	"account-service/internal/domain"
	"account-service/internal/repository"
	"context"
	"errors"
	"fmt"

	"x/shared/utils/id"
	"x/shared/utils/errors"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"


)

type TwoFAService struct {
    repo *repository.TwoFARepository
    sf          *id.Snowflake
}

func NewTwoFAService(repo *repository.TwoFARepository, sf *id.Snowflake,) *TwoFAService {
    return &TwoFAService{repo: repo, sf: sf,}
}

// Step 1: Generate new TOTP secret & URL for QR code
func (s *TwoFAService) InitiateTOTPSetup(ctx context.Context, userID string, email string) (secret, otpURL string, err error) {
    key, err := totp.Generate(totp.GenerateOpts{
        Issuer:      "pxyz",     // replace with your service name
        AccountName: email,       // identifier shown in authenticator
        Period:      30,
        SecretSize:  32,
        Algorithm:   otp.AlgorithmSHA1,
    })
    if err != nil {
        return "", "", err
    }

    return key.Secret(), key.URL(), nil
}

// Step 2: Verify provided OTP code, persist 2FA if valid
func (s *TwoFAService) EnableTwoFA(ctx context.Context, userID string, email, secret, code string) (*domain.UserTwoFA, []string, error) {
	// Verify code
	if !totp.Validate(code, secret) {
		return nil, nil, xerrors.ErrInvalidOrExpiredTOTP
	}

	// Create 2FA record
	twofa := &domain.UserTwoFA{
		UserID:    userID,
		Method:    "totp",
		Secret:    secret,
		IsEnabled: true,
	}
	createdTwoFA, err := s.repo.CreateTwoFA(ctx, twofa)
	if err != nil {
		return nil, nil, err
	}

	// Generate backup codes
	plainCodes, hashedCodes, err := GenerateBackupCodes(10)
	if err != nil {
		return nil, nil, err
	}

	// Save hashes into DB
	if err := s.repo.AddBackupCodes(ctx, createdTwoFA.ID, hashedCodes); err != nil {
		return nil, nil, err
	}

	// Return created 2FA + plaintext backup codes
	return createdTwoFA, plainCodes, nil
}

func (s *TwoFAService) GetTwoFAStatus(ctx context.Context, userId string) (bool, string, error) {
	twofa, err := s.repo.GetTwoFA(ctx, userId, "totp")
	if err != nil {
		if errors.Is(err, xerrors.ErrNotFound) {
			// Not found → treat as "disabled"
			return false, "totp", nil
		}
		// Other errors → propagate
		return false, "totp", err
	}
	return twofa.IsEnabled, "totp", nil
}



func (s *TwoFAService) VerifyTwoFA(
	ctx context.Context,
	userID string,
	method string,
	code string,
	backupCode string,
) (bool, error) {
	// 1. Fetch 2FA config for this user & method
	twofa, err := s.repo.GetTwoFA(ctx, userID, method)
	if err != nil {
		return false, err
	}
	if twofa == nil || !twofa.IsEnabled {
		return false, errors.New("2FA not enabled for this method")
	}

	// 2. If backupCode is provided → check backup codes
	if backupCode != "" {
		ok, err := s.repo.VerifyAndConsumeBackupCode(ctx, twofa.ID, backupCode)
		if err != nil {
			return false, err
		}
		return ok, nil
	}

	// 3. Otherwise → verify the actual 2FA code
	switch method {
	case "totp":
		if !totp.Validate(code, twofa.Secret) {
			return false, errors.New("invalid TOTP code")
		}
	case "sms", "email":
		// Placeholder for verifying SMS/email codes
		if code == "" {
			return false, errors.New("missing verification code")
		}
		// (you’d integrate with OTP service here)
	default:
		return false, errors.New("unsupported 2FA method")
	}

	return true, nil
}


func (s *TwoFAService) DisableTwoFA(ctx context.Context, userID string, method string, code string, backupCode string) (bool, error) {
	// Fetch current 2FA entry
	twofa, err := s.repo.GetTwoFA(ctx, userID, method)
	if err != nil {
		return false, fmt.Errorf("failed to fetch 2FA: %w", err)
	}
	if twofa == nil || !twofa.IsEnabled {
		return false, errors.New("2FA not enabled for this method")
	}

	// --- Step 1: Validate TOTP code ---
	if method == "totp" && code != "" {
		if totp.Validate(code, twofa.Secret) {
			goto disable
		}
	}

	// --- Step 2: Validate Backup code ---
	if backupCode != "" {
		ok, err := s.repo.VerifyAndConsumeBackupCode(ctx, twofa.ID, backupCode)
		if err != nil {
			return false, fmt.Errorf("failed to check backup code: %w", err)
		}
		if ok {
			goto disable
		}
	}

	return false, errors.New("invalid code provided")

disable:
	// Mark 2FA as disabled
	err = s.repo.DeleteTwoFA(ctx, twofa.ID)
	if err != nil {
		return false, fmt.Errorf("failed to disable 2FA: %w", err)
	}

	return true, nil
}


func (s *TwoFAService) RegenerateBackupCodes(ctx context.Context, userID string, method string) ([]string, error) {
	// Fetch the 2FA entry
	twofa, err := s.repo.GetTwoFA(ctx, userID, method)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch 2FA: %w", err)
	}
	if twofa == nil || !twofa.IsEnabled {
		return nil, errors.New("2FA not enabled for this method")
	}

	// Generate new backup codes
	plain, hashes, err := GenerateBackupCodes(10)
	if err != nil {
		return nil, fmt.Errorf("failed to generate backup codes: %w", err)
	}

	// Replace existing codes inside a transaction
	err = s.repo.ReplaceBackupCodes(ctx, twofa.ID, hashes)
	if err != nil {
		return nil, fmt.Errorf("failed to replace backup codes: %w", err)
	}

	// Return plaintext codes to the user (only once!)
	return plain, nil
}


func (s *TwoFAService) GetBackupCodes(ctx context.Context, userID string, method string) ([]string, error) {
	// Fetch the 2FA record
	twofa, err := s.repo.GetTwoFA(ctx, userID, method)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch 2FA: %w", err)
	}
	if twofa == nil || !twofa.IsEnabled {
		return nil, errors.New("2FA not enabled for this method")
	}

	// Get stored backup code hashes
	hashes, err := s.repo.GetBackupCodeHashes(ctx, twofa.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch backup codes: %w", err)
	}

	// ⚠️ Instead of returning actual codes (impossible since we only store hashes),
	// we return masked identifiers (e.g., last 4 chars of the hash).
	masked := make([]string, 0, len(hashes))
	for _, h := range hashes {
		if len(h) > 8 {
			masked = append(masked, "****"+h[len(h)-4:])
		} else {
			masked = append(masked, "****")
		}
	}

	return masked, nil
}
