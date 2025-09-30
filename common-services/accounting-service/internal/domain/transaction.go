package domain

// Ledger aggregates all related entities
type Ledger struct {
    Journal  *Journal
    Postings []*Posting
    Receipt  *Receipt
}