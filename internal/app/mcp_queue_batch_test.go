package app

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
)

type countingMCPQuery struct {
	queryer
	calls int
}

func (q *countingMCPQuery) Query(sql string, args ...any) (*sql.Rows, error) {
	q.calls++
	return q.queryer.Query(sql, args...)
}
func TestMCPReviewAllocationBatchQueryBoundAndOrder(t *testing.T) {
	e := setup(t)
	for i := 0; i < 100; i++ {
		id := seedTransaction(t, e, 1, -100, "2026-10-22", nil)
		e.a.DB.Exec("UPDATE allocations SET amount_cents=-60 WHERE transaction_id=?", id)
		e.a.DB.Exec("INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(?,1,-40)", id)
	}
	q := &countingMCPQuery{queryer: e.a.DB}
	items := []map[string]any{}
	for i := int64(1); i <= 100; i++ {
		items = append(items, map[string]any{"id": i})
	}
	result, err := mcpReviewAllocations(q, items)
	if err != nil || q.calls != 1 || len(result) != 100 {
		t.Fatal("not one page query", q.calls, len(result), err)
	}
	for id, allocations := range result {
		if len(allocations) != 2 || num(allocations[0]["id"]) >= num(allocations[1]["id"]) || num(allocations[0]["amount_cents"])+num(allocations[1]["amount_cents"]) != -100 {
			t.Fatal(id, allocations)
		}
		if allocations[0]["transaction_id"] != nil {
			t.Fatal("extra output field")
		}
	}
	q.calls = 0
	if _, err := mcpReviewAllocations(q, nil); err != nil || q.calls != 0 {
		t.Fatal("empty page queried", err, q.calls)
	}
	// Only page IDs are fetched; no unrelated or lookahead entry is hydrated.
	q.calls = 0
	result, err = mcpReviewAllocations(q, items[:1])
	if err != nil || len(result) != 1 || q.calls != 1 {
		t.Fatal(result, err)
	}
}

func BenchmarkMCPReviewAllocationPage(b *testing.B) {
	app, err := Open(filepath.Join(b.TempDir(), "finance.sqlite"), "http://localhost:8080", filepath.Join(b.TempDir(), "backups"))
	if err != nil {
		b.Fatal(err)
	}
	defer app.Close()
	if _, err := app.DB.Exec(`
 INSERT INTO users(id,username,password) VALUES(1,'synthetic','unused');
 INSERT INTO accounts(id,name,bank_id) VALUES(1,'Synthetic','synthetic-account');
 WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<100)
 INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance)
 SELECT x,1,'2026-10-22',-100,'Synthetic','2026-10-22',-100,'Synthetic','{}' FROM n;
 INSERT INTO allocations(transaction_id,amount_cents) SELECT id,-100 FROM transactions;
 `); err != nil {
		b.Fatal(err)
	}
	for _, size := range []int{1, 50, 100} {
		items := make([]map[string]any, size)
		for i := range items {
			items[i] = map[string]any{"id": int64(i + 1)}
		}
		b.Run(fmt.Sprintf("batch/%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := mcpReviewAllocations(app.DB, items); err != nil {
					b.Fatal(err)
				}
			}
		})
		// The historical per-entry query pattern is retained only as a benchmark
		// comparison, never in a production read path.
		b.Run(fmt.Sprintf("per-entry/%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				for _, item := range items {
					if _, err := data(app.DB, "SELECT l.id,l.category_id,l.amount_cents,c.name category_name,c.kind FROM allocations l LEFT JOIN categories c ON c.id=l.category_id WHERE l.transaction_id=? ORDER BY l.id", item["id"]); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
