// user.go under services/auth-service/internal/domain
package domain


import "time"

type User struct {
	ID              string      `json:"id"`                             // Snowflake ID
	Email           *string    `json:"email,omitempty"`                // Nullable & unique
	Phone           *string    `json:"phone,omitempty"`                // Nullable & unique
	PasswordHash    *string    `json:"-"`                              // Omit from JSON responses
	FirstName       *string    `json:"first_name,omitempty"`          // Optional
	LastName        *string    `json:"last_name,omitempty"`           // Optional
	IsVerified      bool       `json:"is_verified"`
	AccountStatus   string     `json:"account_status"`                // 'active', 'deleted', 'suspended'
	HasPassword     bool       `json:"has_password"`
	AccountRestored bool       `json:"account_restored"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

