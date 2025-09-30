package handler

import (
	//"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"
	"strconv"

	"x/shared/auth/middleware"
	accountingpb "x/shared/genproto/shared/accounting/accountingpb"
	"x/shared/response"

	"google.golang.org/protobuf/types/known/timestamppb"
	//"google.golang.org/protobuf/types/known/timestamppb"
)

// ---------------- Adapter Structs ----------------
type GetAccountsJSON struct {
	OwnerType string `json:"owner_type"` // "USER", "PARTNER", etc
	OwnerID   string `json:"owner_id"`
}

type AccountStatementJSON struct {
	AccountNumber string     `json:"account_number"`
	From      time.Time `json:"from"`
	To        time.Time `json:"to"`
}

type OwnerStatementJSON struct {
	OwnerType string    `json:"owner_type"`
	OwnerID   string    `json:"owner_id"`
	From      time.Time `json:"from"`
	To        time.Time `json:"to"`
}

type JournalPostingsJSON struct {
	JournalID int64 `json:"journal_id"`
}

type DailyReportJSON struct {
	Date time.Time `json:"date"`
}

// ---------------- Helpers ----------------
func mapOwnerType(s string) accountingpb.OwnerType {
	switch s {
	case "user":
		return accountingpb.OwnerType_USER
	case "partner":
		return accountingpb.OwnerType_PARTNER
	case "system":
		return accountingpb.OwnerType_SYSTEM
	case "admin":
		return accountingpb.OwnerType_ADMIN
	default:
		return accountingpb.OwnerType_OWNER_TYPE_UNSPECIFIED
	}
}
// ---------------- Accounting Management ----------------

// POST /accounts
func (h *AdminHandler) CreateAccounts(w http.ResponseWriter, r *http.Request) {
	var req accountingpb.CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Call accounting gRPC
	resp, err := h.accountingClient.Client.CreateAccounts(r.Context(), &req)
	if err != nil {
		// upstream failure — map to Bad Gateway
		response.Error(w, http.StatusBadGateway, "failed to create accounts: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

type PostTransactionDTO struct {
    From        string   `json:"from"`        // debit account
    To          string   `json:"to"`          // credit account
    Amount      float64 `json:"amount"`
    Currency    string  `json:"currency"`
    Description string  `json:"description"`
}

// POST /transactions
func (h *AdminHandler) PostTransaction(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from context
	requestedUserID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || requestedUserID == "" {
		log.Printf("[WARN] Unauthorized access attempt from %s", r.RemoteAddr)
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Decode request body into DTO
	var dto PostTransactionDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Basic validations
	if dto.From == dto.To {
		response.Error(w, http.StatusBadRequest, "from and to accounts must be different")
		return
	}
	if dto.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "amount must be greater than zero")
		return
	}

	// Convert requestedUserID from string to int64
	requestedUserIDInt, err := strconv.ParseInt(requestedUserID, 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	// Construct gRPC request
	grpcReq := &accountingpb.CreateTransactionRequest{
		Description:    dto.Description,
		CreatedByType:  accountingpb.OwnerType_ADMIN, // Adjust based on auth context
		CreatedByUser:  requestedUserIDInt,
		TransactionType: "transfer",
		RequireApproval: true,
		ApplyFee: true,
		Entries: []*accountingpb.TransactionEntry{
			{
				AccountNumber: dto.From,
				DrCr:      accountingpb.DrCr_DR,
				Amount:    dto.Amount,
				Currency:  dto.Currency,
			},
			{
				AccountNumber: dto.To,
				DrCr:      accountingpb.DrCr_CR,
				Amount:    dto.Amount,
				Currency:  dto.Currency,
			},
		},
	}

	// Call gRPC service
	resp, err := h.accountingClient.Client.PostTransaction(r.Context(), grpcReq)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to post transaction: "+err.Error())
		return
	}

	// Send response
	response.JSON(w, http.StatusOK, resp)
}



func (h *AdminHandler) GetUserAccounts(w http.ResponseWriter, r *http.Request) {
	var in GetAccountsJSON
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req := &accountingpb.GetAccountsRequest{
		OwnerType: mapOwnerType(in.OwnerType),
		OwnerId:   in.OwnerID,
	}

	resp, err := h.accountingClient.Client.GetUserAccounts(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to fetch accounts: "+err.Error())
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

func (h *AdminHandler) GetAccountStatement(w http.ResponseWriter, r *http.Request) {
	var in AccountStatementJSON
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req := &accountingpb.AccountStatementRequest{
		AccountNumber: in.AccountNumber,
		From:      timestamppb.New(in.From),
		To:        timestamppb.New(in.To),
	}

	resp, err := h.accountingClient.Client.GetAccountStatement(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to fetch account statement: "+err.Error())
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

func (h *AdminHandler) GetOwnerStatement(w http.ResponseWriter, r *http.Request) {
	var in OwnerStatementJSON
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req := &accountingpb.OwnerStatementRequest{
		OwnerType: mapOwnerType(in.OwnerType),
		OwnerId:   in.OwnerID,
		From:      timestamppb.New(in.From),
		To:        timestamppb.New(in.To),
	}

	stream, err := h.accountingClient.Client.GetOwnerStatement(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to request owner statement: "+err.Error())
		return
	}

	var results []*accountingpb.AccountStatement
	for {
		stmt, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			response.Error(w, http.StatusBadGateway, "error receiving owner statement stream: "+err.Error())
			return
		}
		results = append(results, stmt)
	}

	response.JSON(w, http.StatusOK, results)
}

func (h *AdminHandler) GetJournalPostings(w http.ResponseWriter, r *http.Request) {
	var in JournalPostingsJSON
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req := &accountingpb.JournalPostingsRequest{
		JournalId: in.JournalID,
	}

	stream, err := h.accountingClient.Client.GetJournalPostings(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to request journal postings: "+err.Error())
		return
	}

	var postings []*accountingpb.Posting
	for {
		p, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			response.Error(w, http.StatusBadGateway, "error receiving journal postings stream: "+err.Error())
			return
		}
		postings = append(postings, p)
	}

	response.JSON(w, http.StatusOK, postings)
}

func (h *AdminHandler) GenerateDailyReport(w http.ResponseWriter, r *http.Request) {
	var in DailyReportJSON
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req := &accountingpb.DailyReportRequest{
		Date: timestamppb.New(in.Date),
	}

	stream, err := h.accountingClient.Client.GenerateDailyReport(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "failed to request daily report: "+err.Error())
		return
	}

	var reports []*accountingpb.DailyReport
	for {
		dr, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			response.Error(w, http.StatusBadGateway, "error receiving daily report stream: "+err.Error())
			return
		}
		reports = append(reports, dr)
	}

	response.JSON(w, http.StatusOK, reports)
}