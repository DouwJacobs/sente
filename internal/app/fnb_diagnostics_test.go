package app

import "testing"

func TestFNBLoginDiagnosticsRemainBoundedAndAllowlisted(t *testing.T) {
	input := map[string]int{"login_step": 5, "login_tls_failed": 1, "login_timeout": -1, "password": 1, "login_browser_closed": 10001}
	output := safeFNBDiagnostics(input)
	if len(output) != 2 || output["login_step"] != 5 || output["login_tls_failed"] != 1 {
		t.Fatalf("unexpected diagnostic allowlist: %v", output)
	}
}
