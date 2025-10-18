// user.go under services/admin-auth-service/internal/domain
package domain


import "time"

type User struct {
	ID         string `json:"id"`
	Email      *string `json:"email,omitempty"`
	Phone      *string `json:"phone,omitempty"`
	Password   string `json:"-"` // omit from JSON responses
	FirstName  string `json:"first_name,omitempty"`
	LastName   string `json:"last_name,omitempty"`
	IsBanned   bool   `json:"is_banned"`
	IsVerified bool   `json:"is_verified"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
} 

