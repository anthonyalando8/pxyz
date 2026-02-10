package hrest

import (
	"accounting-service/internal/domain"
	"accounting-service/internal/usecase"
	"encoding/json"
	//"fmt"
	"log"
	"net/http"
	"time"
	"x/shared/response"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
)

type AccountingRestHandler struct {
	accountUC   *usecase.AccountUsecase
	txUC        *usecase.TransactionUsecase
	statementUC *usecase.StatementUsecase
	journalUC   *usecase.JournalUsecase
	ledgerUC    *usecase.LedgerUsecase
	feeUC       *usecase.TransactionFeeUsecase
	feeRuleUC   *usecase.TransactionFeeRuleUsecase
	agentUC     usecase.AgentUsecase
	approvalUC  *usecase.TransactionApprovalUsecase
}

func NewAccountingRestHandler(
    accountUC *usecase. AccountUsecase,
    txUC *usecase.TransactionUsecase,
    statementUC *usecase.StatementUsecase,
    journalUC *usecase.JournalUsecase,
    ledgerUC *usecase.LedgerUsecase,
    feeUC *usecase. TransactionFeeUsecase,
    feeRuleUC *usecase.TransactionFeeRuleUsecase,
    agentUC usecase. AgentUsecase,
    approvalUC *usecase. TransactionApprovalUsecase,
) *AccountingRestHandler {
    return &AccountingRestHandler{
        accountUC:   accountUC,
        txUC:        txUC,
        statementUC: statementUC,
        journalUC:   journalUC,
        ledgerUC:    ledgerUC,
        feeUC:       feeUC,
        feeRuleUC:   feeRuleUC,
        agentUC:     agentUC,
        approvalUC:  approvalUC,  
    }
}
type OwnerStatementJSON struct {
    OwnerType string    `json:"owner_type"`
    OwnerID   string    `json:"owner_id"`
	AccountType string	`json:"account_type"`
    From      time.Time `json:"from"`
    To        time.Time `json:"to"`
}

func (h * AccountingRestHandler) GetOwnerStatement(w http.ResponseWriter, r *http.Request) {
	// Implementation for fetching owner statement
	var in OwnerStatementJSON
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response. Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if in.OwnerID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if in.OwnerType == "" {
		in.OwnerType = "user"
	}
	if in.AccountType == "" {
		in.AccountType = "real"
	}
	if in.From.IsZero() {
		in.From = time.Now().AddDate(0, -1, 0) // Default to 1 month ago
	}
	if in.To.IsZero() {
		in.To = time.Now() // Default to now
	}
	ownerType := mapOwnerType(in.OwnerType)
	accountType := mapAccountType(in.AccountType)
	statement, err := h.statementUC.GetOwnerStatement(r.Context(), ownerType, in.OwnerID, accountType, in.From, in.To)	
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to get owner statement: "+err.Error())
		return
	}
	response.JSON(w, http.StatusOK, statement)
}
func (h *AccountingRestHandler) registerRoutes(r chi.Router) {
	r.Route("/accounting", func(r chi.Router) {
		r.Post("/statement/owner", h.GetOwnerStatement)

		// future routes 👇
		// r.Post("/transaction", h.CreateTransaction)
		// r.Get("/ledger/{id}", h.GetLedger)
	})
}


func (h *AccountingRestHandler) Start(port string) {
	r := chi.NewRouter()

	// Middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Register routes
	h.registerRoutes(r)

	addr := port

	server := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	log.Printf("🚀 Accounting REST service running on %s", addr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("failed to start server: %v", err)
	}
}


func mapOwnerType(ownerType string) domain.OwnerType {
	switch ownerType {
	case "user":
		return domain.OwnerTypeUser
	case "agent":
		return domain.OwnerTypeAgent
	case "system":
		return domain.OwnerTypeSystem
	case "partner":
		return domain.OwnerTypePartner
	default:
		return domain.OwnerTypeUser // Default to user if not specified
	}
}

func mapAccountType(accountType string) domain.AccountType {
	switch accountType {
	case "real":
		return domain.AccountTypeReal
	case "demo":
		return domain.AccountTypeDemo
	default:
		return domain.AccountTypeReal // Default to real if not specified
	}
}