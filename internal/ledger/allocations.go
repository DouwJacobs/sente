// Package ledger owns financial invariants shared by browser and MCP writes.
package ledger

import "finance-tracker/internal/problem"

type Allocation struct {
	CategoryID *int64 `json:"category_id"`
	Amount     int64  `json:"amount_cents"`
	Note       string `json:"note,omitempty"`
}

func ValidateAllocations(amount int64, alloc []Allocation, approve, transfer bool, categoryExists func(int64) bool) error {
	if len(alloc) == 0 || len(alloc) > 100 {
		return problem.New(400, "Provide between 1 and 100 allocations")
	}
	var sum int64
	for _, v := range alloc {
		if (amount < 0 && v.Amount > 0) || (amount > 0 && v.Amount < 0) || (amount == 0 && v.Amount != 0) {
			return problem.New(400, "All allocations must have the same sign as the transaction")
		}
		if len(v.Note) > 500 {
			return problem.New(400, "Allocation note is too long")
		}
		if v.Amount > 900000000000000 || v.Amount < -900000000000000 {
			return problem.New(400, "Allocation is too large")
		}
		sum += v.Amount
		if v.CategoryID != nil && !categoryExists(*v.CategoryID) {
			return problem.New(400, "Unknown category")
		}
		if approve && !transfer && v.CategoryID == nil {
			return problem.New(400, "Categorize every allocation before approval")
		}
	}
	if sum != amount {
		return problem.New(400, "Split allocations must equal the transaction amount exactly")
	}
	return nil
}
