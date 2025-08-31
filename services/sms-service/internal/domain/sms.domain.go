package domain

type Message struct {
    UserID    string
    Recipient string
    Body      string
    Channel   string // "SMS" or "WHATSAPP"
    Type      string // otp, alert, etc.
}
