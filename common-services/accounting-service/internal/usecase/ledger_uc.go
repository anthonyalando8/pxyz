package usecase

import (
	"accounting-service/internal/domain"
	"accounting-service/internal/repository"
	"context"
	"errors"
	"fmt"
	"time"
	"x/shared/utils/id"
	authclient "x/shared/auth"
	receiptclient "x/shared/common/receipt"
	receiptpb "x/shared/genproto/shared/accounting/receiptpb"
	authpb "x/shared/genproto/authpb"
	adminauthpb "x/shared/genproto/admin/authpb"
	patnerauthpb "x/shared/genproto/partner/authpb"

	"github.com/jackc/pgx/v5"
)

type LedgerUsecase struct {
	ledgerRepo repository.LedgerRepository
	sf *id.Snowflake
	authClient *authclient.AuthService
	receiptClient *receiptclient.ReceiptClient
}

func NewLedgerUsecase(
	ledgerRepo repository.LedgerRepository,
	sf *id.Snowflake,
	authClient *authclient.AuthService,
	receiptClient *receiptclient.ReceiptClient,
) *LedgerUsecase {
	return &LedgerUsecase{
		ledgerRepo: ledgerRepo,
		sf: sf,
		authClient: authClient,
		receiptClient: receiptClient,
	}
}

func (uc *LedgerUsecase) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return uc.ledgerRepo.BeginTx(ctx)
}

// CreateTransactionMulti handles a transaction with multiple postings
// Example: user deposit → credit user wallet + debit partner liquidity + fee
func (uc *LedgerUsecase) CreateTransactionMulti(
    ctx context.Context,
    journal *domain.Journal,
    postings []*domain.Posting,
	tx pgx.Tx,
) (journalID int64, err error) {

    if len(postings) == 0 {
        return 0, errors.New("no postings provided")
    }

    // Validate postings
    for _, p := range postings {
        if p.DrCr != "DR" && p.DrCr != "CR" {
            return 0, fmt.Errorf("invalid DR/CR for account %d", p.AccountID)
        }
        if p.Amount <= 0 {
            return 0, fmt.Errorf("amount must be positive for account %d", p.AccountID)
        }
        if p.Currency == "" {
            return 0, fmt.Errorf("currency required for account %d", p.AccountID)
        }
    }

    // Set journal defaults if needed
    if journal.IdempotencyKey == "" {
        journal.IdempotencyKey = uc.sf.Generate()
    }
    if journal.ExternalRef == "" {
        journal.ExternalRef = fmt.Sprintf("TX-%d", time.Now().UnixNano())
    }
    if journal.CreatedAt.IsZero() {
        journal.CreatedAt = time.Now()
    }

    journalID, err = uc.ledgerRepo.ApplyTransaction(ctx, journal, postings, tx)
    if err != nil {
        return 0, fmt.Errorf("failed to apply multi-posting transaction: %w", err)
    }

    return journalID, nil
}


func (uc *LedgerUsecase) createReceiptBackground(ctx context.Context, journalID int64, postings []*domain.Posting) {
	if uc.receiptClient == nil || len(postings) == 0 {
		return
	}

	go func() {
		for _, p := range postings {
			// Determine creditor and debitor based on DR/CR
			var creditorID, debitorID string
			var creditorType, debitorType string

			if p.DrCr == "CR" {
				// CR → account receives money → creditor
				creditorID = fmt.Sprint(p.AccountID)
				creditorType = "user" // default, can fetch actual type if needed
				debitorID = fmt.Sprint(0)
				debitorType = "system"
			} else {
				// DR → account pays money → debitor
				debitorID = fmt.Sprint(p.AccountID)
				debitorType = "user"
				creditorID = fmt.Sprint(0)
				creditorType = "system"
			}

			// Fetch profiles for creditor
			credEmail, credPhone, credFirst, _, resolvedCredType, err := uc.fetchProfile(ctx, creditorType, creditorID)
			if err != nil {
				credEmail, credPhone, credFirst, resolvedCredType = "", "", "", creditorType
			}

			// Fetch profiles for debitor
			debEmail, debPhone, debFirst, _, resolvedDebType, err := uc.fetchProfile(ctx, debitorType, debitorID)
			if err != nil {
				debEmail, debPhone, debFirst, resolvedDebType = "", "", "", debitorType
			}

			req := &receiptpb.CreateReceiptRequest{
				JournalId:    journalID,
				AccountId:    p.AccountID,
				Type:         p.DrCr,
				Amount:       p.Amount,
				Currency:     p.Currency,
				CodedType:    "transaction", // optional, default
				ExternalRef:  "",            // optional, default
				Creditor: &receiptpb.PartyInfo{
					Id:            creditorID,
					Type:          resolvedCredType,
					Name:          credFirst,
					Phone:         credPhone,
					Email:         credEmail,
					AccountNumber: "",
					IsCreditor:    true,
				},
				Debitor: &receiptpb.PartyInfo{
					Id:            debitorID,
					Type:          resolvedDebType,
					Name:          debFirst,
					Phone:         debPhone,
					Email:         debEmail,
					AccountNumber: "",
					IsCreditor:    false,
				},
			}

			// Call receipt service
			_, err = uc.receiptClient.Client.CreateReceipt(ctx, req)
			if err != nil {
				fmt.Printf("[WARN] failed to create receipt for posting %d: %v\n", p.ID, err)
			}
		}
	}()
}


