package domain

import (
	"time"
	receiptpb "x/shared/genproto/shared/accounting/receipt/v3"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ReceiptUpdate holds update information for a receipt
type ReceiptUpdate struct {
	Code                string
	Status              string // "pending", "processing", "completed", etc.
	CreditorStatus      string
	CreditorLedgerID    int64
	DebitorStatus       string
	DebitorLedgerID     int64
	ReversedBy          string
	ReversedAt          *time.Time
	CompletedAt         *time.Time
	ErrorMessage        string
	ReversalReceiptCode string
	MetadataPatch       map[string]any
}

// PartyInfo holds creditor/debitor details
type PartyInfo struct {
	AccountID     int64  `json:"account_id"`
	LedgerID      int64  `json:"ledger_id"`
	OwnerType     string `json:"owner_type"` // "system", "user", "agent", "partner"
	Status        string `json:"status"`     // "pending", "completed", etc.
	ExternalID    string `json:"external_id,omitempty"`
	Name          string `json:"name,omitempty"`
	Phone         string `json:"phone,omitempty"`
	Email         string `json:"email,omitempty"`
	AccountNumber string `json:"account_number,omitempty"`
	IsCreditor    bool   `json:"is_creditor"`
}

// Receipt represents a transaction receipt (STRING-BASED for database compatibility)
type Receipt struct {
	// Primary fields
	LookupID    int64  `json:"lookup_id"`
	Code        string `json:"code"`
	AccountType string `json:"account_type"` // "real", "demo"

	// Transaction details
	TransactionType string `json:"transaction_type"` // "deposit", "withdrawal", etc.
	CodedType       string `json:"coded_type,omitempty"`

	// Amounts
	Amount          float64 `json:"amount"`
	OriginalAmount  float64 `json:"original_amount,omitempty"`
	TransactionCost float64 `json:"transaction_cost"`

	// Currency
	Currency         string `json:"currency"`
	OriginalCurrency string `json:"original_currency,omitempty"`
	ExchangeRate     string `json:"exchange_rate,omitempty"`

	// References
	ExternalRef         string `json:"external_ref,omitempty"`
	ParentReceiptCode   string `json:"parent_receipt_code,omitempty"`
	ReversalReceiptCode string `json:"reversal_receipt_code,omitempty"`

	// Status
	Status         string `json:"status"`          // "pending", "processing", "completed", etc.
	CreditorStatus string `json:"creditor_status"` // "pending", "completed", etc.
	DebitorStatus  string `json:"debitor_status"`  // "pending", "completed", etc.
	ErrorMessage   string `json:"error_message,omitempty"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	ReversedAt  *time.Time `json:"reversed_at,omitempty"`

	// Audit
	CreatedBy  string `json:"created_by,omitempty"`
	ReversedBy string `json:"reversed_by,omitempty"`

	// Parties
	Creditor PartyInfo `json:"creditor"`
	Debitor  PartyInfo `json:"debitor"`

	// Flexible metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ReceiptFilters for querying receipts
type ReceiptFilters struct {
	TransactionTypes []string // Array of string transaction types
	Statuses         []string // Array of string statuses
	AccountType      string   // "real", "demo", or empty for all
	Currency         string
	ExternalID       string
	FromDate         *time.Time
	ToDate           *time.Time
	PageSize         int
	PageToken        string
	SummaryOnly      bool
	IncludeMetadata  bool
}

// ===============================
// ENUM TO STRING CONVERSION HELPERS
// ===============================

// AccountTypeToString converts proto enum to database string
func AccountTypeToString(at receiptpb.AccountType) string {
	switch at {
	case receiptpb.AccountType_ACCOUNT_TYPE_REAL:
		return "real"
	case receiptpb.AccountType_ACCOUNT_TYPE_DEMO:
		return "demo"
	default:
		return "real" // Default
	}
}

// StringToAccountType converts database string to proto enum
func StringToAccountType(s string) receiptpb.AccountType {
	switch s {
	case "real":
		return receiptpb.AccountType_ACCOUNT_TYPE_REAL
	case "demo":
		return receiptpb.AccountType_ACCOUNT_TYPE_DEMO
	default:
		return receiptpb.AccountType_ACCOUNT_TYPE_UNSPECIFIED
	}
}

// TransactionTypeToString converts proto enum to database string
func TransactionTypeToString(tt receiptpb.TransactionType) string {
	switch tt {
	case receiptpb.TransactionType_TRANSACTION_TYPE_DEPOSIT:
		return "deposit"
	case receiptpb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL:
		return "withdrawal"
	case receiptpb.TransactionType_TRANSACTION_TYPE_CONVERSION:
		return "conversion"
	case receiptpb.TransactionType_TRANSACTION_TYPE_TRADE:
		return "trade"
	case receiptpb.TransactionType_TRANSACTION_TYPE_TRANSFER:
		return "transfer"
	case receiptpb.TransactionType_TRANSACTION_TYPE_FEE:
		return "fee"
	case receiptpb.TransactionType_TRANSACTION_TYPE_COMMISSION:
		return "commission"
	case receiptpb.TransactionType_TRANSACTION_TYPE_REVERSAL:
		return "reversal"
	case receiptpb.TransactionType_TRANSACTION_TYPE_ADJUSTMENT:
		return "adjustment"
	case receiptpb.TransactionType_TRANSACTION_TYPE_DEMO_FUNDING:
		return "demo_funding"
	default:
		return "deposit" // Default
	}
}

// StringToTransactionType converts database string to proto enum
func StringToTransactionType(s string) receiptpb.TransactionType {
	switch s {
	case "deposit":
		return receiptpb.TransactionType_TRANSACTION_TYPE_DEPOSIT
	case "withdrawal":
		return receiptpb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL
	case "conversion":
		return receiptpb.TransactionType_TRANSACTION_TYPE_CONVERSION
	case "trade":
		return receiptpb.TransactionType_TRANSACTION_TYPE_TRADE
	case "transfer":
		return receiptpb.TransactionType_TRANSACTION_TYPE_TRANSFER
	case "fee":
		return receiptpb.TransactionType_TRANSACTION_TYPE_FEE
	case "commission":
		return receiptpb.TransactionType_TRANSACTION_TYPE_COMMISSION
	case "reversal":
		return receiptpb.TransactionType_TRANSACTION_TYPE_REVERSAL
	case "adjustment":
		return receiptpb.TransactionType_TRANSACTION_TYPE_ADJUSTMENT
	case "demo_funding":
		return receiptpb.TransactionType_TRANSACTION_TYPE_DEMO_FUNDING
	default:
		return receiptpb.TransactionType_TRANSACTION_TYPE_UNSPECIFIED
	}
}

// TransactionStatusToString converts proto enum to database string
func TransactionStatusToString(ts receiptpb.TransactionStatus) string {
	switch ts {
	case receiptpb.TransactionStatus_TRANSACTION_STATUS_PENDING:
		return "pending"
	case receiptpb.TransactionStatus_TRANSACTION_STATUS_PROCESSING:
		return "processing"
	case receiptpb.TransactionStatus_TRANSACTION_STATUS_COMPLETED:
		return "completed"
	case receiptpb.TransactionStatus_TRANSACTION_STATUS_FAILED:
		return "failed"
	case receiptpb.TransactionStatus_TRANSACTION_STATUS_REVERSED:
		return "reversed"
	case receiptpb.TransactionStatus_TRANSACTION_STATUS_SUSPENDED:
		return "suspended"
	case receiptpb.TransactionStatus_TRANSACTION_STATUS_EXPIRED:
		return "expired"
	default:
		return "pending" // Default
	}
}

// StringToTransactionStatus converts database string to proto enum
func StringToTransactionStatus(s string) receiptpb.TransactionStatus {
	switch s {
	case "pending":
		return receiptpb.TransactionStatus_TRANSACTION_STATUS_PENDING
	case "processing":
		return receiptpb.TransactionStatus_TRANSACTION_STATUS_PROCESSING
	case "completed":
		return receiptpb.TransactionStatus_TRANSACTION_STATUS_COMPLETED
	case "failed":
		return receiptpb.TransactionStatus_TRANSACTION_STATUS_FAILED
	case "reversed":
		return receiptpb.TransactionStatus_TRANSACTION_STATUS_REVERSED
	case "suspended":
		return receiptpb.TransactionStatus_TRANSACTION_STATUS_SUSPENDED
	case "expired":
		return receiptpb.TransactionStatus_TRANSACTION_STATUS_EXPIRED
	default:
		return receiptpb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
	}
}

// OwnerTypeToString converts proto enum to database string
func OwnerTypeToString(ot receiptpb.OwnerType) string {
	switch ot {
	case receiptpb.OwnerType_OWNER_TYPE_SYSTEM:
		return "system"
	case receiptpb.OwnerType_OWNER_TYPE_USER:
		return "user"
	case receiptpb.OwnerType_OWNER_TYPE_AGENT:
		return "agent"
	case receiptpb.OwnerType_OWNER_TYPE_PARTNER:
		return "partner"
	default:
		return "user" // Default
	}
}

// StringToOwnerType converts database string to proto enum
func StringToOwnerType(s string) receiptpb.OwnerType {
	switch s {
	case "system":
		return receiptpb.OwnerType_OWNER_TYPE_SYSTEM
	case "user":
		return receiptpb.OwnerType_OWNER_TYPE_USER
	case "agent":
		return receiptpb.OwnerType_OWNER_TYPE_AGENT
	case "partner":
		return receiptpb.OwnerType_OWNER_TYPE_PARTNER
	default:
		return receiptpb.OwnerType_OWNER_TYPE_UNSPECIFIED
	}
}

// ===============================
// PartyInfo Proto Converters
// ===============================

func (p *PartyInfo) ToProto() *receiptpb.PartyInfo {
	if p == nil {
		return nil
	}
	return &receiptpb.PartyInfo{
		AccountId:  p.AccountID,
		LedgerId:   p.LedgerID,
		OwnerType:  StringToOwnerType(p.OwnerType),
		Status:     StringToTransactionStatus(p.Status),
		ExternalId: p.ExternalID,
		IsCreditor: p.IsCreditor,
	}
}

func PartyInfoFromProto(pb *receiptpb.PartyInfo) PartyInfo {
	if pb == nil {
		return PartyInfo{}
	}
	return PartyInfo{
		AccountID:  pb.GetAccountId(),
		LedgerID:   pb.GetLedgerId(),
		OwnerType:  OwnerTypeToString(pb.GetOwnerType()),
		Status:     TransactionStatusToString(pb.GetStatus()),
		ExternalID: pb.GetExternalId(),
		IsCreditor: pb.GetIsCreditor(),
	}
}

// ===============================
// Receipt Proto Converters
// ===============================

func (r *Receipt) ToProto() *receiptpb.Receipt {
    if r == nil {
        return nil
    }

    metadata, _ := structpb.NewStruct(r.Metadata)

    var updatedAt, completedAt, reversedAt *timestamppb.Timestamp
    if r.UpdatedAt != nil {
        updatedAt = timestamppb. New(*r.UpdatedAt)
    }
    if r.CompletedAt != nil {
        completedAt = timestamppb. New(*r.CompletedAt)
    }
    if r.ReversedAt != nil {
        reversedAt = timestamppb.New(*r.ReversedAt)
    }

    return &receiptpb. Receipt{
        LookupId:            r.LookupID,
        Code:                r.Code,
        TransactionType:     StringToTransactionType(r.TransactionType),
        CodedType:           r.CodedType,
        Amount:              r.Amount,              // ✅ Direct assignment (no conversion)
        OriginalAmount:      r.OriginalAmount,      // ✅ Direct assignment
        TransactionCost:     r.TransactionCost,     // ✅ Direct assignment
        Currency:            r.Currency,
        OriginalCurrency:    r.OriginalCurrency,
        ExchangeRateDecimal: r.ExchangeRate,
        AccountType:         StringToAccountType(r.AccountType),
        Creditor:            r.Creditor. ToProto(),
        Debitor:             r.Debitor.ToProto(),
        Status:              StringToTransactionStatus(r. Status),
        CreditorStatus:      StringToTransactionStatus(r. CreditorStatus),
        DebitorStatus:       StringToTransactionStatus(r.DebitorStatus),
        ExternalRef:         r.ExternalRef,
        ParentReceiptCode:   r.ParentReceiptCode,
        ReversalReceiptCode: r. ReversalReceiptCode,
        CreatedAt:           timestamppb.New(r.CreatedAt),
        UpdatedAt:           updatedAt,
        CompletedAt:         completedAt,
        ReversedAt:          reversedAt,
        CreatedBy:           r.CreatedBy,
        ReversedBy:          r.ReversedBy,
        ErrorMessage:        r.ErrorMessage,
        Metadata:            metadata,
    }
}

func ReceiptFromProto(pb *receiptpb.Receipt) Receipt {
    if pb == nil {
        return Receipt{}
    }

    var updatedAt, completedAt, reversedAt *time.Time
    if pb.GetUpdatedAt() != nil {
        t := pb.GetUpdatedAt().AsTime()
        updatedAt = &t
    }
    if pb.GetCompletedAt() != nil {
        t := pb. GetCompletedAt().AsTime()
        completedAt = &t
    }
    if pb.GetReversedAt() != nil {
        t := pb. GetReversedAt().AsTime()
        reversedAt = &t
    }

    metadata := map[string]interface{}{}
    if pb.GetMetadata() != nil {
        metadata = pb.GetMetadata(). AsMap()
    }

    return Receipt{
        LookupID:            pb.GetLookupId(),
        Code:                pb.GetCode(),
        TransactionType:     TransactionTypeToString(pb.GetTransactionType()),
        CodedType:           pb.GetCodedType(),
        Amount:              pb.GetAmount(),              // ✅ Direct assignment
        OriginalAmount:      pb.GetOriginalAmount(),      // ✅ Direct assignment
        TransactionCost:     pb. GetTransactionCost(),     // ✅ Direct assignment
        Currency:            pb.GetCurrency(),
        OriginalCurrency:    pb.GetOriginalCurrency(),
        ExchangeRate:        pb.GetExchangeRateDecimal(),
        AccountType:         AccountTypeToString(pb.GetAccountType()),
        Creditor:            PartyInfoFromProto(pb.GetCreditor()),
        Debitor:             PartyInfoFromProto(pb.GetDebitor()),
        Status:              TransactionStatusToString(pb.GetStatus()),
        CreditorStatus:      TransactionStatusToString(pb.GetCreditorStatus()),
        DebitorStatus:       TransactionStatusToString(pb.GetDebitorStatus()),
        ExternalRef:         pb. GetExternalRef(),
        ParentReceiptCode:   pb. GetParentReceiptCode(),
        ReversalReceiptCode: pb.GetReversalReceiptCode(),
        CreatedAt:           pb.GetCreatedAt().AsTime(),
        UpdatedAt:           updatedAt,
        CompletedAt:         completedAt,
        ReversedAt:          reversedAt,
        CreatedBy:           pb.GetCreatedBy(),
        ReversedBy:          pb.GetReversedBy(),
        ErrorMessage:        pb.GetErrorMessage(),
        Metadata:            metadata,
    }
}

// ===============================
// ReceiptUpdate Proto Converters
// ===============================

func (ru *ReceiptUpdate) ToProto() *receiptpb.UpdateReceiptRequest {
	if ru == nil {
		return nil
	}

	var reversedAt, completedAt *timestamppb.Timestamp
	if ru.ReversedAt != nil {
		reversedAt = timestamppb.New(*ru.ReversedAt)
	}
	if ru.CompletedAt != nil {
		completedAt = timestamppb.New(*ru.CompletedAt)
	}

	metadataPatch := &structpb.Struct{}
	if ru.MetadataPatch != nil {
		metadataPatch, _ = structpb.NewStruct(ru.MetadataPatch)
	}

	return &receiptpb.UpdateReceiptRequest{
		Code:                ru.Code,
		Status:              StringToTransactionStatus(ru.Status),
		CreditorStatus:      StringToTransactionStatus(ru.CreditorStatus),
		CreditorLedgerId:    ru.CreditorLedgerID,
		DebitorStatus:       StringToTransactionStatus(ru.DebitorStatus),
		DebitorLedgerId:     ru.DebitorLedgerID,
		ReversedBy:          ru.ReversedBy,
		ReversedAt:          reversedAt,
		ReversalReceiptCode: ru.ReversalReceiptCode,
		ErrorMessage:        ru.ErrorMessage,
		CompletedAt:         completedAt,
		MetadataPatch:       metadataPatch,
	}
}

func ReceiptUpdateFromProto(pb *receiptpb.UpdateReceiptRequest) *ReceiptUpdate {
	if pb == nil {
		return &ReceiptUpdate{}
	}

	var reversedAt, completedAt *time.Time
	if pb.GetReversedAt() != nil {
		t := pb.GetReversedAt().AsTime()
		reversedAt = &t
	}
	if pb.GetCompletedAt() != nil {
		t := pb.GetCompletedAt().AsTime()
		completedAt = &t
	}

	var metadataPatch map[string]any
	if pb.GetMetadataPatch() != nil {
		metadataPatch = pb.GetMetadataPatch().AsMap()
	}

	return &ReceiptUpdate{
		Code:                pb.GetCode(),
		Status:              TransactionStatusToString(pb.GetStatus()),
		CreditorStatus:      TransactionStatusToString(pb.GetCreditorStatus()),
		CreditorLedgerID:    pb.GetCreditorLedgerId(),
		DebitorStatus:       TransactionStatusToString(pb.GetDebitorStatus()),
		DebitorLedgerID:     pb.GetDebitorLedgerId(),
		ReversedBy:          pb.GetReversedBy(),
		ReversedAt:          reversedAt,
		ReversalReceiptCode: pb.GetReversalReceiptCode(),
		ErrorMessage:        pb.GetErrorMessage(),
		CompletedAt:         completedAt,
		MetadataPatch:       metadataPatch,
	}
}

// ===============================
// Filters Proto Converter
// ===============================

func FiltersFromProto(req *receiptpb.ListReceiptsRequest) *ReceiptFilters {
	if req == nil {
		return &ReceiptFilters{
			PageSize: 50,
		}
	}

	// Convert proto enum arrays to string arrays
	txnTypes := make([]string, len(req.GetTransactionTypes()))
	for i, tt := range req.GetTransactionTypes() {
		txnTypes[i] = TransactionTypeToString(tt)
	}

	statuses := make([]string, len(req.GetStatuses()))
	for i, s := range req.GetStatuses() {
		statuses[i] = TransactionStatusToString(s)
	}

	filters := &ReceiptFilters{
		TransactionTypes: txnTypes,
		Statuses:         statuses,
		AccountType:      AccountTypeToString(req.GetAccountType()),
		Currency:         req.GetCurrency(),
		ExternalID:       req.GetExternalId(),
		PageSize:         int(req.GetPageSize()),
		PageToken:        req.GetPageToken(),
		SummaryOnly:      req.GetSummaryOnly(),
		IncludeMetadata:  req.GetIncludeMetadata(),
	}

	// Convert timestamps
	if req.GetFromDate() != nil {
		t := req.GetFromDate().AsTime()
		filters.FromDate = &t
	}
	if req.GetToDate() != nil {
		t := req.GetToDate().AsTime()
		filters.ToDate = &t
	}

	// Set default page size if not provided or invalid
	if filters.PageSize <= 0 || filters.PageSize > 100 {
		filters.PageSize = 50
	}

	return filters
}
