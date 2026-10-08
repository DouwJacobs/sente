#!/usr/bin/env python3
"""Run each browser spec against fresh synthetic servers, using one backend build."""
from pathlib import Path
import os
import json
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

SUITES = json.loads((ROOT / "scripts/browser-suites.json").read_text())

def pr_specs(changed):
    if not isinstance(changed, list) or any(not isinstance(name, str) for name in changed):
        raise ValueError("Changed browser specs must be a JSON array of filenames")
    selected = list(SUITES["pr"])
    for name in changed:
        if Path(name).name != name or not name.endswith(".spec.ts"):
            raise ValueError("Changed browser specs must be root spec filenames")
        if (WEB / "e2e" / name).is_file() and name not in selected:
            selected.append(name)
    return selected

def fixture_offsets(spec):
    # New/unlisted specs retain the historical three services until reviewed.
    return SUITES["fixture_offsets"].get(Path(spec).name, [0, 1, 2])

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


def wait_ready(port, servers, offsets):
    # Explicitly bypass proxies for local fixture readiness.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    deadline = time.monotonic() + 60
    pending = {port + offset for offset in offsets}
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
    arguments = sys.argv[1:]
    suite = "full"
    if "--suite" in arguments:
        index = arguments.index("--suite")
        if index + 1 == len(arguments) or arguments[index + 1] not in {"pr", "full"}:
            raise ValueError("--suite requires pr or full")
        suite = arguments[index + 1]
        arguments = arguments[:index] + arguments[index + 2:]
    specs = [arg for arg in arguments if arg.endswith(".spec.ts")]
    options = [arg for arg in arguments if not arg.endswith(".spec.ts")]
    if not specs:
        specs = pr_specs(json.loads(os.environ.get("E2E_CHANGED_SPECS", "[]"))) if suite == "pr" else [path.name for path in sorted((WEB / "e2e").glob("*.spec.ts"))]
    go = os.environ.get("GO") or shutil.which("go") or str(ROOT / "work/toolchain/go/bin/go")
    node = shutil.which("node")
    if not node:
        raise RuntimeError("Activate Linux Node before running browser tests")
    cli = WEB / "node_modules/@playwright/test/cli.js"
    if not cli.is_file():
        raise RuntimeError("Run npm ci in web before browser tests")
    env = {**os.environ, "NO_PROXY": "127.0.0.1,localhost,::1",
           "no_proxy": "127.0.0.1,localhost,::1"}
    run_started = time.monotonic()
    results_root = WEB / "test-results"
    results_root.mkdir(exist_ok=True)
    results = Path(tempfile.mkdtemp(prefix="run-", dir=results_root))
    print(f"[browser results] {results}", flush=True)
    timings = {"build_seconds": None, "specs": []}
    with tempfile.TemporaryDirectory(prefix="sente-browser-") as temporary:
        work = Path(temporary)
        binary = work / "finance"
        build_started = time.monotonic()
        subprocess.run([go, "build", "-o", str(binary), "./cmd/finance"], cwd=ROOT, env=env, check=True)
        timings["build_seconds"] = round(time.monotonic() - build_started, 3)
        with tempfile.NamedTemporaryFile(mode="w", prefix=".sente-browser-", suffix=".config.ts",
                                         dir=WEB, delete=False) as configuration:
            configuration.write("import {defineConfig} from '@playwright/test'\n"
                                "import base from './playwright.config'\n"
                                "export default defineConfig({...base,webServer:undefined,\n"
                                "outputDir:process.env.E2E_RESULT_DIR+'/artifacts',\n"
                                "reporter:[['list'],['json',{outputFile:process.env.E2E_RESULT_DIR+'/results.json'}]]})\n")
        config = Path(configuration.name)
        try:
            for index, spec in enumerate(specs, 1):
                print(f"\n[browser {index}/{len(specs)}] {spec}: fresh synthetic databases", flush=True)
                spec_results = results / f"{index:02d}-{Path(spec).stem}"
                spec_results.mkdir()
                timing = {"spec": Path(spec).name, "setup_seconds": None,
                          "test_seconds": None, "shutdown_seconds": None, "exit_code": None}
                timings["specs"].append(timing)
                setup_started = time.monotonic()
                port, reservations = reserve_ports()
                spec_env = {}
                if Path(spec).name == "pwa.spec.ts":
                    static_copy = work / "pwa-static"
                    shutil.copytree(WEB / "dist", static_copy)
                    spec_env = {"E2E_STATIC_DIR": str(static_copy), "E2E_PWA_STATIC_DIR": str(static_copy)}
                offsets = fixture_offsets(spec)
                timing["fixture_offsets"] = offsets
                servers, logs = [], []
                try:
                    # Release the reserved local ports immediately before startup.
                    for reservation in reservations:
                        reservation.close()
                    for offset in offsets:
                        log = open(spec_results / f"server-{offset}.log", "w+")
                        logs.append(log)
                        server_env = {**env, **spec_env, "E2E_BINARY": str(binary),
                                      "E2E_WORK": str(work / f"fixture-{index}-{offset}"),
                                      "E2E_PORT": str(port + offset),
                                      "E2E_EMPTY": "1" if offset else "0",
                                      "E2E_NOTIFICATIONS": "1" if Path(spec).name == "notifications.spec.ts" else "0"}
                        servers.append(subprocess.Popen(
                            [sys.executable, str(ROOT / "scripts/e2e-server.py")],
                            cwd=ROOT, env=server_env, stdout=log, stderr=subprocess.STDOUT,
                            start_new_session=True))
                    wait_ready(port, servers, offsets)
                    timing["setup_seconds"] = round(time.monotonic() - setup_started, 3)
                    test_started = time.monotonic()
                    result = subprocess.run(
                        [node, str(cli), "test", "--config", str(config),
                         r"(?:^|/)" + re.escape(Path(spec).name) + "$", *options],
                        cwd=WEB, env={**env, **spec_env, "E2E_TEST_PORT": str(port),
                                      "E2E_RESULT_DIR": str(spec_results)})
                    timing["test_seconds"] = round(time.monotonic() - test_started, 3)
                    timing["exit_code"] = result.returncode
                    if result.returncode:
                        return result.returncode
                except Exception:
                    for log in logs:
                        log.flush()
                        log.seek(0)
                        print(log.read(), file=sys.stderr)
                    raise
                finally:
                    shutdown_started = time.monotonic()
                    stop_servers(servers)
                    timing["shutdown_seconds"] = round(time.monotonic() - shutdown_started, 3)
                    for log in logs:
                        log.close()
            return 0
        finally:
            config.unlink(missing_ok=True)
            timings["total_seconds"] = round(time.monotonic() - run_started, 3)
            (results / "timings.json").write_text(json.dumps(timings, indent=2) + "\n")
            print(f"[browser timing] {timings['total_seconds']}s; {results / 'timings.json'}", flush=True)

if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        sys.exit(130)
