package domain

import (
	//"encoding/json"
	"time"
	receiptpb "x/shared/genproto/shared/accounting/receipt/v3"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ReceiptUpdate holds update information for a receipt
type ReceiptUpdate struct {
	Code                string
	Status              receiptpb.TransactionStatus
	CreditorStatus      receiptpb.TransactionStatus
	CreditorLedgerID    int64
	DebitorStatus       receiptpb.TransactionStatus
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
	AccountID     int64                       `json:"account_id"`
	LedgerID      int64                       `json:"ledger_id"`
	OwnerType     receiptpb.OwnerType         `json:"owner_type"`
	Status        receiptpb.TransactionStatus `json:"status"`
	ExternalID    string                      `json:"external_id,omitempty"`
	Name          string                      `json:"name,omitempty"`
	Phone         string                      `json:"phone,omitempty"`
	Email         string                      `json:"email,omitempty"`
	AccountNumber string                      `json:"account_number,omitempty"`
	IsCreditor    bool                        `json:"is_creditor"`
}

// Receipt represents a transaction receipt (matches production schema + v3 proto)
type Receipt struct {
	// Primary fields
	LookupID    int64                     `json:"lookup_id"`
	Code        string                    `json:"code"`
	AccountType receiptpb.AccountType     `json:"account_type"`

	// Transaction details
	TransactionType receiptpb.TransactionType `json:"transaction_type"`
	CodedType       string                    `json:"coded_type,omitempty"`

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
	Status         receiptpb.TransactionStatus `json:"status"`
	CreditorStatus receiptpb.TransactionStatus `json:"creditor_status"`
	DebitorStatus  receiptpb.TransactionStatus `json:"debitor_status"`
	ErrorMessage   string                      `json:"error_message,omitempty"`

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
	TransactionTypes []receiptpb.TransactionType
	Statuses         []receiptpb.TransactionStatus
	AccountType      receiptpb.AccountType
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
// PartyInfo Proto Converters
// ===============================

func (p *PartyInfo) ToProto() *receiptpb.PartyInfo {
	if p == nil {
		return nil
	}
	return &receiptpb.PartyInfo{
		AccountId:  p.AccountID,
		LedgerId:   p.LedgerID,
		OwnerType:  p.OwnerType,
		Status:     p.Status,
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
		OwnerType:  pb.GetOwnerType(),
		Status:     pb.GetStatus(),
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
		updatedAt = timestamppb.New(*r.UpdatedAt)
	}
	if r.CompletedAt != nil {
		completedAt = timestamppb.New(*r.CompletedAt)
	}
	if r.ReversedAt != nil {
		reversedAt = timestamppb.New(*r.ReversedAt)
	}

	// Convert float64 amounts to int64 (multiply by 100 for cents)
	amount := int64(r.Amount * 100)
	originalAmount := int64(r.OriginalAmount * 100)
	transactionCost := int64(r.TransactionCost * 100)

	return &receiptpb.Receipt{
		LookupId:            r.LookupID,
		Code:                r.Code,
		TransactionType:     r.TransactionType,
		CodedType:           r.CodedType,
		Amount:              amount,
		OriginalAmount:      originalAmount,
		TransactionCost:     transactionCost,
		Currency:            r.Currency,
		OriginalCurrency:    r.OriginalCurrency,
		ExchangeRateDecimal: r.ExchangeRate,
		AccountType:         r.AccountType,
		Creditor:            r.Creditor.ToProto(),
		Debitor:             r.Debitor.ToProto(),
		Status:              r.Status,
		CreditorStatus:      r.CreditorStatus,
		DebitorStatus:       r.DebitorStatus,
		ExternalRef:         r.ExternalRef,
		ParentReceiptCode:   r.ParentReceiptCode,
		ReversalReceiptCode: r.ReversalReceiptCode,
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
		t := pb.GetCompletedAt().AsTime()
		completedAt = &t
	}
	if pb.GetReversedAt() != nil {
		t := pb.GetReversedAt().AsTime()
		reversedAt = &t
	}

	metadata := map[string]interface{}{}
	if pb.GetMetadata() != nil {
		metadata = pb.GetMetadata().AsMap()
	}

	// Convert int64 amounts (cents) to float64 (dollars)
	amount := float64(pb.GetAmount()) / 100.0
	originalAmount := float64(pb.GetOriginalAmount()) / 100.0
	transactionCost := float64(pb.GetTransactionCost()) / 100.0

	return Receipt{
		LookupID:            pb.GetLookupId(),
		Code:                pb.GetCode(),
		TransactionType:     pb.GetTransactionType(),
		CodedType:           pb.GetCodedType(),
		Amount:              amount,
		OriginalAmount:      originalAmount,
		TransactionCost:     transactionCost,
		Currency:            pb.GetCurrency(),
		OriginalCurrency:    pb.GetOriginalCurrency(),
		ExchangeRate:        pb.GetExchangeRateDecimal(),
		AccountType:         pb.GetAccountType(),
		Creditor:            PartyInfoFromProto(pb.GetCreditor()),
		Debitor:             PartyInfoFromProto(pb.GetDebitor()),
		Status:              pb.GetStatus(),
		CreditorStatus:      pb.GetCreditorStatus(),
		DebitorStatus:       pb.GetDebitorStatus(),
		ExternalRef:         pb.GetExternalRef(),
		ParentReceiptCode:   pb.GetParentReceiptCode(),
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
		Status:              ru.Status,
		CreditorStatus:      ru.CreditorStatus,
		CreditorLedgerId:    ru.CreditorLedgerID,
		DebitorStatus:       ru.DebitorStatus,
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
		Status:              pb.GetStatus(),
		CreditorStatus:      pb.GetCreditorStatus(),
		CreditorLedgerID:    pb.GetCreditorLedgerId(),
		DebitorStatus:       pb.GetDebitorStatus(),
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
// MISSING: Filters Proto Converter
// ===============================

// FiltersFromProto converts ListReceiptsRequest to ReceiptFilters
func FiltersFromProto(req *receiptpb.ListReceiptsRequest) *ReceiptFilters {
	if req == nil {
		return &ReceiptFilters{
			PageSize: 50, // Default
		}
	}

	filters := &ReceiptFilters{
		TransactionTypes: req.GetTransactionTypes(),
		Statuses:         req.GetStatuses(),
		AccountType:      req.GetAccountType(),
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

// ===============================
// Helper Functions
// ===============================

// TransactionTypeToString converts enum to string
func TransactionTypeToString(t receiptpb.TransactionType) string {
	return t.String()
}

// TransactionStatusToString converts enum to string
func TransactionStatusToString(s receiptpb.TransactionStatus) string {
	return s.String()
}

// AccountTypeToString converts enum to string
func AccountTypeToString(a receiptpb.AccountType) string {
	return a.String()
}

// OwnerTypeToString converts enum to string
func OwnerTypeToString(o receiptpb.OwnerType) string {
	return o.String()
}

// ParseTransactionType converts string to enum
func ParseTransactionType(s string) receiptpb.TransactionType {
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

// ParseTransactionStatus converts string to enum
func ParseTransactionStatus(s string) receiptpb.TransactionStatus {
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

// ParseAccountType converts string to enum
func ParseAccountType(s string) receiptpb.AccountType {
	switch s {
	case "real":
		return receiptpb.AccountType_ACCOUNT_TYPE_REAL
	case "demo":
		return receiptpb.AccountType_ACCOUNT_TYPE_DEMO
	default:
		return receiptpb.AccountType_ACCOUNT_TYPE_UNSPECIFIED
	}
}

// ParseOwnerType converts string to enum
func ParseOwnerType(s string) receiptpb.OwnerType {
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