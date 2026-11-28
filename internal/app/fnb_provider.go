package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func runFNBProvider(ctx context.Context, credentials fnbCredentials, manual bool) (fnbSnapshot, error) {
	return runFNBProviderGrace(ctx, credentials, manual, fnbCleanupGrace)
}
func runFNBProviderGrace(ctx context.Context, credentials fnbCredentials, manual bool, grace time.Duration) (snapshot fnbSnapshot, returned error) {
	cwd, err := os.Getwd()
	if err != nil {
		return snapshot, err
	}
	executable := os.Getenv("FNB_NODE_EXECUTABLE")
	runner := os.Getenv("FNB_RUNNER_PATH")
	if executable == "" {
		windows := "/mnt/c/Program Files/nodejs/node.exe"
		if _, err := os.Stat(windows); err == nil {
			executable = windows
			if runner == "" {
				runner = `\\wsl.localhost\Ubuntu` + strings.ReplaceAll(filepath.Join(cwd, "connectors/fnb/owner/refresh.mjs"), "/", `\`)
			}
		} else {
			executable = "node"
		}
	}
	if runner == "" {
		runner = filepath.Join(cwd, "connectors/fnb/owner/refresh.mjs")
	}
	nodeEnv, bridgeErr := fnbNodeEnvironment(ctx, executable)
	if bridgeErr != nil {
		return fnbSnapshot{Error: "CONNECTOR_START_FAILED", Diagnostics: map[string]int{"connector_interop_failed": 1}}, nil
	}
	profileHost, profileWorker, profileErr := fnbProfile(ctx, executable, nodeEnv)
	if profileErr != nil {
		return fnbSnapshot{Error: "CONNECTOR_START_FAILED", Diagnostics: map[string]int{"connector_process_failed": 1}}, nil
	}
	defer func() {
		if err := os.RemoveAll(profileHost); err != nil {
			snapshot = fnbSnapshot{Error: "CONNECTOR_START_FAILED", Diagnostics: map[string]int{"profile_cleanup_failed": 1}}
			returned = nil
		}
	}()
	payload, _ := json.Marshal(map[string]any{"username": credentials.Username, "password": credentials.Password, "visible": manual, "hidden": credentials.Hidden, "transaction_accounts": credentials.TransactionAccounts, "run_id": credentials.RunID, "profile_directory": profileWorker})
	cmd := exec.CommandContext(ctx, executable, runner)
	cmd.Env = nodeEnv
	defer clear(payload)
	var output bytes.Buffer
	cmd.Stdout = &limitedFNBWriter{writer: &output, remaining: 16 << 20}
	if err = runFNBCommand(ctx, cmd, payload, grace); err != nil {
		code := "CONNECTOR_START_FAILED"
		diagnostics := map[string]int{"connector_process_failed": 1}
		if ctx.Err() != nil {
			code = "CONNECTOR_TIMEOUT"
			diagnostics["refresh_timed_out"] = 1
		}
		return fnbSnapshot{Error: code, Diagnostics: diagnostics}, nil
	}
	d := json.NewDecoder(&output)
	d.DisallowUnknownFields()
	if err = d.Decode(&snapshot); err != nil {
		return fnbSnapshot{Error: "CONNECTOR_RESPONSE_INVALID", Diagnostics: map[string]int{"connector_response_invalid": 1}}, nil
	}
	if d.Decode(new(any)) != io.EOF {
		return fnbSnapshot{Error: "CONNECTOR_RESPONSE_INVALID", Diagnostics: map[string]int{"connector_response_invalid": 1}}, nil
	}
	return snapshot, nil
}

type limitedFNBWriter struct {
	writer    io.Writer
	remaining int
}

// Read-only prerequisite check, never a bank login or browser launch. Browser
// compatibility remains checked by the provider when an approved request runs.
func (a *App) bankRefreshRuntimeReady() bool {
	if a.fnbProvider != nil {
		return true
	}
	executable := os.Getenv("FNB_NODE_EXECUTABLE")
	if executable == "" {
		if _, err := os.Stat("/mnt/c/Program Files/nodejs/node.exe"); err == nil {
			executable = "/mnt/c/Program Files/nodejs/node.exe"
		} else {
			executable = "node"
		}
	}
	if _, err := exec.LookPath(executable); err != nil {
		return false
	}
	runner := os.Getenv("FNB_RUNNER_PATH")
	if runner == "" {
		runner = "connectors/fnb/owner/refresh.mjs"
	}
	info, err := os.Stat(runner)
	return err == nil && !info.IsDir()
}

func (w *limitedFNBWriter) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		return 0, fmt.Errorf("connector output exceeded")
	}
	w.remaining -= len(p)
	return w.writer.Write(p)
}
