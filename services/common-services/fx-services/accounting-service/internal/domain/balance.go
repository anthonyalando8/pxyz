package domain

import "time"

// Balance represents current balance of an account
// Separate table for faster queries and optimistic locking
type Balance struct {
	AccountID        int64     `json:"account_id" db:"account_id"`
	Balance          float64   `json:"balance" db:"balance"`                     // Total balance in smallest currency unit
	AvailableBalance float64   `json:"available_balance" db:"available_balance"` // Available for withdrawal/trading
	PendingDebit     float64   `json:"pending_debit" db:"pending_debit"`         // Pending outgoing funds
	PendingCredit    float64   `json:"pending_credit" db:"pending_credit"`       // Pending incoming funds
	LastLedgerID     *int64    `json:"last_ledger_id,omitempty" db:"last_ledger_id"`
	Version          int64     `json:"version" db:"version"` // For optimistic locking
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// BalanceUpdate represents a balance update operation
type BalanceUpdate struct {
	AccountID     int64
	Amount        float64
	DrCr          string // "DR" or "CR"
	LedgerID      int64
	UpdatePending bool // If true, updates pending_debit/pending_credit
}

// BalanceLock represents a hold on funds
type BalanceLock struct {
	AccountID int64
	Amount    float64
	Version   int64 // Expected version for optimistic locking
}

// BalanceSnapshot represents balance at a specific point in time
// Used for balance history visualization and time-series analysis
type BalanceSnapshot struct {
	AccountID        int64       `json:"account_id"`
	AccountNumber    string      `json:"account_number,omitempty"`
	Timestamp        time.Time   `json:"timestamp"`
	Balance          float64     `json:"balance"`           // Total balance at this time
	AvailableBalance float64     `json:"available_balance"` // Available balance at this time
	Currency         string      `json:"currency"`
	AccountType      AccountType `json:"account_type"`
}

// BalanceHistoryRequest represents parameters for balance history queries
type BalanceHistoryRequest struct {
	AccountNumber string
	AccountType   AccountType
	StartDate     time.Time
	EndDate       time.Time
	Interval      BalanceInterval // Aggregation interval
}

// BalanceInterval represents time bucket for balance aggregation
type BalanceInterval string

const (
	IntervalHour  BalanceInterval = "hour"
	IntervalDay   BalanceInterval = "day"
	IntervalWeek  BalanceInterval = "week"
	IntervalMonth BalanceInterval = "month"
)

// BalanceHistory represents a series of balance snapshots over time
type BalanceHistory struct {
	AccountID     int64              `json:"account_id"`
	AccountNumber string             `json:"account_number"`
	AccountType   AccountType        `json:"account_type"`
	Currency      string             `json:"currency"`
	StartDate     time.Time          `json:"start_date"`
	EndDate       time.Time          `json:"end_date"`
	Interval      BalanceInterval    `json:"interval"`
	Snapshots     []*BalanceSnapshot `json:"snapshots"`
	MinBalance    float64            `json:"min_balance"`
	MaxBalance    float64            `json:"max_balance"`
	AvgBalance    float64            `json:"avg_balance"`
}

// AddSnapshot adds a snapshot to the history and updates statistics
func (h *BalanceHistory) AddSnapshot(snapshot *BalanceSnapshot) {
	h.Snapshots = append(h.Snapshots, snapshot)

	// Update statistics
	if len(h.Snapshots) == 1 {
		// First snapshot
		h.MinBalance = snapshot.Balance
		h.MaxBalance = snapshot.Balance
		h.AvgBalance = snapshot. Balance
	} else {
		// Update min/max
		if snapshot.Balance < h. MinBalance {
			h.MinBalance = snapshot.Balance
		}
		if snapshot.Balance > h.MaxBalance {
			h. MaxBalance = snapshot.Balance
		}

		// ✅ Incremental average calculation (more efficient)
		// Formula: new_avg = old_avg + (new_value - old_avg) / n
		n := float64(len(h.Snapshots))
		h.AvgBalance = h.AvgBalance + (snapshot.Balance - h.AvgBalance) / n
	}
}

// GetSnapshotAt returns the snapshot closest to the given time
func (h *BalanceHistory) GetSnapshotAt(t time.Time) *BalanceSnapshot {
	if len(h.Snapshots) == 0 {
		return nil
	}

	var closest *BalanceSnapshot
	var minDiff time.Duration

	for _, snapshot := range h.Snapshots {
		diff := snapshot.Timestamp.Sub(t)
		if diff < 0 {
			diff = -diff
		}

		if closest == nil || diff < minDiff {
			closest = snapshot
			minDiff = diff
		}
	}

	return closest
}

// GetBalanceChange returns the change in balance over the period
func (h *BalanceHistory) GetBalanceChange() float64 {
	if len(h.Snapshots) < 2 {
		return 0
	}

	first := h.Snapshots[0]
	last := h.Snapshots[len(h.Snapshots)-1]

	return last.Balance - first.Balance
}

// GetBalanceChangePercent returns the percentage change in balance
func (h *BalanceHistory) GetBalanceChangePercent() float64 {
	if len(h.Snapshots) < 2 {
		return 0
	}

	first := h.Snapshots[0]
	last := h.Snapshots[len(h.Snapshots)-1]

	if first.Balance == 0 {
		return 0
	}

	change := last.Balance - first.Balance
	return (float64(change) / float64(first.Balance)) * 100
}
