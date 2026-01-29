# Cashier Module - WebSocket API Documentation

## Table of Contents
1. [Connection](#connection)
2. [Message Format](#message-format)
3. [Verification & Authentication](#verification--authentication)
4. [Account Operations](#account-operations)
5. [Deposit Operations](#deposit-operations)
6. [Withdrawal Operations](#withdrawal-operations)
7. [Transfer Operations](#transfer-operations)
8. [Transaction History](#transaction-history)
9. [Statements & Reports](#statements--reports)
10. [Fee Calculation](#fee-calculation)
11. [Currency Conversion](#currency-conversion)
12. [Partner Operations](#partner-operations)
13. [Event Notifications](#event-notifications)

---

## Connection

### WebSocket Endpoint
```
wss://api.safarigari.com/api/v1/cashier/svc/ws
```

### Authentication
Connection requires authentication via JWT token in request context.

### Connection Response
```json
{
  "type": "connected",
  "data": {
    "user_id": "12345",
    "timestamp": 1706400000
  },
  "timestamp": 1706400000
}
```

---

## Message Format

### Request Structure
All client messages follow this format:
```json
{
  "type": "message_type",
  "data": {
    // request-specific data
  }
}
```

### Response Structure
All server responses follow this format:
```json
{
  "type": "success" | "error",
  "data": {
    "message": "Operation result message",
    "data": {
      // response-specific data
    }
  },
  "timestamp": 1706400000
}
```

### Error Response
```json
{
  "type": "error",
  "data": {
    "message": "Error description"
  },
  "timestamp": 1706400000
}
```

---

## Verification & Authentication

### 1. Request Verification

#### Message Type: `verification_request`

**Purpose**: Initiate verification process for sensitive operations (withdrawal, transfer, etc.)

**Request**:
```json
{
  "type": "verification_request",
  "data": {
    "method": "totp" | "otp_email" | "otp_sms" | "otp_whatsapp" | "auto or empty" ,
    "purpose": "withdrawal" | "transfer" | "sensitive_operation" | "for now just withdrawal"
  }
}
```

**Response (TOTP)**:
```json
{
  "type": "success",
  "data": {
    "message": "2FA enabled. Please provide your TOTP code",
    "data": {
      "method": "totp",
      "purpose": "withdrawal",
      "next_step": "verify_totp"
    }
  },
  "timestamp": 1706400000
}
```

**Response (OTP)**:
```json
{
  "type": "success",
  "data": {
    "message": "OTP sent successfully",
    "data": {
      "method": "otp_sms",
      "channel": "sms",
      "purpose": "withdrawal",
      "masked_recipient": "***1234",
      "next_step": "verify_otp",
      "expires_in": 180
    }
  },
  "timestamp": 1706400000
}
```

**Auto-Selection Logic**:
1. Check if 2FA/TOTP is enabled → use TOTP
2. Check if phone number exists → use SMS
3. Check if email exists → use Email
4. If none available → return error

---

### 2. Verify TOTP

#### Message Type: `verify_totp`

**Request**:
```json
{
  "type": "verify_totp",
  "data": {
    "code": "123456",
    "purpose": "withdrawal"
  }
}
```

**Success Response**:
```json
{
  "type": "success",
  "data": {
    "message": "verification successful",
    "data": {
      "verification_token": "a1b2c3d4e5f6...",
      "purpose": "withdrawal",
      "method": "totp",
      "expires_in": 300,
      "message": "Use this token for your next withdrawal request"
    }
  },
  "timestamp": 1706400000
}
```

**Error Response**:
```json
{
  "type": "error",
  "data": {
    "message": "invalid TOTP code"
  },
  "timestamp": 1706400000
}
```

---

### 3. Verify OTP

#### Message Type: `verify_otp`

**Request**:
```json
{
  "type": "verify_otp",
  "data": {
    "code": "123456",
    "purpose": "withdrawal"
  }
}
```

**Success Response**:
```json
{
  "type": "success",
  "data": {
    "message": "verification successful",
    "data": {
      "verification_token": "a1b2c3d4e5f6...",
      "purpose": "withdrawal",
      "method": "otp_sms",
      "expires_in": 300,
      "message": "Use this token for your next withdrawal request"
    }
  },
  "timestamp": 1706400000
}
```

---

## Account Operations

### 1. Get Accounts

#### Message Type: `get_accounts`

**Request**:
```json
{
  "type": "get_accounts",
  "data": {}
}
```

**Response**:
```json
{
  "type": "success",
  "data": {
    "message": "accounts retrieved",
    "data": {
      "accounts": [
        {
          "id": 123,
          "account_number": "ACC-USD-12345",
          "currency": "USD",
          "purpose": "WALLET",
          "account_type": "REAL",
          "is_active": true,
          "is_locked": false,
          "created_at": "2024-01-01T00:00:00Z"
        }
      ],
      "count": 1
    }
  },
  "timestamp": 1706400000
}
```

---

### 2. Get Account Balance

#### Message Type: `get_account_balance`

**Request**:
```json
{
  "type": "get_account_balance",
  "data": {
    "account_number": "ACC-USD-12345"
  }
}
```

**Response**:
```json
{
  "type": "success",
  "data": {
    "message": "balance retrieved",
    "data": {
      "account_number": "ACC-USD-12345",
      "balance": 1000.50,
      "available_balance": 950.50,
      "pending_debit": 50.00,
      "pending_credit": 0.00,
      "currency": "USD",
      "last_transaction": "2024-01-15T10:30:00Z"
    }
  },
  "timestamp": 1706400000
}
```

---

### 3. Get Owner Summary

#### Message Type: `get_owner_summary`

**Request**:
```json
{
  "type": "get_owner_summary",
  "data": {}
}
```

**Response**:
```json
{
  "type": "success",
  "data": {
    "message": "owner summary",
    "data": {
      "account_balances": [
        {
          "account_number": "ACC-USD-12345",
          "currency": "USD",
          "balance": 1000.50,
          "available_balance": 950.50
        },
        {
          "account_number": "ACC-BTC-12345",
          "currency": "BTC",
          "balance": 0.05,
          "available_balance": 0.05
        }
      ],
      "total_balance_usd": 3500.75,
      "total_accounts": 2
    }
  },
  "timestamp": 1706400000
}
```

---

### 4. Create Account

#### Message Type: `create_account`

**Request**:
```json
{
  "type": "create_account",
  "data": {
    "currency": "BTC",
    "account_type": "real" | "demo"
  }
}
```

**Response**:
```json
{
  "type": "success",
  "data": {
    "message": "account created successfully",
    "data": {
      "account": {
        "id": 456,
        "account_number": "ACC-BTC-67890",
        "currency": "BTC",
        "purpose": "WALLET",
        "account_type": "REAL",
        "is_active": true,
        "is_locked": false,
        "created_at": "2024-01-27T12:00:00Z"
      },
      "message": "BTC REAL wallet account created"
    }
  },
  "timestamp": 1706400000
}
```

**Error Response**:
```json
{
  "type": "error",
  "data": {
    "message": "BTC REAL wallet account already exists: ACC-BTC-67890"
  },
  "timestamp": 1706400000
}
```

---

### 5. Get Supported Currencies

#### Message Type: `get_supported_currencies`

**Request**:
```json
{
  "type": "get_supported_currencies",
  "data": {}
}
```

**Response**:
```json
{
  "type": "success",
  "data": {
    "message": "supported currencies",
    "data": {
      "currencies": [
        {
          "code": "USD",
          "name": "US Dollar",
          "symbol": "$",
          "type": "fiat"
        },
        {
          "code": "BTC",
          "name": "Bitcoin",
          "symbol": "₿",
          "type": "crypto"
        },
        {
          "code": "USDT",
          "name": "Tether",
          "symbol": "₮",
          "type": "crypto"
        }
      ],
      "count": 3
    }
  },
  "timestamp": 1706400000
}
```

---

## Deposit Operations

### Deposit Flow Overview

```
1. User initiates deposit → deposit_request
2. System determines deposit type (partner/agent/crypto)
3. Partner: STK push sent → awaiting confirmation
4. Agent: Request created → agent fulfills via transfer
5. Crypto: Wallet address returned → awaiting blockchain confirmation
6. Deposit completed → deposit_completed notification
```

### Message Type: `deposit_request`

**Partner Deposit Request (M-Pesa)**:
```json
{
  "type": "deposit_request",
  "data": {
    "amount": 1000,
    "local_currency": "KES",
    "target_currency": "USD",
    "service": "mpesa",
    "partner_id": "safaricom_mpesa"
  }
}
```

**Partner Deposit Response**:
```json
{
  "type": "success",
  "data": {
    "message": "deposit request created",
    "data": {
      "request_ref": "DEP-KES-20240127-001",
      "amount_local": 1000.00,
      "local_currency": "KES",
      "amount_usd": 7.50,
      "exchange_rate": 133.33,
      "status": "pending_stk_push",
      "partner": {
        "id": "safaricom_mpesa",
        "name": "Safaricom M-Pesa"
      },
      "phone_number": "254712***890",
      "instructions": "Please enter your M-Pesa PIN to complete the deposit"
    }
  },
  "timestamp": 1706400000
}
```

**Agent Deposit Request**:
```json
{
  "type": "deposit_request",
  "data": {
    "amount": 100,
    "local_currency": "USD",
    "service": "agent",
    "agent_id": "AGT-001"
  }
}
```

**Agent Deposit Response**:
```json
{
  "type": "success",
  "data": {
    "message": "deposit request created",
    "data": {
      "request_ref": "DEP-USD-20240127-002",
      "amount": 100.00,
      "currency": "USD",
      "status": "sent_to_agent",
      "agent": {
        "id": "AGT-001",
        "name": "Agent John Doe",
        "phone": "***1234"
      },
      "instructions": "Visit the agent with this reference code to complete your deposit"
    }
  },
  "timestamp": 1706400000
}
```

**Crypto Deposit Request**:
```json
{
  "type": "deposit_request",
  "data": {
    "amount": 100,
    "local_currency": "USDT",
    "service": "crypto"
  }
}
```

**Crypto Deposit Response**:
```json
{
  "type": "success",
  "data": {
    "message": "crypto deposit address generated",
    "data": {
      "request_ref": "DEP-USDT-20240127-003",
      "amount": 100.00,
      "currency": "USDT",
      "chain": "TRC20",
      "wallet_address": "TYzK7gD3K4VwY8uP...",
      "qr_code_url": "https://api.qrserver.com/v1/create-qr-code/?data=TYzK7gD3K4VwY8uP...",
      "status": "awaiting_deposit",
      "instructions": "Send exactly 100.00 USDT to the address above",
      "minimum_confirmations": 1
    }
  },
  "timestamp": 1706400000
}
```

---

### Get Deposit Status

#### Message Type: `get_deposit_status`

**Request**:
```json
{
  "type": "get_deposit_status",
  "data": {
    "request_ref": "DEP-KES-20240127-001"
  }
}
```

**Response**:
```json
{
  "type": "success",
  "data": {
    "message": "deposit status",
    "data": {
      "request_ref": "DEP-KES-20240127-001",
      "status": "completed",
      "amount_local": 1000.00,
      "local_currency": "KES",
      "amount_usd": 7.50,
      "receipt_code": "RCP-123456",
      "completed_at": "2024-01-27T12:30:00Z"
    }
  },
  "timestamp": 1706400000
}
```

---

### Cancel Deposit

#### Message Type: `cancel_deposit`

**Request**:
```json
{
  "type": "cancel_deposit",
  "data": {
    "request_ref": "DEP-KES-20240127-001"
  }
}
```

**Response**:
```json
{
  "type": "success",
  "data": {
    "message": "deposit cancelled",
    "data": {
      "request_ref": "DEP-KES-20240127-001",
      "status": "cancelled"
    }
  },
  "timestamp": 1706400000
}
```

---

## Withdrawal Operations

### Withdrawal Flow Overview

```
1. User requests verification → verification_request
2. User verifies (TOTP/OTP) → verify_totp/verify_otp
3. System returns verification_token (5 min TTL)
4. User submits withdrawal with token → withdraw_request
5. System validates token & processes withdrawal
6. Withdrawal completed → withdraw_completed notification
```

### Message Type: `withdraw_request`

**Partner Withdrawal Request (M-Pesa)**:
```json
{
  "type": "withdraw_request",
  "data": {
    "amount": 5.00,
    "local_currency": "KES",
    "service": "mpesa",
    "destination": "254712345678",
    "verification_token": "a1b2c3d4e5f6...",
    "consent": true
  }
}
```

**Partner Withdrawal Response**:
```json
{
  "type": "success",
  "data": {
    "message": "withdrawal initiated",
    "data": {
      "request_ref": "WTH-KES-20240127-001",
      "receipt_code": "RCP-789012",
      "amount_usd": 5.00,
      "amount_local": 666.67,
      "local_currency": "KES",
      "exchange_rate": 133.33,
      "fee": 0.50,
      "destination": "254712***678",
      "partner": {
        "id": "safaricom_mpesa",
        "name": "Safaricom M-Pesa"
      },
      "status": "processing",
      "estimated_completion": "2024-01-27T12:35:00Z"
    }
  },
  "timestamp": 1706400000
}
```

**Agent Withdrawal Request**:
```json
{
  "type": "withdraw_request",
  "data": {
    "amount": 50.00,
    "local_currency": "USD",
    "agent_id": "AGT-001",
    "destination": "agent_cash",
    "verification_token": "a1b2c3d4e5f6...",
    "consent": true
  }
}
```

**Agent Withdrawal Response**:
```json
{
  "type": "success",
  "data": {
    "message": "withdrawal request created",
    "data": {
      "request_ref": "WTH-USD-20240127-002",
      "receipt_code": "RCP-789013",
      "amount": 50.00,
      "currency": "USD",
      "fee": 1.00,
      "agent": {
        "id": "AGT-001",
        "name": "Agent John Doe",
        "phone": "***1234"
      },
      "status": "pending_agent",
      "instructions": "Visit Agent John Doe with reference WTH-USD-20240127-002 to collect your cash"
    }
  },
  "timestamp": 1706400000
}
```

**Crypto Withdrawal Request**:
```json
{
  "type": "withdraw_request",
  "data": {
    "amount": 100.00,
    "local_currency": "USDT",
    "service": "crypto",
    "destination": "TYzK7gD3K4VwY8uP...",
    "verification_token": "a1b2c3d4e5f6...",
    "consent": true
  }
}
```

**Crypto Withdrawal Response**:
```json
{
  "type": "success",
  "data": {
    "message": "crypto withdrawal initiated",
    "data": {
      "request_ref": "WTH-USDT-20240127-003",
      "receipt_code": "RCP-789014",
      "amount": 100.00,
      "currency": "USDT",
      "chain": "TRC20",
      "destination_address": "TYzK7gD3K4VwY8uP...",
      "network_fee": 1.50,
      "platform_fee": 0.50,
      "total_fee": 2.00,
      "amount_to_receive": 98.00,
      "status": "processing",
      "tx_hash": null,
      "estimated_completion": "2024-01-27T12:40:00Z"
    }
  },
  "timestamp": 1706400000
}
```

---

## Transfer Operations

### Transfer Flow Overview

```
1. P2P Transfer: User → User
2. Agent Deposit Fulfillment: Agent → User (with deposit_request_ref)
```

### Message Type: `transfer`

**P2P Transfer Request**:
```json
{
  "type": "transfer",
  "data": {
    "to_user_id": "67890",
    "amount": 25.00,
    "currency": "USD",
    "description": "Payment for services"
  }
}
```

**P2P Transfer Response**:
```json
{
  "type": "success",
  "data": {
    "message": "transfer completed",
    "data": {
      "receipt_code": "RCP-345678",
      "journal_id": 12345,
      "amount": 25.00,
      "currency": "USD",
      "fee": 0.00,
      "to_user_id": "67890",
      "transfer_type": "p2p",
      "created_at": "2024-01-27T13:00:00Z"
    }
  },
  "timestamp": 1706400000
}
```

**Agent Deposit Fulfillment Request**:
```json
{
  "type": "transfer",
  "data": {
    "to_user_id": "12345",
    "amount": 100.00,
    "currency": "USD",
    "deposit_request_ref": "DEP-USD-20240127-002",
    "description": "Deposit fulfillment"
  }
}
```

**Agent Deposit Fulfillment Response**:
```json
{
  "type": "success",
  "data": {
    "message": "deposit fulfilled successfully",
    "data": {
      "receipt_code": "RCP-345679",
      "journal_id": 12346,
      "amount": 100.00,
      "currency": "USD",
      "fee": 0.00,
      "to_user_id": "12345",
      "transfer_type": "agent_deposit",
      "agent_id": "AGT-001",
      "agent_name": "Agent John Doe",
      "agent_commission": 2.00,
      "deposit_request_ref": "DEP-USD-20240127-002",
      "deposit_completed": true,
      "created_at": "2024-01-27T13:05:00Z"
    }
  },
  "timestamp": 1706400000
}
```

**Transfer Recipient Notification**:
```json
{
  "type": "transfer_received",
  "data": {
    "from_user_id": "12345",
    "amount": 25.00,
    "currency": "USD",
    "description": "Payment for services",
    "receipt_code": "RCP-345678",
    "transfer_type": "p2p"
  },
  "timestamp": 1706400000
}
```

---

## Transaction History

### Message Type: `get_history`

**Request**:
```json
{
  "type": "get_history",
  "data": {
    "type": "deposits" | "withdrawals" | "all",
    "limit": 20,
    "offset": 0
  }
}
```

**Response**:
```json
{
  "type": "success",
  "data": {
    "message": "transaction history",
    "data": {
      "deposits": [
        {
          "request_ref": "DEP-KES-20240127-001",
          "amount": 7.50,
          "currency": "USD",
          "status": "completed",
          "created_at": "2024-01-27T12:00:00Z"
        }
      ],
      "withdrawals": [
        {
          "request_ref": "WTH-KES-20240127-001",
          "amount": 5.00,
          "currency": "USD",
          "status": "completed",
          "created_at": "2024-01-27T12:30:00Z"
        }
      ]
    }
  },
  "timestamp": 1706400000
}
```

---

### Get Transaction by Receipt

#### Message Type: `get_transaction_by_receipt`

**Request**:
```json
{
  "type": "get_transaction_by_receipt",
  "data": {
    "receipt_code": "RCP-123456"
  }
}
```

**Response**:
```json
{
  "type": "success",
  "data": {
    "message": "transaction details",
    "data": {
      "journal": {
        "id": 12345,
        "transaction_type": "DEPOSIT",
        "description": "M-Pesa deposit from 254712345678",
        "created_at": "2024-01-27T12:30:00Z"
      },
      "ledgers": [
        {
          "account_number": "ACC-USD-12345",
          "amount": 7.50,
          "type": "CREDIT",
          "balance_after": 107.50,
          "description": "Deposit credited"
        }
      ],
      "fees": [
        {
          "type": "PLATFORM_FEE",
          "amount": 0.50,
          "currency": "USD"
        }
      ]
    }
  },
  "timestamp": 1706400000
}
```

---

## Statements & Reports

### 1. Get Account Statement

#### Message Type: `get_account_statement`

**Request**:
```json
{
  "type": "get_account_statement",
  "data": {
    "account_number": "ACC-USD-12345",
    "from": "2024-01-01T00:00:00Z",
    "to": "2024-01-31T23:59:59Z"
  }
}
```

**Response**:
```json
{
  "type": "success",
  "data": {
    "message": "account statement",
    "data": {
      "account_number": "ACC-USD-12345",
      "opening_balance": 100.00,
      "closing_balance": 107.50,
      "total_debits": 50.00,
      "total_credits": 57.50,
      "period_start": "2024-01-01T00:00:00Z",
      "period_end": "2024-01-31T23:59:59Z",
      "ledgers": [
        {
          "id": 1,
          "amount": 7.50,
          "type": "CREDIT",
          "currency": "USD",
          "balance_after": 107.50,
          "description": "Deposit",
          "receipt_code": "RCP-123456",
          "created_at": "2024-01-27T12:30:00Z"
        }
      ],
      "transaction_count": 1
    }
  },
  "timestamp": 1706400000
}
```

---

### 2. Get Owner Statement

#### Message Type: `get_owner_statement`

**Request**:
```json
{
  "type": "get_owner_statement",
  "data": {
    "from": "2024-01-01T00:00:00Z",
    "to": "2024-01-31T23:59:59Z"
  }
}
```

**Response**:
```json
{
  "type": "success",
  "data": {
    "message": "owner statement",
    "data": {
      "statements": [
        {
          "account_number": "ACC-USD-12345",
          "opening_balance": 100.00,
          "closing_balance": 107.50,
          "total_debits": 50.00,
          "total_credits": 57.50,
          "ledgers": [...]
        },
        {
          "account_number": "ACC-BTC-12345",
          "opening_balance": 0.05,
          "closing_balance": 0.06,
          "total_debits": 0.01,
          "total_credits": 0.02,
          "ledgers": [...]
        }
      ],
      "count": 2,
      "period_start": "2024-01-01T00:00:00Z",
      "period_end": "2024-01-31T23:59:59Z"
    }
  },
  "timestamp": 1706400000
}
```

---

### 3. Get Ledgers

#### Message Type: `get_ledgers`

**Request**:
```json
{
  "type": "get_ledgers",
  "data": {
    "account_number": "ACC-USD-12345",
    "from": "2024-01-01T00:00:00Z",
    "to": "2024-01-31T23:59:59Z",
    "limit": 50,
    "offset": 0
  }
}
```

**Response**:
```json
{
  "type": "success",
  "data": {
    "message": "ledgers retrieved",
    "data": {
      "ledgers": [
        {
          "id": 1,
          "journal_id": 12345,
          "amount": 7.50,
          "type": "CREDIT",
          "currency": "USD",
          "balance_after": 107.50,
          "description": "Deposit",
          "receipt_code": "RCP-123456",
          "created_at": "2024-01-27T12:30:00Z"
        }
      ],
      "total": 1,
      "limit": 50,
      "offset": 0
    }
  },
  "timestamp": 1706400000
}
```

---

## Fee Calculation

### Message Type: `calculate_fee`

**Request**:
```json
{
  "type": "calculate_fee",
  "data": {
    "transaction_type": "transfer" | "withdrawal" | "conversion",
    "amount": 100.00,
    "source_currency": "USD",
    "target_currency": "BTC",
    "to_address": "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
  }
}
```

**Response**:
```json
{
  "type": "success",
  "data": {
    "message": "fee calculated",
    "data": {
      "platform_fee": 0.50,
      "network_fee": 1.50,
      "total_fee": 2.00,
      "fee_percentage": 2.0,
      "amount_after_fee": 98.00,
      "currency": "USD"
    }
  },
  "timestamp": 1706400000
}
```

---

## Currency Conversion

### Message Type: `convert_and_transfer`

**Request**:
```json
{
  "type": "convert_and_transfer",
  "data": {
    "from_currency": "USD",
    "to_currency": "BTC",
    "amount": 1000.00,
    "description": "Converting USD to BTC"
  }
}
```

**Response**:
```json
{
  "type": "success",
  "data": {
    "message": "conversion completed",
    "data": {
      "receipt_code": "RCP-567890",
      "journal_id": 12347,
      "source_currency": "USD",
      "dest_currency": "BTC",
      "source_amount": 1000.00,
      "converted_amount": 0.02345678,
      "fx_rate": "42650.50",
      "fx_rate_id": 789,
      "fee": 5.00,
      "created_at": "2024-01-27T14:00:00Z"
    }
  },
  "timestamp": 1706400000
}
```

---

## Partner Operations

### Message Type: `get_partners`

**Request**:
```json
{
  "type": "get_partners",
  "data": {
    "service": "mpesa" | "bank_transfer" | "crypto"
  }
}
```

**Response**:
```json
{
  "type": "success",
  "data": {
    "message": "partners retrieved",
    "data": {
      "partners": [
        {
          "id": "safaricom_mpesa",
          "name": "Safaricom M-Pesa",
          "service": "mpesa",
          "currency": "KES",
          "is_active": true,
          "fees": {
            "deposit": 0.5,
            "withdrawal": 1.0
          }
        }
      ],
      "count": 1
    }
  },
  "timestamp": 1706400000
}
```

---

## Event Notifications

These are server-initiated messages sent to clients when events occur.

### 1. Deposit Completed

```json
{
  "type": "deposit_completed",
  "data": {
    "request_ref": "DEP-KES-20240127-001",
    "amount": 7.50,
    "currency": "USD",
    "receipt_code": "RCP-123456",
    "status": "completed",
    "completed_at": "2024-01-27T12:30:00Z"
  },
  "timestamp": 1706400000
}
```

### 2. Withdrawal Completed

```json
{
  "type": "withdraw_completed",
  "data": {
    "request_ref": "WTH-KES-20240127-001",
    "amount": 5.00,
    "currency": "USD",
    "receipt_code": "RCP-789012",
    "status": "completed",
    "completed_at": "2024-01-27T12:35:00Z"
  },
  "timestamp": 1706400000
}
```

### 3. Transfer Received

```json
{
  "type": "transfer_received",
  "data": {
    "from_user_id": "12345",
    "amount": 25.00,
    "currency": "USD",
    "description": "Payment for services",
    "receipt_code": "RCP-345678",
    "transfer_type": "p2p"
  },
  "timestamp": 1706400000
}
```

### 4. Deposit Completed (Agent)

```json
{
  "type": "deposit_completed",
  "data": {
    "from_user_id": "AGT-001",
    "amount": 100.00,
    "currency": "USD",
    "description": "Deposit fulfillment via agent AGT-001",
    "receipt_code": "RCP-345679",
    "deposit_request_ref": "DEP-USD-20240127-002",
    "is_deposit": true,
    "agent_id": "AGT-001",
    "agent_name": "Agent John Doe"
  },
  "timestamp": 1706400000
}
```

---

## Complete Flow Examples

### Withdrawal Flow (with TOTP)

```
Step 1: Request Verification
→ type: "verification_request"
  data: { method: "totp", purpose: "withdrawal" }

← type: "success"
  data: { method: "totp", next_step: "verify_totp" }

Step 2: Verify TOTP
→ type: "verify_totp"
  data: { code: "123456", purpose: "withdrawal" }

← type: "success"
  data: { verification_token: "a1b2c3...", expires_in: 300 }

Step 3: Submit Withdrawal
→ type: "withdraw_request"
  data: {
    amount: 5.00,
    local_currency: "KES",
    service: "mpesa",
    destination: "254712345678",
    verification_token: "a1b2c3...",
    consent: true
  }

← type: "success"
  data: {
    request_ref: "WTH-KES-20240127-001",
    receipt_code: "RCP-789012",
    status: "processing"
  }

Step 4: Withdrawal Completed (Notification)
← type: "withdraw_completed"
  data: {
    request_ref: "WTH-KES-20240127-001",
    status: "completed"
  }
```

### Agent Deposit Flow

```
Step 1: User Creates Deposit Request
→ type: "deposit_request"
  data: {
    amount: 100,
    local_currency: "USD",
    service: "agent",
    agent_id: "AGT-001"
  }

← type: "success"
  data: {
    request_ref: "DEP-USD-20240127-002",
    status: "sent_to_agent",
    agent: { id: "AGT-001", name: "Agent John Doe" }
  }

Step 2: Agent Fulfills Deposit (Transfer)
→ type: "transfer"
  data: {
    to_user_id: "12345",
    amount: 100.00,
    currency: "USD",
    deposit_request_ref: "DEP-USD-20240127-002"
  }

← type: "success"
  data: {
    receipt_code: "RCP-345679",
    transfer_type: "agent_deposit",
    deposit_completed: true
  }

Step 3: User Receives Notification
← type: "deposit_completed"
  data: {
    deposit_request_ref: "DEP-USD-20240127-002",
    amount: 100.00,
    is_deposit: true,
    agent_id: "AGT-001"
  }
```

---

## Error Codes & Messages

| Error Message | Cause | Solution |
|---------------|-------|----------|
| `verification token is required` | No verification token provided | Complete verification flow first |
| `invalid or expired verification token` | Token expired or invalid | Request new verification |
| `insufficient balance` | Account balance too low | Deposit funds |
| `account not found` | Account doesn't exist | Create account first |
| `unauthorized` | Not account owner | Check account ownership |
| `2FA not enabled` | TOTP requested but not enabled | Use OTP instead or enable 2FA |
| `invalid TOTP code` | Wrong 2FA code | Enter correct code |
| `invalid OTP code` | Wrong OTP code | Request new OTP |
| `currency not supported` | Invalid currency code | Use supported currency |
| `amount must be greater than zero` | Zero or negative amount | Use positive amount |
| `only agents can fulfill deposits` | Non-agent trying to fulfill | Must be agent user |
| `deposit request not found` | Invalid deposit reference | Check reference code |

---

## Rate Limits & Timeouts

- **Verification Token TTL**: 5 minutes (300 seconds)
- **OTP Session TTL**: 3 minutes (180 seconds)
- **WebSocket Ping Interval**: 54 seconds
- **WebSocket Read Timeout**: 60 seconds
- **WebSocket Write Timeout**: 10 seconds

---

## Best Practices

1. **Always verify before sensitive operations**: Withdrawals and transfers require verification
2. **Store verification tokens securely**: Never log or expose tokens
3. **Handle token expiration**: Request new verification if token expires
4. **Listen for event notifications**: Important updates sent via WebSocket
5. **Validate amounts client-side**: Prevent invalid requests
6. **Handle network errors gracefully**: Implement reconnection logic
7. **Cache account balances carefully**: Refresh after transactions
8. **Use idempotency**: Check request_ref before retrying

---

## Supported Currencies

- **Fiat**: USD
- **Crypto**: BTC, USDT, USDC, TRX

(Check `get_supported_currencies` for current list)

---

## Supported Services

- **Partner**: mpesa, bank_transfer
- **Agent**: agent
- **Crypto**: crypto

---

## WebSocket Connection States

1. **Connecting**: Establishing WebSocket connection
2. **Connected**: Connection established, received welcome message
3. **Authenticated**: User authenticated, can send requests
4. **Disconnected**: Connection lost, should attempt reconnection

---

## Notes

- All amounts are in decimal format (float64)
- All timestamps are Unix timestamps (seconds since epoch)
- All dates in ISO 8601 format when returned as strings
- Phone numbers are masked in responses (***1234)
- Email addresses are masked in responses (ab***@example.com)
- Verification tokens are single-use and expire after 5 minutes
- OTP codes expire after 3 minutes
- WebSocket messages are JSON-encoded
- All monetary values support up to 8 decimal places
