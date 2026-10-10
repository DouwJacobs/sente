package app

import "sort"

// dashboardSpending keeps the complete group/category breakdown from the same
// authorized allocation snapshot as the dashboard totals, independent of paging.
func dashboardSpending(rows []map[string]any) []map[string]any {
	type key struct{ group, category int64 }
	totals := map[key]map[string]any{}
	for _, row := range rows {
		amount := num(row["amount_cents"])
		if num(row["is_transfer"]) == 1 || !(row["kind"] == "expense" || row["category_id"] == nil && amount < 0) {
			continue
		}
		k := key{num(row["spending_group_id"]), num(row["category_id"])}
		entry := totals[k]
		if entry == nil {
			group, category := row["spending_group_name"], row["name"]
			if k.group == 0 {
				group = "No spending group"
			}
			if k.category == 0 {
				category = "Uncategorised"
			}
			entry = map[string]any{"group_id": k.group, "group_name": group, "category_id": k.category, "category_name": category, "spent_cents": int64(0)}
			totals[k] = entry
		}
		entry["spent_cents"] = num(entry["spent_cents"]) - amount
	}
	result := make([]map[string]any, 0, len(totals))
	for _, entry := range totals {
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a["group_name"] != b["group_name"] {
			return a["group_name"].(string) < b["group_name"].(string)
		}
		if num(a["group_id"]) != num(b["group_id"]) {
			return num(a["group_id"]) < num(b["group_id"])
		}
		if a["category_name"] != b["category_name"] {
			return a["category_name"].(string) < b["category_name"].(string)
		}
		return num(a["category_id"]) < num(b["category_id"])
	})
	return result
}
