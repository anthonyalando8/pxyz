// security_audit_kafka.go
package kafka

import (
	"audit-service/internal/service/audit"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/IBM/sarama"
)

// ================================
// KAFKA TOPICS
// ================================

const (
	TopicAuthenticationEvents = "auth.events.authentication"
	TopicAccountEvents        = "auth.events.account"
	TopicSecurityEvents       = "auth.events.security"
	TopicAdminEvents          = "auth.events.admin"
	TopicFailedLogins         = "auth.events.failed_login"
)

// ================================
// EVENT MESSAGES
// ================================

type AuthenticationEventMessage struct {
	EventType    string                 `json:"event_type"`
	UserID       *string                `json:"user_id,omitempty"`
	Status       string                 `json:"status"`
	SessionID    *string                `json:"session_id,omitempty"`
	IPAddress    *string                `json:"ip_address,omitempty"`
	UserAgent    *string                `json:"user_agent,omitempty"`
	RequestID    *string                `json:"request_id,omitempty"`
	Description  *string                `json:"description,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	ErrorCode    *string                `json:"error_code,omitempty"`
	ErrorMessage *string                `json:"error_message,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
}

type AccountEventMessage struct {
	EventType     string                 `json:"event_type"`
	UserID        string                 `json:"user_id"`
	TargetUserID  *string                `json:"target_user_id,omitempty"`
	SessionID     *string                `json:"session_id,omitempty"`
	IPAddress     *string                `json:"ip_address,omitempty"`
	UserAgent     *string                `json:"user_agent,omitempty"`
	RequestID     *string                `json:"request_id,omitempty"`
	Action        *string                `json:"action,omitempty"`
	Description   *string                `json:"description,omitempty"`
	PreviousValue map[string]interface{} `json:"previous_value,omitempty"`
	NewValue      map[string]interface{} `json:"new_value,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
}

type SecurityEventMessage struct {
	EventType   string                 `json:"event_type"`
	UserID      *string                `json:"user_id,omitempty"`
	Severity    string                 `json:"severity"`
	SessionID   *string                `json:"session_id,omitempty"`
	IPAddress   *string                `json:"ip_address,omitempty"`
	UserAgent   *string                `json:"user_agent,omitempty"`
	RequestID   *string                `json:"request_id,omitempty"`
	Description *string                `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

type AdminEventMessage struct {
	AdminUserID   string                 `json:"admin_user_id"`
	TargetUserID  string                 `json:"target_user_id"`
	Action        string                 `json:"action"`
	SessionID     *string                `json:"session_id,omitempty"`
	IPAddress     *string                `json:"ip_address,omitempty"`
	UserAgent     *string                `json:"user_agent,omitempty"`
	RequestID     *string                `json:"request_id,omitempty"`
	Description   *string                `json:"description,omitempty"`
	PreviousValue map[string]interface{} `json:"previous_value,omitempty"`
	NewValue      map[string]interface{} `json:"new_value,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
}

type FailedLoginMessage struct {
	Identifier     string    `json:"identifier"`
	IdentifierType string    `json:"identifier_type"`
	IPAddress      string    `json:"ip_address"`
	UserAgent      *string   `json:"user_agent,omitempty"`
	FailureReason  *string   `json:"failure_reason,omitempty"`
	RequestID      *string   `json:"request_id,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// ================================
// KAFKA PRODUCER
// ================================

type SecurityAuditProducer struct {
	producer sarama.SyncProducer
}

func NewSecurityAuditProducer(brokers []string) (*SecurityAuditProducer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	return &SecurityAuditProducer{
		producer: producer,
	}, nil
}

func (p *SecurityAuditProducer) Close() error {
	return p.producer.Close()
}

// PublishAuthenticationEvent publishes authentication events to Kafka
func (p *SecurityAuditProducer) PublishAuthenticationEvent(ctx context.Context, msg *AuthenticationEventMessage) error {
	return p.publishMessage(TopicAuthenticationEvents, msg.EventType, msg)
}

// PublishAccountEvent publishes account events to Kafka
func (p *SecurityAuditProducer) PublishAccountEvent(ctx context.Context, msg *AccountEventMessage) error {
	return p.publishMessage(TopicAccountEvents, msg.UserID, msg)
}

// PublishSecurityEvent publishes security events to Kafka
func (p *SecurityAuditProducer) PublishSecurityEvent(ctx context.Context, msg *SecurityEventMessage) error {
	key := msg.EventType
	if msg.UserID != nil {
		key = *msg.UserID
	}
	return p.publishMessage(TopicSecurityEvents, key, msg)
}

// PublishAdminEvent publishes admin events to Kafka
func (p *SecurityAuditProducer) PublishAdminEvent(ctx context.Context, msg *AdminEventMessage) error {
	return p.publishMessage(TopicAdminEvents, msg.TargetUserID, msg)
}

// PublishFailedLogin publishes failed login events to Kafka
func (p *SecurityAuditProducer) PublishFailedLogin(ctx context.Context, msg *FailedLoginMessage) error {
	return p.publishMessage(TopicFailedLogins, msg.Identifier, msg)
}

func (p *SecurityAuditProducer) publishMessage(topic, key string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(data),
	}

	_, _, err = p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to send message to topic %s: %w", topic, err)
	}

	return nil
}

// ================================
// KAFKA CONSUMER
// ================================

type SecurityAuditConsumer struct {
	consumer     sarama.ConsumerGroup
	auditService *service.SecurityAuditService
	topics       []string
}

func NewSecurityAuditConsumer(
	brokers []string,
	groupID string,
	auditService *service.SecurityAuditService,
) (*SecurityAuditConsumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = sarama.OffsetNewest

	consumer, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	topics := []string{
		TopicAuthenticationEvents,
		TopicAccountEvents,
		TopicSecurityEvents,
		TopicAdminEvents,
		TopicFailedLogins,
	}

	return &SecurityAuditConsumer{
		consumer:     consumer,
		auditService: auditService,
		topics:       topics,
	}, nil
}

func (c *SecurityAuditConsumer) Start(ctx context.Context) error {
	handler := &consumerGroupHandler{
		auditService: c.auditService,
	}

	for {
		if err := c.consumer.Consume(ctx, c.topics, handler); err != nil {
			log.Printf("Error consuming messages: %v", err)
			return err
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

func (c *SecurityAuditConsumer) Close() error {
	return c.consumer.Close()
}

// ================================
// CONSUMER GROUP HANDLER
// ================================

type consumerGroupHandler struct {
	auditService *service.SecurityAuditService
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		ctx := context.Background()

		switch message.Topic {
		case TopicAuthenticationEvents:
			h.handleAuthenticationEvent(ctx, message.Value)
		case TopicAccountEvents:
			h.handleAccountEvent(ctx, message.Value)
		case TopicSecurityEvents:
			h.handleSecurityEvent(ctx, message.Value)
		case TopicAdminEvents:
			h.handleAdminEvent(ctx, message.Value)
		case TopicFailedLogins:
			h.handleFailedLogin(ctx, message.Value)
		default:
			log.Printf("Unknown topic: %s", message.Topic)
		}

		session.MarkMessage(message, "")
	}

	return nil
}

func (h *consumerGroupHandler) handleAuthenticationEvent(ctx context.Context, data []byte) {
	var msg AuthenticationEventMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("Failed to unmarshal authentication event: %v", err)
		return
	}

	auditCtx := &service.AuditContext{
		SessionID:    msg.SessionID,
		IPAddress:    msg.IPAddress,
		UserAgent:    msg.UserAgent,
		RequestID:    msg.RequestID,
		Description:  msg.Description,
		Metadata:     msg.Metadata,
		ErrorCode:    msg.ErrorCode,
		ErrorMessage: msg.ErrorMessage,
	}

	if err := h.auditService.LogAuthenticationEvent(ctx, msg.EventType, msg.UserID, msg.Status, auditCtx); err != nil {
		log.Printf("Failed to log authentication event: %v", err)
	}
}

func (h *consumerGroupHandler) handleAccountEvent(ctx context.Context, data []byte) {
	var msg AccountEventMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("Failed to unmarshal account event: %v", err)
		return
	}

	auditCtx := &service.AuditContext{
		TargetUserID:  msg.TargetUserID,
		SessionID:     msg.SessionID,
		IPAddress:     msg.IPAddress,
		UserAgent:     msg.UserAgent,
		RequestID:     msg.RequestID,
		Action:        msg.Action,
		Description:   msg.Description,
		PreviousValue: msg.PreviousValue,
		NewValue:      msg.NewValue,
		Metadata:      msg.Metadata,
	}

	if err := h.auditService.LogAccountEvent(ctx, msg.EventType, msg.UserID, auditCtx); err != nil {
		log.Printf("Failed to log account event: %v", err)
	}
}

func (h *consumerGroupHandler) handleSecurityEvent(ctx context.Context, data []byte) {
	var msg SecurityEventMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("Failed to unmarshal security event: %v", err)
		return
	}

	auditCtx := &service.AuditContext{
		SessionID:   msg.SessionID,
		IPAddress:   msg.IPAddress,
		UserAgent:   msg.UserAgent,
		RequestID:   msg.RequestID,
		Description: msg.Description,
		Metadata:    msg.Metadata,
	}

	if err := h.auditService.LogSecurityEvent(ctx, msg.EventType, msg.UserID, msg.Severity, auditCtx); err != nil {
		log.Printf("Failed to log security event: %v", err)
	}
}

func (h *consumerGroupHandler) handleAdminEvent(ctx context.Context, data []byte) {
	var msg AdminEventMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("Failed to unmarshal admin event: %v", err)
		return
	}

	auditCtx := &service.AuditContext{
		SessionID:     msg.SessionID,
		IPAddress:     msg.IPAddress,
		UserAgent:     msg.UserAgent,
		RequestID:     msg.RequestID,
		Description:   msg.Description,
		PreviousValue: msg.PreviousValue,
		NewValue:      msg.NewValue,
		Metadata:      msg.Metadata,
	}

	if err := h.auditService.LogAdminAction(ctx, msg.AdminUserID, msg.TargetUserID, msg.Action, auditCtx); err != nil {
		log.Printf("Failed to log admin event: %v", err)
	}
}

func (h *consumerGroupHandler) handleFailedLogin(ctx context.Context, data []byte) {
	var msg FailedLoginMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("Failed to unmarshal failed login: %v", err)
		return
	}

	auditCtx := &service.AuditContext{
		UserAgent: msg.UserAgent,
		RequestID: msg.RequestID,
	}

	if err := h.auditService.RecordFailedLogin(
		ctx,
		msg.Identifier,
		msg.IdentifierType,
		msg.IPAddress,
		stringValue(msg.FailureReason),
		auditCtx,
	); err != nil {
		log.Printf("Failed to record failed login: %v", err)
	}
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
