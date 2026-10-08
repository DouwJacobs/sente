package app

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const cancellationFixture = `import {createInterface} from 'node:readline';
import {spawn} from 'node:child_process';
import {writeFileSync,rmSync} from 'node:fs';
import {join} from 'node:path';
process.on('SIGTERM',()=>{});
process.stderr.write('FNB_WORKER_PID '+process.pid+'\n');
const lines=createInterface({input:process.stdin});let profile,child;
lines.on('line',line=>{
 const input=JSON.parse(line);
 if(!profile){
  profile=input.profile_directory;
  if(process.argv[3]!=='startup'){
   const chromium=process.argv[3]==='chromium-hung';
   const binary=chromium?process.argv[4]:process.execPath;
   const args=chromium?['--headless','--no-sandbox','--disable-background-networking','--disable-component-update','--disable-default-apps','--disable-sync','--no-first-run','--no-default-browser-check','--user-data-dir='+profile,'about:blank']:['-e','setInterval(()=>{},1000)'];
   child=spawn(binary,args,{detached:true,stdio:'ignore'});
   process.stderr.write('FNB_BROWSER_PID '+child.pid+'\n');
  }
  writeFileSync(join(profile,'child.pid'),String(child?.pid||0));
  const ready=()=>{if(child&&child.exitCode!==null)process.exit(2);writeFileSync(process.argv[2],profile)};if(process.argv[3]==='chromium-hung')setTimeout(ready,750);else ready();
 }else if(input.cancel){
  if(process.argv[3]==='hung'||process.argv[3]==='chromium-hung')return;
  try{child?.kill('SIGKILL')}catch{}
  rmSync(profile,{recursive:true,force:true});
  process.stdout.write(JSON.stringify({accounts:[{name:'Synthetic',bank_id:'1234',balance_decimal:'0.01'}],skipped:0}));
  process.exit(0);
 }
});`

func TestFNBCancellationCleansSyntheticProcessTreeAndProfile(t *testing.T) {
	for _, windows := range []bool{false, true} {
		for _, mode := range []string{"startup", "cooperative", "hung", "chromium-hung"} {
			name := mode
			if windows {
				name = "wsl-windows-" + mode
			}
			t.Run(name, func(t *testing.T) {
				executable, _ := exec.LookPath("node")
				if executable == "" || fnbWindowsExecutable(executable) {
					executable = "/home/douw/.nvm/versions/node/v22.21.1/bin/node"
				}
				if windows {
					executable = "/mnt/c/Program Files/nodejs/node.exe"
				}
				if _, err := os.Stat(executable); err != nil {
					t.Skip("synthetic runtime unavailable")
				}
				dir := t.TempDir()
				runner := filepath.Join(dir, "synthetic-cancel.mjs")
				marker := filepath.Join(dir, "ready")
				if err := os.WriteFile(runner, []byte(cancellationFixture), 0600); err != nil {
					t.Fatal(err)
				}
				// A tiny launcher supplies only synthetic readiness/mode arguments.
				launcher := filepath.Join(dir, "launcher.mjs")
				target, ready := runner, marker
				if windows {
					target = `\\wsl.localhost\Ubuntu` + strings.ReplaceAll(target, "/", `\`)
					ready = `\\wsl.localhost\Ubuntu` + strings.ReplaceAll(ready, "/", `\`)
				}
				browser := ""
				if mode == "chromium-hung" {
					home, _ := os.UserHomeDir()
					roots := []string{filepath.Join(home, ".cache", "ms-playwright")}
					if configured := os.Getenv("PLAYWRIGHT_BROWSERS_PATH"); configured != "" && configured != "0" {
						roots = append([]string{configured}, roots...)
					}
					var matches []string
					for _, root := range roots {
						for _, layout := range []string{"chrome-linux64", "chrome-linux"} {
							found, _ := filepath.Glob(filepath.Join(root, "chromium-*", layout, "chrome"))
							matches = append(matches, found...)
						}
					}
					if windows {
						browser = "C:/Program Files/Google/Chrome/Application/chrome.exe"
						if _, err := os.Stat("/mnt/c/Program Files/Google/Chrome/Application/chrome.exe"); err != nil {
							t.Skip("Windows synthetic Chrome unavailable")
						}
					} else {
						if len(matches) == 0 {
							t.Skip("Linux synthetic Chromium unavailable")
						}
						browser = matches[len(matches)-1]
					}
				}
				script := fmtLaunch(target, ready, mode, browser)
				if err := os.WriteFile(launcher, []byte(script), 0600); err != nil {
					t.Fatal(err)
				}
				launcherArg := launcher
				if windows {
					launcherArg = `\\wsl.localhost\Ubuntu` + strings.ReplaceAll(launcher, "/", `\`)
				}
				t.Setenv("FNB_NODE_EXECUTABLE", executable)
				t.Setenv("FNB_RUNNER_PATH", launcherArg)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan fnbSnapshot, 1)
				go func() {
					snapshot, _ := runFNBProviderGrace(ctx, fnbCredentials{Username: "synthetic", Password: "synthetic"}, false, 250*time.Millisecond)
					done <- snapshot
				}()
				deadline := time.Now().Add(8 * time.Second)
				var profile string
				for time.Now().Before(deadline) {
					raw, err := os.ReadFile(marker)
					if err == nil && len(raw) > 0 {
						profile = string(raw)
						break
					}
					select {
					case snapshot := <-done:
						t.Fatalf("worker did not start: %s", snapshot.Error)
					default:
					}
					time.Sleep(20 * time.Millisecond)
				}
				if profile == "" {
					cancel()
					t.Fatal("synthetic worker not ready")
				}
				hostProfile := profile
				if windows {
					hostProfile = "/mnt/" + strings.ToLower(profile[:1]) + "/" + strings.ReplaceAll(profile[3:], `\`, "/")
				}
				raw, err := os.ReadFile(filepath.Join(hostProfile, "child.pid"))
				if err != nil {
					t.Fatal(err)
				}
				pid, _ := strconv.Atoi(string(raw))
				start := time.Now()
				cancel()
				select {
				case snapshot := <-done:
					if snapshot.Error != "CONNECTOR_TIMEOUT" || len(snapshot.Accounts) != 0 {
						t.Fatal("cancelled result accepted", snapshot.Error)
					}
				case <-time.After(8 * time.Second):
					t.Fatal("cancellation exceeded bound")
				}
				if time.Since(start) > 7*time.Second {
					t.Fatal("slow synthetic cancellation")
				}
				if _, err := os.Stat(hostProfile); !os.IsNotExist(err) {
					t.Fatal("profile survived cancellation")
				}
				if windows && pid > 1 {
					probeCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
					defer stop()
					probe := exec.CommandContext(probeCtx, executable, "-e", "try{process.kill("+strconv.Itoa(pid)+",0);process.exit(1)}catch{process.exit(0)}")
					env, _ := fnbNodeEnvironment(probeCtx, executable)
					probe.Env = env
					if err := probe.Run(); err != nil {
						t.Fatal("Windows child survived termination", err)
					}
				} else if pid > 1 {
					// A killed child can briefly remain a zombie until PID 1 reaps it.
					deadline := time.Now().Add(2 * time.Second)
					for {
						stat, _ := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
						if len(stat) == 0 || strings.Contains(string(stat), ") Z ") {
							break
						}
						if time.Now().After(deadline) {
							t.Fatal("Linux child survived termination")
						}
						time.Sleep(20 * time.Millisecond)
					}
				}
			})
		}
	}
}
func fmtLaunch(target, ready, mode, browser string) string {
	values, _ := json.Marshal([]string{target, ready, mode, browser})
	return "const args=" + string(values) + ";process.argv=[process.execPath,...args];await import('node:url').then(async({pathToFileURL})=>import(pathToFileURL(args[0]).href));"
}
