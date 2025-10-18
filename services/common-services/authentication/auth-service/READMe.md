## Repo methods

```go
// UserRepository is the interface contract with repo
type UserRepository interface {
	BatchDeleteByStatus(ctx context.Context, status string, batchSize int, gracePeriodDays int) (int64, error)
	CheckOAuthAccountExists(ctx context.Context, userID string, provider string) (bool, error)
	CleanupExpiredOAuth2Tokens(ctx context.Context) (int64, error)
	CleanupExpiredTokens(ctx context.Context) (int64, error)
	CountOAuthAccountsByProvider(ctx context.Context, provider string) (int64, error)
	CreateAccessToken(ctx context.Context, token *domain.OAuth2AccessToken) error
	CreateAuthorizationCode(ctx context.Context, code *domain.OAuth2AuthorizationCode) error
	CreateOAuth2AuditLog(ctx context.Context, log *domain.OAuth2AuditLog) error
	CreateOAuth2Client(ctx context.Context, client *domain.OAuth2Client) (*domain.OAuth2Client, error)
	CreateOAuthAccount(ctx context.Context, acc *domain.OAuthAccount) error
	CreateRefreshToken(ctx context.Context, token *domain.OAuth2RefreshToken) error
	CreateUserConsent(ctx context.Context, consent *domain.OAuth2UserConsent) error
	CreateUsers(ctx context.Context, users []*domain.User, creds []*domain.UserCredential) ([]*domain.User, []error)
	DeleteOAuth2Client(ctx context.Context, clientID string) error
	DeleteUser(ctx context.Context, userID string) error
	DeleteUsers(ctx context.Context, userIDs []string) error
	FindByProviderUID(ctx context.Context, provider string, providerUID string) (*domain.OAuthAccount, error)
	GetAccessTokenByHash(ctx context.Context, tokenHash string) (*domain.OAuth2AccessToken, error)
	GetAllScopes(ctx context.Context) ([]*domain.OAuth2Scope, error)
	GetAuthorizationCode(ctx context.Context, code string) (*domain.OAuth2AuthorizationCode, error)
	GetBothVerificationStatuses(ctx context.Context, userID string) (emailVerified bool, phoneVerified bool, err error)
	GetCredentialHistory(ctx context.Context, userID string, limit int) ([]*domain.CredentialHistory, error)
	GetEmailVerificationStatus(ctx context.Context, userID string) (bool, error)
	GetOAuth2AuditLogs(ctx context.Context, limit int, offset int) ([]*domain.OAuth2AuditLog, error)
	GetOAuth2ClientByClientID(ctx context.Context, clientID string) (*domain.OAuth2Client, error)
	GetOAuth2ClientsByOwner(ctx context.Context, ownerUserID string) ([]*domain.OAuth2Client, error)
	GetOAuthAccountByID(ctx context.Context, id string) (*domain.OAuthAccount, error)
	GetOAuthAccountsByUserID(ctx context.Context, userID string) ([]*domain.OAuthAccount, error)
	GetPhoneVerificationStatus(ctx context.Context, userID string) (bool, error)
	GetRecentOAuthLinks(ctx context.Context, limit int) ([]*domain.OAuthAccount, error)
	GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*domain.OAuth2RefreshToken, error)
	GetUserByID(ctx context.Context, userID string) (*domain.UserProfile, error)
	GetUserByIdentifier(ctx context.Context, identifier string) (*domain.UserWithCredential, error)
	GetUserConsent(ctx context.Context, userID string, clientID string) (*domain.OAuth2UserConsent, error)
	GetUserConsents(ctx context.Context, userID string) ([]*domain.OAuth2UserConsent, error)
	GetUserIDByProviderUID(ctx context.Context, provider string, providerUID string) (string, error)
	GetUserWithCredentialsWithID(ctx context.Context, userID string) (*domain.User, []*domain.UserCredential, error)
	GetUsersByIDs(ctx context.Context, userIDs []string) ([]*domain.UserProfile, error)
	GetUsersByIdentifiers(ctx context.Context, identifiers []string) ([]*domain.UserWithCredential, error)
	GetVerificationStatuses(ctx context.Context, userIDs []string) ([]*VerificationStatus, error)
	IdentifierExists(ctx context.Context, identifier string) (bool, error)
	InvalidateAllCredentials(ctx context.Context, userID string) error
	MarkAuthorizationCodeAsUsed(ctx context.Context, code string) error
	MarkCredentialAsVerified(ctx context.Context, userID string) error
	PermanentlyDeleteUsers(ctx context.Context, gracePeriodDays int) (int64, error)
	RegenerateClientSecret(ctx context.Context, clientID string, newSecretHash string) error
	RestoreUser(ctx context.Context, userID string) error
	RestoreUsers(ctx context.Context, userIDs []string) error
	RevokeAccessToken(ctx context.Context, tokenHash string) error
	RevokeAccessTokensByUser(ctx context.Context, userID string) error
	RevokeAllUserConsents(ctx context.Context, userID string) error
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeRefreshTokensByUser(ctx context.Context, userID string) error
	RevokeUserConsent(ctx context.Context, userID string, clientID string) error
	SoftDeleteUser(ctx context.Context, userID string) error
	SoftDeleteUsers(ctx context.Context, userIDs []string) error
	UnlinkOAuthAccount(ctx context.Context, userID string, provider string) error
	UpdateEmail(ctx context.Context, userID string, newEmail string) error
	UpdateEmailVerificationStatus(ctx context.Context, userID string, verified bool) error
	UpdateEmailVerificationStatuses(ctx context.Context, userIDs []string, verified bool) error
	UpdateOAuth2Client(ctx context.Context, client *domain.OAuth2Client) (*domain.OAuth2Client, error)
	UpdateOAuthAccountMetadata(ctx context.Context, id string, metadata map[string]interface{}) error
	UpdateOAuthTokens(ctx context.Context, provider string, providerUID string, accessToken *string, refreshToken *string, expiresAt *time.Time) error
	UpdatePassword(ctx context.Context, userID string, newHash string) error
	UpdatePhone(ctx context.Context, userID string, newPhone string, isPhoneVerified bool) error
	UpdatePhoneVerificationStatus(ctx context.Context, userID string, verified bool) error
	UpdatePhoneVerificationStatuses(ctx context.Context, userIDs []string, verified bool) error
	UserExistsByID(ctx context.Context, userID string) (bool, error)
	updateBothVerificationStatuses(ctx context.Context, userID string, emailVerified bool, phoneVerified bool) error
}

# Kafka-Based User Registration with Smart Batching

## Overview
A Kafka-based asynchronous user registration system with intelligent batching, DLQ (Dead Letter Queue), and automatic retry logic.

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   API Layer  │────▶│ Kafka Topic  │────▶│   Consumer   │
│  (Producer)  │     │ Registration │     │  (Batcher)   │
└──────────────┘     └──────────────┘     └──────────────┘
                                                   │
                                                   ▼
                                           ┌──────────────┐
                                           │   Database   │
                                           │ (PostgreSQL) │
                                           └──────────────┘
                                                   │
                                           Failed  │  Success
                                                   │
                                           ┌───────┴────────┐
                                           ▼                ▼
                                    ┌──────────┐    ┌──────────┐
                                    │   DLQ    │    │   Done   │
                                    │  Topic   │    └──────────┘
                                    └──────────┘
                                           │
                                           ▼
                                    ┌──────────────┐
                                    │DLQ Consumer  │
                                    │ (Retry 3x)   │
                                    └──────────────┘
```

## Batching Logic

### Three Flush Triggers:

1. **Max Batch Size (1000 messages)**
   - Flush immediately when 1000 messages accumulated
   - Most efficient for high-volume scenarios

2. **Max Batch Timeout (5 seconds)**
   - Flush after 5 seconds regardless of batch size
   - Prevents indefinite waiting for messages

3. **Idle Timeout (500ms)**
   - Flush if no new message received for 500ms
   - Optimizes for low-latency scenarios

### Flow Diagram:

```
Message Received
      │
      ▼
┌──────────────────┐
│ Add to Batch     │
│ Update Timestamp │
└──────────────────┘
      │
      ▼
┌──────────────────┐      YES    ┌──────────────┐
│ Batch Size       │─────────────▶│ Flush Batch  │
│ >= 1000?         │              └──────────────┘
└──────────────────┘
      │ NO
      ▼
┌──────────────────┐      YES    ┌──────────────┐
│ 5 seconds        │─────────────▶│ Flush Batch  │
│ elapsed?         │              └──────────────┘
└──────────────────┘
      │ NO
      ▼
┌──────────────────┐      YES    ┌──────────────┐
│ 500ms idle       │─────────────▶│ Flush Batch  │
│ timeout?         │              └──────────────┘
└──────────────────┘
      │ NO
      ▼
   Wait for next
   message or timer
```

## DLQ and Retry Logic

### Retry Strategy:
- **Max Retries**: 3 attempts
- **Retry Delay**: 5 seconds between retries
- **Exponential Backoff**: Can be added if needed

### DLQ Flow:

```
User Creation Failed
      │
      ▼
┌──────────────────┐
│ Publish to DLQ   │
│ + Error Details  │
└──────────────────┘
      │
      ▼
┌──────────────────┐
│ DLQ Consumer     │
│ Picks Up Message │
└──────────────────┘
      │
      ▼
┌──────────────────┐
│ Check Retry      │
│ Count < 3?       │
└──────────────────┘
   │           │
   │ YES       │ NO
   ▼           ▼
┌─────────┐  ┌──────────────┐
│ Wait 5s │  │ Permanent    │
│ + Retry │  │ Failure      │
└─────────┘  │ (Alert/Log)  │
   │         └──────────────┘
   ▼
Success? ───YES──▶ Done
   │
   │ NO
   ▼
Increment Retry
Re-publish to DLQ
```

## Implementation Details

### 1. Kafka Topics

**Main Topic**: `user.registration`
- Partitions: 6 (configurable)
- Replication Factor: 3
- Retention: 7 days

**DLQ Topic**: `user.registration.dlq`
- Partitions: 3 (lower than main topic)
- Replication Factor: 3
- Retention: 30 days (longer for debugging)

### 2. Message Format

```json
{
  "email": "user@example.com",
  "phone": null,
  "password": "hashed_password",
  "consent": true,
  "is_email_verified": false,
  "is_phone_verified": false,
  "request_id": "uuid-v4",
  "retry_count": 0,
  "failure_reason": "optional error message"
}
```

### 3. Consumer Configuration

```go
// Batch Settings
MaxBatchSize    = 1000
MaxBatchTimeout = 5 * time.Second
IdleTimeout     = 500 * time.Millisecond

// Kafka Settings
Session.Timeout = 20 * time.Second
Heartbeat.Interval = 6 * time.Second
MaxProcessingTime = 30 * time.Second
```

## API Endpoints

### Async Registration (Recommended)
```http
POST /api/v1/auth/register-async
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "secure_password"
}

Response (202 Accepted):
{
  "message": "registration request accepted",
  "request_id": "abc-123-def-456",
  "status": "processing"
}
```

### Bulk Async Registration
```http
POST /api/v1/auth/register-async-bulk
Content-Type: application/json

{
  "users": [
    {
      "email": "user1@example.com",
      "password": "password1",
      "consent": true
    },
    {
      "email": "user2@example.com",
      "password": "password2",
      "consent": true
    }
  ]
}

Response (202 Accepted):
{
  "message": "registration requests accepted",
  "request_ids": ["id1", "id2"],
  "count": 2,
  "status": "processing"
}
```

### Sync Registration (Legacy)
```http
POST /api/v1/auth/register-sync
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "secure_password"
}

Response (201 Created):
{
  "user_id": "123456789",
  "email": "user@example.com"
}
```

## Deployment

### 1. Environment Variables

```bash
# Database
DATABASE_URL=postgresql://user:pass@host.docker.internal:5432/authdb

# Kafka Brokers
KAFKA_BROKER_1=localhost:9092
KAFKA_BROKER_2=localhost:9093
KAFKA_BROKER_3=localhost:9094

# Application
SNOWFLAKE_NODE_ID=1
```

### 2. Running the Consumer

```bash
# Build
go build -o consumer cmd/consumer/main.go

# Run
./consumer
```

### 3. Docker Compose Example

```yaml
version: '3.8'
services:
  zookeeper:
    image: confluentinc/cp-zookeeper:latest
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181

  kafka:
    image: confluentinc/cp-kafka:latest
    depends_on:
      - zookeeper
    ports:
      - "9092:9092"
    environment:
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092

  consumer:
    build: .
    command: ./consumer
    depends_on:
      - kafka
      - postgres
    environment:
      KAFKA_BROKER_1: kafka:9092
      DATABASE_URL: postgresql://user:pass@postgres:5432/authdb
```

## Monitoring

### Metrics to Track:
1. **Message Rate**: Messages/second into Kafka
2. **Batch Size Distribution**: Average batch size
3. **Processing Time**: Time from message to DB insert
4. **DLQ Rate**: Failed messages percentage
5. **Retry Success Rate**: Successful retries vs failures

### Logging:
- Batch flush events with size
- Individual creation failures
- DLQ publications
- Retry attempts and results

## Performance

### Expected Throughput:
- **Single Message**: ~100-200 ms (sync)
- **Batched (1000)**: ~2-3 seconds total (~333 users/sec)
- **Sustained Load**: 10,000+ users/minute

### Resource Requirements:
- **CPU**: 2-4 cores per consumer instance
- **Memory**: 2-4 GB per consumer instance
- **Network**: Minimal (< 10 Mbps per instance)

## Best Practices

1. **Use Async for High Volume**: Batching significantly improves throughput
2. **Monitor DLQ**: Set up alerts for DLQ message accumulation
3. **Scale Consumers**: Run multiple consumer instances for higher throughput
4. **Partition Strategy**: Partition by request_id for even distribution
5. **Idempotency**: Ensure duplicate message handling
6. **Testing**: Load test with realistic message rates

## Troubleshooting

### High DLQ Rate
- Check database connection pool
- Verify unique constraint violations
- Review error messages in DLQ

### Slow Processing
- Increase consumer instances
- Adjust batch size parameters
- Check database performance

### Message Loss
- Verify Kafka replication factor
- Check consumer offset commits
- Review producer acknowledgments