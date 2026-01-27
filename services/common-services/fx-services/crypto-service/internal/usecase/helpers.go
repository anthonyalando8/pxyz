// internal/usecase/helpers.go
package usecase


// stringPtr returns pointer to string
func stringPtr(s string) *string {
	return &s
}

// int64Ptr returns pointer to int64
func int64Ptr(i int64) *int64 {
	return &i
}