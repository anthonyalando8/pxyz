package domain

// AccountStatement aggregates postings and balance for an account
type AccountStatement struct {
	AccountID int64       `json:"account_id"`
	Postings  []*Posting  `json:"postings"`
	Balance   float64     `json:"balance"`
}
