# P2P Trading Service

A real-time peer-to-peer cryptocurrency trading platform with WebSocket communication, built with Go.

---

## 📑 Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Features](#features)
- [Technology Stack](#technology-stack)
- [Database Schema](#database-schema)
- [Getting Started](#getting-started)
- [API Documentation](#api-documentation)
- [WebSocket Protocol](#websocket-protocol)
- [Project Structure](#project-structure)
- [Development Guide](#development-guide)
- [Security](#security)
- [Contributing](#contributing)

---

## 🎯 Overview

The P2P Trading Service enables users to trade cryptocurrencies directly with each other in a secure, escrow-protected environment. The service uses WebSocket for real-time communication, ensuring instant updates for trades, chats, and notifications.

### Key Capabilities

- **Real-time Trading**: WebSocket-based instant communication
- **Escrow Protection**: Automated crypto escrow for secure transactions
- **Multi-currency Support**: Trade various cryptocurrencies and fiat pairs
- **Dispute Resolution**: Built-in dispute management system
- **Rating System**: User reputation based on completed trades
- **Payment Methods**: Multiple payment method support (Bank Transfer, Mobile Money, etc.)
- **Admin Controls**: Comprehensive admin tools for moderation

---

## 🏗️ Architecture

### System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Client Layer                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ Web Client   │  │ Mobile App   │  │ Admin Panel  │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         │                  │                  │              │
│         └──────────────────┼──────────────────┘              │
│                            │                                 │
└────────────────────────────┼─────────────────────────────────┘
                             │
                    ┌────────▼────────┐
                    │   API Gateway   │
                    │   (Auth Layer)  │
                    └────────┬────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
┌───────▼────────┐  ┌────────▼────────┐  ┌───────▼────────┐
│  REST Handler  │  │  WebSocket Hub  │  │  gRPC Handler  │
│  (Profile Mgmt)│  │  (Real-time)    │  │  (Admin API)   │
└───────┬────────┘  └────────┬────────┘  └───────┬────────┘
        │                    │                    │
        └────────────────────┼────────────────────┘
                             │
                    ┌────────▼────────┐
                    │  Business Logic │
                    │    (Usecases)   │
                    └────────┬────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
┌───────▼────────┐  ┌────────▼────────┐  ┌───────▼────────┐
│  Profile Repo  │  │   Order Repo    │  │   Chat Repo    │
│                │  │                 │  │                │
└───────┬────────┘  └────────┬────────┘  └───────┬────────┘
        │                    │                    │
        └────────────────────┼────────────────────┘
                             │
                    ┌────────▼────────┐
                    │  PostgreSQL DB  │
                    │   (pgxpool)     │
                    └─────────────────┘
```

### Component Overview

#### 1. **API Layer**
- **REST Endpoints**: Profile creation, updates, and queries
- **WebSocket Server**: Real-time bidirectional communication
- **gRPC Server**: Admin operations and service-to-service calls

#### 2. **Business Logic Layer**
- **Usecases**: Business logic and workflow orchestration
- **Validation**: Input validation and business rules
- **Event Handling**: Order state management and notifications

#### 3. **Data Layer**
- **Repositories**: Data access abstraction
- **PostgreSQL**: Primary data store with pgxpool
- **Redis**: Caching and session management (future)

---

## ✨ Features

### Current Features (v1.0)

- [x] User Profile Management
  - Profile creation with consent requirement
  - Profile updates (username, contact info, preferences)
  - Trading statistics tracking
  - Verification and merchant status
  - Profile suspension management

- [x] WebSocket Communication
  - Real-time connection with automatic reconnection
  - Profile operations over WebSocket
  - Concurrent connection management
  - Client heartbeat/ping-pong

### Upcoming Features

- [ ] Advertisement System
  - Create buy/sell ads
  - Price types (fixed/floating)
  - Payment method configuration
  - Ad visibility controls

- [ ] Order Management
  - Place orders on ads
  - Escrow integration
  - Payment confirmation
  - Crypto release mechanism
  - Order expiration

- [ ] Chat System
  - Order-specific chat rooms
  - File attachments (proof of payment)
  - System messages
  - Read receipts

- [ ] Dispute Resolution
  - Raise disputes
  - Evidence submission
  - Admin review workflow
  - Automated resolution rules

- [ ] Review & Rating System
  - Post-trade reviews
  - Rating calculation
  - Review moderation

- [ ] Notification System
  - Real-time notifications
  - Email notifications
  - Push notifications

---

## 🛠️ Technology Stack

| Component | Technology |
|-----------|-----------|
| **Language** | Go 1.21+ |
| **Web Framework** | Chi Router |
| **WebSocket** | Gorilla WebSocket |
| **Database** | PostgreSQL 15+ |
| **DB Driver** | pgx/v5 (pgxpool) |
| **Logging** | Zap |
| **Authentication** | JWT (shared auth middleware) |
| **API Protocol** | REST + WebSocket + gRPC |

---

## 🗄️ Database Schema

### Core Tables

#### **p2p_profiles**
User profiles for P2P trading with statistics and status tracking.

```sql
CREATE TABLE p2p_profiles (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL UNIQUE,
    username VARCHAR(100) UNIQUE,
    phone_number VARCHAR(20),
    email VARCHAR(255),
    profile_picture_url VARCHAR(500),
    
    -- Trading stats
    total_trades INT DEFAULT 0,
    completed_trades INT DEFAULT 0,
    cancelled_trades INT DEFAULT 0,
    avg_rating NUMERIC(3, 2) DEFAULT 0.00,
    total_reviews INT DEFAULT 0,
    
    -- Status
    is_verified BOOLEAN DEFAULT FALSE,
    is_merchant BOOLEAN DEFAULT FALSE,
    is_suspended BOOLEAN DEFAULT FALSE,
    suspension_reason TEXT,
    suspended_until TIMESTAMPTZ,
    
    -- Consent
    has_consent BOOLEAN DEFAULT FALSE,
    consented_at TIMESTAMPTZ,
    
    -- Preferences
    preferred_currency VARCHAR(10),
    preferred_payment_methods JSONB,
    auto_reply_message TEXT,
    
    -- Metadata
    metadata JSONB,
    last_active_at TIMESTAMPTZ,
    joined_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Indexes

```sql
CREATE INDEX idx_p2p_profiles_user_id ON p2p_profiles(user_id);
CREATE INDEX idx_p2p_profiles_username ON p2p_profiles(username) WHERE username IS NOT NULL;
CREATE INDEX idx_p2p_profiles_verified ON p2p_profiles(is_verified) WHERE is_verified = TRUE;
CREATE INDEX idx_p2p_profiles_merchant ON p2p_profiles(is_merchant) WHERE is_merchant = TRUE;
CREATE INDEX idx_p2p_profiles_suspended ON p2p_profiles(is_suspended) WHERE is_suspended = TRUE;
CREATE INDEX idx_p2p_profiles_consent ON p2p_profiles(has_consent) WHERE has_consent = TRUE;
```

---

## 🚀 Getting Started

### Prerequisites

- Go 1.21 or higher
- PostgreSQL 15+
- Redis 7+ (optional, for future features)
- Git

### Installation

1. **Clone the repository**
```bash
git clone https://github.com/your-org/p2p-service.git
cd p2p-service
```

2. **Install dependencies**
```bash
go mod download
```

3. **Set up environment variables**
```bash
cp .env.example .env
# Edit .env with your configuration
```

4. **Run database migrations**
```bash
psql -U postgres -d pxyz_fx_p2p -f migrations/001_init_schema.sql
```

5. **Start the service**
```bash
go run cmd/main.go
```

The service will start on `http://localhost:8030` by default.

---

## 📚 API Documentation

### Base URL
```
http://localhost:8030
```

### Authentication

All authenticated endpoints require a JWT token in the Authorization header:

```http
Authorization: Bearer <your-jwt-token>
```

The middleware extracts `user_id` from the token and adds it to the request context.

---

### REST Endpoints

#### 1. Health Check

**GET** `/p2p/health`

Check service status.

**Response:**
```
P2P service is running
```

---

#### 2. Check Profile Status

**GET** `/api/p2p/profile/check`

Check if the authenticated user has a P2P profile and can connect to WebSocket.

**Headers:**
```http
Authorization: Bearer <token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "exists": true,
    "profile_id": 123,
    "has_consent": true,
    "is_suspended": false,
    "can_connect": true,
    "profile": {
      "id": 123,
      "user_id": "user_abc123",
      "username": "trader_pro",
      "total_trades": 15,
      "completed_trades": 14,
      "avg_rating": 4.8,
      "is_verified": true,
      "is_merchant": false
    }
  }
}
```

**Response (No Profile):**
```json
{
  "success": true,
  "data": {
    "exists": false,
    "has_consent": false,
    "can_connect": false,
    "message": "You have not joined P2P trading yet."
  }
}
```

---

#### 3. Create P2P Profile

**POST** `/api/p2p/profile/create`

Create a new P2P profile. User must accept terms and conditions.

**Headers:**
```http
Authorization: Bearer <token>
Content-Type: application/json
```

**Request Body:**
```json
{
  "username": "trader123",
  "phone_number": "+254712345678",
  "email": "trader@example.com",
  "preferred_currency": "KES",
  "preferred_payment_methods": [1, 2, 3],
  "auto_reply_message": "Hello! I'm available to trade.",
  "has_consent": true
}
```

**Field Descriptions:**
- `username` (optional): Unique username (3-30 chars, alphanumeric + underscore)
- `phone_number` (optional): Contact phone number
- `email` (optional): Contact email
- `preferred_currency` (optional): Default fiat currency (KES, USD, etc.)
- `preferred_payment_methods` (optional): Array of payment method IDs
- `auto_reply_message` (optional): Auto-reply message for trades
- `has_consent` (required): Must be `true` to create profile

**Response:**
```json
{
  "success": true,
  "data": {
    "profile": {
      "id": 123,
      "user_id": "user_abc123",
      "username": "trader123",
      "phone_number": "+254712345678",
      "email": "trader@example.com",
      "total_trades": 0,
      "completed_trades": 0,
      "avg_rating": 0,
      "is_verified": false,
      "is_merchant": false,
      "has_consent": true,
      "consented_at": "2025-01-15T10:30:00Z",
      "joined_at": "2025-01-15T10:30:00Z",
      "created_at": "2025-01-15T10:30:00Z"
    }
  },
  "message": "P2P profile created successfully. You can now connect to the P2P trading platform."
}
```

**Error Response (No Consent):**
```json
{
  "success": false,
  "error": "You must accept the terms and conditions to create a P2P profile"
}
```

**Error Response (Profile Exists):**
```json
{
  "success": false,
  "error": "You already have a P2P profile"
}
```

---

#### 4. Get Profile

**GET** `/api/p2p/profile`

Retrieve the authenticated user's P2P profile.

**Headers:**
```http
Authorization: Bearer <token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "profile": {
      "id": 123,
      "user_id": "user_abc123",
      "username": "trader123",
      "phone_number": "+254712345678",
      "email": "trader@example.com",
      "profile_picture_url": "https://example.com/avatar.jpg",
      "total_trades": 15,
      "completed_trades": 14,
      "cancelled_trades": 1,
      "avg_rating": 4.8,
      "total_reviews": 12,
      "is_verified": true,
      "is_merchant": false,
      "is_suspended": false,
      "preferred_currency": "KES",
      "has_consent": true,
      "last_active_at": "2025-01-15T14:20:00Z",
      "joined_at": "2025-01-10T09:00:00Z",
      "created_at": "2025-01-10T09:00:00Z",
      "updated_at": "2025-01-15T10:30:00Z"
    }
  }
}
```

---

#### 5. Update Profile

**PUT** `/api/p2p/profile`

Update the authenticated user's P2P profile.

**Headers:**
```http
Authorization: Bearer <token>
Content-Type: application/json
```

**Request Body:**
```json
{
  "username": "new_trader_name",
  "phone_number": "+254798765432",
  "email": "newemail@example.com",
  "profile_picture_url": "https://example.com/new-avatar.jpg",
  "preferred_currency": "USD",
  "preferred_payment_methods": [1, 3, 5],
  "auto_reply_message": "Updated auto-reply message"
}
```

**Note:** All fields are optional. Only send fields you want to update.

**Response:**
```json
{
  "success": true,
  "data": {
    "profile": {
      "id": 123,
      "user_id": "user_abc123",
      "username": "new_trader_name",
      "phone_number": "+254798765432",
      "email": "newemail@example.com",
      "updated_at": "2025-01-15T11:00:00Z"
    }
  },
  "message": "Profile updated successfully"
}
```

---

## 🔌 WebSocket Protocol

### Connection

**WebSocket URL:**
```
ws://localhost:8030/api/p2p/ws
```

**Authentication:**

Include JWT token in the connection request:

**Option 1: Query Parameter**
```javascript
const token = 'your-jwt-token';
const ws = new WebSocket(`ws://localhost:8030/api/p2p/ws?token=${token}`);
```

**Option 2: Header (if supported by client)**
```javascript
const ws = new WebSocket('ws://localhost:8030/api/p2p/ws', {
  headers: {
    'Authorization': `Bearer ${token}`
  }
});
```

---

### Connection Flow

```
1. Client connects with JWT token
2. Server validates token and extracts user_id
3. Server checks if user has P2P profile
4. Server checks if user has given consent
5. If valid, connection is upgraded to WebSocket
6. Server sends "connected" message
7. Client can now send/receive messages
```

---

### Connection Events

#### **Connected (Server → Client)**

Sent immediately after successful connection.

```json
{
  "type": "connected",
  "success": true,
  "data": {
    "profile_id": 123,
    "user_id": "user_abc123",
    "username": "trader123"
  },
  "message": "Connected to P2P service"
}
```

---

#### **Connection Error (Server → Client)**

Sent if connection is rejected.

**Error: Profile Not Found**
```json
{
  "type": "connection_error",
  "success": false,
  "error": "NOT_JOINED_P2P",
  "message": "You have not joined P2P trading yet. Please create a P2P profile first."
}
```

**Error: Consent Required**
```json
{
  "type": "connection_error",
  "success": false,
  "error": "CONSENT_REQUIRED",
  "message": "You must accept the P2P trading terms and conditions before connecting."
}
```

*Note: Connection will be closed after sending error message.*

---

### Message Format

All messages follow this structure:

**Client → Server:**
```json
{
  "type": "message_type",
  "data": {
    // Message-specific data
  }
}
```

**Server → Client:**
```json
{
  "type": "message_type",
  "success": true,
  "data": {
    // Response data
  },
  "error": "error_message", // Only if success is false
  "message": "Human-readable message"
}
```

---

### Available Messages (Current Implementation)

#### 1. Get Profile

**Client → Server:**
```json
{
  "type": "profile.get",
  "data": {
    "profile_id": 123
  }
}
```

**Optional:** Leave `data` empty to get your own profile.

**Server → Client:**
```json
{
  "type": "profile.get",
  "success": true,
  "data": {
    "id": 123,
    "user_id": "user_abc123",
    "username": "trader123",
    "total_trades": 15,
    "completed_trades": 14,
    "avg_rating": 4.8,
    "is_verified": true
  }
}
```

---

#### 2. Update Profile

**Client → Server:**
```json
{
  "type": "profile.update",
  "data": {
    "username": "new_username",
    "phone_number": "+254712345678",
    "auto_reply_message": "Available for trades"
  }
}
```

**Server → Client:**
```json
{
  "type": "profile.update",
  "success": true,
  "data": {
    "id": 123,
    "username": "new_username",
    "phone_number": "+254712345678",
    "updated_at": "2025-01-15T12:00:00Z"
  },
  "message": "Profile updated successfully"
}
```

---

#### 3. Get Profile Stats

**Client → Server:**
```json
{
  "type": "profile.stats",
  "data": {
    "profile_id": 123
  }
}
```

**Server → Client:**
```json
{
  "type": "profile.stats",
  "success": true,
  "data": {
    "total_trades": 15,
    "completed_trades": 14,
    "cancelled_trades": 1,
    "completion_rate": 93.33,
    "avg_rating": 4.8,
    "total_reviews": 12
  }
}
```

---

#### 4. Search Profiles

**Client → Server:**
```json
{
  "type": "profile.search",
  "data": {
    "search": "trader",
    "is_verified": true,
    "min_rating": 4.0,
    "limit": 20,
    "offset": 0
  }
}
```

**Server → Client:**
```json
{
  "type": "profile.search",
  "success": true,
  "data": {
    "profiles": [
      {
        "id": 123,
        "username": "trader123",
        "total_trades": 50,
        "avg_rating": 4.9,
        "is_verified": true,
        "is_merchant": true
      }
    ],
    "total": 1
  }
}
```

---

### Placeholder Messages (Coming Soon)

These message types are defined but not yet implemented:

#### **Ads**
- `ad.create` - Create a buy/sell ad
- `ad.list` - List available ads
- `ad.update` - Update your ad
- `ad.delete` - Delete your ad
- `ad.my_ads` - Get your ads

#### **Orders**
- `order.create` - Place an order on an ad
- `order.list` - List your orders
- `order.get` - Get order details
- `order.cancel` - Cancel an order
- `order.confirm_payment` - Confirm payment made
- `order.release_crypto` - Release crypto to buyer

#### **Chat**
- `chat.send_message` - Send chat message
- `chat.get_messages` - Get chat history
- `chat.mark_read` - Mark messages as read

#### **Disputes**
- `dispute.raise` - Raise a dispute
- `dispute.get` - Get dispute details

#### **Reviews**
- `review.create` - Submit a review
- `review.list` - List reviews

#### **Notifications**
- `notification.list` - Get notifications
- `notification.mark_read` - Mark notification as read

**Response for unimplemented features:**
```json
{
  "type": "ad.create",
  "success": false,
  "message": "Ad creation not yet implemented"
}
```

---

### Error Handling

**Invalid Message Format:**
```json
{
  "type": "error",
  "success": false,
  "error": "Invalid message format"
}
```

**Unknown Message Type:**
```json
{
  "type": "error",
  "success": false,
  "error": "Unknown message type: xyz"
}
```

**Operation Failed:**
```json
{
  "type": "profile.update",
  "success": false,
  "error": "Failed to update profile: username already taken"
}
```

---

### Heartbeat / Keep-Alive

The WebSocket server automatically sends **ping** frames every 54 seconds. Clients should respond with **pong** frames to maintain the connection.

**JavaScript Example:**
```javascript
ws.addEventListener('ping', () => {
  ws.pong(); // Most libraries handle this automatically
});
```

**Connection Timeout:** 60 seconds of inactivity will close the connection.

---

### Example Client Implementation

```javascript
class P2PClient {
  constructor(token) {
    this.token = token;
    this.ws = null;
    this.messageHandlers = {};
  }

  connect() {
    this.ws = new WebSocket(`ws://localhost:8030/api/p2p/ws?token=${this.token}`);

    this.ws.onopen = () => {
      console.log('✅ Connected to P2P service');
    };

    this.ws.onmessage = (event) => {
      const message = JSON.parse(event.data);
      console.log('📨 Received:', message);

      // Handle based on type
      if (this.messageHandlers[message.type]) {
        this.messageHandlers[message.type](message);
      }
    };

    this.ws.onerror = (error) => {
      console.error('❌ WebSocket error:', error);
    };

    this.ws.onclose = () => {
      console.log('🔌 Disconnected from P2P service');
      // Implement reconnection logic here
      setTimeout(() => this.connect(), 5000);
    };
  }

  send(type, data = {}) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type, data }));
    } else {
      console.error('WebSocket is not connected');
    }
  }

  on(type, handler) {
    this.messageHandlers[type] = handler;
  }

  // Convenience methods
  getProfile(profileId) {
    this.send('profile.get', { profile_id: profileId });
  }

  updateProfile(updates) {
    this.send('profile.update', updates);
  }

  searchProfiles(filters) {
    this.send('profile.search', filters);
  }
}

// Usage
const client = new P2PClient('your-jwt-token');

client.on('connected', (msg) => {
  console.log('Connected!', msg.data);
});

client.on('profile.get', (msg) => {
  if (msg.success) {
    console.log('Profile:', msg.data);
  }
});

client.connect();

// Get own profile
client.getProfile();

// Search for verified traders
client.searchProfiles({
  is_verified: true,
  min_rating: 4.0,
  limit: 10
});
```

---

## 📁 Project Structure

```
p2p-service/
├── cmd/
│   └── main.go                    # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go              # Configuration management
│   ├── domain/
│   │   ├── p2p_profile.go         # Profile domain models
│   │   ├── p2p_ad.go              # Ad domain models (TODO)
│   │   ├── p2p_order.go           # Order domain models (TODO)
│   │   └── p2p_chat.go            # Chat domain models (TODO)
│   ├── repository/
│   │   ├── p2p_profile_repository.go  # Profile data access
│   │   ├── p2p_ad_repository.go       # Ad data access (TODO)
│   │   └── p2p_order_repository.go    # Order data access (TODO)
│   ├── usecase/
│   │   ├── p2p_profile_usecase.go     # Profile business logic
│   │   ├── p2p_ad_usecase.go          # Ad business logic (TODO)
│   │   └── p2p_order_usecase.go       # Order business logic (TODO)
│   ├── handler/
│   │   ├── p2p_rest_handler.go        # REST API handlers
│   │   ├── p2p_websocket_handler.go   # WebSocket handlers
│   │   ├── p2p_profile_ws.go          # Profile WS operations
│   │   ├── p2p_client.go              # Client connection management
│   │   └── p2p_admin_grpc_handler.go  # Admin gRPC handlers
│   ├── router/
│   │   └── router.go                  # Route definitions
│   └── server/
│       └── server.go                  # Server initialization
├── migrations/
│   ├── 001_init_schema.sql           # Initial schema
│   └── 002_add_consent.sql           # Consent fields
├── pkg/
│   └── utils/
│       └── p2p_helpers.go            # Utility functions
├── .env.example                       # Environment template
├── .gitignore
├── go.mod
├── go.sum
└── README.md
```

---

## 🔧 Development Guide

### Adding New Features

1. **Define Domain Models**
```go
// internal/domain/p2p_ad.go
type P2PAd struct {
    ID          int64
    ProfileID   int64
    AdType      string // "buy" or "sell"
    AssetCode   string
    // ... other fields
}
```

2. **Create Repository**
```go
// internal/repository/p2p_ad_repository.go
type P2PAdRepository struct {
    pool   *pgxpool.Pool
    logger *zap.Logger
}

func (r *P2PAdRepository) Create(ctx context.Context, ad *domain.P2PAd) error {
    // Implementation
}
```

3. **Create Usecase**
```go
// internal/usecase/p2p_ad_usecase.go
type P2PAdUsecase struct {
    adRepo *repository.P2PAdRepository
    logger *zap.Logger
}

func (uc *P2PAdUsecase) CreateAd(ctx context.Context, req *CreateAdRequest) (*domain.P2PAd, error) {
    // Business logic
}
```

4. **Add WebSocket Handlers**
```go
// internal/handler/p2p_websocket_handler.go
func (h *P2PWebSocketHandler) handleCreateAd(ctx context.Context, client *Client, data json.RawMessage) {
    // Parse request
    // Call usecase
    // Send response
}
```

5. **Update Message Router**
```go
case "ad.create":
    h.handleCreateAd(ctx, client, msg.Data)
```

---

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/usecase/...

# Run with verbose output
go test -v ./...
```

---

### Database Migrations

```bash
# Apply migration
psql -U postgres -d pxyz_fx_p2p -f migrations/002_new_migration.sql

# Rollback (if you created a down migration)
psql -U postgres -d pxyz_fx_p2p -f migrations/002_new_migration_down.sql
```

---

## 🔒 Security

### Authentication
- All endpoints require valid JWT token
- User ID extracted from token context
- No ability to impersonate other users

### Consent Management
- Users must explicitly consent to terms
- Consent tracked with timestamp
- Cannot connect to WebSocket without consent

### Profile Privacy
- Users can only update their own profiles
- Admin operations separate from user operations
- Sensitive data not exposed in public APIs

### WebSocket Security
- Connection upgrade requires authentication
- Profile existence validated before connection
- Automatic disconnection on suspension
- Message validation on every request

### Rate Limiting
- Global rate limiting (100 req/min)
- Per-user rate limiting (future)
- WebSocket message throttling (future)

---

## 🤝 Contributing

### Code Style

- Follow Go best practices
- Use `gofmt` for formatting
- Add comments for exported functions
- Write tests for new features

### Commit Messages

```
feat: add order creation functionality
fix: resolve profile update race condition
docs: update WebSocket protocol documentation
refactor: simplify profile repository queries
```

### Pull Request Process

1. Create a feature branch
2. Implement changes with tests
3. Update documentation
4. Submit PR with description
5. Address review comments
6. Squash and merge

---

## 📄 License

This project is proprietary software. All rights reserved.

---

## 📞 Support

For questions or issues:
- **Email**: support@example.com
- **Slack**: #p2p-service
- **Documentation**: https://docs.example.com/p2p

---

## 🗺️ Roadmap

### Q1 2025
- [x] Profile management
- [x] WebSocket infrastructure
- [ ] Advertisement system
- [ ] Order placement

### Q2 2025
- [ ] Chat system
- [ ] Dispute resolution
- [ ] Review system
- [ ] Notification system

### Q3 2025
- [ ] Mobile app support
- [ ] Advanced filtering
- [ ] Analytics dashboard
- [ ] Multi-language support

### Q4 2025
- [ ] Escrow automation
- [ ] Price oracle integration
- [ ] Advanced security features
- [ ] API rate limiting improvements

---

**Built with ❤️ by the P2P Team**