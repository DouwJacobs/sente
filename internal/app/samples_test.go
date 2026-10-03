package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSuppliedFNBPair(t *testing.T) {
	dir := os.Getenv("FNB_SAMPLE_DIR")
	if dir == "" {
		t.Skip("set FNB_SAMPLE_DIR to inspect supplied exports in memory")
	}
	parsed := []ParsedFile{}
	for _, name := range []string{"transaction_history_Douw_Fusion.zip", "transaction_history_Douw_Fusion (1).zip"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal("supplied archive is unavailable")
		}
		expanded, err := expand([]inputFile{{name, b}})
		if err != nil {
			t.Fatal(err)
		}
		if len(expanded) != 1 {
			t.Fatal("expected one supported export per supplied archive")
		}
		p := parseFile(expanded[0])
		if p.Error != "" {
			t.Fatal(p.Error)
		}
		for _, r := range p.Rows {
			if r.Error != "" {
				t.Fatalf("parse error at row %d: %s", r.Row, r.Error)
			}
		}
		parsed = append(parsed, p)
	}
	if parsed[0].AccountID != parsed[1].AccountID || parsed[0].Currency != "ZAR" || parsed[1].Currency != "ZAR" {
		t.Fatal("account/currency mismatch")
	}
	counts := map[string]int{}
	for _, r := range parsed[0].Rows {
		counts[r.Date+"|"+fmt.Sprint(r.Amount)+"|"+normalize(r.Description)]++
	}
	ids := map[string]bool{}
	for _, r := range parsed[1].Rows {
		counts[r.Date+"|"+fmt.Sprint(r.Amount)+"|"+normalize(r.Description)]--
		if r.FITID == "" || ids[r.FITID] {
			t.Fatal("missing or nonunique FITID in supplied OFX")
		}
		ids[r.FITID] = true
	}
	for _, n := range counts {
		if n != 0 {
			t.Fatal("CSV/OFX transaction mismatch")
		}
	}
	if len(parsed[0].Rows) != 60 || len(parsed[1].Rows) != 60 {
		t.Fatal("unexpected sample transaction count")
	}
	t.Log("Both supplied exports parsed: 60 matching transactions; 60 unique OFX IDs. No source data persisted.")
}
