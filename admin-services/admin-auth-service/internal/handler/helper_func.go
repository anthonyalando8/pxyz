package handler

import "strings"

func toPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func safeString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

// maskEmail masks email addresses like a***g@gmail.com
func maskEmail(email string) string {
	atIdx := strings.Index(email, "@")
	if atIdx <= 1 {
		return "***" // not a valid email, return masked
	}
	return email[:1] + "***" + email[atIdx-1:]
}

// maskPhone masks phone numbers like +2547****89
func maskPhone(phone string) string {
	if len(phone) < 6 {
		return "****"
	}
	return phone[:5] + "****" + phone[len(phone)-2:]
}