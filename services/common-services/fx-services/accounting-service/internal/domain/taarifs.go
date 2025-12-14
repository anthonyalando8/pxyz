// domain/tariff.go
package domain

// import "time"

// // ============================================================================
// // TARIFF MODELS
// // ============================================================================

// type Tariff struct {
// 	ID          int64                  `json:"id"`
// 	TariffCode  string                 `json:"tariff_code"`
// 	TariffName  string                 `json:"tariff_name"`
// 	Description *string                `json:"description,omitempty"`
// 	TariffType  string                 `json:"tariff_type"` // 'user', 'agent', 'partner', 'system'
// 	IsDefault   bool                   `json:"is_default"`
// 	IsActive    bool                   `json:"is_active"`
// 	Priority    int                    `json:"priority"`
// 	ValidFrom   time.Time              `json:"valid_from"`
// 	ValidTo     *time.Time             `json:"valid_to,omitempty"`
// 	Metadata    map[string]interface{} `json:"metadata,omitempty"`
// 	CreatedAt   time.Time              `json:"created_at"`
// 	UpdatedAt   time.Time              `json:"updated_at"`
// 	CreatedBy   *string                `json:"created_by,omitempty"`
// }

// type TariffFeeRule struct {
// 	ID               int64      `json:"id"`
// 	TariffID         int64      `json:"tariff_id"`
// 	FeeRuleID        int64      `json:"fee_rule_id"`
// 	OverrideFeeValue *float64   `json:"override_fee_value,omitempty"`
// 	OverrideMinFee   *float64   `json:"override_min_fee,omitempty"`
// 	OverrideMaxFee   *float64   `json:"override_max_fee,omitempty"`
// 	Priority         int        `json:"priority"`
// 	IsActive         bool       `json:"is_active"`
// 	CreatedAt        time.Time  `json:"created_at"`
// 	UpdatedAt        time.Time  `json:"updated_at"`
// }

// // ============================================================================
// // CREATE/UPDATE DTOs
// // ============================================================================

// type TariffCreate struct {
// 	TariffCode  string                 `json:"tariff_code"`
// 	TariffName  string                 `json:"tariff_name"`
// 	Description *string                `json:"description,omitempty"`
// 	TariffType  string                 `json:"tariff_type"`
// 	IsDefault   bool                   `json:"is_default"`
// 	IsActive    bool                   `json:"is_active"`
// 	Priority    int                    `json:"priority"`
// 	ValidFrom   time.Time              `json:"valid_from"`
// 	ValidTo     *time.Time             `json:"valid_to,omitempty"`
// 	Metadata    map[string]interface{} `json:"metadata,omitempty"`
// 	CreatedBy   *string                `json:"created_by,omitempty"`
// }

// type TariffUpdate struct {
// 	TariffName  *string                `json:"tariff_name,omitempty"`
// 	Description *string                `json:"description,omitempty"`
// 	IsActive    *bool                  `json:"is_active,omitempty"`
// 	Priority    *int                   `json:"priority,omitempty"`
// 	ValidTo     *time.Time             `json:"valid_to,omitempty"`
// 	Metadata    map[string]interface{} `json:"metadata,omitempty"`
// }

// type TariffFeeRuleCreate struct {
// 	TariffID         int64    `json:"tariff_id"`
// 	FeeRuleID        int64    `json:"fee_rule_id"`
// 	OverrideFeeValue *float64 `json:"override_fee_value,omitempty"`
// 	OverrideMinFee   *float64 `json:"override_min_fee,omitempty"`
// 	OverrideMaxFee   *float64 `json:"override_max_fee,omitempty"`
// 	Priority         int      `json:"priority"`
// 	IsActive         bool     `json:"is_active"`
// }

// // ============================================================================
// // FILTERS
// // ============================================================================

// type TariffFilter struct {
// 	TariffType *string    `json:"tariff_type,omitempty"`
// 	IsDefault  *bool      `json:"is_default,omitempty"`
// 	IsActive   *bool      `json:"is_active,omitempty"`
// 	ValidAt    *time.Time `json:"valid_at,omitempty"`
// 	Limit      int        `json:"limit,omitempty"`
// 	Offset     int        `json:"offset,omitempty"`
// }

// // ============================================================================
// // RESPONSE WITH FEE RULES
// // ============================================================================

// type TariffWithFeeRules struct {
// 	Tariff   *Tariff                      `json:"tariff"`
// 	FeeRules []*TariffFeeRuleWithDetails  `json:"fee_rules"`
// }

// type TariffFeeRuleWithDetails struct {
// 	TariffFeeRule        *TariffFeeRule        `json:"tariff_fee_rule"`
// 	BaseFeeRule          *TransactionFeeRule   `json:"base_fee_rule"`
// 	EffectiveFeeValue    float64               `json:"effective_fee_value"`
// 	EffectiveMinFee      *float64              `json:"effective_min_fee,omitempty"`
// 	EffectiveMaxFee      *float64              `json:"effective_max_fee,omitempty"`
// }