package domain

import "time"

type Country struct {
    ID           int64
    ISO2         string
    ISO3         string
    Name         string
    PhoneCode    string
    CurrencyCode string
    CurrencyName string
    Region       string
    Subregion    string
    FlagURL      string
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
