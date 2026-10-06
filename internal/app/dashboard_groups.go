package app

import (
	"net/http"
	"sort"
	"strconv"
)

// Buckets follow each parent's group, while money follows its allocations.
// Each group/category limit contributes to exactly one bucket budget.
func dashboardGroups(r *http.Request, rows []map[string]any, categories map[int64]map[string]any, budgets []map[string]any) ([]map[string]any, int, error) {
	page := func(key string) (int, error) {
		if !r.URL.Query().Has(key) {
			return 0, nil
		}
		v, err := strconv.Atoi(r.URL.Query().Get(key))
		if err != nil || v < 0 || v > 1000000 {
			return 0, fail(400, "Choose a valid spending group page")
		}
		return v, nil
	}
	gp, err := page("group_page")
	if err != nil {
		return nil, 0, err
	}
	cp, err := page("group_category_page")
	if err != nil {
		return nil, 0, err
	}
	selected, _ := strconv.ParseInt(r.URL.Query().Get("group_id"), 10, 64)
	groups := map[int64]map[string]any{}
	children := map[int64]map[int64]map[string]any{}
	ensure := func(row map[string]any) (map[string]any, map[string]any) {
		gid, cid := num(row["spending_group_id"]), num(row["category_id"])
		g := groups[gid]
		if g == nil {
			name, color := row["spending_group_name"], row["spending_group_color"]
			if gid == 0 {
				name, color = "No spending group", "slate"
			}
			g = map[string]any{"id": gid, "name": name, "color": color, "spent_cents": int64(0), "target_cents": int64(0), "pending_cents": int64(0)}
			groups[gid], children[gid] = g, map[int64]map[string]any{}
		}
		c := children[gid][cid]
		if c == nil {
			name := row["name"]
			if cid == 0 {
				name = "Needs category"
			}
			c = map[string]any{"id": cid, "name": name, "spent_cents": int64(0), "target_cents": int64(0), "pending_cents": int64(0)}
			if global := categories[cid]; global != nil {
				c["total_spent_cents"], c["total_target_cents"] = global["spent_cents"], global["target_cents"]
			}
			children[gid][cid] = c
		}
		return g, c
	}
	for _, budget := range budgets {
		g, c := ensure(budget)
		g["target_cents"] = num(g["target_cents"]) + num(budget["amount_cents"])
		c["target_cents"] = num(budget["amount_cents"])
	}
	for _, row := range rows {
		amount := num(row["amount_cents"])
		if num(row["is_transfer"]) == 1 || !(row["kind"] == "expense" || row["category_id"] == nil && amount < 0) {
			continue
		}
		g, c := ensure(row)
		g["spent_cents"], c["spent_cents"] = num(g["spent_cents"])-amount, num(c["spent_cents"])-amount
		if row["review_state"] == "pending_review" {
			g["pending_cents"], c["pending_cents"] = num(g["pending_cents"])-amount, num(c["pending_cents"])-amount
		}
	}
	result := []map[string]any{}
	for gid, g := range groups {
		cats := []map[string]any{}
		for _, c := range children[gid] {
			cats = append(cats, c)
		}
		budgetSort(cats, r.URL.Query().Get("sort"))
		g["category_total"] = len(cats)
		current := 0
		if selected == gid {
			current = cp
		}
		g["categories"] = dashboardSlice(cats, current)
		result = append(result, g)
	}
	budgetSort(result, r.URL.Query().Get("sort"))
	return dashboardSlice(result, gp), len(result), nil
}

func dashboardSlice(rows []map[string]any, page int) []map[string]any {
	start := page * 20
	if start > len(rows) {
		start = len(rows)
	}
	end := start + 20
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end]
}

func budgetSort(rows []map[string]any, mode string) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if mode == "spending" && num(a["spent_cents"]) != num(b["spent_cents"]) {
			return num(a["spent_cents"]) > num(b["spent_cents"])
		}
		if mode == "remaining" {
			at, bt := num(a["target_cents"]), num(b["target_cents"])
			if (at > 0) != (bt > 0) {
				return at > 0
			}
			ar, br := at-num(a["spent_cents"]), bt-num(b["spent_cents"])
			if at > 0 && ar != br {
				return ar < br
			}
		}
		if a["name"] != b["name"] {
			return a["name"].(string) < b["name"].(string)
		}
		return num(a["id"]) < num(b["id"])
	})
}

// Income is the complement of spending allocations, excluding explicit transfers.
func dashboardIncome(r *http.Request, rows []map[string]any) ([]map[string]any, int, error) {
	page := 0
	if r.URL.Query().Has("income_page") {
		v, err := strconv.Atoi(r.URL.Query().Get("income_page"))
		if err != nil || v < 0 || v > 1000000 {
			return nil, 0, fail(400, "Choose a valid income page")
		}
		page = v
	}
	categories := map[int64]map[string]any{}
	for _, row := range rows {
		amount := num(row["amount_cents"])
		if num(row["is_transfer"]) == 1 || row["kind"] == "expense" || row["category_id"] == nil && amount < 0 {
			continue
		}
		id := num(row["category_id"])
		c := categories[id]
		if c == nil {
			name := row["name"]
			if id == 0 {
				name = "Needs category"
			}
			c = map[string]any{"id": id, "name": name, "income_cents": int64(0)}
			categories[id] = c
		}
		c["income_cents"] = num(c["income_cents"]) + amount
	}
	out := []map[string]any{}
	for _, c := range categories {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["name"].(string) < out[j]["name"].(string) })
	return dashboardSlice(out, page), len(out), nil
}
