package app

// Both the dashboard and alert producers use this allocation arithmetic.
func categoryExpenseTotals(rows []map[string]any) map[int64]int64 {
	totals := map[int64]int64{}
	for _, row := range rows {
		if num(row["is_transfer"]) == 0 && row["kind"] == "expense" && row["category_id"] != nil {
			totals[num(row["category_id"])] -= num(row["amount_cents"])
		}
	}
	return totals
}
