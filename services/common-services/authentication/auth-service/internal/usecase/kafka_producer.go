// usecase/producer.go
package usecase
import (
	"context"
)

type UserRegistrationMessage struct {
	UserID        string
	Email           *string
	Phone           *string
	Password        *string
	AccountType   string
	Consent         bool
	IsEmailVerified bool
	IsPhoneVerified bool
	RequestID       string
	RetryCount      int
	FailureReason   string
}

// Producer interface expected by usecase
type UserEventProducer interface {
	PublishRegistration(ctx context.Context, msg *UserRegistrationMessage) error
	PublishToDLQ(ctx context.Context, msg *UserRegistrationMessage) error
}