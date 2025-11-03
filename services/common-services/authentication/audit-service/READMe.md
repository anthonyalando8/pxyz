# Security Audit System - Quick Reference

## 📋 Overview

A comprehensive security audit system with multi-protocol support:
- **gRPC**: For service-to-service communication
- **Kafka**: For async event logging and processing
- **WebSocket**: For real-time notifications
- **Background Workers**: For automated maintenance and detection

---

## 🏗️ Architecture

```
┌─────────────┐
│ Auth API    │──┐
└─────────────┘  │
                 ├─► Kafka Topics ──► Consumer ──► Database
┌─────────────┐  │                                    │
│ Admin API   │──┘                                    │
└─────────────┘                                       │
                                                      │
┌─────────────┐                                       │
│ Services    │────► gRPC ─────────────────────────►Database
└─────────────┘                                       │
                                                      │
┌─────────────┐                                       │
│ Dashboard   │◄──── WebSocket ◄─── Notifier ◄───────┘
└─────────────┘
```

---

## 📡 gRPC Endpoints

### Account Lock Management
```protobuf
rpc CheckAccountLockStatus(CheckAccountLockStatusRequest) returns (CheckAccountLockStatusResponse)
rpc CheckAccountsLockStatus(CheckAccountsLockStatusRequest) returns (CheckAccountsLockStatusResponse)
rpc UnlockAccount(UnlockAccountRequest) returns (UnlockAccountResponse)
rpc UnlockAccounts(UnlockAccountsRequest) returns (UnlockAccountsResponse)
```

### Suspicious Activity
```protobuf
rpc DetectSuspiciousActivity(DetectSuspiciousActivityRequest) returns (DetectSuspiciousActivityResponse)
rpc ReportSuspiciousActivity(ReportSuspiciousActivityRequest) returns (ReportSuspiciousActivityResponse)
rpc ReportSuspiciousActivities(ReportSuspiciousActivitiesRequest) returns (ReportSuspiciousActivitiesResponse)
rpc ResolveSuspiciousActivity(ResolveSuspiciousActivityRequest) returns (ResolveSuspiciousActivityResponse)
rpc GetActiveSuspiciousActivities(GetActiveSuspiciousActivitiesRequest) returns (GetActiveSuspiciousActivitiesResponse)
rpc GetHighRiskUsers(GetHighRiskUsersRequest) returns (GetHighRiskUsersResponse)
```

### Audit Queries
```protobuf
rpc GetUserAuditHistory(GetUserAuditHistoryRequest) returns (GetUserAuditHistoryResponse)
rpc GetUsersAuditHistory(GetUsersAuditHistoryRequest) returns (GetUsersAuditHistoryResponse)
rpc QueryAuditLogs(QueryAuditLogsRequest) returns (QueryAuditLogsResponse)
rpc GetCriticalEvents(GetCriticalEventsRequest) returns (GetCriticalEventsResponse)
rpc GetSecuritySummary(GetSecuritySummaryRequest) returns (GetSecuritySummaryResponse)
rpc GetUserRiskScore(GetUserRiskScoreRequest) returns (GetUserRiskScoreResponse)
rpc GetUsersRiskScores(GetUsersRiskScoresRequest) returns (GetUsersRiskScoresResponse)
```

### Rate Limiting
```protobuf
rpc CheckRateLimit(CheckRateLimitRequest) returns (CheckRateLimitResponse)
rpc CheckRateLimits(CheckRateLimitsRequest) returns (CheckRateLimitsResponse)
```

---

## 📨 Kafka Topics

### Topic Structure

| Topic | Purpose | Message Type |
|-------|---------|--------------|
| `auth.events.authentication` | Login/logout events | `AuthenticationEventMessage` |
| `auth.events.account` | Account changes | `AccountEventMessage` |
| `auth.events.security` | Security events | `SecurityEventMessage` |
| `auth.events.admin` | Admin actions | `AdminEventMessage` |
| `auth.events.failed_login` | Failed login attempts | `FailedLoginMessage` |

### Publishing Events

```go
// Publish failed login
failedLoginMsg := &kafka.FailedLoginMessage{
    Identifier:     email,
    IdentifierType: domain.IdentifierTypeEmail,
    IPAddress:      ipAddress,
    FailureReason:  stringPtr("invalid_credentials"),
    Timestamp:      time.Now(),
}
producer.PublishFailedLogin(ctx, failedLoginMsg)

// Publish authentication event
authMsg := &kafka.AuthenticationEventMessage{
    EventType: domain.EventLoginSuccess,
    UserID:    &userID,
    Status:    domain.StatusSuccess,
    IPAddress: &ipAddress,
    Timestamp: time.Now(),
}
producer.PublishAuthenticationEvent(ctx, authMsg)
```

---

## 🔌 WebSocket Real-Time Notifications

### Connection
```javascript
const ws = new WebSocket('ws://localhost:8080/ws/security-audit');

ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    console.log('Received:', msg.type, msg.data);
};
```

### Message Types

| Type | Target | Purpose |
|------|--------|---------|
| `suspicious_activity` | User + Admin | Suspicious behavior detected |
| `critical_event` | Admin | Critical security event |
| `high_risk_user` | Admin | High-risk user alert |
| `account_locked` | User + Admin | Account locked notification |
| `rate_limit_exceeded` | Admin | Rate limit exceeded |

### Example Messages

```json
// Suspicious Activity
{
    "type": "suspicious_activity",
    "timestamp": "2025-10-05T10:30:00Z",
    "data": {
        "user_id": "123456",
        "activity_type": "multiple",
        "risk_score": 75,
        "ip_address": "192.168.1.1",
        "activities": ["unusual_location", "rapid_ip_changes"]
    },
    "metadata": {
        "risk_level": "high"
    }
}

// Critical Event
{
    "type": "critical_event",
    "timestamp": "2025-10-05T10:35:00Z",
    "data": {
        "event_type": "account_locked",
        "user_id": "123456",
        "severity": "critical",
        "description": "Account locked due to 5 failed login attempts"
    }
}
```

---

## 🤖 Background Workers

### Worker Schedule

| Worker | Interval | Purpose |
|--------|----------|---------|
| Maintenance | 1 hour | Auto-unlock, cleanup old data |
| Suspicious Activity Detector | 5 minutes | Analyze patterns, detect anomalies |
| Auto-Resolve | 15 minutes | Resolve old suspicious activities |
| Critical Event Monitor | 1 minute | Push critical events to WebSocket |
| High-Risk User Notifier | 5 minutes | Notify admins of high-risk users |

### Manual Tasks

```go
// Run daily tasks (via cron)
workers.RunDailyTasks()

// Run weekly tasks (via cron)
workers.RunWeeklyTasks()
```

---

## 🔧 Usage Examples

### 1. Login Flow with Audit

```go
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
    // 1. Check rate limit (gRPC)
    rateLimited, _ := checkRateLimitViaGRPC(ctx, email, ipAddress)
    if rateLimited {
        return
    }

    // 2. Check account lock (gRPC)
    lockStatus, _ := checkAccountLockViaGRPC(ctx, userID)
    if lockStatus.IsLocked {
        return
    }

    // 3. Authenticate
    user, err := authenticate(email, password)
    if err != nil {
        // Publish failed login (Kafka)
        producer.PublishFailedLogin(ctx, failedLoginMsg)
        return
    }

    // 4. Publish success (Kafka)
    producer.PublishAuthenticationEvent(ctx, authMsg)

    // 5. Detect suspicious activity (async)
    go detectAndNotify(userID, ipAddress)
}
```

### 2. Admin Dashboard Real-Time Monitoring

```javascript
// Frontend WebSocket connection
const ws = new WebSocket('ws://api.example.com/ws/security-audit');

ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    
    switch(msg.type) {
        case 'critical_event':
            showCriticalAlert(msg.data);
            break;
        case 'high_risk_user':
            updateRiskDashboard(msg.data);
            break;
        case 'suspicious_activity':
            showSuspiciousActivityAlert(msg.data);
            break;
    }
};
```

### 3. Batch Operations via gRPC

```go
// Check multiple accounts at once
client := pb.NewSecurityAuditServiceClient(conn)

resp, err := client.CheckAccountsLockStatus(ctx, &pb.CheckAccountsLockStatusRequest{
    UserIds: []string{"123", "456", "789"},
})

for _, status := range resp.Statuses {
    fmt.Printf("User %s locked: %v\n", status.UserId, status.IsLocked)
}
```

---

## 🔒 Security Features

### Automated Protection
- ✅ Rate limiting per IP and identifier
- ✅ Account lockout after N failed attempts (default: 5)
- ✅ Auto-unlock after timeout (default: 30 minutes)
- ✅ Brute force detection
- ✅ Unusual location detection
- ✅ Risk scoring (0-100)

### Audit Trail
- ✅ Complete audit log with JSONB metadata
- ✅ Change tracking (before/after values)
- ✅ Geolocation tracking
- ✅ Session and request correlation
- ✅ Configurable retention policies

### Compliance
- ✅ GDPR-compliant data retention
- ✅ Audit trail for all security events
- ✅ Soft delete with grace period (30 days)
- ✅ Permanent deletion after retention period

---

## 📊 Database Schema Highlights

### Key Tables
- `security_audit_log` - Main audit trail
- `failed_login_attempts` - Rate limiting data
- `account_lockouts` - Active and historical locks
- `suspicious_activity` - Flagged activities
- `audit_retention_policy` - Configurable retention

### Materialized View
- `security_events_summary` - Pre-aggregated daily stats

### Helper Functions
- `is_account_locked(user_id)` - Check lock status
- `count_recent_failed_logins(identifier, minutes)` - Count failures
- `get_user_risk_score(user_id)` - Calculate risk

---

## 🚀 Deployment

### Environment Variables
```bash
DATABASE_URL=postgres://localhost/auth_db
GRPC_ADDRESS=:50051
HTTP_ADDRESS=:8080
KAFKA_BROKERS=localhost:9092,localhost:9093
```

### Docker Compose Example
```yaml
version: '3.8'
services:
  auth-service:
    build: .
    environment:
      - DATABASE_URL=postgres://db:5432/auth_db
      - KAFKA_BROKERS=kafka:9092
    depends_on:
      - db
      - kafka
```

---

## 📈 Monitoring

### Key Metrics to Track
- Failed login rate
- Account lockout rate
- High-risk user count
- Critical event frequency
- Average risk scores
- Audit log growth rate

### Health Checks
- Database connectivity
- Kafka producer/consumer status
- WebSocket connection count
- Background worker status

---

## 🎯 Best Practices

1. **Always use batch operations** for multiple users (gRPC)
2. **Publish events asynchronously** (Kafka) to avoid blocking
3. **Monitor WebSocket connections** for memory leaks
4. **Configure retention policies** based on compliance requirements
5. **Run maintenance workers** during low-traffic periods
6. **Set up alerts** for critical events and high-risk users
7. **Regularly review** suspended activities and false positives

---

## 📝 Event Categories Reference

| Category | Events | Severity |
|----------|--------|----------|
| Authentication | login_success, login_failed, logout, password_changed | info/warning |
| Account | account_created, account_deleted, account_suspended | info/critical |
| Security | suspicious_activity, brute_force_detected, account_locked | warning/critical |
| Admin | admin_action, user_suspended, user_restored | warning |
| OAuth2 | token_issued, consent_granted, client_registered | info |

---

## 🔗 Integration Points

```
HTTP Handlers ──► Kafka Producer ──► Kafka Topics
                                         │
                                         ▼
                                    Kafka Consumer ──► Service ──► Database
                                                                       │
gRPC Clients ─────► gRPC Server ──► Service ──────────────────────────┤
                                                                       │
Background Workers ──► Service ────────────────────────────────────────┤
                                                                       │
                                                                       ▼
                                                                  Notifier ──► WebSocket ──► Clients
```
// main.go - Complete integration example
package main

import (
	"auth-service/internal/grpc"
	"auth-service/internal/kafka"
	"auth-service/internal/repository"
	"auth-service/internal/service"
	"auth-service/internal/websocket"
	"auth-service/internal/workers"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "auth-service/internal/grpc/pb"
	grpcServer "google.golang.org/grpc"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Load configuration
	config := loadConfig()

	// Initialize database connection
	db, err := pgxpool.New(context.Background(), config.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	userRepo := repository.NewUserRepository(db)

	// Initialize services
	geoIPService := initGeoIPService() // Your GeoIP implementation
	auditService := service.NewSecurityAuditService(userRepo, geoIPService)

	// ================================
	// SETUP GRPC SERVER
	// ================================
	grpcAuditServer := grpc.NewSecurityAuditGRPCServer(auditService)
	
	grpcSrv := grpcServer.NewServer()
	pb.RegisterSecurityAuditServiceServer(grpcSrv, grpcAuditServer)

	go func() {
		listener, err := net.Listen("tcp", config.GRPCAddress)
		if err != nil {
			log.Fatalf("Failed to listen for gRPC: %v", err)
		}
		log.Printf("gRPC server listening on %s", config.GRPCAddress)
		if err := grpcSrv.Serve(listener); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// ================================
	// SETUP KAFKA
	// ================================
	
	// Kafka Producer
	kafkaProducer, err := kafka.NewSecurityAuditProducer(config.KafkaBrokers)
	if err != nil {
		log.Fatalf("Failed to create Kafka producer: %v", err)
	}
	defer kafkaProducer.Close()

	// Kafka Consumer
	kafkaConsumer, err := kafka.NewSecurityAuditConsumer(
		config.KafkaBrokers,
		"security-audit-consumer-group",
		auditService,
	)
	if err != nil {
		log.Fatalf("Failed to create Kafka consumer: %v", err)
	}
	defer kafkaConsumer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Kafka consumer
	go func() {
		if err := kafkaConsumer.Start(ctx); err != nil {
			log.Printf("Kafka consumer error: %v", err)
		}
	}()

	// ================================
	// SETUP WEBSOCKET
	// ================================
	
	wsHub := websocket.NewHub()
	go wsHub.Run()

	notifier := websocket.NewSecurityAuditNotifier(wsHub, auditService)

	// ================================
	// SETUP BACKGROUND WORKERS
	// ================================
	
	auditWorkers := workers.NewSecurityAuditWorkers(auditService, notifier)
	auditWorkers.Start()
	defer auditWorkers.Stop()

	// ================================
	// SETUP HTTP SERVER (for WebSocket)
	// ================================
	
	r := chi.NewRouter()

	// WebSocket endpoint
	r.Get("/ws/security-audit", func(w http.ResponseWriter, req *http.Request) {
		websocket.ServeWebSocket(wsHub, w, req)
	})

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	httpServer := &http.Server{
		Addr:    config.HTTPAddress,
		Handler: r,
	}

	go func() {
		log.Printf("HTTP server listening on %s", config.HTTPAddress)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// ================================
	// GRACEFUL SHUTDOWN
	// ================================
	
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down servers...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Stop gRPC server
	grpcSrv.GracefulStop()

	// Cancel context for workers
	cancel()

	log.Println("Server stopped gracefully")
}

// ================================
// EXAMPLE: USAGE IN AUTH HANDLERS
// ================================

type AuthHandlers struct {
	auditService  *service.SecurityAuditService
	kafkaProducer *kafka.SecurityAuditProducer
	notifier      *websocket.SecurityAuditNotifier
}

// Example: Login handler with audit logging via Kafka
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	var req LoginRequest
	// ... decode request ...

	ipAddress := getClientIP(r)
	userAgent := r.UserAgent()

	// Attempt authentication
	user, err := authenticateUser(ctx, req.Email, req.Password)
	if err != nil {
		// Publish failed login to Kafka
		failedLoginMsg := &kafka.FailedLoginMessage{
			Identifier:     req.Email,
			IdentifierType: domain.IdentifierTypeEmail,
			IPAddress:      ipAddress,
			UserAgent:      &userAgent,
			FailureReason:  stringPtr("invalid_credentials"),
			RequestID:      stringPtr(getRequestID(ctx)),
			Timestamp:      time.Now(),
		}
		
		h.kafkaProducer.PublishFailedLogin(ctx, failedLoginMsg)
		
		respondError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Check rate limit via gRPC
	rateLimited, _ := checkRateLimitViaGRPC(ctx, req.Email, ipAddress)
	if rateLimited {
		h.notifier.NotifyRateLimitExceeded(ctx, req.Email, ipAddress)
		respondError(w, http.StatusTooManyRequests, "Too many attempts")
		return
	}

	// Check if account is locked via gRPC
	lockStatus, _ := checkAccountLockViaGRPC(ctx, user.ID)
	if lockStatus != nil && lockStatus.IsLocked {
		respondError(w, http.StatusForbidden, "Account is locked")
		return
	}

	// Successful login - publish to Kafka
	authEventMsg := &kafka.AuthenticationEventMessage{
		EventType: domain.EventLoginSuccess,
		UserID:    &user.ID,
		Status:    domain.StatusSuccess,
		SessionID: stringPtr(generateSessionID()),
		IPAddress: &ipAddress,
		UserAgent: &userAgent,
		RequestID: stringPtr(getRequestID(ctx)),
		Timestamp: time.Now(),
	}
	
	h.kafkaProducer.PublishAuthenticationEvent(ctx, authEventMsg)

	// Detect suspicious activity asynchronously
	go func() {
		activities, _ := h.auditService.GetActiveSuspiciousActivities(context.Background(), user.ID)
		if len(activities) > 0 {
			h.notifier.NotifySuspiciousActivity(
				context.Background(),
				user.ID,
				ipAddress,
				extractActivityTypes(activities),
				calculateTotalRiskScore(activities),
			)
		}
	}()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"token":   generateToken(user),
		"user_id": user.ID,
	})
}

// Example: Password change handler
func (h *AuthHandlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	
	var req ChangePasswordRequest
	// ... decode request ...

	// Change password
	err := changeUserPassword(ctx, userID, req.OldPassword, req.NewPassword)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Failed to change password")
		return
	}

	// Publish account event to Kafka
	accountEventMsg := &kafka.AccountEventMessage{
		EventType: domain.EventPasswordChanged,
		UserID:    userID,
		SessionID: stringPtr(getSessionID(ctx)),
		IPAddress: stringPtr(getClientIP(r)),
		UserAgent: stringPtr(r.UserAgent()),
		RequestID: stringPtr(getRequestID(ctx)),
		Action:    stringPtr("password_change"),
		Description: stringPtr("User changed their password"),
		Timestamp: time.Now(),
	}
	
	h.kafkaProducer.PublishAccountEvent(ctx, accountEventMsg)

	respondJSON(w, http.StatusOK, map[string]string{"message": "Password changed successfully"})
}

// Example: Admin suspends user
func (h *AuthHandlers) SuspendUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	adminID := getUserIDFromContext(ctx)
	targetUserID := chi.URLParam(r, "userID")

	// Suspend user
	err := suspendUserAccount(ctx, targetUserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to suspend user")
		return
	}

	// Publish admin event to Kafka
	adminEventMsg := &kafka.AdminEventMessage{
		AdminUserID:  adminID,
		TargetUserID: targetUserID,
		Action:       "suspend_account",
		SessionID:    stringPtr(getSessionID(ctx)),
		IPAddress:    stringPtr(getClientIP(r)),
		UserAgent:    stringPtr(r.UserAgent()),
		RequestID:    stringPtr(getRequestID(ctx)),
		Description:  stringPtr("Admin suspended user account"),
		Timestamp:    time.Now(),
	}
	
	h.kafkaProducer.PublishAdminEvent(ctx, adminEventMsg)

	respondJSON(w, http.StatusOK, map[string]string{"message": "User suspended"})
}

// ================================
// EXAMPLE: GRPC CLIENT USAGE
// ================================

func checkRateLimitViaGRPC(ctx context.Context, identifier, ipAddress string) (bool, error) {
	// Assuming you have a gRPC client connection
	client := pb.NewSecurityAuditServiceClient(getGRPCConnection())
	
	resp, err := client.CheckRateLimit(ctx, &pb.CheckRateLimitRequest{
		Identifier: identifier,
		IpAddress:  ipAddress,
	})
	if err != nil {
		return false, err
	}
	
	return resp.IsLimited, nil
}

func checkAccountLockViaGRPC(ctx context.Context, userID string) (*pb.CheckAccountLockStatusResponse, error) {
	client := pb.NewSecurityAuditServiceClient(getGRPCConnection())
	
	return client.CheckAccountLockStatus(ctx, &pb.CheckAccountLockStatusRequest{
		UserId: userID,
	})
}

func getUserAuditHistoryViaGRPC(ctx context.Context, userID string, limit int) ([]*pb.SecurityAuditLog, error) {
	client := pb.NewSecurityAuditServiceClient(getGRPCConnection())
	
	resp, err := client.GetUserAuditHistory(ctx, &pb.GetUserAuditHistoryRequest{
		UserId: userID,
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, err
	}
	
	return resp.Logs, nil
}

// Batch check rate limits
func checkRateLimitsBatch(ctx context.Context, checks []*pb.RateLimitCheck) ([]*pb.RateLimitResult, error) {
	client := pb.NewSecurityAuditServiceClient(getGRPCConnection())
	
	resp, err := client.CheckRateLimits(ctx, &pb.CheckRateLimitsRequest{
		Checks: checks,
	})
	if err != nil {
		return nil, err
	}
	
	return resp.Results, nil
}

// ================================
// CONFIGURATION
// ================================

type Config struct {
	DatabaseURL   string
	GRPCAddress   string
	HTTPAddress   string
	KafkaBrokers  []string
}

func loadConfig() Config {
	return Config{
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://localhost/auth_db"),
		GRPCAddress:  getEnv("GRPC_ADDRESS", ":50051"),
		HTTPAddress:  getEnv("HTTP_ADDRESS", ":8080"),
		KafkaBrokers: getEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}

// ================================
// HELPER FUNCTIONS
// ================================

func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}
	return ip
}

func getRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value("request_id").(string); ok {
		return requestID
	}
	return generateRequestID()
}

func getUserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value("user_id").(string); ok {
		return userID
	}
	return ""
}

func getSessionID(ctx context.Context) string {
	if sessionID, ok := ctx.Value("session_id").(string); ok {
		return sessionID
	}
	return ""
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func extractActivityTypes(activities []*domain.SuspiciousActivity) []string {
	types := make([]string, 0, len(activities))
	for _, activity := range activities {
		types = append(types, activity.ActivityType)
	}
	return types
}

func calculateTotalRiskScore(activities []*domain.SuspiciousActivity) int {
	total := 0
	for _, activity := range activities {
		total += activity.RiskScore
	}
	if total > 100 {
		total = 100
	}
	return total
}

// Placeholder functions (implement these based on your auth logic)
func initGeoIPService() service.GeoIPService {
	// Initialize your GeoIP service (e.g., MaxMind, IP2Location)
	return nil
}

func authenticateUser(ctx context.Context, email, password string) (*domain.User, error) {
	// Your authentication logic
	return nil, nil
}

func changeUserPassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	// Your password change logic
	return nil
}

func suspendUserAccount(ctx context.Context, userID string) error {
	// Your suspend account logic
	return nil
}

func generateSessionID() string {
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}

func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

func generateToken(user *domain.User) string {
	// Your JWT token generation logic
	return "jwt_token_here"
}

func getGRPCConnection() *grpcServer.ClientConn {
	// Return your gRPC client connection (should be initialized once and reused)
	return nil
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// ================================
// MIDDLEWARE EXAMPLE
// ================================

func SecurityAuditMiddleware(kafkaProducer *kafka.SecurityAuditProducer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Generate request ID
			requestID := generateRequestID()
			ctx := context.WithValue(r.Context(), "request_id", requestID)
			r = r.WithContext(ctx)
			
			// Call next handler
			next.ServeHTTP(w, r)
			
			// Log request after completion
			duration := time.Since(start)
			
			// Publish security event for sensitive endpoints
			if isSensitiveEndpoint(r.URL.Path) {
				go func() {
					ipAddress := getClientIP(r)
					userAgent := r.UserAgent()
					
					msg := &kafka.SecurityEventMessage{
						EventType:   "api_access",
						Severity:    domain.SeverityInfo,
						IPAddress:   &ipAddress,
						UserAgent:   &userAgent,
						RequestID:   &requestID,
						Description: stringPtr(fmt.Sprintf("Access to %s %s", r.Method, r.URL.Path)),
						Metadata: map[string]interface{}{
							"method":   r.Method,
							"path":     r.URL.Path,
							"duration": duration.Milliseconds(),
						},
						Timestamp: time.Now(),
					}
					
					kafkaProducer.PublishSecurityEvent(context.Background(), msg)
				}()
			}
		})
	}
}

func isSensitiveEndpoint(path string) bool {
	sensitivePatterns := []string{
		"/api/auth/login",
		"/api/auth/password",
		"/api/admin/",
		"/api/users/delete",
	}
	
	for _, pattern := range sensitivePatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// Required imports
import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	
	"github.com/go-chi/chi/v5"
)

// LoginRequest and ChangePasswordRequest structs
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}