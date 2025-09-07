package domain

import (
	"time"

	structpb "google.golang.org/protobuf/types/known/structpb"
)

type UserPreferences struct {
	UserID      string                             `json:"user_id"`
	Preferences map[string]*structpb.Value        `json:"preferences"` // flexible key-value with proper types
	UpdatedAt   time.Time                         `json:"updated_at"`
}

func DefaultPreferences() map[string]*structpb.Value {
	return map[string]*structpb.Value{
		"dark_mode":              structpb.NewBoolValue(false),
		"dark_mode_emails":       structpb.NewBoolValue(false),
		"location_tracking":      structpb.NewBoolValue(false),
		"auto_login":             structpb.NewBoolValue(false),
		"marketing_emails":       structpb.NewBoolValue(false),
		"push_notifications":     structpb.NewBoolValue(true),
		"sms_notifications":      structpb.NewBoolValue(true),
		"chart_message_sound":    structpb.NewBoolValue(true),
		"notification_sound":     structpb.NewBoolValue(true),
	}
}
