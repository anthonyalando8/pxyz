// pkg/utils/p2p_helpers.go
package utils

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidateUsername checks if username is valid
func ValidateUsername(username string) error {
	if username == "" {
		return nil // Optional field
	}

	if len(username) < 3 {
		return fmt.Errorf("username must be at least 3 characters")
	}

	if len(username) > 30 {
		return fmt.Errorf("username must be at most 30 characters")
	}

	// Only alphanumeric and underscores
	matched, _ := regexp.MatchString("^[a-zA-Z0-9_]+$", username)
	if !matched {
		return fmt.Errorf("username can only contain letters, numbers, and underscores")
	}

	return nil
}

// ValidatePhoneNumber validates phone number format
func ValidatePhoneNumber(phone string) error {
	if phone == "" {
		return nil // Optional field
	}

	// Remove spaces and dashes
	cleaned := strings.ReplaceAll(strings.ReplaceAll(phone, " ", ""), "-", "")

	// Basic validation (can be enhanced)
	if len(cleaned) < 10 || len(cleaned) > 15 {
		return fmt.Errorf("invalid phone number length")
	}

	matched, _ := regexp.MatchString("^[+]?[0-9]+$", cleaned)
	if !matched {
		return fmt.Errorf("phone number can only contain numbers and optional +")
	}

	return nil
}

// ValidateEmail validates email format
func ValidateEmail(email string) error {
	if email == "" {
		return nil // Optional field
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}

	return nil
}

// CalculateCompletionRate calculates trade completion rate
func CalculateCompletionRate(completed, total int) float64 {
	if total == 0 {
		return 0
	}
	return (float64(completed) / float64(total)) * 100
}