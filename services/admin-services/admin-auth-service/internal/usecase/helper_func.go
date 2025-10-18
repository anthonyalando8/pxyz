package usecase

func toPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}