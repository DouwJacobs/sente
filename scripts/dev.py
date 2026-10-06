#!/usr/bin/env python3
"""Run Vite HMR and a restarting Go backend in WSL, without Docker."""
from pathlib import Path
import os
import shutil
import sqlite3
import signal
import socket
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[1]
WEB = ROOT / "web"
WORK = ROOT / "work/dev"
processes = []

def stop(proc):
    if proc is None or proc.poll() is not None:
        return
    try:
        os.killpg(proc.pid, signal.SIGTERM)
    except ProcessLookupError:
        return
    try:
        proc.wait(timeout=15)
    except subprocess.TimeoutExpired:
        os.killpg(proc.pid, signal.SIGKILL)
        proc.wait()

def interrupted(*_):
    raise KeyboardInterrupt

def sources():
    paths = [ROOT / "go.mod", ROOT / "go.sum"]
    for folder in ("cmd", "internal"):
        paths.extend(p for p in (ROOT / folder).rglob("*")
                     if p.is_file() and p.suffix in (".go", ".sql")
                     and not p.name.endswith("_test.go"))
    return tuple((str(p), p.stat().st_mtime_ns, p.stat().st_size) for p in sorted(paths) if p.exists())

def free_port(value):
    port = int(value)
    if not 1 <= port <= 65535:
        raise ValueError("Development ports must be between 1 and 65535")
    with socket.socket() as check:
        check.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        try:
            check.bind(("127.0.0.1", port))
        except OSError:
            raise RuntimeError(f"Port {port} is in use. Stop the previous dev session or choose another DEV_PORT / DEV_API_PORT.")
    return port

def main():
    env = os.environ.copy()
    # WSL may inherit Windows npm; choose a Linux Node installation.
    node = shutil.which("node")
    if not node or node.startswith("/mnt/"):
        versions = Path(env.get("NVM_DIR", str(Path.home() / ".nvm"))) / "versions/node"
        choices = sorted(versions.glob("v*/bin/node"),
                         key=lambda p: tuple(int(n) for n in p.parents[1].name.lstrip("v").split(".")))
        if not choices:
            raise RuntimeError("Install Node 22.21+ in WSL, or activate it with nvm first.")
        node = str(choices[-1])
        env["PATH"] = str(Path(node).parent) + os.pathsep + env.get("PATH", "")
    npm = shutil.which("npm", path=env["PATH"])
    if not npm or npm.startswith("/mnt/"):
        raise RuntimeError("Activate a Linux Node/npm installation in WSL.")
    go = shutil.which(env.get("GO", "go"), path=env["PATH"])
    if not go and env.get("GO", "go") == "go":
        go = str(ROOT / "work/toolchain/go/bin/go")
    if not go or not Path(go).is_file():
        raise RuntimeError("Install Go in WSL or set GO to your Go executable.")
    ui_port = free_port(env.get("DEV_PORT", "5173"))
    api_port = free_port(env.get("DEV_API_PORT", "8081"))
    if ui_port == api_port:
        raise RuntimeError("DEV_PORT and DEV_API_PORT must be different.")
    origin = env.get("DEV_PUBLIC_URL", f"http://127.0.0.1:{ui_port}").rstrip("/")
    env.update(DEV_RESTART_MANAGED="1", PORT=str(api_port), PUBLIC_URL=origin,
               DATABASE_PATH=env.get("DATABASE_PATH", str(ROOT / "data/dev/finance.sqlite")),
               BACKUP_DIR=env.get("BACKUP_DIR", str(ROOT / "backups/dev")),
               STATIC_DIR=str(WEB / "dist"), DEV_API_TARGET=f"http://127.0.0.1:{api_port}")
    WORK.mkdir(parents=True, exist_ok=True)
    if not (WEB / "node_modules/.bin/vite").exists():
        subprocess.run([npm, "ci"], cwd=WEB, env=env, check=True)
    binary = WORK / "finance"
    candidate = WORK / "finance.next"

    def build():
        print("[dev] Building Go backend…", flush=True)
        result = subprocess.run([go, "build", "-o", str(candidate), "./cmd/finance"],
                                cwd=ROOT, env=env)
        if result.returncode:
            print("[dev] Build failed. Fix the error and save; the last working backend stays running.", flush=True)
            return False
        candidate.replace(binary)
        return True

    def backend():
        proc = subprocess.Popen([str(binary), "serve"], cwd=ROOT, env=env, start_new_session=True)
        processes.append(proc)
        return proc

    observed = sources()
    if not build():
        raise RuntimeError("Initial backend build failed.")
    server = backend()
    def frontend():
        public_url = origin
        for attempt in range(100):
            try:
                with sqlite3.connect(f"file:{env['DATABASE_PATH']}?mode=ro", uri=True) as db:
                    row = db.execute("SELECT enabled,public_url FROM network_settings WHERE id=1").fetchone()
                if row and row[0]:
                    public_url = row[1]
                break
            except sqlite3.OperationalError:
                if attempt == 99:
                    raise RuntimeError("Backend network settings are unavailable")
                time.sleep(0.1)
        env["DEV_PUBLIC_URL"] = public_url
        proc = subprocess.Popen([npm, "run", "dev", "--", "--port", str(ui_port), "--strictPort"],
                                cwd=WEB, env=env, start_new_session=True)
        processes.append(proc)
        return proc
    vite = frontend()
    print(f"[dev] Open {origin}. UI edits hot reload; Go/schema edits rebuild and restart the API.", flush=True)
    print("[dev] Uses a separate persistent development database. Ctrl+C stops both servers.", flush=True)
    while True:
        time.sleep(0.5)
        if vite.poll() is not None:
            raise RuntimeError("Vite stopped. Read its output above.")
        if server.poll() == 75:
            stop(vite)
            server = backend()
            vite = frontend()
            print("[dev] Network restart complete; backend and Vite restarted.", flush=True)
        elif server.poll() is not None:
            raise RuntimeError("The Go backend stopped. Read its output above.")
        current = sources()
        if current != observed:
            time.sleep(0.3)  # Coalesce editor save events.
            observed = sources()
            if build():
                stop(server)
                server = backend()
                print("[dev] Backend restarted.", flush=True)

if __name__ == "__main__":
    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    code = 0
    try:
        main()
    except KeyboardInterrupt:
        pass
    except (RuntimeError, ValueError, OSError, subprocess.CalledProcessError) as err:
        print(f"[dev] {err}", file=sys.stderr)
        code = 1
    finally:
        for proc in reversed(processes):
            stop(proc)
    sys.exit(code)
