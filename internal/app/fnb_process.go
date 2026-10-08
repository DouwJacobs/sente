package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const fnbCleanupGrace = 30 * time.Second

// stderr is an IPC channel for two numeric PIDs only. Never retain worker errors.
type fnbProcessInfo struct {
	mu              sync.Mutex
	line            []byte
	worker, browser int
	dropping        bool
}

func (p *fnbProcessInfo) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range b {
		if c == '\n' {
			p.dropping = false
			fields := strings.Fields(string(p.line))
			p.line = nil
			if len(fields) == 2 {
				pid, err := strconv.Atoi(fields[1])
				if err == nil && pid > 1 {
					switch fields[0] {
					case "FNB_WORKER_PID":
						p.worker = pid
					case "FNB_BROWSER_PID":
						p.browser = pid
					}
				}
			}
		} else if !p.dropping {
			candidate := string(append(p.line, c))
			valid := false
			for _, prefix := range []string{"FNB_WORKER_PID ", "FNB_BROWSER_PID "} {
				if strings.HasPrefix(prefix, candidate) {
					valid = true
				}
				if strings.HasPrefix(candidate, prefix) {
					suffix := strings.TrimPrefix(candidate, prefix)
					valid = len(suffix) <= 10
					for _, digit := range suffix {
						if digit < '0' || digit > '9' {
							valid = false
						}
					}
				}
				if valid {
					break
				}
			}
			if valid {
				p.line = append(p.line, c)
			} else {
				p.line = nil
				p.dropping = true
			}
		}
	}
	return len(b), nil
}
func (p *fnbProcessInfo) pids() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.worker, p.browser
}

func fnbWindowsExecutable(executable string) bool {
	return strings.HasSuffix(strings.ToLower(executable), ".exe")
}

// Go creates the exact profile directory and removes only that directory after
// child-tree termination. WSL Chrome needs a local Windows temporary directory.
func fnbProfile(ctx context.Context, executable string, env []string) (host, worker string, err error) {
	if !fnbWindowsExecutable(executable) {
		host, err = os.MkdirTemp("", "finance-fnb-refresh-")
		return host, host, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	probe := exec.CommandContext(probeCtx, executable, "-p", "JSON.stringify(require('node:os').tmpdir())")
	probe.Env = env
	probe.Stderr = io.Discard
	var raw strings.Builder
	probe.Stdout = &limitedFNBWriter{writer: &raw, remaining: 4096}
	if err = probe.Run(); err != nil {
		return
	}
	var winTemp string
	if err = json.Unmarshal([]byte(raw.String()), &winTemp); err != nil {
		return
	}
	if len(winTemp) < 3 || winTemp[1] != ':' || winTemp[2] != '\\' {
		err = fmt.Errorf("invalid connector temporary directory")
		return
	}
	linuxTemp := "/mnt/" + strings.ToLower(winTemp[:1]) + "/" + strings.ReplaceAll(winTemp[3:], "\\", "/")
	host, err = os.MkdirTemp(linuxTemp, "finance-fnb-refresh-")
	if err == nil {
		worker = strings.TrimRight(winTemp, "\\") + "\\" + filepath.Base(host)
	}
	return
}

func fnbKillTree(cmd *exec.Cmd, info *fnbProcessInfo, windows bool) error {
	worker, browser := info.pids()
	if windows {
		if worker <= 1 {
			return fmt.Errorf("connector worker identity unavailable")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		kill := exec.CommandContext(ctx, "/mnt/c/Windows/System32/taskkill.exe", "/PID", strconv.Itoa(worker), "/T", "/F")
		kill.Env = cmd.Env
		kill.Stdout = io.Discard
		kill.Stderr = io.Discard
		err := kill.Run()
		// The interop launcher must also be reaped if Windows termination failed.
		_ = cmd.Process.Kill()
		return err
	}
	// Chromium may create a separate process group; capture all descendants before
	// killing the worker and also retain its reported browser group.
	var killDescendants func(int)
	killDescendants = func(pid int) {
		raw, _ := os.ReadFile(fmt.Sprintf("/proc/%d/task/%d/children", pid, pid))
		for _, field := range strings.Fields(string(raw)) {
			child, e := strconv.Atoi(field)
			if e == nil && child > 1 {
				killDescendants(child)
				_ = syscall.Kill(-child, syscall.SIGKILL)
				_ = syscall.Kill(child, syscall.SIGKILL)
			}
		}
	}
	killDescendants(cmd.Process.Pid)
	if browser > 1 {
		_ = syscall.Kill(-browser, syscall.SIGKILL)
		_ = syscall.Kill(browser, syscall.SIGKILL)
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	return cmd.Process.Kill()
}

// A stdin control frame works on Linux and Windows, unlike Unix signals through
// WSL interop. Cancellation discards output even if the child exits successfully.
func runFNBCommand(ctx context.Context, cmd *exec.Cmd, payload []byte, grace time.Duration) error {
	windows := fnbWindowsExecutable(cmd.Path)
	if !windows {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	input, control, err := os.Pipe()
	if err != nil {
		return err
	}
	defer input.Close()
	defer control.Close()
	cmd.Stdin = input
	info := &fnbProcessInfo{}
	cmd.Stderr = info
	// CommandContext's default kill must not bypass cooperative cleanup.
	cmd.Cancel = func() error {
		_, err := control.Write([]byte("{\"cancel\":true}\n"))
		if !windows {
			signalErr := cmd.Process.Signal(syscall.SIGTERM)
			if err == nil {
				return signalErr
			}
		}
		return err
	}
	cmd.WaitDelay = grace + 5*time.Second
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	input.Close()
	frame := append(append([]byte(nil), payload...), '\n')
	defer clear(frame)
	if _, err = control.Write(frame); err != nil {
		_ = fnbKillTree(cmd, info, windows)
		_ = cmd.Wait()
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err = <-done:
		if ctx.Err() != nil {
			_, browser := info.pids()
			if browser > 1 {
				if windows {
					fnbKillWindowsBrowser(cmd, browser)
				} else {
					_ = syscall.Kill(-browser, syscall.SIGKILL)
				}
			}
			return ctx.Err()
		}
		return err
	case <-ctx.Done():
		// Send immediately even if CommandContext's watcher has not run yet.
		_, _ = control.Write([]byte("{\"cancel\":true}\n"))
		timer := time.NewTimer(grace)
		defer timer.Stop()
		select {
		case <-done:
			// Cleanup is still required if the worker exited but detached Chrome survived.
			if windows {
				_, browser := info.pids()
				if browser > 1 {
					fnbKillWindowsBrowser(cmd, browser)
				}
			} else {
				_, browser := info.pids()
				if browser > 1 {
					_ = syscall.Kill(-browser, syscall.SIGKILL)
				}
			}
		case <-timer.C:
			_ = fnbKillTree(cmd, info, windows)
			<-done
		}
		return ctx.Err()
	}
}
func fnbKillWindowsBrowser(cmd *exec.Cmd, pid int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	kill := exec.CommandContext(ctx, "/mnt/c/Windows/System32/taskkill.exe", "/PID", strconv.Itoa(pid), "/T", "/F")
	kill.Env = cmd.Env
	kill.Stdout = io.Discard
	kill.Stderr = io.Discard
	_ = kill.Run()
}
