package app

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Scope and version are pointers so omitted values cannot silently select the
// ungrouped budget or overwrite a never-customized preference.
type mcpBudgetAlertScope struct {
	CategoryID int64  `json:"category_id"`
	GroupID    *int64 `json:"group_id"`
}
type mcpBudgetAlertInput struct {
	mcpBudgetAlertScope
	Version   *int64 `json:"version"`
	Enabled   *bool  `json:"enabled,omitempty"`
	Threshold *int64 `json:"threshold,omitempty"`
	Reset     bool   `json:"reset,omitempty"`
}

func authorizeMCPBudgetAlerts(q queryer, identity mcpIdentity, p mcpPermissions, write bool) error {
	if !p.ReadBudgetAlerts || (write && !p.Capabilities.ManageBudgetAlerts) || len(p.Constraints.AccountIDs) > 0 || p.Constraints.SelectedAccounts || !identity.User.Member {
		return fail(403, "Personal budget alert consent and unrestricted household access required")
	}
	if queryInt(q, "SELECT COUNT(*) FROM users WHERE id=? AND budget_member=1 AND disabled=0 AND deleted_at IS NULL", identity.User.ID) != 1 {
		return fail(403, "Household access is unavailable")
	}
	return nil
}

// This explicit output contract never includes global preferences, inbox
// content, notification conditions, diagnostics or push device credentials.
func readBudgetAlertPreference(q queryer, u User, scope mcpBudgetAlertScope) (budgetAlertPreference, error) {
	var item budgetAlertPreference
	if scope.CategoryID <= 0 || scope.GroupID == nil || *scope.GroupID < 0 {
		return item, fail(400, "Provide category_id and explicit group_id (0 means No spending group)")
	}
	err := q.QueryRow(`SELECT c.id,s.group_id,c.name,COALESCE(g.name,'No spending group'),COALESCE(p.enabled,1),p.threshold,COALESCE(p.version,0)
 FROM (`+budgetAlertPairsSQL+`) s JOIN categories c ON c.id=s.category_id LEFT JOIN spending_groups g ON g.id=s.group_id
 LEFT JOIN notification_budget_preferences p ON p.user_id=? AND p.category_id=c.id AND p.group_id=s.group_id
 WHERE c.kind='expense' AND s.category_id=? AND s.group_id=?`, u.ID, scope.CategoryID, *scope.GroupID).Scan(&item.CategoryID, &item.GroupID, &item.CategoryName, &item.GroupName, &item.Enabled, &item.Threshold, &item.Version)
	if err == sql.ErrNoRows {
		return item, fail(409, "Budget category or spending group changed; reload before continuing")
	}
	return item, err
}
func (a *App) mcpBudgetAlertReadTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpBudgetAlertScope) (*mcp.CallToolResult, any, error) {
	identity := ctx.Value(mcpKey).(mcpIdentity)
	tx, err := a.DB.Begin()
	if err != nil {
		return mcpFailure(err)
	}
	defer tx.Rollback()
	p, err := readMCPPermissions(tx, identity)
	if err == nil {
		err = authorizeMCPBudgetAlerts(tx, identity, p, false)
	}
	if err != nil {
		return mcpFailure(err)
	}
	item, err := readBudgetAlertPreference(tx, identity.User, in)
	if err != nil {
		return mcpFailure(err)
	}
	item.CategoryName, item.GroupName = mcpRedact(item.CategoryName), mcpRedact(item.GroupName)
	return mcpResult(item), nil, nil
}
func readMCPBudgetAlertSelection(q queryer, u User, inputs []mcpBudgetAlertInput) ([]budgetAlertPreference, error) {
	items := make([]budgetAlertPreference, 0, len(inputs))
	for _, input := range inputs {
		item, err := readBudgetAlertPreference(q, u, input.mcpBudgetAlertScope)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
func prepareMCPBudgetAlerts(tx *sql.Tx, identity mcpIdentity, b mcpChange) ([]budgetAlertPreference, error) {
	p, err := readMCPPermissions(tx, identity)
	if err == nil {
		err = authorizeMCPBudgetAlerts(tx, identity, p, true)
	}
	if err != nil {
		return nil, err
	}
	if len(b.BudgetAlerts) < 1 || len(b.BudgetAlerts) > 100 {
		return nil, fail(400, "Select 1-100 personal budget alert scopes")
	}
	for _, input := range b.BudgetAlerts {
		if input.Version == nil || *input.Version < 0 || (!input.Reset && input.Enabled == nil) || (input.Reset && (input.Enabled != nil || input.Threshold != nil)) {
			return nil, fail(400, "Provide current version and enabled, or reset=true without enabled/threshold")
		}
	}
	return readMCPBudgetAlertSelection(tx, identity.User, b.BudgetAlerts)
}
func sameMCPBudgetAlerts(a, b []budgetAlertPreference) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}
func (a *App) applyMCPBudgetAlerts(tx *sql.Tx, identity mcpIdentity, exact mcpExactChange) error {
	current, err := prepareMCPBudgetAlerts(tx, identity, exact.Change)
	if err != nil {
		return err
	}
	if !sameMCPBudgetAlerts(current, exact.BudgetAlertBefore) {
		return fail(409, "Budget alert settings or scope changed; prepare a new proposal")
	}
	inputs := make([]budgetAlertPreferenceInput, 0, len(exact.Change.BudgetAlerts))
	for _, input := range exact.Change.BudgetAlerts {
		enabled, threshold := input.Enabled, input.Threshold
		if input.Reset {
			value := true
			enabled, threshold = &value, nil
		}
		inputs = append(inputs, budgetAlertPreferenceInput{CategoryID: input.CategoryID, GroupID: *input.GroupID, Version: input.Version, Enabled: enabled, Threshold: threshold})
	}
	if err := saveBudgetAlertPreferencesTx(tx, identity.User, inputs); err != nil {
		return err
	}
	return a.baselineBudgetAlertPreferencesTx(tx, identity.User, inputs)
}
