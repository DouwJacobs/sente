package money

import "testing"

func TestMoney(t *testing.T) {
	for s, want := range map[string]int64{"0.29": 29, "-123.45": -12345, "1.2": 120, "+1": 100} {
		got, err := Cents(s)
		if err != nil || got != want {
			t.Fatalf("%s: %d %v", s, got, err)
		}
	}
	for _, s := range []string{"1e2", "1.001", "NaN", "1x", "9223372036854775807", ""} {
		if _, err := Cents(s); err == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}
