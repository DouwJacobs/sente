package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// A long-lived WSL server can retain a dead Windows interop socket. Probe only
// Node's version, with no credentials, before selecting a live bridge. Prefer
// the WSL init socket, whose lifetime exceeds an interactive launch session.
func fnbNodeEnvironment(ctx context.Context, executable string) ([]string, error) {
	if runtime.GOOS != "linux" || !strings.HasPrefix(executable, "/mnt/") || !strings.HasSuffix(strings.ToLower(executable), ".exe") {
		return nil, nil
	}
	paths, _ := filepath.Glob("/run/WSL/*_interop")
	sort.Slice(paths, func(i, j int) bool {
		a, ae := os.Stat(paths[i])
		b, be := os.Stat(paths[j])
		if ae != nil {
			return false
		}
		if be != nil {
			return true
		}
		return a.ModTime().After(b.ModTime())
	})
	candidates := append([]string{"/run/WSL/1_interop", os.Getenv("WSL_INTEROP")}, paths...)
	seen := map[string]bool{}
	attempts := 0
	for _, socket := range candidates {
		if socket == "" || seen[socket] {
			continue
		}
		seen[socket] = true
		if _, err := os.Stat(socket); err != nil {
			continue
		}
		attempts++
		if attempts > 8 {
			break
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		env := []string{}
		for _, item := range os.Environ() {
			if !strings.HasPrefix(item, "WSL_INTEROP=") {
				env = append(env, item)
			}
		}
		env = append(env, "WSL_INTEROP="+socket)
		probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		probe := exec.CommandContext(probeCtx, executable, "--version")
		probe.Env = env
		probe.Stdout = io.Discard
		probe.Stderr = io.Discard
		err := probe.Run()
		cancel()
		if err == nil {
			return env, nil
		}
	}
	return nil, fmt.Errorf("Windows interop unavailable")
}
