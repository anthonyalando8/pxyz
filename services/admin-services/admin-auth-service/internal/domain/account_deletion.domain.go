package domain

import "time"

// AccountDeletionRequest represents a request to delete a user account.
// It includes the reason for deletion, timestamps for when the request was made,
// and when it was processed, along with the ID of the admin or system that processed it.

type AccountDeletionRequest struct {
	ID            int64      `json:"id"`
	UserID        int64      `json:"user_id"`
	DeletionReason *string   `json:"deletion_reason,omitempty"`
	RequestedAt   time.Time  `json:"requested_at"`
	ProcessedAt   *time.Time `json:"processed_at,omitempty"`
	ProcessedBy   *int64     `json:"processed_by,omitempty"` // admin or system actor
}
