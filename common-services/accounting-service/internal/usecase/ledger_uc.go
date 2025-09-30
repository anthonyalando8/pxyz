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
	receiptpb "x/shared/genproto/shared/accounting/receipt/v2"
	authpb "x/shared/genproto/authpb"
	partnerclient "x/shared/partner"
	adminauthpb "x/shared/genproto/admin/authpb"
	//patnerauthpb "x/shared/genproto/partner/authpb"
	partnersvcpb "x/shared/genproto/partner/svcpb"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

)

type LedgerUsecase struct {
	ledgerRepo repository.LedgerRepository
	br repository.BalanceRepository

	sf *id.Snowflake
	authClient *authclient.AuthService
	receiptClient *receiptclient.ReceiptClientV2
	partnerClient    *partnerclient.PartnerService
	redisClient *redis.Client
	accountUC   *AccountUsecase
	ruleUC *TransactionFeeRuleUsecase
}

func NewLedgerUsecase(
	ledgerRepo repository.LedgerRepository,
	br repository.BalanceRepository,
	sf *id.Snowflake,
	authClient *authclient.AuthService,
	receiptClient *receiptclient.ReceiptClientV2,
	partnerClient *partnerclient.PartnerService,
	redisClient *redis.Client,
	accountUC   *AccountUsecase,
	ruleUC *TransactionFeeRuleUsecase,
) *LedgerUsecase {
	return &LedgerUsecase{
		ledgerRepo: ledgerRepo,
		br: br,
		sf: sf,
		authClient: authClient,
		receiptClient: receiptClient,
		partnerClient:    partnerClient,
		redisClient: redisClient,
		accountUC: accountUC,
		ruleUC: ruleUC,
	}
}

func (uc *LedgerUsecase) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return uc.ledgerRepo.BeginTx(ctx)
}

// CreateTransactionMulti handles a transaction with multiple postings
func (uc *LedgerUsecase) CreateTransactionMulti(
	ctx context.Context,
	transactionType string,
	transactionAmount float64,
	currency string, // currency of the amount
	journal *domain.Journal,
	postings []*domain.Posting,
	tx pgx.Tx,
) (*domain.Ledger, error) {
	if len(postings) < 2 {
		return nil, errors.New("transaction must have at least 2 entries (DR & CR)")
	}

	// Identify DR and CR postings
	var drPosting, crPosting *domain.Posting
	for _, p := range postings {
		switch p.DrCr {
		case "DR":
			drPosting = p
		case "CR":
			crPosting = p
		}
	}
	if drPosting == nil || crPosting == nil {
		return nil, errors.New("both DR and CR postings are required")
	}

	// Ensure journal defaults
	if journal.IdempotencyKey == "" {
		journal.IdempotencyKey = uc.sf.Generate()
	}
	if journal.ExternalRef == "" {
		journal.ExternalRef = fmt.Sprintf("TX-%d", time.Now().UnixNano())
	}
	if journal.CreatedAt.IsZero() {
		journal.CreatedAt = time.Now()
	}

	// Calculate fee in DR currency
	fee, err := uc.ruleUC.CalculateFee(
		ctx,
		transactionType,
		drPosting.Currency,          // source
		crPosting.Currency,          // target
		transactionAmount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate fee: %w", err)
	}

	// Total amount to debit from DR
	totalDebit := transactionAmount

	// Check DR balance
	balance, err := uc.br.GetCachedBalance(ctx, drPosting.AccountData.AccountNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch DR account balance: %w", err)
	}
	if balance.Balance < totalDebit {
		return nil, fmt.Errorf("insufficient funds: balance=%f, required=%f", balance.Balance, totalDebit)
	}

	// Adjust DR posting amount (already in DR currency)
	drPosting.Amount = totalDebit

	// Convert transaction amount to CR currency
	convertedAmount, err := ConvertCurrency(transactionAmount - fee, drPosting.Currency, crPosting.Currency)
	if err != nil {
		return nil, fmt.Errorf("currency conversion failed: %w", err)
	}
	crPosting.Amount = convertedAmount

	// Add profit posting if fee > 0
	if fee > 0 {
		profitAcc, err := uc.accountUC.GetSystemAccount(ctx, drPosting.Currency, "profits")
		if err != nil {
			return nil, fmt.Errorf("failed to fetch profit account: %w", err)
		}

		postings = append(postings, &domain.Posting{
			DrCr:        "CR",
			Amount:      fee,
			Currency:    drPosting.Currency,
			JournalID:   journal.ID,
			AccountID:   profitAcc.ID,
			AccountData: profitAcc,
		})
	}

	// Apply transaction atomically
	ledger, err := uc.ledgerRepo.ApplyTransaction(ctx, journal, postings, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to apply transaction: %w", err)
	}

	return ledger, nil
}

  // Background side-effect (like receipts)
    //uc.createReceiptBackground(ctx, journal.ID, postings)

func (uc *LedgerUsecase) createReceiptBackground(_ context.Context, journalID int64, postings []*domain.Posting) {
	if uc.receiptClient == nil || len(postings) != 2 {
		return
	}

	go func() {
		bgCtx := context.Background() // detached
		var creditor, debitor *domain.Posting
		for _, p := range postings {
			switch p.DrCr {
			case "CR":
				creditor = p
			case "DR":
				debitor = p
			}
		}

		if creditor == nil || debitor == nil {
			fmt.Println("[WARN] invalid postings: missing DR or CR")
			return
		}

		resolveProfile := func(p *domain.Posting) (email, phone, name, ownerType string) {
			if p.AccountData != nil {
				if p.AccountData.OwnerType == "system" {
					return "", "", p.AccountData.OwnerID, "system"
				}
				e, ph, nm, _, ot, err := uc.fetchProfile(bgCtx, p.AccountData.OwnerType, fmt.Sprint(p.AccountData.OwnerID))
				if err != nil {
					return "", "", p.AccountData.OwnerID, p.AccountData.OwnerType
				}
				return e, ph, nm, ot
			}
			return "", "", "", "user"
		}

		credEmail, credPhone, credName, credType := resolveProfile(creditor)
		debEmail, debPhone, debName, debType := resolveProfile(debitor)

		req := &receiptpb.CreateReceiptRequest{
			Type:        creditor.DrCr,
			Amount:      creditor.Amount,
			Currency:    creditor.Currency,
			CodedType:   "transaction",
			ExternalRef: "",
			Creditor: &receiptpb.PartyInfo{
				AccountType:          credType,
				Name:          credName,
				Phone:         credPhone,
				Email:         credEmail,
				AccountNumber: creditor.AccountData.AccountNumber,
				IsCreditor:    true,
			},
			Debitor: &receiptpb.PartyInfo{
				AccountType:          debType,
				Name:          debName,
				Phone:         debPhone,
				Email:         debEmail,
				AccountNumber: debitor.AccountData.AccountNumber,
				IsCreditor:    false,
			},
		}

		_, err := uc.receiptClient.Client.CreateReceipt(bgCtx, req)
		if err != nil {
			fmt.Printf("[WARN] failed to create receipt for journal %d: %v\n", journalID, err)
		}
	}()
}


func (uc *LedgerUsecase) fetchProfile(ctx context.Context, ownerType, ownerID string) (email, phone, firstName, lastName string, resolvedOwnerType string, err error) {
	if uc.authClient == nil && uc.partnerClient == nil {
		return "", "", "", "", "", fmt.Errorf("clients not initialized")
	}

	type result struct {
		email, phone, firstName, lastName, ownerType string
		ok                                           bool
	}

	tryFetch := func(clientType string) result {
		switch clientType {
		case "user":
			if uc.authClient.UserClient == nil {
				return result{}
			}
			resp, e := uc.authClient.UserClient.GetUserProfile(ctx, &authpb.GetUserProfileRequest{UserId: ownerID})
			if e != nil || resp == nil || !resp.Ok || resp.User == nil {
				return result{}
			}
			return result{resp.User.Email, resp.User.Phone, resp.User.FirstName, resp.User.LastName, "user", true}

		case "partner":
			if uc.partnerClient == nil || uc.partnerClient.Client == nil {
				return result{}
			}

			// Fetch partner info by ID
			resp, e := uc.partnerClient.Client.GetPartners(ctx, &partnersvcpb.GetPartnersRequest{
				PartnerIds: []string{ownerID},
			})
			if e != nil || resp == nil || len(resp.Partners) == 0 {
				return result{}
			}

			partner := resp.Partners[0]
			return result{
				email:     partner.ContactEmail,
				phone:     partner.ContactPhone,
				firstName: partner.Name, // using name as firstName for partner
				lastName:  "",           // no last name for partner
				ownerType: "partner",
				ok:        true,
			}


		case "admin":
			if uc.authClient.AdminClient == nil {
				return result{}
			}
			resp, e := uc.authClient.AdminClient.GetUserProfile(ctx, &adminauthpb.GetUserProfileRequest{UserId: ownerID})
			if e != nil || resp == nil || !resp.Ok || resp.User == nil {
				return result{}
			}
			return result{resp.User.Email, resp.User.Phone, resp.User.FirstName, resp.User.LastName, "admin", true}
		}
		return result{}
	}

	// If ownerType is known, fetch directly
	if ownerType != "" {
		res := tryFetch(ownerType)
		if !res.ok {
			return "", "", "", "", "", fmt.Errorf("failed to fetch profile for type: %s", ownerType)
		}
		return res.email, res.phone, res.firstName, res.lastName, res.ownerType, nil
	}

	// ownerType unknown → concurrent fetch
	types := []string{"user", "partner", "admin"}
	ch := make(chan result, len(types))

	for _, t := range types {
		go func(t string) {
			ch <- tryFetch(t)
		}(t)
	}

	for i := 0; i < len(types); i++ {
		res := <-ch
		if res.ok {
			return res.email, res.phone, res.firstName, res.lastName, res.ownerType, nil
		}
	}

	return "", "", "", "", "", fmt.Errorf("profile not found for ownerID: %s", ownerID)
}


