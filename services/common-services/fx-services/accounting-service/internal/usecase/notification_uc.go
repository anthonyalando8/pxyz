package usecase

import (
	"context"
	"fmt"
	"sync"
	"time"

	notificationclient "x/shared/notification"
	notificationpb "x/shared/genproto/shared/notificationpb"
)

// ===============================
// NOTIFICATION BATCHER
// ===============================

type NotificationBatcher struct {
	client        *notificationclient.NotificationService
	batch         []*notificationpb.Notification
	batchSize     int
	flushInterval time.Duration
	mu            sync.Mutex
	stopChan      chan struct{}
}

func NewNotificationBatcher(client *notificationclient.NotificationService, batchSize int, interval time.Duration) *NotificationBatcher {
	return &NotificationBatcher{
		client:        client,
		batchSize:     batchSize,
		flushInterval: interval,
		stopChan:      make(chan struct{}),
	}
}

func (nb *NotificationBatcher) Start() {
	go nb.worker()
}

func (nb *NotificationBatcher) Stop() {
	close(nb.stopChan)
}

func (nb *NotificationBatcher) Add(notif *notificationpb.Notification) {
	nb.mu.Lock()
	nb.batch = append(nb.batch, notif)
	shouldFlush := len(nb.batch) >= nb.batchSize
	nb.mu.Unlock()

	if shouldFlush {
		nb.flush()
	}
}

func (nb *NotificationBatcher) worker() {
	ticker := time.NewTicker(nb.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			nb.flush()
		case <-nb.stopChan:
			return
		}
	}
}

func (nb *NotificationBatcher) flush() {
	nb.mu.Lock()
	if len(nb.batch) == 0 {
		nb.mu.Unlock()
		return
	}
	batch := nb.batch
	nb.batch = nil
	nb.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Call notification service with batch
	resp, err := nb.client.Client.CreateNotification(ctx, &notificationpb.CreateNotificationsRequest{
		Notifications: batch,
	})

	if err != nil {
		fmt.Printf("[NOTIFICATION BATCHER] Failed to send batch of %d notifications: %v\n", len(batch), err)
		return
	}

	// Log successful sends
	if resp != nil {
		successCount := len(resp.Notifications)
		failureCount := len(resp.Errors)
		
		fmt.Printf("[NOTIFICATION BATCHER] Sent %d/%d notifications successfully", successCount, len(batch))
		if failureCount > 0 {
			fmt.Printf(" (%d failures)\n", failureCount)
			
			// Log individual failures
			for _, err := range resp.Errors {
				fmt.Printf("[NOTIFICATION ERROR] Index: %d | Code: %s | Message: %s\n", 
					err.Index, err.ErrorCode, err.ErrorMessage)
			}
		} else {
			fmt.Println()
		}
	}
}