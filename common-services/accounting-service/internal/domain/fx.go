package domain

import(
	"time"
)

type Currency struct {
	Code      string
	Name      string
	Decimals  int16
	CreatedAt time.Time
	UpdatedAt time.Time
}

type FXRate struct {
	ID           int64
	BaseCurrency  string
	QuoteCurrency string
	Rate          float64
	AsOf          time.Time
	CreatedAt     time.Time
}
