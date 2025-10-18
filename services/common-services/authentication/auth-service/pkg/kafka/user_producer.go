// pkg/kafka/user_producer.go
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"auth-service/internal/usecase"

	"github.com/IBM/sarama"
)

const (
	TopicUserRegistration = "user.registration"
	TopicUserRegistrationDLQ = "user.registration.dlq"
)

type UserRegistrationProducer struct {
	producer sarama.SyncProducer
}

func NewUserRegistrationProducer(brokers []string) (*UserRegistrationProducer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.RequiredAcks = sarama.WaitForAll // Wait for all replicas
	config.Producer.Retry.Max = 3
	config.Producer.Compression = sarama.CompressionSnappy
	config.Producer.Idempotent = true // Exactly-once semantics
	config.Net.MaxOpenRequests = 1

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	return &UserRegistrationProducer{producer: producer}, nil
}

// PublishRegistration sends a user registration message to Kafka
func (p *UserRegistrationProducer) PublishRegistration(ctx context.Context, msg *usecase.UserRegistrationMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	kafkaMsg := &sarama.ProducerMessage{
		Topic: TopicUserRegistration,
		Key:   sarama.StringEncoder(msg.RequestID), // Partition by request ID
		Value: sarama.ByteEncoder(data),
	}

	partition, offset, err := p.producer.SendMessage(kafkaMsg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	fmt.Printf("Message sent to partition %d at offset %d\n", partition, offset)
	return nil
}

// PublishToDLQ sends failed messages to the Dead Letter Queue
func (p *UserRegistrationProducer) PublishToDLQ(ctx context.Context, msg *usecase.UserRegistrationMessage) error {
	msg.RetryCount++
	
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal DLQ message: %w", err)
	}

	kafkaMsg := &sarama.ProducerMessage{
		Topic: TopicUserRegistrationDLQ,
		Key:   sarama.StringEncoder(msg.RequestID),
		Value: sarama.ByteEncoder(data),
	}

	partition, offset, err := p.producer.SendMessage(kafkaMsg)
	if err != nil {
		return fmt.Errorf("failed to send DLQ message: %w", err)
	}

	fmt.Printf("DLQ message sent to partition %d at offset %d\n", partition, offset)
	return nil
}

func (p *UserRegistrationProducer) Close() error {
	return p.producer.Close()
}