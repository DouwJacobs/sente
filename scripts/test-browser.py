#!/usr/bin/env python3
"""Run each browser spec against fresh synthetic servers, using one backend build."""
from pathlib import Path
import os
import re
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time
import urllib.request
import urllib.error

ROOT = Path(__file__).resolve().parents[1]
WEB = ROOT / "web"

def reserve_ports():
    requested = os.environ.get("E2E_TEST_PORT")
    for _ in range(100):
        reservations = []
        try:
            first = socket.socket()
            first.bind(("127.0.0.1", int(requested) if requested else 0))
            reservations.append(first)
            port = first.getsockname()[1]
            if port > 65533:
                raise OSError("No consecutive ports available")
            for number in (port + 1, port + 2):
                reservation = socket.socket()
                reservations.append(reservation)
                reservation.bind(("127.0.0.1", number))
            return port, reservations
        except OSError:
            for reservation in reservations:
                reservation.close()
            if requested:
                raise RuntimeError("The requested browser-test ports are in use")
    raise RuntimeError("Could not reserve three browser-test ports")

def stop_servers(servers):
    for server in servers:
        if server.poll() is None:
            server.terminate()
    # One deadline for the complete fixture set, not one delay per server.
    deadline = time.monotonic() + 5
    for server in servers:
        try:
            server.wait(timeout=max(0.01, deadline - time.monotonic()))
        except subprocess.TimeoutExpired:
            try:
                os.killpg(server.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            server.wait()


def wait_ready(port, servers):
    # Explicitly bypass proxies for local fixture readiness.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    deadline = time.monotonic() + 60
    pending = set(range(port, port + 3))
    while pending and time.monotonic() < deadline:
        if any(server.poll() is not None for server in servers):
            raise RuntimeError("A synthetic browser-test server exited during setup")
        for number in list(pending):
            try:
                with opener.open(f"http://127.0.0.1:{number}/api/health", timeout=1) as response:
                    if response.status == 200:
                        pending.remove(number)
            except (OSError, urllib.error.URLError):
                pass
        if pending:
            time.sleep(0.1)
    if pending:
        raise RuntimeError("Synthetic browser-test servers did not become ready")

def main():
    specs = [arg for arg in sys.argv[1:] if arg.endswith(".spec.ts")]
    options = [arg for arg in sys.argv[1:] if not arg.endswith(".spec.ts")]
    if not specs:
        specs = [path.name for path in sorted((WEB / "e2e").glob("*.spec.ts"))]
    go = os.environ.get("GO") or shutil.which("go") or str(ROOT / "work/toolchain/go/bin/go")
    node = shutil.which("node")
    if not node:
        raise RuntimeError("Activate Linux Node before running browser tests")
    cli = WEB / "node_modules/@playwright/test/cli.js"
    if not cli.is_file():
        raise RuntimeError("Run npm ci in web before browser tests")
    env = {**os.environ, "NO_PROXY": "127.0.0.1,localhost,::1",
           "no_proxy": "127.0.0.1,localhost,::1"}
    with tempfile.TemporaryDirectory(prefix="sente-browser-") as temporary:
        work = Path(temporary)
        binary = work / "finance"
        subprocess.run([go, "build", "-o", str(binary), "./cmd/finance"], cwd=ROOT, env=env, check=True)
        with tempfile.NamedTemporaryFile(mode="w", prefix=".sente-browser-", suffix=".config.ts",
                                         dir=WEB, delete=False) as configuration:
            configuration.write("import {defineConfig} from '@playwright/test'\n"
                                "import base from './playwright.config'\n"
                                "export default defineConfig({...base,webServer:undefined})\n")
        config = Path(configuration.name)
        try:
            for index, spec in enumerate(specs, 1):
                print(f"\n[browser {index}/{len(specs)}] {spec}: fresh synthetic databases", flush=True)
                port, reservations = reserve_ports()
                servers, logs = [], []
                try:
                    # Release the reserved local ports immediately before startup.
                    for reservation in reservations:
                        reservation.close()
                    for offset in range(3):
                        log = open(work / f"server-{offset}.log", "w+")
                        logs.append(log)
                        server_env = {**env, "E2E_BINARY": str(binary),
                                      "E2E_WORK": str(work / f"fixture-{index}-{offset}"),
                                      "E2E_PORT": str(port + offset),
                                      "E2E_EMPTY": "1" if offset else "0",
                                      "E2E_NOTIFICATIONS": "1" if Path(spec).name == "notifications.spec.ts" else "0"}
                        servers.append(subprocess.Popen(
                            [sys.executable, str(ROOT / "scripts/e2e-server.py")],
                            cwd=ROOT, env=server_env, stdout=log, stderr=subprocess.STDOUT,
                            start_new_session=True))
                    wait_ready(port, servers)
                    result = subprocess.run(
                        [node, str(cli), "test", "--config", str(config),
                         r"(?:^|/)" + re.escape(Path(spec).name) + "$", *options],
                        cwd=WEB, env={**env, "E2E_TEST_PORT": str(port)})
                    if result.returncode:
                        return result.returncode
                except Exception:
                    for log in logs:
                        log.flush()
                        log.seek(0)
                        print(log.read(), file=sys.stderr)
                    raise
                finally:
                    stop_servers(servers)
                    for log in logs:
                        log.close()
            return 0
        finally:
            config.unlink(missing_ok=True)

if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        sys.exit(130)
