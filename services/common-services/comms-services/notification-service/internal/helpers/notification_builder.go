package helpers

import (
	"context"
	"fmt"
	"log"
	"time"

	"notification-service/internal/domain"
)

// NotificationBuilder helps build notification messages with proper data enrichment
type NotificationBuilder struct {
	profileFetcher *ProfileFetcher
}

// NewNotificationBuilder creates a new notification builder
func NewNotificationBuilder(profileFetcher *ProfileFetcher) *NotificationBuilder {
	return &NotificationBuilder{
		profileFetcher: profileFetcher,
	}
}

// BuildMessage creates a domain.Message from a notification with enriched data
func (nb *NotificationBuilder) BuildMessage(ctx context.Context, n *domain.Notification) (*domain.Message, error) {
	if n == nil {
		return nil, fmt.Errorf("notification is nil")
	}

	log.Printf("[MESSAGE BUILD] Building message for notification ID: %d, OwnerID: %s, Type: %s", 
		n.ID, n.OwnerID, n.EventType)

	// Initialize template data with payload
	templateData := make(map[string]any)
	for k, v := range n.Payload {
		templateData[k] = v
	}

	// Initialize recipients map
	msgRecipients := make(map[string]string)

	// Start with provided recipient info
	if n.RecipientEmail != "" {
		msgRecipients["email"] = n.RecipientEmail
		log.Printf("[MESSAGE BUILD] Using provided email: %s", maskEmail(n.RecipientEmail))
	}
	if n.RecipientPhone != "" {
		msgRecipients["phone"] = n.RecipientPhone
		log.Printf("[MESSAGE BUILD] Using provided phone: %s", maskPhone(n.RecipientPhone))
	}
	if n.RecipientName != "" {
		templateData["UserName"] = n.RecipientName
		log.Printf("[MESSAGE BUILD] Using provided name: %s", n.RecipientName)
	}

	// Determine if we need to fetch profile
	needEmail := containsChannel(n.ChannelHint, "email") && n.RecipientEmail == ""
	needPhone := (containsChannel(n.ChannelHint, "sms") || containsChannel(n.ChannelHint, "whatsapp")) && n.RecipientPhone == ""

	if needEmail || needPhone {
		log.Printf("[MESSAGE BUILD] Need to fetch profile - Email: %v, Phone: %v", needEmail, needPhone)
		
		if err := nb.enrichWithProfile(ctx, n, msgRecipients, templateData); err != nil {
			log.Printf("[MESSAGE BUILD WARN] Failed to enrich with profile: %v", err)
			// Continue anyway - we'll send what we have
		}
	}

	// Add year for templates
	templateData["Year"] = time.Now().Year()

	// Pick primary recipient for backward compatibility
	recipient := nb.selectPrimaryRecipient(msgRecipients)

	// Build final message
	msg := &domain.Message{
		OwnerID:    n.OwnerID,
		OwnerType:  n.OwnerType,
		Recipient:  recipient,
		Recipients: msgRecipients,
		Title:      n.Title,
		Body:       n.Body,
		Metadata:   n.Metadata,
		Channels:   n.ChannelHint,
		Type:       domain.NotificationType(n.EventType),
		Data:       templateData,
		Ctx:        ctx,
	}

	log.Printf("[MESSAGE BUILD SUCCESS] Message built for notification ID: %d - Channels: %v, Recipient: %s", 
		n.ID, n.ChannelHint, maskEmail(recipient))

	return msg, nil
}

// enrichWithProfile fetches profile and enriches recipients and template data
func (nb *NotificationBuilder) enrichWithProfile(
	ctx context.Context,
	n *domain.Notification,
	msgRecipients map[string]string,
	templateData map[string]any,
) error {
	profile, err := nb.profileFetcher.FetchProfile(ctx, n.OwnerType, n.OwnerID)
	if err != nil {
		return fmt.Errorf("failed to fetch profile: %w", err)
	}

	// Enrich recipients
	if profile.Email != "" && msgRecipients["email"] == "" {
		msgRecipients["email"] = profile.Email
		log.Printf("[MESSAGE BUILD] Enriched with email from profile: %s", maskEmail(profile.Email))
	}
	if profile.Phone != "" && msgRecipients["phone"] == "" {
		msgRecipients["phone"] = profile.Phone
		log.Printf("[MESSAGE BUILD] Enriched with phone from profile: %s", maskPhone(profile.Phone))
	}

	// Enrich template data
	if templateData["UserName"] == nil || templateData["UserName"] == "" {
		fullName := buildFullName(profile.FirstName, profile.LastName)
		if fullName != "" {
			templateData["UserName"] = fullName
			log.Printf("[MESSAGE BUILD] Enriched with name from profile: %s", fullName)
		}
	}
	
	templateData["UserEmail"] = profile.Email
	templateData["UserPhone"] = profile.Phone

	// Update OwnerType if it was empty
	if n.OwnerType == "" && profile.OwnerType != "" {
		n.OwnerType = profile.OwnerType
		log.Printf("[MESSAGE BUILD] Resolved OwnerType: %s", profile.OwnerType)
	}

	return nil
}

// selectPrimaryRecipient chooses the primary recipient (email preferred)
func (nb *NotificationBuilder) selectPrimaryRecipient(msgRecipients map[string]string) string {
	if email, ok := msgRecipients["email"]; ok && email != "" {
		return email
	}
	if phone, ok := msgRecipients["phone"]; ok && phone != "" {
		return phone
	}
	return ""
}

// buildFullName creates a full name from first and last name
func buildFullName(firstName, lastName string) string {
	if firstName != "" && lastName != "" {
		return fmt.Sprintf("%s %s", firstName, lastName)
	}
	if firstName != "" {
		return firstName
	}
	if lastName != "" {
		return lastName
	}
	return ""
}

// containsChannel checks if a channel exists in the slice
func containsChannel(channels []string, target string) bool {
	for _, ch := range channels {
		if ch == target {
			return true
		}
	}
	return false
}