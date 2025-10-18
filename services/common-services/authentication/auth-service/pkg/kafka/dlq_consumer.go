// pkg/kafka/dlq_consumer.go
package kafka

import (
	"auth-service/internal/usecase"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/IBM/sarama"
)

const (
	MaxRetries = 3
	RetryDelay = 5 * time.Second
)

type DLQConsumer struct {
	consumer    sarama.ConsumerGroup
	userUsecase *usecase.UserUsecase
	producer    *UserRegistrationProducer
}

func NewDLQConsumer(
	brokers []string,
	groupID string,
	userUsecase *usecase.UserUsecase,
	producer *UserRegistrationProducer,
) (*DLQConsumer, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	consumer, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create DLQ consumer: %w", err)
	}

	return &DLQConsumer{
		consumer:    consumer,
		userUsecase: userUsecase,
		producer:    producer,
	}, nil
}

func (c *DLQConsumer) Start(ctx context.Context) error {
    topics := []string{TopicUserRegistrationDLQ}
    handler := &dlqHandler{consumer: c}

    for {
        if err := c.consumer.Consume(ctx, topics, handler); err != nil {
            log.Printf("DLQ consumer error: %v", err)
        }

        if ctx.Err() != nil {
            log.Println("Context cancelled, shutting down DLQ consumer")
            return nil
        }
    }
}



func (c *DLQConsumer) Close() error {
	return c.consumer.Close()
}

type dlqHandler struct {
	consumer *DLQConsumer
}

func (h *dlqHandler) Setup(sarama.ConsumerGroupSession) error {
	log.Println("DLQ consumer session started")
	return nil
}

func (h *dlqHandler) Cleanup(sarama.ConsumerGroupSession) error {
	log.Println("DLQ consumer session ended")
	return nil
}

func (h *dlqHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		var msg usecase.UserRegistrationMessage
		if err := json.Unmarshal(message.Value, &msg); err != nil {
			log.Printf("Failed to unmarshal DLQ message: %v", err)
			session.MarkMessage(message, "")
			continue
		}

		log.Printf("Processing DLQ message: RequestID=%s, RetryCount=%d, Reason=%s",
			msg.RequestID, msg.RetryCount, msg.FailureReason)

		// Check if max retries exceeded
		if msg.RetryCount >= MaxRetries {
			log.Printf("Max retries exceeded for RequestID=%s, moving to permanent failure", msg.RequestID)
			// TODO: Store in permanent failure table or alert
			session.MarkMessage(message, "")
			continue
		}

		// Wait before retry
		time.Sleep(RetryDelay)

		// Retry creation
		ctx := context.Background()
		request := usecase.RegisterUserRequest{
			UserID: msg.UserID,
			Email:           msg.Email,
			Phone:           msg.Phone,
			Password:        msg.Password,
			AccountType:  msg.AccountType,
			Consent:         msg.Consent,
			IsEmailVerified: msg.IsEmailVerified,
			IsPhoneVerified: msg.IsPhoneVerified,
		}

		results, errs := h.consumer.userUsecase.RegisterUsers(ctx, []usecase.RegisterUserRequest{request})

		if len(errs) > 0 && errs[0] != nil {
			log.Printf("Retry failed for RequestID=%s: %v", msg.RequestID, errs[0])
			
			// Re-publish to DLQ with incremented retry count
			msg.FailureReason = errs[0].Error()
			if err := h.consumer.producer.PublishToDLQ(ctx, &msg); err != nil {
				log.Printf("Failed to re-publish to DLQ: %v", err)
			}
		} else {
			log.Printf("Retry successful for RequestID=%s, User ID=%s", msg.RequestID, results[0].User.ID)
		}

		session.MarkMessage(message, "")
	}

	return nil
}