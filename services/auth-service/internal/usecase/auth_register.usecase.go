// usecase/auth_usecase.go
package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"auth-service/internal/domain"
	"auth-service/internal/repository"
	"auth-service/pkg/utils"
	"x/shared/utils/id"
)

type UserUsecase struct {
	userRepo *repository.UserRepository
	Sf       *id.Snowflake
}

func NewUserUsecase(userRepo *repository.UserRepository, sf *id.Snowflake) *UserUsecase {
	return &UserUsecase{
		userRepo: userRepo,
		Sf:       sf,
	}
}

func (uc *UserUsecase) RegisterUser(ctx context.Context, email, password, firstName, lastName, roleName string) (*domain.User, error) {
	if email == "" {
		return nil, errors.New("email required")
	}

	newUser := &domain.User{
		ID:        uc.Sf.Generate(),
		Email:     toPtr(email),
		LastName:  toPtr(lastName),
		FirstName: toPtr(firstName),
		AccountType: "password",
		AccountStatus: "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Set password if provided and update signup stage
	if password != "" {
		hashedPassword, err := utils.HashPassword(password)
		if err != nil {
			return nil, err
		}
		newUser.PasswordHash = &hashedPassword
		newUser.SignupStage = "password_set"
	} else {
		newUser.SignupStage = "email_or_phone_submitted"
	}

	// Save user
	createdUser, err := uc.userRepo.CreateUser(ctx, newUser)
	if err != nil {
		return nil, err
	}

	// Lookup predefined role
	var selectedRole *domain.Role
	for _, r := range domain.PredefinedRoles {
		if r.Name == roleName {
			selectedRole = &r
			break
		}
	}
	if selectedRole == nil {
		return createdUser, fmt.Errorf("invalid role: %s", roleName)
	}

	// Assign role asynchronously
	go func(userID string, role domain.Role) {
		bgCtx := context.Background()
		if err := uc.AssignRoleToUserHelper(bgCtx, userID, role); err != nil {
			fmt.Printf("failed to assign role %s to user %s: %v\n", role.Name, userID, err)
		}
	}(createdUser.ID, *selectedRole)

	return createdUser, nil
}




func (uc *UserUsecase) CreatePartialUser(ctx context.Context, email string) (*domain.User, error){
	newUser := &domain.User{
		ID:        uc.Sf.Generate(),
		Email:      toPtr(email),
	}

	// Save to database
	return uc.userRepo.CreateUser(ctx, newUser)
}

func (uc *UserUsecase) VerifyEmail(ctx context.Context, userID string) (bool, error){
	if err := uc.userRepo.VerifyEmail(ctx, userID); err != nil{
		return false, err
	}
	return true, nil
}

func (uc *UserUsecase) VerifyPhone(ctx context.Context, userID string) (bool, error){
	if err := uc.userRepo.VerifyPhone(ctx, userID); err != nil{
		return false, err
	}
	return true, nil
}