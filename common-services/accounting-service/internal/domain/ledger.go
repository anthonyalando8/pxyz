package domain

// Ledger aggregates all related entities
type Ledger struct {
    Account  *Account
    Journal  *Journal
    Postings []*Posting
    Balance  *Balance
    Receipt  *Receipt
}