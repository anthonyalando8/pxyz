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
DATABASE_URL=postgresql://user:pass@212.95.35.81:5432/authdb

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


# OAuth2 Integration Guide

## Overview
This guide explains how to integrate OAuth2 authorization with your existing authentication flow. Users can sign in through your platform to authorize third-party applications.

## Architecture

### Flow Types
1. **Regular Login**: User logs in to your platform directly
2. **OAuth2 Authorization**: User logs in to authorize a third-party app

## How It Works

### 1. Third-Party App Initiates Authorization

```
GET /oauth2/authorize?
  response_type=code&
  client_id=client_xxx&
  redirect_uri=https://thirdparty.com/callback&
  scope=read%20email&
  state=random_state&
  code_challenge=xxx&
  code_challenge_method=S256
```

### 2. User Authentication Check

- **If user NOT authenticated**: Redirect to login with OAuth2 context
- **If user authenticated**: Check consent status

### 3. Login Flow with OAuth2 Context

When redirected to login, the OAuth2 context is preserved:

```json
{
  "identifier": "user@example.com",
  "oauth2_context": {
    "client_id": "client_xxx",
    "redirect_uri": "https://thirdparty.com/callback",
    "scope": "read email",
    "state": "random_state",
    "code_challenge": "xxx",
    "code_challenge_method": "S256"
  }
}
```

The `oauth2_context` is passed through all authentication steps:
- Submit identifier
- Verify OTP
- Set/Enter password

### 4. Post-Authentication

After successful authentication:

**A. If user has granted consent before**:
- Generate authorization code immediately
- Redirect back to third-party app

**B. If consent required**:
- Show consent screen with app details and requested permissions
- User approves/denies

### 5. Authorization Code Exchange

Third-party app exchanges code for tokens:

```
POST /oauth2/token
Content-Type: application/x-www-form-urlencoded

grant_type=authorization_code&
code=auth_code_xxx&
redirect_uri=https://thirdparty.com/callback&
client_id=client_xxx&
client_secret=secret_xxx&
code_verifier=xxx
```

Response:
```json
{
  "access_token": "token_xxx",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "refresh_xxx",
  "scope": "read email"
}
```

## Implementation Steps

### Step 1: Initialize Services

```go
// main.go or dependency injection setup
oauth2Repo := repository.NewUserRepository(db) // Uses same repo
oauth2Svc := service.NewOAuth2Service(oauth2Repo)
oauth2Handler := handler.NewOAuth2Handler(oauth2Svc, authUsecase)
```

### Step 2: Update Auth Handler

```go
// Add oauth2Svc to your AuthHandler
type AuthHandler struct {
    uc        *usecase.AuthUsecase
    oauth2Svc *service.OAuth2Service // ADD THIS
    // ... other fields
}

func NewAuthHandler(uc *usecase.AuthUsecase, oauth2Svc *service.OAuth2Service) *AuthHandler {
    return &AuthHandler{
        uc:        uc,
        oauth2Svc: oauth2Svc,
    }
}
```

### Step 3: Setup Routes

```go
// routes/routes.go
func SetupRoutes(mux *http.ServeMux, handlers *Handlers, middleware *AuthMiddleware) {
    // Existing auth routes
    mux.HandleFunc("POST /api/v1/auth/submit-identifier", handlers.Auth.SubmitIdentifier)
    mux.HandleFunc("POST /api/v1/auth/verify-identifier", 
        middleware.ValidateToken(handlers.Auth.VerifyIdentifier))
    mux.HandleFunc("POST /api/v1/auth/set-password", 
        middleware.ValidateToken(handlers.Auth.SetPassword))
    mux.HandleFunc("POST /api/v1/auth/login-password", 
        middleware.ValidateToken(handlers.Auth.LoginWithPassword))
    
    // OAuth2 routes
    SetupOAuth2Routes(mux, handlers.OAuth2, middleware)
}
```

### Step 4: Frontend Integration

#### A. Regular Login (No Changes)

```javascript
// Regular login flow remains unchanged
const response = await fetch('/api/v1/auth/submit-identifier', {
  method: 'POST',
  body: JSON.stringify({ identifier: 'user@example.com' })
});
```

#### B. OAuth2 Authorization Flow

```javascript
// 1. Parse OAuth2 params from URL
const urlParams = new URLSearchParams(window.location.search);
const oauth2Context = {
  client_id: urlParams.get('client_id'),
  redirect_uri: urlParams.get('redirect_uri'),
  scope: urlParams.get('scope'),
  state: urlParams.get('state'),
  code_challenge: urlParams.get('code_challenge'),
  code_challenge_method: urlParams.get('code_challenge_method')
};

// 2. Submit identifier with OAuth2 context
const response = await fetch('/api/v1/auth/submit-identifier', {
  method: 'POST',
  body: JSON.stringify({
    identifier: 'user@example.com',
    oauth2_context: oauth2Context
  })
});

const data = await response.json();
// Store token and oauth2_context for next steps

// 3. Verify OTP with context
await fetch('/api/v1/auth/verify-identifier', {
  method: 'POST',
  headers: { 'Authorization': `Bearer ${token}` },
  body: JSON.stringify({
    code: '123456',
    oauth2_context: oauth2Context
  })
});

// 4. Login with password
const loginResponse = await fetch('/api/v1/auth/login-password', {
  method: 'POST',
  headers: { 'Authorization': `Bearer ${token}` },
  body: JSON.stringify({
    password: 'userpassword',
    oauth2_context: oauth2Context
  })
});

const loginData = await loginResponse.json();

// 5. Handle consent if required
if (loginData.requires_consent) {
  // Show consent screen with loginData.consent_info
  // After user approves:
  const consentResponse = await fetch('/oauth2/consent', {
    method: 'POST',
    body: JSON.stringify({
      client_id: oauth2Context.client_id,
      scope: oauth2Context.scope,
      redirect_uri: oauth2Context.redirect_uri,
      state: oauth2Context.state,
      approved: true
    })
  });
  
  const consentData = await consentResponse.json();
  window.location.href = consentData.redirect_url;
} else if (loginData.redirect_url) {
  // User already consented, redirect immediately
  window.location.href = loginData.redirect_url;
}
```

## Client Registration

### Register a New OAuth2 Client

```bash
curl -X POST http://localhost:8080/api/v1/oauth2/clients \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "client_name": "My Awesome App",
    "client_uri": "https://myapp.com",
    "logo_uri": "https://myapp.com/logo.png",
    "redirect_uris": ["https://myapp.com/callback"],
    "scope": "read email"
  }'
```

Response:
```json
{
  "client_id": "client_abc123",
  "client_secret": "secret_xyz789",  // SAVE THIS - shown only once!
  "client_name": "My Awesome App",
  "redirect_uris": ["https://myapp.com/callback"],
  "grant_types": ["authorization_code", "refresh_token"],
  "scope": "read email",
  "created_at": "2025-10-19T10:30:00Z"
}
```

## Security Features

### 1. PKCE (Proof Key for Code Exchange)
- Protects against authorization code interception
- Required for public clients
- Recommended for all clients

### 2. State Parameter
- Prevents CSRF attacks
- Client generates random state
- Server returns same state in callback

### 3. Token Hashing
- Access and refresh tokens are hashed (SHA-256) before storage
- Plain tokens never stored in database

### 4. Consent Management
- Users control which apps access their data
- Granular scope permissions
- Easy revocation

### 5. Audit Logging
- All OAuth2 operations logged
- IP address and user agent tracking
- Event types: authorization_granted, token_issued, consent_revoked, etc.

## Database Maintenance

### Cleanup Expired Tokens

```go
// Run periodically (e.g., daily cron job)
deleted, err := oauth2Svc.CleanupExpiredTokens(ctx)
log.Printf("Cleaned up %d expired tokens", deleted)
```

## Testing

### Test Authorization Flow

```bash
# 1. Start authorization
curl "http://localhost:8080/oauth2/authorize?\
response_type=code&\
client_id=client_abc123&\
redirect_uri=http://localhost:3000/callback&\
scope=read&\
state=xyz123"

# 2. Login and authorize (follow redirects)

# 3. Exchange code for token
curl -X POST http://localhost:8080/oauth2/token \
  -d "grant_type=authorization_code" \
  -d "code=AUTH_CODE_HERE" \
  -d "redirect_uri=http://localhost:3000/callback" \
  -d "client_id=client_abc123" \
  -d "client_secret=secret_xyz789"

# 4. Use access token
curl http://localhost:8080/api/v1/user/profile \
  -H "Authorization: Bearer ACCESS_TOKEN_HERE"

# 5. Refresh token
curl -X POST http://localhost:8080/oauth2/token \
  -d "grant_type=refresh_token" \
  -d "refresh_token=REFRESH_TOKEN_HERE" \
  -d "client_id=client_abc123" \
  -d "client_secret=secret_xyz789"
```

## Error Handling

### OAuth2 Error Responses

All OAuth2 errors follow RFC 6749 format:

```json
{
  "error": "invalid_grant",
  "error_description": "Authorization code has expired"
}
```

Common error codes:
- `invalid_request`: Malformed request
- `invalid_client`: Client authentication failed
- `invalid_grant`: Invalid authorization code or refresh token
- `unauthorized_client`: Client not authorized for this grant type
- `unsupported_grant_type`: Grant type not supported
- `invalid_scope`: Invalid or unknown scope
- `access_denied`: User denied authorization

## UI Components Needed

### 1. Login Page Modifications

Detect OAuth2 context and show appropriate UI:

```javascript
// Check if this is OAuth2 authorization flow
const isOAuth2Flow = urlParams.has('client_id') || 
                     sessionStorage.getItem('oauth2_context');

if (isOAuth2Flow) {
  // Show: "AppName wants to access your account"
  // Display app logo and name
}
```

### 2. Consent Screen Component

```jsx
function ConsentScreen({ consentInfo, onApprove, onDeny }) {
  return (
    <div className="consent-screen">
      <div className="app-info">
        {consentInfo.logo_uri && (
          <img src={consentInfo.logo_uri} alt={consentInfo.client_name} />
        )}
        <h2>{consentInfo.client_name}</h2>
        <p>wants to access your account</p>
      </div>

      <div className="permissions">
        <h3>This app will be able to:</h3>
        <ul>
          {consentInfo.requested_scopes.map(scope => (
            <li key={scope}>
              <strong>{scope}:</strong> {consentInfo.scope_descriptions[scope]}
            </li>
          ))}
        </ul>
      </div>

      <div className="actions">
        <button onClick={onDeny}>Deny</button>
        <button onClick={onApprove}>Allow</button>
      </div>
    </div>
  );
}
```

### 3. User Settings - Connected Apps

```jsx
function ConnectedApps({ consents, onRevoke }) {
  return (
    <div className="connected-apps">
      <h2>Connected Applications</h2>
      {consents.map(consent => (
        <div key={consent.client_id} className="app-card">
          <div className="app-details">
            <h3>{consent.client_name}</h3>
            <p>Granted: {new Date(consent.granted_at).toLocaleDateString()}</p>
            <p>Permissions: {consent.scope}</p>
          </div>
          <button onClick={() => onRevoke(consent.client_id)}>
            Revoke Access
          </button>
        </div>
      ))}
    </div>
  );
}
```

## Middleware Integration

### Extract User from OAuth2 Access Token

```go
// middleware/oauth2_auth.go
package middleware

import (
	"auth-service/internal/service"
	"context"
	"net/http"
	"strings"
	"x/shared/response"
)

type OAuth2Middleware struct {
	oauth2Svc *service.OAuth2Service
}

func NewOAuth2Middleware(oauth2Svc *service.OAuth2Service) *OAuth2Middleware {
	return &OAuth2Middleware{oauth2Svc: oauth2Svc}
}

// ValidateOAuth2Token validates OAuth2 access tokens for API access
func (m *OAuth2Middleware) ValidateOAuth2Token(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			response.Error(w, http.StatusUnauthorized, "Authorization header required")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Error(w, http.StatusUnauthorized, "Invalid authorization header format")
			return
		}

		token := parts[1]

		// Validate the access token
		accessToken, err := m.oauth2Svc.ValidateAccessToken(r.Context(), token)
		if err != nil {
			response.Error(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		// Add user and client info to context
		ctx := r.Context()
		if accessToken.UserID != nil {
			ctx = context.WithValue(ctx, "user_id", *accessToken.UserID)
		}
		ctx = context.WithValue(ctx, "client_id", accessToken.ClientID)
		ctx = context.WithValue(ctx, "scope", accessToken.Scope)

		next(w, r.WithContext(ctx))
	}
}

// CheckScope ensures the token has required scope
func (m *OAuth2Middleware) CheckScope(requiredScope string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			scope, ok := r.Context().Value("scope").(string)
			if !ok {
				response.Error(w, http.StatusForbidden, "Insufficient permissions")
				return
			}

			// Check if required scope is present
			scopes := strings.Split(scope, " ")
			hasScope := false
			for _, s := range scopes {
				if s == requiredScope {
					hasScope = true
					break
				}
			}

			if !hasScope {
				response.Error(w, http.StatusForbidden, "Insufficient scope")
				return
			}

			next(w, r)
		}
	}
}
```

### Usage in Routes

```go
// Protect API endpoints with OAuth2
mux.HandleFunc("GET /api/v1/user/profile", 
	oauth2Middleware.ValidateOAuth2Token(
		oauth2Middleware.CheckScope("read")(userHandler.GetProfile)))

mux.HandleFunc("PUT /api/v1/user/profile", 
	oauth2Middleware.ValidateOAuth2Token(
		oauth2Middleware.CheckScope("write")(userHandler.UpdateProfile)))
```

## Complete Flow Diagram

```
┌─────────────────┐
│  Third-Party    │
│      App        │
└────────┬────────┘
         │
         │ 1. GET /oauth2/authorize
         ↓
┌─────────────────┐
│  Your Platform  │ → Check if user authenticated
│  Auth Server    │
└────────┬────────┘
         │
         │ 2. Redirect to login (if not authenticated)
         │    with oauth2_context preserved
         ↓
┌─────────────────┐
│  Login UI       │
│  (Your App)     │ → Shows "AppName wants to access..."
└────────┬────────┘
         │
         │ 3. User logs in (submit-identifier → verify → password)
         │    oauth2_context passed through all steps
         ↓
┌─────────────────┐
│  Post-Auth      │ → Check consent status
│  Handler        │
└────────┬────────┘
         │
         ├─→ Consent exists?
         │   ├─→ YES: Generate code, redirect to app
         │   └─→ NO: Show consent screen
         │
         │ 4. User approves consent
         ↓
┌─────────────────┐
│  Auth Server    │ → Generate authorization code
│                 │ → Redirect to app callback
└────────┬────────┘
         │
         │ 5. Redirect: app.com/callback?code=xxx&state=xyz
         ↓
┌─────────────────┐
│  Third-Party    │
│      App        │
└────────┬────────┘
         │
         │ 6. POST /oauth2/token (exchange code)
         ↓
┌─────────────────┐
│  Auth Server    │ → Validate code
│                 │ → Issue access + refresh tokens
└────────┬────────┘
         │
         │ 7. Return tokens
         ↓
┌─────────────────┐
│  Third-Party    │ → Store tokens
│      App        │ → Use access_token for API calls
└─────────────────┘
```

## Session Management During OAuth2 Flow

The key difference from regular login:

### Regular Login
```
submit-identifier → verify → password → CREATE_SESSION → return token
```

### OAuth2 Flow
```
submit-identifier → verify → password → CHECK_CONSENT → 
  ├─→ if consented: generate_code → redirect_to_app
  └─→ if not: show_consent → generate_code → redirect_to_app
```

**Important**: During OAuth2 flow, you still create a temporary session for the login process, but the final token is the OAuth2 authorization code, not a session token.

## Monitoring & Analytics

### Track OAuth2 Usage

```sql
-- Most popular OAuth2 apps
SELECT 
    oc.client_name,
    COUNT(DISTINCT ouc.user_id) as total_users,
    COUNT(oat.id) as total_tokens
FROM oauth2_clients oc
LEFT JOIN oauth2_user_consents ouc ON oc.client_id = ouc.client_id
LEFT JOIN oauth2_access_tokens oat ON oc.client_id = oat.client_id
WHERE ouc.revoked = false
GROUP BY oc.client_id, oc.client_name
ORDER BY total_users DESC;

-- Recent OAuth2 activity
SELECT 
    event_type,
    COUNT(*) as count,
    DATE(created_at) as date
FROM oauth2_audit_log
WHERE created_at > NOW() - INTERVAL '30 days'
GROUP BY event_type, DATE(created_at)
ORDER BY date DESC;

-- Token refresh rate
SELECT 
    client_id,
    COUNT(*) as refresh_count
FROM oauth2_audit_log
WHERE event_type = 'token_refreshed'
    AND created_at > NOW() - INTERVAL '7 days'
GROUP BY client_id
ORDER BY refresh_count DESC;
```

## Best Practices

### 1. Token Expiration
- **Access tokens**: 1 hour (short-lived)
- **Refresh tokens**: 30 days (can be longer)
- **Authorization codes**: 10 minutes

### 2. Security
- Always use HTTPS in production
- Validate redirect URIs strictly
- Implement rate limiting on token endpoint
- Log all OAuth2 operations
- Regularly cleanup expired tokens

### 3. Scope Design
```
read          → Basic profile information
write         → Modify user data
email         → Access email address
phone         → Access phone number
profile:full  → Complete profile access
admin         → Administrative actions (careful!)
```

### 4. Error Messages
- Be specific in development
- Be generic in production (avoid leaking info)
- Always include `state` parameter in error redirects

## Troubleshooting

### Common Issues

**1. "Invalid redirect URI"**
- Ensure exact match including protocol, domain, path
- No wildcards allowed
- Register all callback URLs

**2. "Code already used"**
- Authorization codes are single-use
- Generate new code by re-authorizing

**3. "Token expired"**
- Use refresh token to get new access token
- Check server time synchronization

**4. "Consent required" loop**
- Check consent storage/retrieval
- Verify user_id and client_id matching

**5. OAuth2 context not preserved**
- Store in session/state during redirects
- Pass through all auth steps
- Use signed/encrypted state parameter

## Next Steps

1. **Test the integration**: Start with a test OAuth2 client
2. **Build consent UI**: Create user-friendly consent screens
3. **Add monitoring**: Set up dashboards for OAuth2 metrics
4. **Documentation**: Document your OAuth2 endpoints for developers
5. **Rate limiting**: Implement token endpoint rate limits
6. **Webhooks**: Consider adding webhooks for revocation events