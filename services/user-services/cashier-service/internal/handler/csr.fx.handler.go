package handler

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	partnersvcpb "x/shared/genproto/partner/svcpb"
	accountingpb "x/shared/genproto/shared/accounting/v1"
)

// Seed the global PRNG once.
func init() {
	rand.Seed(time.Now().UnixNano())
}

// ============================================================================
// ENUM MAPPERS
// ============================================================================

func mapOwnerType(s string) accountingpb.OwnerType {
	switch s {
	case "user":
		return accountingpb.OwnerType_OWNER_TYPE_USER
	case "partner":
		return accountingpb.OwnerType_OWNER_TYPE_PARTNER
	case "system":
		return accountingpb.OwnerType_OWNER_TYPE_SYSTEM
	case "admin":
		return accountingpb.OwnerType_OWNER_TYPE_ADMIN
	case "agent":
		return accountingpb.OwnerType_OWNER_TYPE_AGENT
	default:
		return accountingpb.OwnerType_OWNER_TYPE_UNSPECIFIED
	}
}

func mapAccountType(s string) accountingpb.AccountType {
	switch s {
	case "demo":
		return accountingpb.AccountType_ACCOUNT_TYPE_DEMO
	case "real":
		return accountingpb.AccountType_ACCOUNT_TYPE_REAL
	default:
		return accountingpb.AccountType_ACCOUNT_TYPE_REAL // Default to real
	}
}

// ============================================================================
// PARTNER HELPERS
// ============================================================================

// GetPartnersByService fetches partners offering a specific service
func (h *PaymentHandler) GetPartnersByService(ctx context.Context, service string) ([]*partnersvcpb.Partner, error) {
	if service == "" {
		return nil, fmt.Errorf("service cannot be empty")
	}

	req := &partnersvcpb.GetPartnersByServiceRequest{
		Service: service,
	}

	res, err := h.partnerClient.Client.GetPartnersByService(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch partners by service '%s': %w", service, err)
	}

	if len(res.Partners) == 0 {
		return nil, fmt.Errorf("no partners found for service '%s'", service)
	}

	return res.Partners, nil
}

// GetPartnerByID fetches a specific partner by ID
func (h *PaymentHandler) GetPartnerByID(ctx context.Context, partnerID string) (*partnersvcpb.Partner, error) {
	if partnerID == "" {
		return nil, fmt.Errorf("partner_id cannot be empty")
	}

	req := &partnersvcpb.GetPartnersRequest{
		PartnerIds: []string{partnerID},
	}

	res, err := h.partnerClient.Client.GetPartners(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch partner '%s': %w", partnerID, err)
	}

	if len(res.Partners) == 0 {
		return nil, fmt.Errorf("partner '%s' not found", partnerID)
	}

	return res.Partners[0], nil
}

// SelectRandomPartner selects a random partner from a list

// SelectRandomPartner selects a random partner from the list
func SelectRandomPartner(partners []*partnersvcpb.Partner) *partnersvcpb.Partner {
	if len(partners) == 0 {
		return nil
	}
	if len(partners) == 1 {
		return partners[0]
	}
	return partners[rand.Intn(len(partners))]
}

// ============================================================================
// ACCOUNTING HELPERS
// ============================================================================

// GetAccounts fetches accounts for a given owner ID and ownerType
func (h *PaymentHandler) GetAccounts(ctx context.Context, ownerID, ownerType string) (*accountingpb.GetAccountsByOwnerResponse, error) {
	if ownerID == "" || ownerType == "" {
		return nil, fmt.Errorf("ownerID and ownerType cannot be empty")
	}

	req := &accountingpb.GetAccountsByOwnerRequest{
		OwnerType:   mapOwnerType(ownerType),
		OwnerId:     ownerID,
		AccountType: accountingpb.AccountType_ACCOUNT_TYPE_REAL, // Default to real accounts
	}

	resp, err := h.accountingClient.Client.GetAccountsByOwner(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch accounts for ownerID=%s, ownerType=%s: %w", ownerID, ownerType, err)
	}

	return resp, nil
}

// GetAccountByCurrency fetches a single account number for a given ownerID, ownerType, and currency
func (h *PaymentHandler) GetAccountByCurrency(ctx context.Context, ownerID, ownerType, currency string, purpose *accountingpb.AccountPurpose) (string, error) {
	if ownerID == "" || ownerType == "" || currency == "" {
		return "", fmt.Errorf("ownerID, ownerType, and currency cannot be empty")
	}
	accountPurpose := accountingpb.AccountPurpose_ACCOUNT_PURPOSE_WALLET
	if purpose != nil {
		accountPurpose = *purpose
	}

	resp, err := h.GetAccounts(ctx, ownerID, ownerType)
	if err != nil {
		return "", err
	}

	// Find account with matching currency and wallet purpose
	for _, acct := range resp.Accounts {
		if acct.Currency == currency && acct.Purpose == accountPurpose {
			return acct.AccountNumber, nil
		}
	}

	// If no wallet account found, return first account with matching currency
	for _, acct := range resp.Accounts {
		if acct.Currency == currency {
			return acct.AccountNumber, nil
		}
	}

	return "", fmt.Errorf("no account found for ownerID=%s, ownerType=%s with currency=%s", ownerID, ownerType, currency)
}

// GetAccountBalance fetches the balance for a specific account number
func (h *PaymentHandler) GetAccountBalance(ctx context.Context, accountNumber string) (*accountingpb.Balance, error) {
	if accountNumber == "" {
		return nil, fmt.Errorf("account_number cannot be empty")
	}

	req := &accountingpb.GetBalanceRequest{
		Identifier: &accountingpb.GetBalanceRequest_AccountNumber{
			AccountNumber: accountNumber,
		},
	}

	resp, err := h.accountingClient.Client.GetBalance(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch balance for account %s: %w", accountNumber, err)
	}

	return resp.Balance, nil
}

// ============================================================================
// COMBINED HELPERS
// ============================================================================

// GetPartnerAndUserAccounts fetches random partner and their account + user account concurrently
func (h *PaymentHandler) GetPartnerAndUserAccounts(ctx context.Context, service, currency, userID string) (partnerAccount, userAccount string, partner *partnersvcpb.Partner, err error) {
	if service == "" || currency == "" || userID == "" {
		return "", "", nil, fmt.Errorf("service, currency, and userID cannot be empty")
	}

	var wg sync.WaitGroup
	var partnerErr, userErr error
	var partners []*partnersvcpb.Partner

	wg.Add(2)

	// Fetch partners concurrently
	go func() {
		defer wg.Done()
		partners, partnerErr = h.GetPartnersByService(ctx, service)
		if partnerErr != nil || len(partners) == 0 {
			if partnerErr == nil {
				partnerErr = fmt.Errorf("no partners found for service %s", service)
			}
			return
		}
		partner = SelectRandomPartner(partners)
	}()

	// Fetch user account concurrently
	go func() {
		defer wg.Done()
		userAccount, userErr = h.GetAccountByCurrency(ctx, userID, "user", currency, nil)
	}()

	wg.Wait()

	if partnerErr != nil {
		return "", "", nil, fmt.Errorf("partner error: %w", partnerErr)
	}
	if userErr != nil {
		return "", "", nil, fmt.Errorf("user account error: %w", userErr)
	}
	if partner == nil {
		return "", "", nil, fmt.Errorf("failed to select partner")
	}

	// Fetch selected partner's account
	partnerAccount, err = h.GetAccountByCurrency(ctx, partner.Id, "partner", currency, nil)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to fetch partner account: %w", err)
	}

	return partnerAccount, userAccount, partner, nil
}

// GetPartnerAndUserAccountsByPartnerID fetches specific partner and accounts
func (h *PaymentHandler) GetPartnerAndUserAccountsByPartnerID(ctx context.Context, partnerID, currency, userID string) (partnerAccount, userAccount string, partner *partnersvcpb.Partner, err error) {
	if partnerID == "" || currency == "" || userID == "" {
		return "", "", nil, fmt.Errorf("partnerID, currency, and userID cannot be empty")
	}

	var wg sync.WaitGroup
	var partnerErr, partnerAcctErr, userErr error

	wg.Add(3)

	// Fetch partner details
	go func() {
		defer wg.Done()
		partner, partnerErr = h.GetPartnerByID(ctx, partnerID)
	}()

	// Fetch partner account
	go func() {
		defer wg.Done()
		partnerAccount, partnerAcctErr = h.GetAccountByCurrency(ctx, partnerID, "partner", currency, nil)
	}()

	// Fetch user account
	go func() {
		defer wg.Done()
		userAccount, userErr = h.GetAccountByCurrency(ctx, userID, "user", currency,nil)
	}()

	wg.Wait()

	if partnerErr != nil {
		return "", "", nil, fmt.Errorf("failed to fetch partner: %w", partnerErr)
	}
	if partnerAcctErr != nil {
		return "", "", nil, fmt.Errorf("failed to fetch partner account: %w", partnerAcctErr)
	}
	if userErr != nil {
		return "", "", nil, fmt.Errorf("failed to fetch user account: %w", userErr)
	}

	return partnerAccount, userAccount, partner, nil
}

// ============================================================================
// VALIDATION HELPERS
// ============================================================================

// ValidatePartnerService checks if a partner offers a specific service
func (h *PaymentHandler) ValidatePartnerService(ctx context.Context, partnerID, service string) error {
	partners, err := h.GetPartnersByService(ctx, service)
	if err != nil {
		return err
	}

	for _, p := range partners {
		if p.Id == partnerID {
			return nil // Partner found and offers this service
		}
	}

	return fmt.Errorf("partner '%s' does not offer service '%s'", partnerID, service)
}

// ValidateAccountOwnership verifies that an account belongs to a specific owner
func (h *PaymentHandler) ValidateAccountOwnership(ctx context.Context, accountNumber, ownerID, ownerType string) error {
	req := &accountingpb.GetAccountRequest{
		Identifier: &accountingpb.GetAccountRequest_AccountNumber{
			AccountNumber: accountNumber,
		},
	}

	resp, err := h.accountingClient.Client.GetAccount(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to fetch account: %w", err)
	}

	if resp.Account.OwnerId != ownerID {
		return fmt.Errorf("account does not belong to owner %s", ownerID)
	}

	expectedOwnerType := mapOwnerType(ownerType)
	if resp.Account.OwnerType != expectedOwnerType {
		return fmt.Errorf("account owner type mismatch")
	}

	return nil
}

// ============================================================================
// CONVERSION HELPERS
// ============================================================================

// Note: atomic-unit conversion helpers removed — handlers should pass decimal float64 amounts directly
// FormatCurrency formats an amount with currency symbol
func FormatCurrency(amount float64, currency string) string {
	return fmt.Sprintf("%.2f %s", amount, currency)
}