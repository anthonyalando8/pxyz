package utils

import (
	"errors"
	"regexp"
	"strings"
)

// ValidateEmail checks if an email address is valid.
func ValidateEmail(email string) bool {
	if strings.TrimSpace(email) == "" {
		return false
	}

	const emailRegexPattern = `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
	emailRegex := regexp.MustCompile(emailRegexPattern)

	return emailRegex.MatchString(email)
}

func ValidatePhone(phone string) bool { 
	e164Regex := regexp.MustCompile(`^\+?[1-9]\d{6,14}$`)
	if !e164Regex.MatchString(phone) {
		return false
	}
	return true
}
// ValidatePhoneWithCountry checks if a phone number is valid and matches the expected country code.
func ValidatePhoneWithCountry(phone string, countryPhoneCode string) bool {
	phone = strings.TrimSpace(phone)
	if phone == "" || countryPhoneCode == "" {
		return false
	}

	// E.164 format check
	e164Regex := regexp.MustCompile(`^\+?[1-9]\d{6,14}$`)
	if !e164Regex.MatchString(phone) {
		return false
	}

	// Ensure phone starts with the country phone code
	normalizedPhone := strings.TrimPrefix(phone, "+")
	normalizedCode := strings.TrimPrefix(countryPhoneCode, "+")
	if !strings.HasPrefix(normalizedPhone, normalizedCode) {
		return false
	}

	return true
}


func ValidatePassword(password string) (bool, error) {
	// Length check
	if len(password) < 8 {
		return false, errors.New("password must be at least 8 characters long")
	}
	if len(password) > 100 {
		return false, errors.New("password must not exceed 100 characters")
	}

	// Must contain at least one uppercase letter
	upper := regexp.MustCompile(`[A-Z]`)
	if !upper.MatchString(password) {
		return false, errors.New("password must include at least one uppercase letter")
	}

	// Must contain at least one lowercase letter
	lower := regexp.MustCompile(`[a-z]`)
	if !lower.MatchString(password) {
		return false, errors.New("password must include at least one lowercase letter")
	}

	// Must contain at least one digit
	digit := regexp.MustCompile(`[0-9]`)
	if !digit.MatchString(password) {
		return false, errors.New("password must include at least one digit")
	}

	// Must contain at least one special character
	special := regexp.MustCompile(`[!@#\$%\^&\*\(\)_\+\-=\[\]\{\}\\|;:'",.<>\/?]`)
	if !special.MatchString(password) {
		return false, errors.New("password must include at least one special character")
	}

	return true, nil
}

