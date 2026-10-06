package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestFNBWindowsInteropRecoversStaleInheritedSocket(t *testing.T) {
	node := "/mnt/c/Program Files/nodejs/node.exe"
	if runtime.GOOS != "linux" {
		t.Skip("WSL integration")
	}
	if _, err := os.Stat(node); err != nil {
		t.Skip("Windows Node unavailable")
	}
	if _, err := os.Stat("/run/WSL/1_interop"); err != nil {
		t.Skip("WSL bridge unavailable")
	}
	t.Setenv("WSL_INTEROP", "/run/WSL/nonexistent-inherited-bridge")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	env, err := fnbNodeEnvironment(ctx, node)
	if err != nil || len(env) == 0 {
		t.Fatal("interop recovery failed", err)
	}
	// Invalid synthetic input returns a worker-stage result before any browser
	// launch or login, proving that the actual process transport works.
	t.Setenv("FNB_NODE_EXECUTABLE", node)
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("FNB_RUNNER_PATH", `\\wsl.localhost\Ubuntu`+strings.ReplaceAll(filepath.Join(root, "connectors/fnb/owner/refresh.mjs"), "/", `\`))
	report, err := runFNBProvider(ctx, fnbCredentials{}, false)
	if err != nil || report.Diagnostics["refresh_phase"] != 1 || report.Diagnostics["connector_process_failed"] != 0 {
		t.Fatal("worker transport failed", err, report.Error, report.Diagnostics)
	}
}
func TestFNBLocalNodeDoesNotChangeEnvironment(t *testing.T) {
	env, err := fnbNodeEnvironment(context.Background(), "node")
	if err != nil || env != nil {
		t.Fatal("native runtime changed", err)
	}
}
