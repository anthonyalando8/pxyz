package service

import (
	"context"
	"fmt"

	"time"
	"log"

	emailclient "x/shared/email"
	smsclient "x/shared/sms"
	notificationclient "x/shared/notification" // ✅ added

	notificationpb "x/shared/genproto/shared/notificationpb"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
	"x/shared/utils/id"

	"otp-service/internal/rate"
	"otp-service/internal/repository"
	"github.com/redis/go-redis/v9"

	"x/shared/utils/cache"


)

type OTPService struct {
	repo        *repository.OTPRepo
	limiter     *rate.Limiter
	sf          *id.Snowflake
	emailClient *emailclient.EmailClient
	smsClient   *smsclient.SMSClient
	cache *cache.Cache

	notificationClient *notificationclient.NotificationService // ✅ added

	ttl         time.Duration
}

func NewOTPService(
	repo *repository.OTPRepo,
	limiter *rate.Limiter,
	sf *id.Snowflake,
	emailClient *emailclient.EmailClient, // inject ready client
	smsClient *smsclient.SMSClient,
		cache *cache.Cache,

	ttl time.Duration,
	notificationClient *notificationclient.NotificationService,

) *OTPService {
	return &OTPService{
		repo:        repo,
		limiter:     limiter,
		sf:          sf,
		emailClient: emailClient,
		smsClient:   smsClient,
		cache:   cache,
		ttl:         ttl,
		notificationClient: notificationClient,
	}
}

func (s *OTPService) GenerateOTP(ctx context.Context, userID string, purpose, channel, recipient string) error {
	// 1. Rate-limit
	if err := s.limiter.CanRequest(ctx, userID, purpose); err != nil {
		return err
	}

	// 2. Create OTP entity
	otp, err := s.createOTP(userID, purpose, channel)
	if err != nil {
		return err
	}

	// 3. Save to Redis (live verification)
	key := fmt.Sprintf("%s:%s", userID, purpose)
	err = s.cache.Set(ctx, "otp", key, otp.Code, s.ttl)
	if err != nil {
		return err
	}
	log.Printf("Stored OTP in Redis | Key=%s | Code=%s | ExpiresIn=%s", key, otp.Code, s.ttl.String())

	// 4. Log to DB (audit)
	if purpose != "sys_test" {
		go func() { // async so we don’t block
			if err := s.repo.Create(context.Background(), otp); err != nil {
				log.Printf("Failed to insert OTP log: %v", err)
			}
		}()
	} else {
		log.Printf("Generated test OTP | UserID=%s | Code=%s | Purpose=%s | Channel=%s", userID, otp.Code, purpose, channel)
	}

	// 5. Send via correct channel
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

func (s *OTPService) VerifyOTP(ctx context.Context, userID int64, purpose, code string) (bool, error) {
	key := fmt.Sprintf("%d:%s", userID, purpose)

	// 1. Get OTP from Redis
	val, err := s.cache.Get(ctx,"otp", key)
	if err == redis.Nil {
		log.Printf("OTP not found or expired | UserID=%d | Purpose=%s | Key=%s | Code=%s", userID, purpose, key, code)
		return false, nil // not found (expired or never existed)
	} else if err != nil {
		log.Printf("Failed to get OTP from Redis: %v", err)
		return false, err
	}

	// 2. Compare
	if val != code {
		log.Printf("OTP verification failed | UserID=%d | Purpose=%s | Provided=%s | Expected=%s", userID, purpose, code, val)
		return false, nil
	}
	log.Printf("OTP verified successfully | UserID=%d | Purpose=%s", userID, purpose)

	// 3. Invalidate in Redis
	if err := s.cache.Delete(ctx,"otp", key); err != nil {
		log.Printf("Failed to delete OTP from Redis: %v", err)
	}

	// 4. Async update DB (mark verified)
	go func() {
		_, err := s.repo.VerifyAndInvalidate(context.Background(), userID, purpose, code)
		if err != nil {
			log.Printf("DB OTP verify update failed: %v", err)
		}
	}()

	return true, nil
}


func (s *OTPService) sendEmailOTP(_ context.Context, userID, recipient, purpose, code string) error {
	if s.notificationClient == nil {
		return fmt.Errorf("notification client not configured")
	}

	ttlMinutes := int(s.ttl.Minutes())
	userName := ""

	payload := map[string]interface{}{
		"UserName":      userName,
		"OTP":           code,
		"Purpose":       formatPurpose(purpose),
		"ExpiryMinutes": ttlMinutes,
	}

	// Send asynchronously in background
	go func() {
		bgCtx := context.Background() // background context for async processing

		_, err := s.notificationClient.Client.CreateNotification(bgCtx, &notificationpb.CreateNotificationsRequest{
			Notifications: []*notificationpb.Notification{
				{
					RequestId:      uuid.New().String(),
					OwnerType:      "",
					OwnerId:        userID,
					EventType:      "OTP",
					Title:          "OTP Code",
					Body:           s.formatOTPMessage(purpose, code),
					ChannelHint:    []string{"email"},
					Payload: func() *structpb.Struct {
						s, _ := structpb.NewStruct(payload)
						return s
					}(),
					VisibleInApp:   false,
					RecipientEmail: recipient,
					Priority:       "high",
					Status:         "pending",
				},
			},
		})

		if err != nil {
			log.Printf("[WARN] failed to send OTP email to %s: %v", recipient, err)
		} else {
			log.Printf("Successfully queued OTP notification | Recipient=%s | TTL=%dm", recipient, ttlMinutes)
		}
	}()


	return nil
}

func (s *OTPService) sendSMSOrWhatsAppOTP(_ context.Context, userID, recipient, purpose, code, channel string) error {
	if s.notificationClient == nil {
		return fmt.Errorf("%s client not configured", channel)
	}

	ttlMinutes := int(s.ttl.Minutes())

	payload := map[string]interface{}{
		"OTP":           code,
		"Purpose":       formatPurpose(purpose),
		"ExpiryMinutes": ttlMinutes,
	}

	eventType := ""
	switch channel {
	case "sms":
		eventType = "OTP"
	case "whatsapp":
		eventType = "OTP"
	default:
		return fmt.Errorf("unsupported channel: %s", channel)
	}

	// Send asynchronously in background
	go func() {
		bgCtx := context.Background() // background context for async processing

		_, err := s.notificationClient.Client.CreateNotification(bgCtx, &notificationpb.CreateNotificationsRequest{
			Notifications: []*notificationpb.Notification{
				{
					RequestId:      uuid.New().String(),
					OwnerType:      "user",
					OwnerId:        userID,
					EventType:      eventType,
					Title:          "OTP Code",
					Body:           s.formatOTPMessage(purpose, code),
					ChannelHint:    []string{channel},
					Payload: func() *structpb.Struct {
						s, _ := structpb.NewStruct(payload)
						return s
					}(),
					VisibleInApp:   false,
					RecipientName:  "", // Optional: fill if you have user name
					RecipientPhone: recipient,
					Priority:       "high",
					Status:         "pending",
				},
			},
		})

		if err != nil {
			log.Printf("[WARN] failed to send OTP via %s to %s: %v", channel, recipient, err)
		} else {
			log.Printf("Successfully queued OTP notification via %s | Recipient=%s | TTL=%dm | Purpose=%s",
				channel, recipient, ttlMinutes, purpose)
		}
	}()


	return nil
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
