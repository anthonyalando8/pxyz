package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	emailclient "x/shared/email"
	"x/shared/genproto/emailpb"
	"x/shared/utils/id"

	"otp-service/internal/rate"
	"otp-service/internal/repository"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type OTPService struct {
	repo        *repository.OTPRepo
	limiter     *rate.Limiter
	sf          *id.Snowflake
	emailClient *emailclient.EmailClient
	ttl         time.Duration
}

func NewOTPService(
	repo *repository.OTPRepo,
	limiter *rate.Limiter,
	sf *id.Snowflake,
	emailClient *emailclient.EmailClient, // inject ready client
	ttl time.Duration,
) *OTPService {
	return &OTPService{
		repo:        repo,
		limiter:     limiter,
		sf:          sf,
		emailClient: emailClient,
		ttl:         ttl,
	}
}

func (s *OTPService) GenerateOTP(ctx context.Context, userID string, purpose, channel, recipient string) error {
	// rate limit check
	if err := s.limiter.CanRequest(ctx, userID); err != nil {
		return err
	}

	// generate 6-digit code
	code := randomCode(6)
	now := time.Now()
	o := &repository.OTP{
		ID:         s.sf.Generate(),
		UserID:     userID,
		Code:       code,
		Channel:    channel,
		Purpose:    purpose,
		IssuedAt:   now,
		ValidUntil: now.Add(s.ttl),
		IsVerified: false,
		IsActive:   true,
		UpdatedAt:  now,
	}

	if err := s.repo.Create(ctx, o); err != nil {
		return err
	}

	// Send email if needed
	if channel == "email" && s.emailClient != nil {
		subject := fmt.Sprintf("Your OTP code for %s", formatPurpose(purpose))
		ttlSeconds := int(s.ttl.Minutes())

		body := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<head>
		<meta charset="UTF-8">
		<title>OTP Code</title>
		</head>
		<body style="font-family: Arial, sans-serif; background-color: #f9f9f9; padding: 20px;">
		<div style="max-width: 600px; background-color: #ffffff; padding: 20px; border-radius: 8px; box-shadow: 0px 2px 5px rgba(0,0,0,0.1);">
			<h2 style="color: #333333;">Hello,</h2>
			<p style="font-size: 16px; color: #555555;">
			Your One-Time Password (OTP) is:
			</p>
			<p style="font-size: 20px; font-weight: bold; color: #2E86C1; background: #f1f1f1; padding: 10px; display: inline-block; border-radius: 5px;">
			%s
			</p>
			<p style="font-size: 14px; color: #777777;">
			This code is valid for the next <strong>%d</strong> minute(s).<br>
			Please do not share it with anyone.
			</p>
			<p style="font-size: 14px; color: #777777;">
			If you did not request this, please ignore this email.
			</p>
			<p style="margin-top: 30px; font-size: 14px; color: #999999;">
			Thank you,<br>
			<strong>Pxyz Security Team</strong>
			</p>
		</div>
		</body>
		</html>
		`, code, ttlSeconds)

		_, err := s.emailClient.SendEmail(ctx, &emailpb.SendEmailRequest{
			UserId:         userID,
			RecipientEmail: recipient,
			Subject:        subject,
			Body:           body,
			Type:           "otp",
		})
		if err != nil {
			// Don't block OTP persistence if email fails
			return fmt.Errorf("failed to send email: %w", err)
		}
	}

	return nil
}

func (s *OTPService) VerifyOTP(ctx context.Context, userID int64, purpose, code string) (bool, error) {
	return s.repo.VerifyAndInvalidate(ctx, userID, purpose, code)
}

func randomCode(digits int) string {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil) // 10^digits
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		panic(err) // handle appropriately in prod
	}
	return fmt.Sprintf("%0*d", digits, n.Int64())
}

func formatPurpose(purpose string) string {
    p := strings.ReplaceAll(purpose, "_", " ")
    return cases.Title(language.English).String(p)
}