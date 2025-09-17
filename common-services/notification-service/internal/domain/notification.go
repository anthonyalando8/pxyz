package domain

import "time"

type Notification struct {
	ID            int64
	RequestID     string
	OwnerType     string // user, admin, partner
	OwnerID       string
	EventType     string
	ChannelHint   []string
	Title         string
	Body          string
	Payload       map[string]interface{}
	Priority      string
	Status        string
	VisibleInApp  bool        // NEW: should show up in app bell
	ReadAt        *time.Time  // NEW: null = unread, ts = when marked read
	CreatedAt     time.Time
	DeliveredAt   *time.Time
	Metadata      map[string]interface{}
}

type NotificationDelivery struct {
	ID             int64
	NotificationID int64
	Channel        string
	Recipient      string
	TemplateName   string
	Status         string
	AttemptCount   int
	LastAttemptAt  *time.Time
	LastError      *string
	CreatedAt      time.Time
}

type NotificationPreference struct {
	ID                 int64
	OwnerType          string
	OwnerID            string
	ChannelPreferences map[string]string   // {"email":"enabled","sms":"disabled"}
	QuietHours         []map[string]string // [{"start":"22:00","end":"07:00"}]
	CreatedAt          time.Time
}
