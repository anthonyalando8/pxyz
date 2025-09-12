// user.go under services/admin-auth-service/internal/domain
package domain


import "time"

type User struct {
	ID               string     `json:"id"`                             // Snowflake ID
	Email            *string    `json:"email,omitempty"`                // Nullable & unique
	Phone            *string    `json:"phone,omitempty"`                // Nullable & unique
	PasswordHash     *string    `json:"-"`                               // Omit from JSON responses
	FirstName        *string    `json:"first_name,omitempty"`           // Optional
	LastName         *string    `json:"last_name,omitempty"`            // Optional
	IsEmailVerified  bool       `json:"is_email_verified"`
	IsPhoneVerified  bool       `json:"is_phone_verified"`
	IsTempPass       bool		`json:"is_temp_pass"`
	AccountStatus    string     `json:"account_status"`                  // 'active', 'deleted', 'suspended'
	AccountType      string     `json:"account_type"`                    // 'password', 'social', 'hybrid'
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

