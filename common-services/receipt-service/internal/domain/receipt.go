package domain

import (
	"time"
	receiptpb "x/shared/genproto/shared/accounting/receipt/v2"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ReceiptUpdate struct {
	Code           string
	Status         string
	CreditorStatus string
	DebitorStatus  string
	ReversedBy     string
	ReversedAt     *time.Time
	MetadataPatch  map[string]any
}

// PartyInfo holds creditor/debitor details
type PartyInfo struct {
	AccountID     int64  `json:"account_id"`
	LedgerID      int64  `json:"ledger_id"`
	AccountType   string `json:"account_type"`            // user, partner, system
	Status        string `json:"status"`                  // pending, success, failed
	Name          string `json:"name,omitempty"`
	Phone         string `json:"phone,omitempty"`
	Email         string `json:"email,omitempty"`
	AccountNumber string `json:"account_number,omitempty"` // optional external identifier
	IsCreditor    bool   `json:"is_creditor"`
}

// Receipt represents a transaction receipt
type Receipt struct {
	ID              int64     `json:"id"`
	Code            string    `json:"code"`                    // unique receipt code
	Type            string    `json:"type"`                    // deposit, withdrawal, transfer, conversion, admin_credit
	CodedType       string    `json:"coded_type,omitempty"`    // subtype: fee, cashback, etc.
	Amount          float64   `json:"amount"`
	TransactionCost float64   `json:"transaction_cost"`         // new field: txn fee, maybe 0
	Currency        string    `json:"currency"`
	ExternalRef     string    `json:"external_ref,omitempty"`  // bank txn id, blockchain tx hash, etc.
	Status          string    `json:"status"`                  // pending, success, failed, reversed

	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
	CreatedBy  string     `json:"created_by,omitempty"`
	ReversedAt *time.Time `json:"reversed_at,omitempty"`
	ReversedBy string     `json:"reversed_by,omitempty"`

	Creditor PartyInfo `json:"creditor"`
	Debitor  PartyInfo `json:"debitor"`

	Metadata map[string]interface{} `json:"metadata,omitempty"` // flexible JSON metadata
}

// --- PartyInfo Proto helpers ---
func (p *PartyInfo) ToProto() *receiptpb.PartyInfo {
	if p == nil {
		return nil
	}
	return &receiptpb.PartyInfo{
		AccountId:     p.AccountID,
		LedgerId:      p.LedgerID,
		AccountType:   p.AccountType,
		Status:        p.Status,
		Name:          p.Name,
		Phone:         p.Phone,
		Email:         p.Email,
		AccountNumber: p.AccountNumber,
		IsCreditor:    p.IsCreditor,
	}
}

func PartyInfoFromProto(pb *receiptpb.PartyInfo) PartyInfo {
	if pb == nil {
		return PartyInfo{}
	}
	return PartyInfo{
		AccountID:     pb.GetAccountId(),
		LedgerID:      pb.GetLedgerId(),
		AccountType:   pb.GetAccountType(),
		Status:        pb.GetStatus(),
		Name:          pb.GetName(),
		Phone:         pb.GetPhone(),
		Email:         pb.GetEmail(),
		AccountNumber: pb.GetAccountNumber(),
		IsCreditor:    pb.GetIsCreditor(),
	}
}

// --- Receipt Proto helpers ---
func (r *Receipt) ToProto() *receiptpb.Receipt {
	if r == nil {
		return nil
	}

	metadata, _ := structpb.NewStruct(r.Metadata)

	var updatedAt, reversedAt *timestamppb.Timestamp
	if r.UpdatedAt != nil {
		updatedAt = timestamppb.New(*r.UpdatedAt)
	}
	if r.ReversedAt != nil {
		reversedAt = timestamppb.New(*r.ReversedAt)
	}

	return &receiptpb.Receipt{
		Id:              r.ID,
		Code:            r.Code,
		Type:            r.Type,
		CodedType:       r.CodedType,
		Amount:          r.Amount,
		TransactionCost: r.TransactionCost,  // added
		Currency:        r.Currency,
		ExternalRef:     r.ExternalRef,
		Status:          r.Status,

		CreatedAt:  timestamppb.New(r.CreatedAt),
		UpdatedAt:  updatedAt,
		CreatedBy:  r.CreatedBy,
		ReversedAt: reversedAt,
		ReversedBy: r.ReversedBy,

		Creditor: r.Creditor.ToProto(),
		Debitor:  r.Debitor.ToProto(),

		Metadata: metadata,
	}
}

func ReceiptFromProto(pb *receiptpb.Receipt) Receipt {
	if pb == nil {
		return Receipt{}
	}

	var updatedAt, reversedAt *time.Time
	if pb.GetUpdatedAt() != nil {
		t := pb.GetUpdatedAt().AsTime()
		updatedAt = &t
	}
	if pb.GetReversedAt() != nil {
		t := pb.GetReversedAt().AsTime()
		reversedAt = &t
	}

	metadata := map[string]interface{}{}
	if pb.GetMetadata() != nil {
		metadata = pb.GetMetadata().AsMap()
	}

	return Receipt{
		ID:              pb.GetId(),
		Code:            pb.GetCode(),
		Type:            pb.GetType(),
		CodedType:       pb.GetCodedType(),
		Amount:          pb.GetAmount(),
		TransactionCost: pb.GetTransactionCost(), // added
		Currency:        pb.GetCurrency(),
		ExternalRef:     pb.GetExternalRef(),
		Status:          pb.GetStatus(),

		CreatedAt:  pb.GetCreatedAt().AsTime(),
		UpdatedAt:  updatedAt,
		CreatedBy:  pb.GetCreatedBy(),
		ReversedAt: reversedAt,
		ReversedBy: pb.GetReversedBy(),

		Creditor: PartyInfoFromProto(pb.GetCreditor()),
		Debitor:  PartyInfoFromProto(pb.GetDebitor()),

		Metadata: metadata,
	}
}

// --- ReceiptUpdate helpers remain unchanged ---
func (ru *ReceiptUpdate) ToProto() *receiptpb.UpdateReceiptRequest {
	if ru == nil {
		return nil
	}

	var reversedAt *timestamppb.Timestamp
	if ru.ReversedAt != nil {
		reversedAt = timestamppb.New(*ru.ReversedAt)
	}

	metadataPatch := &structpb.Struct{}
	if ru.MetadataPatch != nil {
		metadataPatch, _ = structpb.NewStruct(ru.MetadataPatch)
	}

	return &receiptpb.UpdateReceiptRequest{
		Code:           ru.Code,
		Status:         ru.Status,
		CreditorStatus: ru.CreditorStatus,
		DebitorStatus:  ru.DebitorStatus,
		ReversedBy:     ru.ReversedBy,
		ReversedAt:     reversedAt,
		MetadataPatch:  metadataPatch,
	}
}

func ReceiptUpdateFromProto(pb *receiptpb.UpdateReceiptRequest) *ReceiptUpdate {
	if pb == nil {
		return &ReceiptUpdate{}
	}

	var reversedAt *time.Time
	if pb.ReversedAt != nil {
		t := pb.ReversedAt.AsTime()
		reversedAt = &t
	}

	var metadataPatch map[string]any
	if pb.MetadataPatch != nil {
		metadataPatch = pb.MetadataPatch.AsMap()
	}

	return &ReceiptUpdate{
		Code:           pb.GetCode(),
		Status:         pb.GetStatus(),
		CreditorStatus: pb.GetCreditorStatus(),
		DebitorStatus:  pb.GetDebitorStatus(),
		ReversedBy:     pb.GetReversedBy(),
		ReversedAt:     reversedAt,
		MetadataPatch:  metadataPatch,
	}
}
