package app

import (
	"database/sql"
	"encoding/json"
)

type mcpPeriodInput struct {
	Name  string `json:"name"`
	Start string `json:"start_date"`
	End   string `json:"end_date"`
}

// Bind automatic reassignment and copied recurring limits, including source
// versions/names. Transaction descriptions, notes and private accounts are excluded.
func prepareMCPPeriod(tx *sql.Tx, input *mcpPeriodInput) (any, error) {
	if input == nil {
		return nil, fail(400, "Provide period with name, start_date and end_date")
	}
	b := periodInput{Name: input.Name, Start: input.Start, End: input.End}
	if err := validatePeriod(b); err != nil {
		return nil, err
	}
	if queryInt(tx, "SELECT COUNT(*) FROM periods WHERE start_date=? AND end_date=?", b.Start, b.End) > 0 {
		return nil, fail(409, "A period with these dates already exists; use list_budget_periods")
	}
	preview, err := periodChanges(tx, 0, b)
	if err != nil {
		return nil, err
	}
	previous := queryInt(tx, "SELECT id FROM periods ORDER BY start_date DESC,id DESC LIMIT 1")
	budget, err := budgetEvidenceTx(tx, previous)
	if err != nil {
		return nil, err
	}
	return map[string]any{"reassignment": preview, "copy_from_period_id": previous, "source_budget": budget}, nil
}

func (a *App) applyMCPPeriod(tx *sql.Tx, identity mcpIdentity, exact mcpExactChange, applying bool) (int64, error) {
	current, err := prepareMCPPeriod(tx, exact.Change.Period)
	if err != nil {
		return 0, err
	}
	expected, _ := json.Marshal(exact.PeriodEffects)
	actual, _ := json.Marshal(current)
	if exact.PeriodEffects == nil || string(expected) != string(actual) {
		return 0, fail(409, "Periods, transactions or recurring budgets changed; prepare the proposal again")
	}
	input := exact.Change.Period
	preview := current.(map[string]any)["reassignment"].(map[string]any)
	id, err := a.createPeriodTx(tx, identity.User, periodInput{Name: input.Name, Start: input.Start, End: input.End, PreviewToken: preview["preview_token"].(string)})
	if err != nil {
		return 0, err
	}
	if applying {
		after, err := budgetEvidenceTx(tx, id)
		if err != nil {
			return 0, err
		}
		expected, _ := json.Marshal(exact.PeriodAfter)
		actual, _ := json.Marshal(after)
		if string(expected) != string(actual) {
			return 0, fail(409, "Copied budget effects changed; prepare the proposal again")
		}
	}
	return id, nil
}
