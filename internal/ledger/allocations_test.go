package ledger

import (
	"errors"
	"testing"

	"finance-tracker/internal/problem"
)

func TestAllocationFinancialInvariants(t *testing.T) {
	category := int64(1)
	unknown := int64(99)
	known := func(id int64) bool { return id == category }
	cases := []struct {
		name                     string
		amount                   int64
		allocations              []Allocation
		approve, transfer, valid bool
	}{
		{"exact split", -29, []Allocation{{CategoryID: &category, Amount: -20}, {CategoryID: &category, Amount: -9}}, true, false, true},
		{"refund", 29, []Allocation{{CategoryID: &category, Amount: 29}}, true, false, true},
		{"unclassified pending", -29, []Allocation{{Amount: -29}}, false, false, true},
		{"transfer exemption", -29, []Allocation{{Amount: -29}}, true, true, true},
		{"missing category", -29, []Allocation{{Amount: -29}}, true, false, false},
		{"unknown category", -29, []Allocation{{CategoryID: &unknown, Amount: -29}}, false, false, false},
		{"wrong sign", -29, []Allocation{{Amount: -30}, {Amount: 1}}, false, false, false},
		{"wrong total", -29, []Allocation{{Amount: -28}}, false, false, false},
		{"zero sign", 0, []Allocation{{Amount: 1}, {Amount: -1}}, false, false, false},
		{"no allocations", 0, nil, false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateAllocations(c.amount, c.allocations, c.approve, c.transfer, known)
			if (err == nil) != c.valid {
				t.Fatalf("validation: %v", err)
			}
			if err != nil {
				var response problem.Error
				if !errors.As(err, &response) || response.Code != 400 {
					t.Fatal("validation lost its safe API status", err)
				}
			}
		})
	}
}
