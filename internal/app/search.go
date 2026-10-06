package app

import (
	"net/http"
	"strings"
)

type globalSearchResult struct {
	Categories     []map[string]any `json:"categories"`
	SpendingGroups []map[string]any `json:"spending_groups"`
	Transactions   []map[string]any `json:"transactions"`
	MerchantRules  []map[string]any `json:"merchant_rules"`
	Rules          []map[string]any `json:"rules"`
}

func (a *App) globalSearch(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	res := globalSearchResult{
		Categories:     []map[string]any{},
		SpendingGroups: []map[string]any{},
		Transactions:   []map[string]any{},
		MerchantRules:  []map[string]any{},
		Rules:          []map[string]any{},
	}
	if len(q) < 2 {
		send(w, res)
		return nil
	}
	if len(q) > 100 {
		q = q[:100]
	}

	pattern := "%" + q + "%"

	// 1. Categories
	cats, err := data(a.DB, `
		SELECT c.id, c.name, c.kind, c.archived, c.version,
		       s.id spending_group_id, s.name spending_group_name, s.color spending_group_color
		FROM categories c
		LEFT JOIN spending_groups s ON s.id=c.spending_group_id
		WHERE finance_normalize(c.name) LIKE finance_normalize(?)
		ORDER BY c.kind, c.name
		LIMIT 10
	`, pattern)
	if err == nil && cats != nil {
		res.Categories = cats
	}

	// 2. Spending Groups
	groups, err := data(a.DB, `
		SELECT s.id, s.name, s.color, s.version
		FROM spending_groups s
		WHERE finance_normalize(s.name) LIKE finance_normalize(?)
		ORDER BY s.name
		LIMIT 10
	`, pattern)
	if err == nil && groups != nil {
		res.SpendingGroups = groups
	}

	// 3. Transactions
	cleanAmount := strings.TrimLeft(strings.TrimSpace(q), "R$€£+- ")
	amountCents, amountErr := Cents(cleanAmount)
	txQuery := `
		SELECT t.id, t.account_id, t.date, t.description, t.amount_cents, t.version,
		       COALESCE(c.name, '') category_name, COALESCE(c.kind, '') category_kind,
		       COALESCE(m.name, '') merchant_name, COALESCE(m.logo_data, '') merchant_logo,
		       COALESCE(s.name, '') spending_group_name, COALESCE(s.color, '') spending_group_color,
		       a.name account_name
		FROM transactions t
		JOIN accounts a ON a.id=t.account_id
		LEFT JOIN allocations l ON l.transaction_id=t.id
		LEFT JOIN categories c ON c.id=l.category_id
		LEFT JOIN merchants m ON m.id=t.merchant_id
		LEFT JOIN spending_groups s ON s.id=COALESCE(t.spending_group_id, c.spending_group_id)
		WHERE ` + accountAccessSQL(u) + ` AND (
		  finance_normalize(t.description) LIKE finance_normalize(?)
		  OR (m.name IS NOT NULL AND finance_normalize(m.name) LIKE finance_normalize(?))
		  OR (c.name IS NOT NULL AND finance_normalize(c.name) LIKE finance_normalize(?))
	`
	txArgs := []any{u.Member, u.ID, pattern, pattern, pattern}
	if amountErr == nil && amountCents > 0 {
		txQuery += ` OR abs(t.amount_cents) = ? `
		txArgs = append(txArgs, amountCents)
	}
	txQuery += `) GROUP BY t.id ORDER BY t.date DESC, t.id DESC LIMIT 15`

	txs, err := data(a.DB, txQuery, txArgs...)
	if err == nil && txs != nil {
		res.Transactions = txs
	}

	// 4. Merchant Rules
	mRules, err := data(a.DB, `
		SELECT mr.id, mr.account_id, mr.pattern, mr.direction, mr.priority, mr.enabled, mr.version,
		       m.id merchant_id, m.name merchant_name, m.logo_data merchant_logo, m.version merchant_version,
		       m.category_id, c.name category_name,
		       m.spending_group_id, s.name spending_group_name, s.color spending_group_color,
		       CASE WHEN mr.account_id IS NULL THEN 'All accounts' ELSE a.name END account_name
		FROM merchant_rules mr
		JOIN merchants m ON m.id=mr.merchant_id
		LEFT JOIN categories c ON c.id=m.category_id
		LEFT JOIN spending_groups s ON s.id=m.spending_group_id
		LEFT JOIN accounts a ON a.id=mr.account_id
		WHERE (mr.account_id IS NULL OR `+accountAccessSQL(u)+`)
		  AND (
		    finance_normalize(mr.pattern) LIKE finance_normalize(?)
		    OR finance_normalize(m.name) LIKE finance_normalize(?)
		    OR (c.name IS NOT NULL AND finance_normalize(c.name) LIKE finance_normalize(?))
		  )
		ORDER BY mr.priority DESC, mr.id ASC
		LIMIT 10
	`, u.Member, u.ID, pattern, pattern, pattern)
	if err == nil && mRules != nil {
		res.MerchantRules = mRules
	}

	// 5. Normal Rules (custom + builtin)
	rules, err := data(a.DB, `
		WITH visible AS (
		  SELECT r.id, r.account_id, r.pattern, r.category_id, r.priority, r.spending_group_id, r.direction, r.enabled, r.version,
		         0 builtin, c.name category_name, a.name account_name, s.name spending_group_name
		  FROM rules r
		  JOIN accounts a ON a.id=r.account_id
		  JOIN categories c ON c.id=r.category_id
		  LEFT JOIN spending_groups s ON s.id=r.spending_group_id
		  WHERE `+accountAccessSQL(u)+`
		  UNION ALL
		  SELECT -b.id, 0, b.pattern, b.category_id, b.priority, b.spending_group_id, b.direction, b.enabled, b.version,
		         1, c.name, 'All enabled accounts', s.name
		  FROM builtin_rules b
		  JOIN categories c ON c.id=b.category_id
		  LEFT JOIN spending_groups s ON s.id=b.spending_group_id
		)
		SELECT id, account_id, pattern, category_id, priority, spending_group_id, direction, enabled, version,
		       builtin, category_name, account_name, spending_group_name
		FROM visible
		WHERE finance_normalize(pattern) LIKE finance_normalize(?)
		   OR finance_normalize(category_name) LIKE finance_normalize(?)
		   OR (spending_group_name IS NOT NULL AND finance_normalize(spending_group_name) LIKE finance_normalize(?))
		ORDER BY builtin ASC, priority DESC, id ASC
		LIMIT 10
	`, u.Member, u.ID, pattern, pattern, pattern)
	if err == nil && rules != nil {
		res.Rules = rules
	}

	send(w, res)
	return nil
}
