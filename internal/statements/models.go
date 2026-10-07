package statements

import (
	"finance-tracker/internal/classification"
)

type SourceRow struct {
	TransactionID         int64                 `json:"transaction_id,omitempty"`
	SourceBankRow         int                   `json:"source_bank_row,omitempty"`
	SourceComponent       string                `json:"source_component,omitempty"`
	SourceBankDescription string                `json:"source_bank_description,omitempty"`
	SourceServiceFee      *int64                `json:"source_service_fee_cents,omitempty"`
	SourceNetKey          string                `json:"source_net_key,omitempty"`
	SourceReference       string                `json:"source_reference,omitempty"`
	SpendingGroupID       *int64                `json:"spending_group_id,omitempty"`
	RuleMatchCount        int                   `json:"rule_match_count,omitempty"`
	RuleMatches           []classification.Rule `json:"rule_matches,omitempty"`
	RuleConflict          bool                  `json:"rule_conflict,omitempty"`
	Row                   int                   `json:"row"`
	Date                  string                `json:"date"`
	Amount                int64                 `json:"amount_cents"`
	Description           string                `json:"description"`
	FITID                 string                `json:"fitid,omitempty"`
	Balance               *int64                `json:"balance_cents,omitempty"`
	CategoryID            *int64                `json:"category_id,omitempty"`
	MerchantID            *int64                `json:"merchant_id,omitempty"`
	MerchantName          string                `json:"merchant_name,omitempty"`
	Suggestion            string                `json:"suggestion,omitempty"`
	Error                 string                `json:"error,omitempty"`
	Duplicate             string                `json:"duplicate,omitempty"`
	Candidates            []map[string]any      `json:"candidates,omitempty"`
}
type ParsedFile struct {
	AccountName           string            `json:"account_name,omitempty"`
	RunID                 string            `json:"run_id,omitempty"`
	Coverage              *FNBCoverage      `json:"coverage,omitempty"`
	ClassificationVersion string            `json:"classification_version,omitempty"`
	Decisions             map[string]string `json:"decisions,omitempty"`
	Inserted              int               `json:"inserted"`
	Skipped               int               `json:"skipped"`
	ID                    int64             `json:"id"`
	Name                  string            `json:"name"`
	Hash                  string            `json:"hash"`
	Format                string            `json:"format"`
	AccountID             string            `json:"bank_id"`
	Currency              string            `json:"currency"`
	AccountType           string            `json:"account_type,omitempty"`
	StartDate             string            `json:"start_date,omitempty"`
	EndDate               string            `json:"end_date,omitempty"`
	Balance               *int64            `json:"balance_cents,omitempty"`
	BalanceDate           string            `json:"balance_date,omitempty"`
	Rows                  []SourceRow       `json:"rows"`
	Error                 string            `json:"error,omitempty"`
	AlreadyImported       bool              `json:"already_imported"`
}
type InputFile struct {
	Name    string
	Content []byte
}
