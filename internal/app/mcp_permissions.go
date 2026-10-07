package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

type mcpCapabilities struct {
	ManageMerchants     bool `json:"manage_merchants"`
	ManageMerchantRules bool `json:"manage_merchant_rules"`
	DeleteMerchantRule  bool `json:"delete_merchant_rule"`
	AssignMerchants     bool `json:"assign_merchants"`
	AssignMissing       bool `json:"assign_missing"`
	Recategorize        bool `json:"recategorize"`
	FinancialEdit       bool `json:"financial_edit"`
	CreateCategory      bool `json:"create_category"`
	CreateRule          bool `json:"create_rule"`
	UpdateRule          bool `json:"update_rule"`
	DeleteRule          bool `json:"delete_rule"`
	UpdateBudget        bool `json:"update_budget"`
	ChangeSeen          bool `json:"change_seen"`
}
type mcpConstraints struct {
	SelectedAccounts bool    `json:"selected_accounts"`
	AccountIDs       []int64 `json:"account_ids,omitempty"`
	MissingOnly      bool    `json:"missing_categories_only"`
	ConstrainedRules bool    `json:"constrained_rules"`
}
type mcpPermissions struct {
	ReadContext   bool            `json:"read_context"`
	ReadMerchants bool            `json:"read_merchants"`
	SchemaVersion int             `json:"schema_version"`
	Preset        string          `json:"preset"`
	Automatic     mcpCapabilities `json:"automatic_approval"`
	Capabilities  mcpCapabilities `json:"capabilities"`
	Constraints   mcpConstraints  `json:"constraints"`
}

func legacyMCPPermissions(write bool) mcpPermissions {
	p := mcpPermissions{SchemaVersion: 1, Preset: "review"}
	if write {
		p.Preset = "legacy_finance"
		p.Capabilities = mcpCapabilities{AssignMissing: true, Recategorize: true, FinancialEdit: true, CreateCategory: true, CreateRule: true, UpdateRule: true, DeleteRule: true, UpdateBudget: true}
	}
	return p
}
func (p mcpPermissions) writes() bool { return p.Capabilities != (mcpCapabilities{}) }
func (p mcpPermissions) allowsAccount(account int64) bool {
	if len(p.Constraints.AccountIDs) == 0 {
		return true
	}
	for _, id := range p.Constraints.AccountIDs {
		if id == account {
			return true
		}
	}
	return false
}
func readMCPPermissions(q queryer, identity mcpIdentity) (mcpPermissions, error) {
	var raw string
	var write bool
	if err := q.QueryRow("SELECT permissions,can_write FROM mcp_tokens WHERE id=? AND user_id=? AND expires_at>?", identity.TokenID, identity.User.ID, time.Now().Unix()).Scan(&raw, &write); err != nil {
		return mcpPermissions{}, fail(403, "MCP connection is unavailable")
	}
	if raw == "{}" {
		return legacyMCPPermissions(write), nil
	}
	var p mcpPermissions
	if json.Unmarshal([]byte(raw), &p) != nil || p.SchemaVersion != 1 || (p.Constraints.SelectedAccounts && len(p.Constraints.AccountIDs) == 0) {
		return p, fail(403, "MCP connection permissions are unavailable")
	}
	return p, nil
}
func (a *App) authorizeMCPExact(q queryer, identity mcpIdentity, exact mcpExactChange) error {
	p, err := readMCPPermissions(q, identity)
	if err != nil {
		return err
	}
	denied := fail(403, "This connection does not permit these changes")
	b := exact.Change
	c := p.Capabilities
	if !p.writes() {
		return denied
	}
	switch b.Operation {
	case "save_merchant", "save_merchant_rule", "assign_merchants", "delete_merchant_rule":
		return a.authorizeMCPMerchant(q, identity, p, exact)
	case "set_seen":
		if !c.ChangeSeen || len(exact.SeenStates) == 0 {
			return denied
		}
		for _, item := range exact.SeenStates {
			account := queryInt(q, "SELECT account_id FROM transactions WHERE id=?", item.ID)
			if !p.allowsAccount(account) || !a.can(q, identity.User, account, false) || queryInt(q, "SELECT COUNT(*) FROM accounts WHERE id=? AND sync_hidden=0", account) != 1 {
				return denied
			}
		}
	case "assign_categories", "edit_transactions":
		if len(exact.Transactions) == 0 {
			return denied
		}
		for _, edit := range exact.Transactions {
			account := queryInt(q, "SELECT account_id FROM transactions WHERE id=?", edit.ID)
			if !p.allowsAccount(account) || !ruleAccess(q, a, identity.User, account) {
				return denied
			}
			before, after := edit.Before, edit.After
			if b.Operation == "assign_categories" && (!c.AssignMissing || before.Transfer || after.Transfer) {
				return denied
			}
			// Separate category grants from financial/description/notes/group/period edits.
			categoryChanged := false
			financial := before.Date != after.Date || before.Amount != after.Amount || before.Description != after.Description || before.Transfer != after.Transfer || before.Assignment != after.Assignment || !reflect.DeepEqual(before.PeriodID, after.PeriodID) || !reflect.DeepEqual(before.SpendingGroupID, after.SpendingGroupID) || len(before.Allocations) != len(after.Allocations)
			for index, allocation := range after.Allocations {
				if index >= len(before.Allocations) {
					if allocation.CategoryID != nil && !c.AssignMissing {
						return denied
					}
					continue
				}
				original := before.Allocations[index]
				financial = financial || original.Amount != allocation.Amount || original.Note != allocation.Note
				if !reflect.DeepEqual(original.CategoryID, allocation.CategoryID) {
					categoryChanged = true
					if original.CategoryID == nil {
						if !c.AssignMissing {
							return denied
						}
					} else if !c.Recategorize || p.Constraints.MissingOnly {
						return denied
					}
				}
			}
			for index := len(after.Allocations); index < len(before.Allocations); index++ {
				if before.Allocations[index].CategoryID != nil && (!c.Recategorize || p.Constraints.MissingOnly) {
					return denied
				}
			}
			if !financial && !categoryChanged && !c.FinancialEdit {
				return denied
			}
			if financial && (!c.FinancialEdit || p.Constraints.MissingOnly) {
				return denied
			}
			if p.Constraints.MissingOnly && (before.Transfer || after.Transfer) {
				return denied
			}
		}
	case "create_category":
		if !c.CreateCategory || !identity.User.Member {
			return denied
		}
	case "save_rule":
		if b.Rule == nil {
			return denied
		}
		if b.ID == 0 && !c.CreateRule || b.ID != 0 && !c.UpdateRule {
			return denied
		}
		if b.ID >= 0 && (!p.allowsAccount(b.Rule.AccountID) || !ruleAccess(q, a, identity.User, b.Rule.AccountID)) {
			return denied
		}
		if b.ID < 0 && (!identity.User.Member || len(p.Constraints.AccountIDs) > 0) {
			return denied
		}
		if b.ID > 0 {
			originalAccount := queryInt(q, "SELECT account_id FROM rules WHERE id=?", b.ID)
			if !p.allowsAccount(originalAccount) || !ruleAccess(q, a, identity.User, originalAccount) {
				return denied
			}
		}
		if p.Constraints.ConstrainedRules {
			if b.ID != 0 || len(p.Constraints.AccountIDs) == 0 || b.Rule.AccountID <= 0 || len(b.Rule.Pattern) < 2 || len(b.Rule.Pattern) > 200 || normalize(b.Rule.Pattern) == "" {
				return denied
			}
		}
	case "delete_rule":
		if !c.DeleteRule {
			return denied
		}
		if b.ID < 0 && (!identity.User.Member || len(p.Constraints.AccountIDs) > 0) {
			return denied
		}
		if b.ID > 0 {
			account := queryInt(q, "SELECT account_id FROM rules WHERE id=?", b.ID)
			if account == 0 {
				account = mcpBeforeRuleAccount(exact.Before)
				if account == 0 {
					account = queryInt(q, "SELECT account_id FROM audit WHERE entity='rule' AND entity_id=? AND action='deleted' AND user_id=? ORDER BY id DESC LIMIT 1", b.ID, identity.User.ID)
				}
			}
			if !p.allowsAccount(account) || !ruleAccess(q, a, identity.User, account) {
				return denied
			}
		}
	case "update_budget":
		if !c.UpdateBudget || !identity.User.Member || len(p.Constraints.AccountIDs) > 0 {
			return denied
		}
	default:
		return denied
	}
	return nil
}

func mcpBeforeRuleAccount(before any) int64 {
	raw, _ := json.Marshal(before)
	var rows []struct {
		AccountID int64 `json:"account_id"`
	}
	if json.Unmarshal(raw, &rows) != nil || len(rows) != 1 {
		return 0
	}
	return rows[0].AccountID
}

// Account scope is an additional bound, never a replacement for current grants.
func mcpAccountAllowed(u User, account int64) bool {
	if len(u.MCPAccounts) == 0 {
		return true
	}
	for _, id := range u.MCPAccounts {
		if id == account {
			return true
		}
	}
	return false
}
func accountScopeSQL(u User) string {
	if len(u.MCPAccounts) == 0 {
		return ""
	}
	ids := make([]string, len(u.MCPAccounts))
	for i, id := range u.MCPAccounts {
		ids[i] = strconv.FormatInt(id, 10)
	}
	return " AND a.id IN (" + strings.Join(ids, ",") + ")"
}
func accountAccessSQL(u User) string { return "(" + accessSQL + accountScopeSQL(u) + ")" }

func (a *App) validateMCPPermissions(q queryer, u User, p mcpPermissions) (mcpPermissions, error) {
	if p.SchemaVersion != 1 {
		return p, fail(400, "Unsupported permission format")
	}
	switch p.Preset {
	case "review":
		p.Automatic = mcpCapabilities{}
		p.Capabilities = mcpCapabilities{}
		p.Constraints.MissingOnly = false
		p.Constraints.ConstrainedRules = false
	case "categorization":
		p.Capabilities = mcpCapabilities{AssignMissing: true, CreateRule: true}
		p.Constraints.MissingOnly = true
		p.Constraints.ConstrainedRules = true
	case "finance":
		p.Capabilities = legacyMCPPermissions(true).Capabilities
		p.Constraints.MissingOnly = false
		p.Constraints.ConstrainedRules = false
	case "custom":
	default:
		return p, fail(400, "Choose a permission preset")
	}
	if p.Constraints.SelectedAccounts && len(p.Constraints.AccountIDs) == 0 {
		return p, fail(400, "Select at least one account")
	}
	p.Constraints.SelectedAccounts = len(p.Constraints.AccountIDs) > 0
	if len(p.Constraints.AccountIDs) > 100 {
		return p, fail(400, "Select at most 100 accounts")
	}
	if len(p.Constraints.AccountIDs) > 0 {
		if err := distinctPositiveIDs(p.Constraints.AccountIDs, "accounts"); err != nil {
			return p, err
		}
		for _, id := range p.Constraints.AccountIDs {
			if !a.can(q, u, id, false) || queryInt(q, "SELECT COUNT(*) FROM accounts WHERE id=? AND sync_hidden=0", id) != 1 {
				return p, fail(403, "Selected account access is unavailable")
			}
		}
		sort.Slice(p.Constraints.AccountIDs, func(i, j int) bool { return p.Constraints.AccountIDs[i] < p.Constraints.AccountIDs[j] })
	}
	if p.Constraints.ConstrainedRules && p.Capabilities.CreateRule && len(p.Constraints.AccountIDs) == 0 {
		return p, fail(400, "Select accounts for constrained rule creation")
	}
	if p.Constraints.ConstrainedRules && (p.Capabilities.UpdateRule || p.Capabilities.DeleteRule) {
		return p, fail(400, "Constrained rules permit creation only")
	}
	if p.Constraints.MissingOnly && (p.Capabilities.FinancialEdit || p.Capabilities.Recategorize) {
		return p, fail(400, "Missing-category-only access cannot include financial edits or recategorization")
	}
	if len(p.Constraints.AccountIDs) > 0 && p.Capabilities.UpdateBudget {
		return p, fail(400, "Shared budget editing requires all accessible accounts")
	}
	if !p.ReadMerchants && (p.Capabilities.ManageMerchants || p.Capabilities.ManageMerchantRules || p.Capabilities.DeleteMerchantRule || p.Capabilities.AssignMerchants) {
		return p, fail(400, "Enable merchant reads for merchant changes")
	}
	if !mcpCapabilitiesContain(p.Capabilities, p.Automatic) {
		return p, fail(400, "Automatic approval requires the corresponding change permission")
	}
	return p, nil
}
func (a *App) mcpPermissionAccounts(q queryer, u User) ([]map[string]any, error) {
	return data(q, "SELECT a.id,a.name,CASE WHEN a.household=1 AND ?=1 THEN 1 ELSE EXISTS(SELECT 1 FROM grants g WHERE g.account_id=a.id AND g.user_id=? AND g.role='editor') END can_edit FROM accounts a WHERE "+accountAccessSQL(u)+" ORDER BY a.name,a.id", u.Member, u.ID, u.Member, u.ID)
}
func (a *App) updateMCPPermissions(w http.ResponseWriter, r *http.Request) error {
	var b struct {
		Version     int64          `json:"version"`
		Permissions mcpPermissions `json:"permissions"`
		Consent     bool           `json:"consent"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if !b.Consent {
		return fail(400, "Confirm the displayed connection permissions")
	}
	err := a.write(func(tx *sql.Tx) error {
		u := Current(r)
		id := parseID(r)
		var write bool
		var raw string
		var version int64
		if err := tx.QueryRow("SELECT can_write,permissions,permission_version FROM mcp_tokens WHERE id=? AND user_id=? AND expires_at>?", id, u.ID, time.Now().Unix()).Scan(&write, &raw, &version); err != nil {
			return fail(404, "Connection unavailable")
		}
		if version != b.Version {
			return fail(409, "Connection permissions changed; reload before saving")
		}
		p, err := a.validateMCPPermissions(tx, u, b.Permissions)
		if err != nil {
			return err
		}
		if p.writes() && !write {
			return fail(403, "This connection was approved for reads only; reconnect from the agent to request proposal access")
		}
		encoded, err := json.Marshal(p)
		if err != nil {
			return err
		}
		res, err := tx.Exec("UPDATE mcp_tokens SET permissions=?,permission_version=permission_version+1 WHERE id=? AND user_id=? AND permission_version=?", string(encoded), id, u.ID, b.Version)
		if err != nil {
			return err
		}
		if err = affected(res); err != nil {
			return err
		}
		var before any
		json.Unmarshal([]byte(raw), &before)
		return audit(tx, u, nil, "mcp", id, "permissions_changed", map[string]any{"connection_id": id, "before": before, "after": p, "version": b.Version + 1})
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}

func mcpCapabilitiesContain(granted, required mcpCapabilities) bool {
	g, r := reflect.ValueOf(granted), reflect.ValueOf(required)
	for i := 0; i < r.NumField(); i++ {
		if r.Field(i).Bool() && !g.Field(i).Bool() {
			return false
		}
	}
	return true
}
func mcpRequiredCapabilities(exact mcpExactChange) mcpCapabilities {
	var needed mcpCapabilities
	b := exact.Change
	switch b.Operation {
	case "save_merchant":
		needed.ManageMerchants = true
	case "save_merchant_rule":
		needed.ManageMerchantRules = true
		if b.MerchantRule != nil && b.MerchantRule.Logo != nil {
			needed.ManageMerchants = true
		}
	case "delete_merchant_rule":
		needed.DeleteMerchantRule = true
	case "assign_merchants":
		needed.AssignMerchants = true
	case "set_seen":
		needed.ChangeSeen = true
	case "assign_categories":
		needed.AssignMissing = true
	case "create_category":
		needed.CreateCategory = true
	case "save_rule":
		if b.ID == 0 {
			needed.CreateRule = true
		} else {
			needed.UpdateRule = true
		}
	case "delete_rule":
		needed.DeleteRule = true
	case "update_budget":
		needed.UpdateBudget = true
	case "edit_transactions":
		for _, edit := range exact.Transactions {
			before, after := edit.Before, edit.After
			financial := before.Date != after.Date || before.Amount != after.Amount || before.Description != after.Description || before.Transfer != after.Transfer || before.Assignment != after.Assignment || !reflect.DeepEqual(before.PeriodID, after.PeriodID) || !reflect.DeepEqual(before.SpendingGroupID, after.SpendingGroupID) || len(before.Allocations) != len(after.Allocations)
			changed := false
			for i, allocation := range after.Allocations {
				if i >= len(before.Allocations) {
					if allocation.CategoryID != nil {
						needed.AssignMissing = true
						changed = true
					}
					continue
				}
				original := before.Allocations[i]
				financial = financial || original.Amount != allocation.Amount || original.Note != allocation.Note
				if !reflect.DeepEqual(original.CategoryID, allocation.CategoryID) {
					changed = true
					if original.CategoryID == nil {
						needed.AssignMissing = true
					} else {
						needed.Recategorize = true
					}
				}
			}
			for i := len(after.Allocations); i < len(before.Allocations); i++ {
				if before.Allocations[i].CategoryID != nil {
					needed.Recategorize = true
				}
			}
			if financial || !changed {
				needed.FinancialEdit = true
			}
		}
	}
	return needed
}
func (a *App) mcpAutomaticApproval(q queryer, identity mcpIdentity, exact mcpExactChange) (bool, error) {
	if err := a.authorizeMCPExact(q, identity, exact); err != nil {
		return false, err
	}
	p, err := readMCPPermissions(q, identity)
	if err != nil {
		return false, err
	}
	needed := mcpRequiredCapabilities(exact)
	return needed != (mcpCapabilities{}) && mcpCapabilitiesContain(p.Automatic, needed), nil
}
