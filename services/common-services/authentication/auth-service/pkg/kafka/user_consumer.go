// pkg/kafka/user_consumer.go
package kafka

import (
	"auth-service/internal/usecase"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/IBM/sarama"
)

const (
	MaxBatchSize        = 500               // Flush at 1000 messages
	MaxBatchTimeout     = 5 * time.Second    // Flush after 5 seconds
	IdleTimeout         = 500 * time.Millisecond // Flush after 500ms of inactivity
)

type UserRegistrationConsumer struct {
	consumer      sarama.ConsumerGroup
	userUsecase   *usecase.UserUsecase
	producer      *UserRegistrationProducer
	
	// Batching state
	batch         []*usecase.UserRegistrationMessage
	batchMutex    sync.Mutex
	lastMessageAt time.Time
	
	// Timers
	batchTimer    *time.Timer
	idleTimer     *time.Timer
	
	// Channels
	flushChan     chan struct{}
	doneChan      chan struct{}
}

func NewUserRegistrationConsumer(
	brokers []string,
	groupID string,
	userUsecase *usecase.UserUsecase,
	producer *UserRegistrationProducer,
) (*UserRegistrationConsumer, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Group.Session.Timeout = 20 * time.Second
	config.Consumer.Group.Heartbeat.Interval = 6 * time.Second
	config.Consumer.MaxProcessingTime = 30 * time.Second

	consumer, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	c := &UserRegistrationConsumer{
		consumer:      consumer,
		userUsecase:   userUsecase,
		producer:      producer,
		batch:         make([]*usecase.UserRegistrationMessage, 0, MaxBatchSize),
		lastMessageAt: time.Now(),
		flushChan:     make(chan struct{}, 1),
		doneChan:      make(chan struct{}),
	}

	// Initialize timers
	c.batchTimer = time.NewTimer(MaxBatchTimeout)
	c.idleTimer = time.NewTimer(IdleTimeout)

	return c, nil
}

func (c *UserRegistrationConsumer) Start(ctx context.Context) error {
    topics := []string{TopicUserRegistration}
    go c.flushManager(ctx)
    handler := &consumerGroupHandler{consumer: c}

    for {
        if err := c.consumer.Consume(ctx, topics, handler); err != nil {
            log.Printf("Error from consumer: %v", err)
        }

        // if ctx is cancelled, exit cleanly
        if ctx.Err() != nil {
            log.Println("Context cancelled, shutting down consumer")
            return nil
        }
    }
}

// flushManager handles automatic batch flushing based on timers
func (c *UserRegistrationConsumer) flushManager(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// Final flush on shutdown
			c.flushBatch(ctx)
			close(c.doneChan)
			return
			
		case <-c.batchTimer.C:
			// Max timeout reached (5 seconds)
			c.batchMutex.Lock()
			hasMessages := len(c.batch) > 0
			c.batchMutex.Unlock()

			if hasMessages {
				log.Println("Batch timeout reached (5s), flushing...")
				c.flushBatch(ctx)
			}
			c.batchTimer.Reset(MaxBatchTimeout)
			
		case <-c.idleTimer.C:
			// Idle timeout reached (500ms)
			c.batchMutex.Lock()
			timeSinceLastMsg := time.Since(c.lastMessageAt)
			batchSize := len(c.batch)
			c.batchMutex.Unlock()
			
			if timeSinceLastMsg >= IdleTimeout && batchSize > 0 {
				log.Printf("Idle timeout reached (500ms), flushing %d messages...", batchSize)
				c.flushBatch(ctx)
			}
			c.idleTimer.Reset(IdleTimeout)
			
		case <-c.flushChan:
			// Manual flush trigger (when batch size reaches 1000)
			log.Println("Batch size reached 1000, flushing...")
			c.flushBatch(ctx)
		}
	}
}

// addToBatch adds a message to the batch and triggers flush if needed
func (c *UserRegistrationConsumer) addToBatch(msg *usecase.UserRegistrationMessage) {
	c.batchMutex.Lock()
	defer c.batchMutex.Unlock()
	
	c.batch = append(c.batch, msg)
	c.lastMessageAt = time.Now()
	
	// Reset idle timer since we got a new message
	c.idleTimer.Reset(IdleTimeout)
	
	// Check if we've reached max batch size
	if len(c.batch) >= MaxBatchSize {
		// Trigger flush
		select {
		case c.flushChan <- struct{}{}:
		default:
			// Channel already has a flush signal
		}
	}
}

// flushBatch processes the accumulated batch
func (c *UserRegistrationConsumer) flushBatch(ctx context.Context) {
	c.batchMutex.Lock()
	if len(c.batch) == 0 {
		c.batchMutex.Unlock()
		return
	}
	
	// Take ownership of the batch
	currentBatch := c.batch
	c.batch = make([]*usecase.UserRegistrationMessage, 0, MaxBatchSize)
	c.batchMutex.Unlock()
	
	log.Printf("Flushing batch of %d messages", len(currentBatch))
	
	// Convert to RegisterUserRequest
	requests := make([]usecase.RegisterUserRequest, len(currentBatch))
	messageMap := make(map[int]*usecase.UserRegistrationMessage) // For DLQ tracking
	
	for i, msg := range currentBatch {
		requests[i] = usecase.RegisterUserRequest{
			UserID: msg.UserID,
			Email:           msg.Email,
			Phone:           msg.Phone,
			Password:        msg.Password,
			AccountType:  msg.AccountType,
			Consent:         msg.Consent,
			IsEmailVerified: msg.IsEmailVerified,
			IsPhoneVerified: msg.IsPhoneVerified,
		}
		messageMap[i] = msg
	}
	
	// Batch create users
	results, errs := c.userUsecase.RegisterUsers(ctx, requests)
	
	// Handle results and errors
	successCount := 0
	failureCount := 0
	
	for i, err := range errs {
		if err != nil {
			failureCount++
			log.Printf("Failed to create user %d: %v", i, err)
			
			// Send to DLQ
			originalMsg := messageMap[i]
			originalMsg.FailureReason = err.Error()
			
			if dlqErr := c.producer.PublishToDLQ(ctx, originalMsg); dlqErr != nil {
				log.Printf("Failed to publish to DLQ: %v", dlqErr)
			}
		} else {
			successCount++
		}
	}
	
	log.Printf("Batch processing complete: %d success, %d failures", successCount, failureCount)
	
	// Log created users
	for _, result := range results {
		if result != nil {
			log.Printf("Created user: ID=%s, Email=%v, Phone=%v",
				result.User.ID,
				result.Credential.Email,
				result.Credential.Phone,
			)
		}
	}
}

func (c *UserRegistrationConsumer) Close() error {
	// Wait for final flush
	<-c.doneChan
	return c.consumer.Close()
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler
type consumerGroupHandler struct {
	consumer *UserRegistrationConsumer
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	log.Println("Consumer group session started")
	return nil
}

func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	log.Println("Consumer group session ended")
	return nil
}

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		var msg usecase.UserRegistrationMessage
		if err := json.Unmarshal(message.Value, &msg); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			session.MarkMessage(message, "")
			continue
		}
		
		log.Printf("Received message: RequestID=%s, Email=%v, Phone=%v",
			msg.RequestID, msg.Email, msg.Phone)
		
		// Add to batch
		h.consumer.addToBatch(&msg)
		
		// Mark message as consumed
		session.MarkMessage(message, "")
	}
	
	return nil
}