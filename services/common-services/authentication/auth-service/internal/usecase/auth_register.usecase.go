// usecase/auth_usecase.go
package usecase

import (
	"auth-service/internal/domain"
	"auth-service/pkg/utils"
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// RegisterUserAsyncBulk publishes multiple user registrations to Kafka
func (uc *UserUsecase) RegisterUserAsyncBulk(ctx context.Context, requests []RegisterUserRequest) ([]string, error) {
	if len(requests) == 0 {
		return nil, nil
	}

	requestIDs := make([]string, len(requests))

	for i, req := range requests {
		if (req.Email == nil || *req.Email == "") && (req.Phone == nil || *req.Phone == "") {
			return nil, errors.New("either email or phone is required")
		}

		// Generate request ID
		requestID := uuid.New().String()
		requestIDs[i] = requestID

		// Create Kafka message
		msg := &UserRegistrationMessage{
			UserID: req.UserID,
			Email:           req.Email,
			Phone:           req.Phone,
			Password:        req.Password,
			AccountType: req.AccountType,
			Consent:         req.Consent,
			IsEmailVerified: req.IsEmailVerified,
			IsPhoneVerified: req.IsPhoneVerified,
			RequestID:       requestID,
			RetryCount:      0,
		}

		// Publish to Kafka
		if err := uc.kafkaProducer.PublishRegistration(ctx, msg); err != nil {
			return nil, fmt.Errorf("failed to publish registration %d: %w", i, err)
		}
	}

	return requestIDs, nil
}


// RegisterUsers creates multiple users synchronously (used by Kafka consumer)
func (uc *UserUsecase) RegisterUsers(ctx context.Context, requests []RegisterUserRequest) ([]*domain.UserWithCredential, []error) {
	if len(requests) == 0 {
		return nil, nil
	}

	users := make([]*domain.User, 0, len(requests))
	credentials := make([]*domain.UserCredential, 0, len(requests))

	// Prepare all users and credentials
	for _, req := range requests {
		if (req.Email == nil || *req.Email == "") && (req.Phone == nil || *req.Phone == "") {
			return nil, []error{errors.New("either email or phone is required")}
		}

		// Generate IDs
		userID := req.UserID
		if userID == "" {
			userID = uc.Sf.Generate()
		}
		credID := uc.Sf.Generate()

		// Create user
		user := &domain.User{
			ID:              userID,
			AccountStatus:   "active",
			AccountType:     req.AccountType,
			AccountRestored: false,
			Consent:         req.Consent,
		}

		// Create credential
		var hashedPassword *string
		if req.Password != nil && *req.Password != "" {
			hashed, err := utils.HashPassword(*req.Password)
			if err != nil {
				return nil, []error{err}
			}
			hashedPassword = &hashed
		}

		credential := &domain.UserCredential{
			ID:              credID,
			UserID:          userID,
			Email:           req.Email,
			Phone:           req.Phone,
			PasswordHash:    hashedPassword,
			IsEmailVerified: req.IsEmailVerified,
			IsPhoneVerified: req.IsPhoneVerified,
			Valid:           true,
		}

		users = append(users, user)
		credentials = append(credentials, credential)
	}

	// Batch create
	savedUsers, errs := uc.userRepo.CreateUsers(ctx, users, credentials)
	if len(errs) > 0 {
		return nil, errs
	}

	// Combine users with their credentials
	result := make([]*domain.UserWithCredential, len(savedUsers))
	for i, user := range savedUsers {
		result[i] = &domain.UserWithCredential{
			User:       *user,
			Credential: *credentials[i],
		}
	}

	return result, nil
}

// RegisterUserRequest represents a single user registration request
type RegisterUserRequest struct {
	UserID		  string
	Email           *string
	Phone           *string
	Password        *string
	AccountType   string
	Consent         bool
	IsEmailVerified bool
	IsPhoneVerified bool
}

