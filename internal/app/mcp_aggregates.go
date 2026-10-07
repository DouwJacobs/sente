package app

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpFinancialInput struct {
	AccountID  int64  `json:"account_id,omitempty" jsonschema:"Optional authorized enabled account. With a period and no account, only household accounts are included."`
	PeriodID   int64  `json:"period_id,omitempty" jsonschema:"Saved budget period; household assignment semantics, or date range for an explicitly selected private account."`
	DateFrom   string `json:"date_from,omitempty" jsonschema:"Inclusive YYYY-MM-DD; can narrow a saved period."`
	DateTo     string `json:"date_to,omitempty" jsonschema:"Inclusive YYYY-MM-DD"`
	CategoryID int64  `json:"category_id,omitempty" jsonschema:"Only matching allocations contribute to spending/income; cash movement counts each matching parent once."`
	MerchantID int64  `json:"merchant_id,omitempty" jsonschema:"Requires merchant read consent."`
	Query      string `json:"q,omitempty" jsonschema:"Normalized description search"`
	GroupBy    string `json:"group_by,omitempty" jsonschema:"none (default), category, merchant, account or month"`
	Page       int    `json:"page,omitempty" jsonschema:"Zero-based group page"`
	PageSize   *int   `json:"page_size,omitempty" jsonschema:"1-100; default 50"`
}
type mcpFinancialTotals struct {
	TransactionCount int64 `json:"transaction_count"`
	Income           int64 `json:"income_cents"`
	Spent            int64 `json:"spent_cents"`
	Net              int64 `json:"net_cents"`
	CashIn           int64 `json:"cash_in_cents"`
	CashOut          int64 `json:"cash_out_cents"`
	CashNet          int64 `json:"cash_net_cents"`
}
type mcpFinancialGroup struct {
	ID               any    `json:"id"`
	Name             string `json:"name"`
	TransactionCount int64  `json:"transaction_count"`
	Income           int64  `json:"income_cents"`
	Spent            int64  `json:"spent_cents"`
	Net              int64  `json:"net_cents"`
}
type mcpFinancialResult struct {
	Currency string              `json:"currency"`
	Totals   mcpFinancialTotals  `json:"totals"`
	GroupBy  string              `json:"group_by"`
	Items    []mcpFinancialGroup `json:"items"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
	More     bool                `json:"more"`
}

func (a *App) mcpFinancialSummary(ctx context.Context, q queryer, identity mcpIdentity, in mcpFinancialInput) (mcpFinancialResult, error) {
	out := mcpFinancialResult{Currency: "ZAR", Items: []mcpFinancialGroup{}, GroupBy: in.GroupBy, Page: in.Page, PageSize: 50}
	if out.GroupBy == "" {
		out.GroupBy = "none"
	}
	if in.PageSize != nil {
		out.PageSize = *in.PageSize
	}
	if in.AccountID < 0 || in.PeriodID < 0 || in.CategoryID < 0 || in.MerchantID < 0 || in.Page < 0 || in.Page > 1000000 || out.PageSize < 1 || out.PageSize > 100 {
		return out, fail(400, "Choose positive IDs, a valid page and a page size from 1 to 100")
	}
	dimensions := map[string]string{"category": "l.category_id", "merchant": "t.merchant_id", "account": "t.account_id", "month": "substr(t.date,1,7)", "none": "NULL"}
	dimension, ok := dimensions[out.GroupBy]
	if !ok {
		return out, fail(400, "Choose none, category, merchant, account or month")
	}
	permissions, err := readMCPPermissions(q, identity)
	if err != nil {
		return out, err
	}
	if (in.MerchantID > 0 || out.GroupBy == "merchant") && !permissions.ReadMerchants {
		return out, fail(403, "Merchant read permission required")
	}
	values := url.Values{}
	for key, id := range map[string]int64{"account": in.AccountID, "period": in.PeriodID, "category": in.CategoryID, "merchant": in.MerchantID} {
		if id > 0 {
			values.Set(key, strconv.FormatInt(id, 10))
		}
	}
	values.Set("date_from", in.DateFrom)
	values.Set("date_to", in.DateTo)
	values.Set("q", in.Query)
	request, _ := http.NewRequestWithContext(ctx, "GET", "http://internal/transactions?"+values.Encode(), nil)
	scopedUser := identity.User
	scopedUser.MCPAccounts = permissions.Constraints.AccountIDs
	query, args, err := a.transactionQuery(q, request, scopedUser)
	if err != nil {
		return out, err
	}
	scope := query[strings.Index(query, " FROM transactions"):]
	if in.PeriodID > 0 && in.AccountID == 0 {
		scope += " AND a.household=1"
	}
	// Select compact authorized fields only, without notes or source/identity data.
	matched := "WITH matched AS (SELECT t.id,t.date,t.account_id,t.merchant_id,t.amount_cents,t.is_transfer" + scope + ") "
	allocationFilter := ""
	allocationArgs := append([]any{}, args...)
	if in.CategoryID > 0 {
		allocationFilter = " WHERE l.category_id=?"
		allocationArgs = append(allocationArgs, in.CategoryID)
	}
	allocated := matched + "SELECT COUNT(DISTINCT t.id) transaction_count,COALESCE(SUM(CASE WHEN t.is_transfer=0 AND (c.kind='income' OR c.id IS NULL AND COALESCE(l.amount_cents,t.amount_cents)>=0) THEN COALESCE(l.amount_cents,t.amount_cents) ELSE 0 END),0) income_cents,COALESCE(SUM(CASE WHEN t.is_transfer=0 AND (c.kind='expense' OR c.id IS NULL AND COALESCE(l.amount_cents,t.amount_cents)<0) THEN -COALESCE(l.amount_cents,t.amount_cents) ELSE 0 END),0) spent_cents FROM matched t LEFT JOIN allocations l ON l.transaction_id=t.id LEFT JOIN categories c ON c.id=l.category_id" + allocationFilter
	totals, err := data(q, allocated, allocationArgs...)
	if err != nil {
		return out, err
	}
	cash, err := data(q, matched+"SELECT COUNT(*) transaction_count,COALESCE(SUM(CASE WHEN amount_cents>0 THEN amount_cents ELSE 0 END),0) cash_in_cents,COALESCE(SUM(CASE WHEN amount_cents<0 THEN -amount_cents ELSE 0 END),0) cash_out_cents FROM matched", args...)
	if err != nil {
		return out, err
	}
	out.Totals = mcpFinancialTotals{TransactionCount: num(cash[0]["transaction_count"]), Income: num(totals[0]["income_cents"]), Spent: num(totals[0]["spent_cents"]), CashIn: num(cash[0]["cash_in_cents"]), CashOut: num(cash[0]["cash_out_cents"])}
	out.Totals.Net, err = mcpAggregateNet(out.Totals.Income, out.Totals.Spent)
	if err != nil {
		return out, err
	}
	out.Totals.CashNet = out.Totals.CashIn - out.Totals.CashOut
	if out.GroupBy == "none" {
		return out, nil
	}
	name := "COALESCE(c.name,'Uncategorized')"
	join := ""
	switch out.GroupBy {
	case "merchant":
		name = "COALESCE(m.name,'No merchant')"
		join = " LEFT JOIN merchants m ON m.id=t.merchant_id"
	case "account":
		name = "'Account '||t.account_id"
	case "month":
		name = "substr(t.date,1,7)"
	}
	grouped := strings.Replace(allocated, "SELECT COUNT(DISTINCT t.id)", "SELECT "+dimension+" id,"+name+" name,COUNT(DISTINCT t.id)", 1)
	if join != "" {
		grouped = strings.Replace(grouped, " LEFT JOIN allocations l", join+" LEFT JOIN allocations l", 1)
	}
	grouped += " GROUP BY " + dimension
	count, err := data(q, "SELECT COUNT(*) total FROM ("+grouped+")", allocationArgs...)
	if err != nil {
		return out, err
	}
	out.Total = int(num(count[0]["total"]))
	rows, err := data(q, grouped+" ORDER BY spent_cents DESC,income_cents DESC,id LIMIT ? OFFSET ?", append(append([]any{}, allocationArgs...), out.PageSize, in.Page*out.PageSize)...)
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		income, spent := num(row["income_cents"]), num(row["spent_cents"])
		net, err := mcpAggregateNet(income, spent)
		if err != nil {
			return out, err
		}
		label := row["name"].(string)
		if out.GroupBy != "month" {
			label = mcpRedact(label)
		}
		out.Items = append(out.Items, mcpFinancialGroup{ID: row["id"], Name: label, TransactionCount: num(row["transaction_count"]), Income: income, Spent: spent, Net: net})
	}
	out.More = (out.Page+1)*out.PageSize < out.Total
	return out, nil
}
func (a *App) mcpFinancialSummaryTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpFinancialInput) (*mcp.CallToolResult, any, error) {
	tx, err := a.DB.Begin()
	if err != nil {
		return mcpFailure(err)
	}
	defer tx.Rollback()
	result, err := a.mcpFinancialSummary(ctx, tx, ctx.Value(mcpKey).(mcpIdentity), in)
	if err != nil {
		return mcpFailure(err)
	}
	return mcpResult(result), nil, nil
}

type mcpPeriodComparisonInput struct {
	AccountID  int64   `json:"account_id,omitempty"`
	CategoryID int64   `json:"category_id,omitempty"`
	MerchantID int64   `json:"merchant_id,omitempty"`
	Query      string  `json:"q,omitempty"`
	PeriodIDs  []int64 `json:"period_ids" jsonschema:"2-12 distinct saved periods, returned in requested order. No account means household budget scope; explicit private accounts use period dates."`
}

func (a *App) mcpPeriodComparisonTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpPeriodComparisonInput) (*mcp.CallToolResult, any, error) {
	if len(in.PeriodIDs) < 2 || len(in.PeriodIDs) > 12 {
		return mcpFailure(fail(400, "Choose 2-12 distinct saved periods"))
	}
	if err := distinctPositiveIDs(in.PeriodIDs, "periods"); err != nil {
		return mcpFailure(err)
	}
	identity := ctx.Value(mcpKey).(mcpIdentity)
	tx, err := a.DB.Begin()
	if err != nil {
		return mcpFailure(err)
	}
	defer tx.Rollback()
	items := []map[string]any{}
	for _, id := range in.PeriodIDs {
		result, err := a.mcpFinancialSummary(ctx, tx, identity, mcpFinancialInput{AccountID: in.AccountID, PeriodID: id, CategoryID: in.CategoryID, MerchantID: in.MerchantID, Query: in.Query})
		if err != nil {
			return mcpFailure(err)
		}
		periods, err := data(tx, "SELECT name,start_date,end_date FROM periods WHERE id=?", id)
		if err != nil {
			return mcpFailure(err)
		}
		items = append(items, map[string]any{"period_id": id, "period_name": mcpRedact(periods[0]["name"].(string)), "start_date": periods[0]["start_date"], "end_date": periods[0]["end_date"], "totals": result.Totals})
	}
	return mcpResult(map[string]any{"currency": "ZAR", "items": items}), nil, nil
}

// SQLite SUM rejects integer overflow. Keep the derived net exact as well,
// including a large income total combined with negative spending from refunds.
func mcpAggregateNet(income, spent int64) (int64, error) {
	if spent > 0 && income < math.MinInt64+spent || spent < 0 && income > math.MaxInt64+spent {
		return 0, fail(400, "Aggregate total is too large; narrow the account or date range")
	}
	return income - spent, nil
}
