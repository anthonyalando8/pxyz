package handler

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

