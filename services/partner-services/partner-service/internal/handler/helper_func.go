package handler

// StringPtr converts a string to a string pointer
// Returns nil if the string is empty
func StringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// StringValue converts a string pointer to a string
// Returns empty string if the pointer is nil
func StringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}