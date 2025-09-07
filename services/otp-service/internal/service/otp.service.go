package service

import (
	"context"
	"fmt"

	"time"
	"log"

	emailclient "x/shared/email"
	smsclient "x/shared/sms"
	"x/shared/genproto/emailpb"
	"x/shared/genproto/smswhatsapppb"
	"x/shared/utils/id"

	"otp-service/internal/rate"
	"otp-service/internal/repository"


)

type OTPService struct {
	repo        *repository.OTPRepo
	limiter     *rate.Limiter
	sf          *id.Snowflake
	emailClient *emailclient.EmailClient
	smsClient   *smsclient.SMSClient
	ttl         time.Duration
}

func NewOTPService(
	repo *repository.OTPRepo,
	limiter *rate.Limiter,
	sf *id.Snowflake,
	emailClient *emailclient.EmailClient, // inject ready client
	smsClient *smsclient.SMSClient,
	ttl time.Duration,
) *OTPService {
	return &OTPService{
		repo:        repo,
		limiter:     limiter,
		sf:          sf,
		emailClient: emailClient,
		smsClient:   smsClient,
		ttl:         ttl,
	}
}

func (s *OTPService) GenerateOTP(ctx context.Context, userID string, purpose, channel, recipient string) error {
	// 1. Rate-limit
	if err := s.limiter.CanRequest(ctx, userID); err != nil {
		return err
	}

	// 2. Create OTP entity
	otp, err := s.createOTP(userID, purpose, channel)
	if err != nil {
		return err
	}

	// 3. Persist OTP
	if purpose == "sys_test" {
		log.Printf("Generated test OTP | UserID=%s | Code=%s | Purpose=%s | Channel=%s", userID, otp.Code, purpose, channel)
	}else{
		if err := s.repo.Create(ctx, otp); err != nil {
			return err
		}
	}
	
	

	// 4. Send via correct channel
	switch channel {
	case "email":
		return s.sendEmailOTP(ctx, userID, recipient, purpose, otp.Code)
	case "sms", "whatsapp":
		return s.sendSMSOrWhatsAppOTP(ctx, userID, recipient, purpose, otp.Code, channel)
	default:
		return fmt.Errorf("unsupported channel: %s", channel)
	}
}
func (s *OTPService) createOTP(userID, purpose, channel string) (*repository.OTP, error) {
	code := randomCode(6)
	now := time.Now()

	return &repository.OTP{
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
	}, nil
}


func (s *OTPService) sendEmailOTP(ctx context.Context, userID, recipient, purpose, code string) error {
	if s.emailClient == nil {
		return fmt.Errorf("email client not configured")
	}

	subject := fmt.Sprintf("Your OTP code for %s", formatPurpose(purpose))
	ttlMinutes := int(s.ttl.Minutes())

	body := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head><meta charset="UTF-8"><title>OTP Code</title></head>
	<body style="font-family: Arial, sans-serif; background-color: #f9f9f9; padding: 20px;">
		<div style="max-width: 600px; background-color: #ffffff; padding: 20px; border-radius: 8px; box-shadow: 0px 2px 5px rgba(0,0,0,0.1);">
			<h2>Hello,</h2>
			<p>Your One-Time Password (OTP) is:</p>
			<p style="font-size: 20px; font-weight: bold; color: #2E86C1; background: #f1f1f1; padding: 10px; border-radius: 5px;">%s</p>
			<p>This code is valid for the next <strong>%d</strong> minute(s). Please do not share it.</p>
			<p>If you did not request this, please ignore this email.</p>
			<p style="margin-top: 30px; font-size: 14px; color: #999999;">Thank you,<br><strong>Pxyz Security Team</strong></p>
		</div>
	</body>
	</html>
	`, code, ttlMinutes)

	_, err := s.emailClient.SendEmail(ctx, &emailpb.SendEmailRequest{
		UserId:         userID,
		RecipientEmail: recipient,
		Subject:        subject,
		Body:           body,
		Type:           "otp",
	})
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("Successfully sent OTP via email | Recipient=%s | TTL=%dm", recipient, ttlMinutes)
	return nil
}

func (s *OTPService) sendSMSOrWhatsAppOTP(ctx context.Context, userID, recipient, purpose, code, channel string) error {
	if s.smsClient == nil {
		return fmt.Errorf("%s client not configured", channel)
	}

	body := s.formatOTPMessage(purpose, code)

	var ch smswhatsapppb.Channel
	if channel == "sms" {
		ch = smswhatsapppb.Channel_SMS
	} else {
		ch = smswhatsapppb.Channel_WHATSAPP
	}

	_, err := s.smsClient.SendMessage(ctx, &smswhatsapppb.SendMessageRequest{
		UserId:    userID,
		Recipient: recipient,
		Body:      body,
		Channel:   ch,
		Type:      "otp",
	})
	if err != nil {
		log.Printf("failed to send %s: %v", channel, err)
		return fmt.Errorf("failed to send %s: %w", channel, err)
	}

	log.Printf("Successfully sent OTP via %s | Recipient=%s | TTL=%dm | Purpose=%s",
		channel, recipient, int(s.ttl.Minutes()), purpose)
	return nil
}



func (s *OTPService) VerifyOTP(ctx context.Context, userID int64, purpose, code string) (bool, error) {
	return s.repo.VerifyAndInvalidate(ctx, userID, purpose, code)
}


func (s *OTPService) formatOTPMessage(purpose, code string) string {
	ttlMinutes := int(s.ttl.Minutes())

	// Special case: system test purpose
	if purpose == "sys_test" {
		return fmt.Sprintf(
			"This is a test OTP. Your code is %s and it is valid for %d minutes.",
			code, ttlMinutes,
		)
	}

	// Default message
	return fmt.Sprintf(
		"Your OTP code for %s is %s. It is valid for %d minutes.",
		formatPurpose(purpose), code, ttlMinutes,
	)
}
