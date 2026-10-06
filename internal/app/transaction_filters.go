package app

import (
	"net/http"
	"strconv"
	"strings"
)

type transactionFilters struct{ Category, Group, Direction, Query string }

func readTransactionFilters(r *http.Request) (transactionFilters, error) {
	p := r.URL.Query()
	f := transactionFilters{p.Get("category"), p.Get("spending_group"), p.Get("direction"), strings.TrimSpace(p.Get("q"))}
	for _, v := range []struct {
		value   *string
		special string
	}{{&f.Category, "uncategorized"}, {&f.Group, "unassigned"}} {
		if *v.value == "" || *v.value == v.special {
			continue
		}
		id, err := strconv.ParseInt(*v.value, 10, 64)
		if err != nil || id <= 0 {
			return f, fail(400, "Choose a valid category or spending group filter")
		}
		*v.value = strconv.FormatInt(id, 10)
	}
	if f.Direction != "" && f.Direction != "in" && f.Direction != "out" {
		return f, fail(400, "Choose money in or money out")
	}
	return f, nil
}
func (f transactionFilters) active() bool {
	return f.Category != "" || f.Group != "" || f.Direction != "" || f.Query != ""
}
func (f transactionFilters) sql(alias string) (string, []any) {
	clauses := []string{}
	args := []any{}
	if f.Query != "" {
		clauses = append(clauses, "instr(finance_normalize("+alias+".description),finance_normalize(?))>0")
		args = append(args, f.Query)
	}
	if f.Category == "uncategorized" {
		clauses = append(clauses, alias+".is_transfer=0 AND ("+alias+".id IN (SELECT transaction_id FROM allocations WHERE category_id IS NULL) OR "+alias+".id NOT IN (SELECT transaction_id FROM allocations))")
	} else if f.Category != "" {
		clauses = append(clauses, alias+".id IN (SELECT transaction_id FROM allocations WHERE category_id=?)")
		args = append(args, f.Category)
	}
	if f.Group == "unassigned" {
		clauses = append(clauses, alias+".spending_group_id IS NULL")
	} else if f.Group != "" {
		clauses = append(clauses, alias+".spending_group_id=?")
		args = append(args, f.Group)
	}
	if f.Direction == "in" {
		clauses = append(clauses, alias+".amount_cents>0")
	} else if f.Direction == "out" {
		clauses = append(clauses, alias+".amount_cents<0")
	}
	if len(clauses) == 0 {
		return "1=1", args
	}
	return "(" + strings.Join(clauses, ") AND (") + ")", args
}
func (f transactionFilters) matches(row SourceRow) bool {
	if row.Error != "" {
		return false
	}
	if f.Query != "" && !strings.Contains(normalize(row.Description), normalize(f.Query)) {
		return false
	}
	if f.Category == "uncategorized" && row.CategoryID != nil {
		return false
	}
	if f.Category != "" && f.Category != "uncategorized" && (row.CategoryID == nil || strconv.FormatInt(*row.CategoryID, 10) != f.Category) {
		return false
	}
	if f.Group == "unassigned" && row.SpendingGroupID != nil {
		return false
	}
	if f.Group != "" && f.Group != "unassigned" && (row.SpendingGroupID == nil || strconv.FormatInt(*row.SpendingGroupID, 10) != f.Group) {
		return false
	}
	return (f.Direction != "in" || row.Amount > 0) && (f.Direction != "out" || row.Amount < 0)
}

// Ledger-only extensions do not change staged import classification semantics.
type ledgerFilters struct {
	transactionFilters
	DateFrom, DateTo, Categorization, Acceptance string
	Merchant, Tag                                string
	CategoryIDs                                  []int64
	MinAmount, MaxAmount                         *int64
}

func readLedgerFilters(r *http.Request) (ledgerFilters, error) {
	base, err := readTransactionFilters(r)
	f := ledgerFilters{transactionFilters: base}
	if err != nil {
		return f, err
	}
	p := r.URL.Query()
	f.Merchant, f.Tag = p.Get("merchant"), p.Get("tag")
	for _, v := range []string{f.Merchant, f.Tag} {
		if v != "" {
			n, e := strconv.ParseInt(v, 10, 64)
			if e != nil || n <= 0 {
				return f, fail(400, "Choose a merchant or tag")
			}
		}
	}
	f.DateFrom, f.DateTo = p.Get("date_from"), p.Get("date_to")
	for _, value := range []string{f.DateFrom, f.DateTo} {
		if value != "" && !validDate(value) {
			return f, fail(400, "Use calendar dates in YYYY-MM-DD format")
		}
	}
	if f.DateFrom != "" && f.DateTo != "" && f.DateFrom > f.DateTo {
		return f, fail(400, "date_from must not follow date_to")
	}
	f.Categorization, f.Acceptance = p.Get("categorization"), p.Get("acceptance")
	for _, v := range []struct {
		value   string
		allowed []string
	}{
		{f.Categorization, []string{"", "any", "categorized", "uncategorized"}},
		{f.Acceptance, []string{"", "any", "accepted", "needs_category"}},
	} {
		found := false
		for _, allowed := range v.allowed {
			found = found || allowed == v.value
		}
		if !found {
			return f, fail(400, "Choose a valid categorization or acceptance filter")
		}
	}
	if raw := p.Get("category_ids"); raw != "" {
		values := strings.Split(raw, ",")
		if len(values) > 100 {
			return f, fail(400, "Select at most 100 categories")
		}
		seen := map[int64]bool{}
		for _, value := range values {
			id, err := strconv.ParseInt(value, 10, 64)
			if err != nil || id <= 0 || seen[id] {
				return f, fail(400, "Category IDs must be distinct positive integers")
			}
			seen[id] = true
			f.CategoryIDs = append(f.CategoryIDs, id)
		}
	}
	if f.Category != "" && len(f.CategoryIDs) > 0 {
		return f, fail(400, "Use category or category_ids, not both")
	}
	if (f.Category == "uncategorized" && f.Categorization == "categorized") || (p.Get("pending") == "1" && f.Acceptance == "accepted") || (f.Categorization == "categorized" && f.Acceptance == "needs_category") || (f.Categorization == "uncategorized" && f.Acceptance == "accepted") {
		return f, fail(400, "Contradictory transaction filters")
	}
	for _, bound := range []struct {
		key    string
		target **int64
	}{{"min_amount_cents", &f.MinAmount}, {"max_amount_cents", &f.MaxAmount}} {
		if raw := p.Get(bound.key); raw != "" {
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return f, fail(400, "Amount bounds must be signed integer cents")
			}
			*bound.target = &n
		}
	}
	if f.MinAmount != nil && f.MaxAmount != nil && *f.MinAmount > *f.MaxAmount {
		return f, fail(400, "Minimum amount must not exceed maximum amount")
	}
	if (f.Direction == "in" && f.MaxAmount != nil && *f.MaxAmount <= 0) || (f.Direction == "out" && f.MinAmount != nil && *f.MinAmount >= 0) {
		return f, fail(400, "Amount bounds contradict money direction")
	}
	return f, nil
}
func (f ledgerFilters) sql(alias string) (string, []any) {
	base, args := f.transactionFilters.sql(alias)
	clauses := []string{base}
	if f.Merchant != "" {
		clauses = append(clauses, alias+".merchant_id=?")
		args = append(args, f.Merchant)
	}
	if f.Tag != "" {
		clauses = append(clauses, alias+".id IN (SELECT transaction_id FROM transaction_tags WHERE tag_id=?)")
		args = append(args, f.Tag)
	}
	if f.DateFrom != "" {
		clauses = append(clauses, alias+".date>=?")
		args = append(args, f.DateFrom)
	}
	if f.DateTo != "" {
		clauses = append(clauses, alias+".date<=?")
		args = append(args, f.DateTo)
	}
	if len(f.CategoryIDs) > 0 {
		placeholders := make([]string, len(f.CategoryIDs))
		for i, id := range f.CategoryIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		clauses = append(clauses, alias+".id IN (SELECT transaction_id FROM allocations WHERE category_id IN ("+strings.Join(placeholders, ",")+"))")
	}
	missing := alias + ".is_transfer=0 AND (" + alias + ".id IN (SELECT transaction_id FROM allocations WHERE category_id IS NULL) OR " + alias + ".id NOT IN (SELECT transaction_id FROM allocations))"
	if f.Categorization == "uncategorized" {
		clauses = append(clauses, missing)
	}
	if f.Categorization == "categorized" {
		clauses = append(clauses, "NOT ("+missing+")")
	}
	if f.Acceptance == "accepted" {
		clauses = append(clauses, alias+".review_state='approved'")
	}
	if f.Acceptance == "needs_category" {
		clauses = append(clauses, alias+".review_state='pending_review'")
	}
	if f.MinAmount != nil {
		clauses = append(clauses, alias+".amount_cents>=?")
		args = append(args, *f.MinAmount)
	}
	if f.MaxAmount != nil {
		clauses = append(clauses, alias+".amount_cents<=?")
		args = append(args, *f.MaxAmount)
	}
	return "(" + strings.Join(clauses, ") AND (") + ")", args
}
