package helpers

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	authclient "x/shared/auth"
	authpb "x/shared/genproto/authpb"
	adminauthpb "x/shared/genproto/admin/authpb"
	patnerauthpb "x/shared/genproto/partner/authpb"
)

// ProfileFetcher handles fetching user profiles from different auth services
type ProfileFetcher struct {
	authClient *authclient.AuthService
}

// Profile represents user profile data
type Profile struct {
	Email     string
	Phone     string
	BankAccount string
	FirstName string
	LastName  string
	OwnerType string // "user", "partner", or "admin"
}

// NewProfileFetcher creates a new profile fetcher
func NewProfileFetcher(authClient *authclient.AuthService) *ProfileFetcher {
	return &ProfileFetcher{
		authClient: authClient,
	}
}

// FetchProfile retrieves profile information based on ownerType and ownerID
func (pf *ProfileFetcher) FetchProfile(ctx context.Context, ownerType, ownerID string) (*Profile, error) {
	if pf.authClient == nil {
		return nil, fmt.Errorf("auth client not initialized")
	}

	if ownerID == "" {
		return nil, fmt.Errorf("ownerID is required")
	}

	// Add timeout to prevent hanging
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// If ownerType is known, fetch directly
	if ownerType != "" {
		return pf.fetchFromSpecificService(ctx, ownerType, ownerID)
	}

	// ownerType unknown → try all services concurrently
	return pf.fetchFromAllServices(ctx, ownerID)
}

// fetchFromSpecificService fetches profile from a specific service type
func (pf *ProfileFetcher) fetchFromSpecificService(ctx context.Context, ownerType, ownerID string) (*Profile, error) {
	startTime := time.Now()
	
	log.Printf("[PROFILE FETCH] Attempting to fetch profile for %s ID: %s", ownerType, ownerID)

	var profile *Profile
	var err error

	switch ownerType {
	case "user":
		profile, err = pf.fetchUserProfile(ctx, ownerID)
	case "partner":
		profile, err = pf.fetchPartnerProfile(ctx, ownerID)
	case "admin":
		profile, err = pf.fetchAdminProfile(ctx, ownerID)
	default:
		err = fmt.Errorf("unknown ownerType: %s", ownerType)
	}

	duration := time.Since(startTime)

	if err != nil {
		log.Printf("[PROFILE FETCH ERROR] Failed to fetch %s profile for ID %s (took %v): %v", 
			ownerType, ownerID, duration, err)
		return nil, err
	}

	if profile == nil {
		err = fmt.Errorf("profile not found for %s ID: %s", ownerType, ownerID)
		log.Printf("[PROFILE FETCH ERROR] %v (took %v)", err, duration)
		return nil, err
	}

	log.Printf("[PROFILE FETCH SUCCESS] Retrieved %s profile for ID %s (took %v) - Email: %s, Phone: %s", 
		ownerType, ownerID, duration, maskEmail(profile.Email), maskPhone(profile.Phone))

	return profile, nil
}

// fetchFromAllServices tries all auth services concurrently
func (pf *ProfileFetcher) fetchFromAllServices(ctx context.Context, ownerID string) (*Profile, error) {
	log.Printf("[PROFILE FETCH] OwnerType unknown for ID %s, trying all services concurrently", ownerID)
	startTime := time.Now()

	type fetchResult struct {
		profile *Profile
		err     error
		service string
	}

	services := []struct {
		name string
		fn   func(context.Context, string) (*Profile, error)
	}{
		{"user", pf.fetchUserProfile},
		{"partner", pf.fetchPartnerProfile},
		{"admin", pf.fetchAdminProfile},
	}

	resultChan := make(chan fetchResult, len(services))
	var wg sync.WaitGroup

	// Launch concurrent fetches
	for _, svc := range services {
		wg.Add(1)
		go func(name string, fetchFn func(context.Context, string) (*Profile, error)) {
			defer wg.Done()
			
			profile, err := fetchFn(ctx, ownerID)
			resultChan <- fetchResult{
				profile: profile,
				err:     err,
				service: name,
			}
		}(svc.name, svc.fn)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var errors []string
	for result := range resultChan {
		if result.err == nil && result.profile != nil {
			duration := time.Since(startTime)
			log.Printf("[PROFILE FETCH SUCCESS] Found profile in %s service for ID %s (took %v)", 
				result.service, ownerID, duration)
			return result.profile, nil
		}
		if result.err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", result.service, result.err))
		}
	}

	duration := time.Since(startTime)
	combinedErr := fmt.Sprintf("profile not found in any service for ID %s (took %v). Errors: %v", 
		ownerID, duration, errors)
	
	log.Printf("[PROFILE FETCH ERROR] %s", combinedErr)
	return nil, fmt.Errorf("%s", combinedErr)
}

// fetchUserProfile fetches from user auth service
func (pf *ProfileFetcher) fetchUserProfile(ctx context.Context, ownerID string) (*Profile, error) {
	if pf.authClient.UserClient == nil {
		return nil, fmt.Errorf("user auth client not initialized")
	}

	resp, err := pf.authClient.UserClient.GetUserProfile(ctx, &authpb.GetUserProfileRequest{
		UserId: ownerID,
	})

	if err != nil {
		return nil, fmt.Errorf("user service error: %w", err)
	}

	if resp == nil || !resp.Ok {
		return nil, fmt.Errorf("user service returned invalid response")
	}

	if resp.User == nil {
		return nil, fmt.Errorf("user not found")
	}

	return &Profile{
		Email:     resp.User.Email,
		Phone:     resp.User.Phone,
		FirstName: resp.User.FirstName,
		LastName:  resp.User.LastName,
		OwnerType: "user",
	}, nil
}

// fetchPartnerProfile fetches from partner auth service
func (pf *ProfileFetcher) fetchPartnerProfile(ctx context.Context, ownerID string) (*Profile, error) {
	if pf.authClient.PartnerClient == nil {
		return nil, fmt.Errorf("partner auth client not initialized")
	}

	resp, err := pf.authClient.PartnerClient.GetUserProfile(ctx, &patnerauthpb.GetUserProfileRequest{
		UserId: ownerID,
	})

	if err != nil {
		return nil, fmt.Errorf("partner service error: %w", err)
	}

	if resp == nil || !resp.Ok {
		return nil, fmt.Errorf("partner service returned invalid response")
	}

	if resp.User == nil {
		return nil, fmt.Errorf("partner not found")
	}

	return &Profile{
		Email:     resp.User.Email,
		Phone:     resp.User.Phone,
		FirstName: resp.User.FirstName,
		LastName:  resp.User.LastName,
		OwnerType: "partner",
	}, nil
}

// fetchAdminProfile fetches from admin auth service
func (pf *ProfileFetcher) fetchAdminProfile(ctx context.Context, ownerID string) (*Profile, error) {
	if pf.authClient.AdminClient == nil {
		return nil, fmt.Errorf("admin auth client not initialized")
	}

	resp, err := pf.authClient.AdminClient.GetUserProfile(ctx, &adminauthpb.GetUserProfileRequest{
		UserId: ownerID,
	})

	if err != nil {
		return nil, fmt.Errorf("admin service error: %w", err)
	}

	if resp == nil || !resp.Ok {
		return nil, fmt.Errorf("admin service returned invalid response")
	}

	if resp.User == nil {
		return nil, fmt.Errorf("admin not found")
	}

	return &Profile{
		Email:     resp.User.Email,
		Phone:     resp.User.Phone,
		FirstName: resp.User.FirstName,
		LastName:  resp.User.LastName,
		OwnerType: "admin",
	}, nil
}

// maskEmail masks email for logging privacy
func maskEmail(email string) string {
	if email == "" {
		return "[empty]"
	}
	if len(email) < 5 {
		return "***@***"
	}
	// Show first 2 chars and domain
	parts := splitEmail(email)
	if len(parts) != 2 {
		return "***@***"
	}
	local := parts[0]
	domain := parts[1]
	
	if len(local) <= 2 {
		return "**@" + domain
	}
	return local[:2] + "***@" + domain
}

// maskPhone masks phone for logging privacy
func maskPhone(phone string) string {
	if phone == "" {
		return "[empty]"
	}
	if len(phone) < 4 {
		return "***"
	}
	// Show last 4 digits
	return "***" + phone[len(phone)-4:]
}

// splitEmail splits email into local and domain parts
func splitEmail(email string) []string {
	for i := 0; i < len(email); i++ {
		if email[i] == '@' {
			return []string{email[:i], email[i+1:]}
		}
	}
	return []string{email}
}