package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

type mcpMerchantInput struct {
	Account         int64   `json:"account_id"`
	Name            string  `json:"name"`
	Version         int64   `json:"version,omitempty"`
	Logo            *string `json:"logo_data,omitempty"`
	CategoryID      *int64  `json:"category_id,omitempty"`
	SpendingGroupID *int64  `json:"spending_group_id,omitempty"`
}
type mcpMerchantAssignment struct {
	ID       int64  `json:"id"`
	Version  int64  `json:"version"`
	Merchant *int64 `json:"merchant_id"`
}
type mcpMerchantRead struct {
	Filters     map[string]string `json:"filters,omitempty"`
	IncludeLogo bool              `json:"include_logo,omitempty"`
}

func mcpMerchantScope(q queryer, a *App, u User, p mcpPermissions, account int64) bool {
	if account == 0 {
		return u.Member && len(p.Constraints.AccountIDs) == 0
	}
	return p.allowsAccount(account) && ruleAccess(q, a, u, account)
}
func (a *App) authorizeMCPMerchant(q queryer, i mcpIdentity, p mcpPermissions, e mcpExactChange) error {
	b := e.Change
	c := p.Capabilities
	denied := fail(403, "Enable the corresponding merchant permission in Settings → MCP")
	if !p.ReadMerchants {
		return denied
	}
	switch b.Operation {
	case "save_merchant":
		if !c.ManageMerchants || b.Merchant == nil || !mcpMerchantScope(q, a, i.User, p, b.Merchant.Account) {
			return denied
		}
		if b.ID > 0 {
			var scope sql.NullInt64
			if q.QueryRow("SELECT account_id FROM merchants WHERE id=?", b.ID).Scan(&scope) != nil {
				return fail(404, "Merchant not found")
			}
			if scope.Int64 != b.Merchant.Account || !mcpMerchantScope(q, a, i.User, p, scope.Int64) {
				return denied
			}
		}
	case "save_merchant_rule":
		if !c.ManageMerchantRules || b.MerchantRule == nil || !mcpMerchantScope(q, a, i.User, p, b.MerchantRule.Account) {
			return denied
		}
		if b.ID > 0 {
			var scope sql.NullInt64
			if q.QueryRow("SELECT account_id FROM merchant_rules WHERE id=?", b.ID).Scan(&scope) != nil {
				return fail(404, "Merchant rule not found")
			}
			if !mcpMerchantScope(q, a, i.User, p, scope.Int64) {
				return denied
			}
		}
		if b.MerchantRule.Logo != nil {
			if !c.ManageMerchants {
				return denied
			}
			var scope sql.NullInt64
			if q.QueryRow("SELECT account_id FROM merchants WHERE id=?", b.MerchantRule.Merchant).Scan(&scope) != nil || !mcpMerchantScope(q, a, i.User, p, scope.Int64) {
				return denied
			}
		}
	case "delete_merchant_rule":
		if !c.DeleteMerchantRule {
			return denied
		}
		var scope sql.NullInt64
		if q.QueryRow("SELECT account_id FROM merchant_rules WHERE id=?", b.ID).Scan(&scope) != nil {
			return fail(404, "Merchant rule not found")
		}
		if !mcpMerchantScope(q, a, i.User, p, scope.Int64) {
			return denied
		}
	case "assign_merchants":
		if !c.AssignMerchants || len(e.Transactions) < 1 || len(e.Transactions) > 100 {
			return denied
		}
		for _, edit := range e.Transactions {
			account := queryInt(q, "SELECT account_id FROM transactions WHERE id=?", edit.ID)
			if !p.allowsAccount(account) || !ruleAccess(q, a, i.User, account) {
				return denied
			}
		}
	}
	return nil
}
func (a *App) saveMCPMerchant(tx *sql.Tx, u User, id int64, b *mcpMerchantInput) (int64, error) {
	if b == nil {
		return 0, fail(400, "Provide merchant")
	}
	name := strings.TrimSpace(b.Name)
	if utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 100 {
		return 0, fail(400, "Use a merchant name of 2–100 characters")
	}
	if id == 0 {
		if queryInt(tx, "SELECT COUNT(*) FROM merchants WHERE COALESCE(account_id,0)=? AND finance_normalize(name)=finance_normalize(?)", b.Account, name) > 0 {
			return 0, fail(409, "Merchant already exists; use its ID and version")
		}
		var err error
		id, err = labelID(tx, u, a, b.Account, "merchant", name, true)
		if err != nil {
			return 0, err
		}
		if b.CategoryID != nil || b.SpendingGroupID != nil {
			var catID, groupID any
			if b.CategoryID != nil {
				catID = *b.CategoryID
			}
			if b.SpendingGroupID != nil {
				groupID = *b.SpendingGroupID
			}
			if _, err = tx.Exec("UPDATE merchants SET category_id=?, spending_group_id=? WHERE id=?", catID, groupID, id); err != nil {
				return 0, err
			}
		}
		if b.Logo != nil {
			if err = updateMerchantLogo(tx, a, u, id, 1, *b.Logo); err != nil {
				return 0, err
			}
		}
		return id, nil
	}
	if queryInt(tx, "SELECT COUNT(*) FROM merchants WHERE id!=? AND COALESCE(account_id,0)=? AND finance_normalize(name)=finance_normalize(?)", id, b.Account, name) > 0 {
		return 0, fail(409, "Merchant name already exists")
	}
	if b.Logo != nil {
		if err := updateMerchantLogo(tx, a, u, id, b.Version, *b.Logo); err != nil {
			return 0, err
		}
		bversion := b.Version + 1
		res, err := tx.Exec("UPDATE merchants SET name=?,category_id=COALESCE(?,category_id),spending_group_id=COALESCE(?,spending_group_id) WHERE id=? AND version=?", name, b.CategoryID, b.SpendingGroupID, id, bversion)
		if err != nil {
			return 0, err
		}
		if err = affected(res); err != nil {
			return 0, err
		}
	} else {
		res, err := tx.Exec("UPDATE merchants SET name=?,category_id=COALESCE(?,category_id),spending_group_id=COALESCE(?,spending_group_id),version=version+1 WHERE id=? AND version=?", name, b.CategoryID, b.SpendingGroupID, id, b.Version)
		if err != nil {
			return 0, err
		}
		if err = affected(res); err != nil {
			return 0, err
		}
	}
	return id, audit(tx, u, nullableAccount(b.Account), "merchant", id, "updated", map[string]any{"name": name, "logo_changed": b.Logo != nil})
}
func (a *App) prepareMCPMerchantAssignments(tx *sql.Tx, i mcpIdentity, b mcpChange) ([]mcpExactEdit, error) {
	if len(b.MerchantItems) < 1 || len(b.MerchantItems) > 100 {
		return nil, fail(400, "Select 1–100 merchant assignments")
	}
	out := []mcpExactEdit{}
	seen := map[int64]bool{}
	for _, item := range b.MerchantItems {
		if item.ID <= 0 || seen[item.ID] {
			return nil, fail(400, "Choose distinct transaction IDs")
		}
		seen[item.ID] = true
		before, err := a.mcpTransaction(tx, i.User, item.ID)
		if err != nil {
			return nil, err
		}
		if before.Version != item.Version {
			return nil, fail(409, "Transaction changed; reload it")
		}
		var old sql.NullInt64
		if err = tx.QueryRow("SELECT merchant_id FROM transactions WHERE id=?", item.ID).Scan(&old); err != nil {
			return nil, err
		}
		if old.Valid {
			v := old.Int64
			before.MerchantID = &v
		}
		after := before
		after.MerchantID = item.Merchant
		after.ClearMerchant = item.Merchant == nil
		out = append(out, mcpExactEdit{ID: item.ID, Before: before, After: after})
	}
	return out, nil
}
func (a *App) mcpMerchantReadTool(h handler, path string) func(context.Context, *mcp.CallToolRequest, mcpMerchantRead) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in mcpMerchantRead) (*mcp.CallToolResult, any, error) {
		i := ctx.Value(mcpKey).(mcpIdentity)
		p, err := readMCPPermissions(a.DB, i)
		if err != nil {
			return mcpFailure(err)
		}
		if !p.ReadMerchants {
			return mcpFailure(fail(403, "Enable Read merchants and logos in Settings → MCP"))
		}
		values := url.Values{"page": {"0"}, "page_size": {"20"}}
		if path == "/labels" {
			values.Set("kind", "merchant")
		}
		for k, v := range in.Filters {
			switch k {
			case "page", "page_size", "q", "id", "account", "scope", "list_version":
				values.Set(k, v)
			default:
				return mcpFailure(fail(400, "Unknown merchant filter"))
			}
		}
		r, _ := http.NewRequestWithContext(context.WithValue(ctx, authKey, authContext{User: i.User}), "GET", "http://internal/api"+path+"?"+values.Encode(), nil)
		w := httptest.NewRecorder()
		if err = h(w, r); err != nil {
			return mcpFailure(err)
		}
		var raw map[string]any
		if err = json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
			return mcpFailure(err)
		}
		safe := mcpSafe(raw).(map[string]any)
		items := raw["items"].([]any)
		out := safe["items"].([]any)
		for n, v := range items {
			source := v.(map[string]any)
			row := out[n].(map[string]any)
			for _, key := range []string{"merchant_id", "logo_version", "merchant_version", "category_id", "category_name", "spending_group_id", "spending_group_name", "spending_group_color"} {
				if value, ok := source[key]; ok {
					row[key] = value
				}
			}
			if name, ok := source["merchant_name"].(string); ok {
				row["merchant_name"] = mcpRedact(name)
			}
			if in.IncludeLogo {
				for _, key := range []string{"logo_data", "merchant_logo"} {
					if value, ok := source[key]; ok {
						row[key] = value
					}
				}
			} else {
				row["has_logo"] = source["logo_data"] != "" && source["logo_data"] != nil || source["merchant_logo"] != "" && source["merchant_logo"] != nil
			}
		}
		return mcpResult(safe), nil, nil
	}
}

type mcpMerchantPreviewInput struct {
	ID   int64 `json:"id"`
	Page int   `json:"page,omitempty"`
}

func (a *App) mcpMerchantPreviewTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpMerchantPreviewInput) (*mcp.CallToolResult, any, error) {
	i := ctx.Value(mcpKey).(mcpIdentity)
	p, err := readMCPPermissions(a.DB, i)
	if err != nil {
		return mcpFailure(err)
	}
	if !p.ReadMerchants {
		return mcpFailure(fail(403, "Merchant read permission required"))
	}
	if in.ID < 1 || in.Page < 0 || in.Page > 1000000 {
		return mcpFailure(fail(400, "Choose a valid rule and page"))
	}
	r, _ := http.NewRequestWithContext(context.WithValue(ctx, authKey, authContext{User: i.User}), "POST", "http://internal/api/merchant-rules/preview?page="+strconv.Itoa(in.Page), nil)
	r.SetPathValue("id", strconv.FormatInt(in.ID, 10))
	w := httptest.NewRecorder()
	if err = a.merchantRulePreview(w, r); err != nil {
		return mcpFailure(err)
	}
	var raw any
	if err = json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		return mcpFailure(err)
	}
	safe := mcpSafe(raw).(map[string]any)
	safe["rule_version"] = raw.(map[string]any)["rule_version"]
	return mcpResult(safe), nil, nil
}
