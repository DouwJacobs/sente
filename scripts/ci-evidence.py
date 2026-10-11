#!/usr/bin/env python3
"""Reuse successful source gates only for an identical Git tree; fail closed."""
import argparse
import io
import json
import os
from pathlib import Path
import subprocess
import time
import zipfile

ROOT = Path(__file__).resolve().parents[1]
ARTIFACT = "source-verification"
SCHEMA = 1

def git_tree():
    return subprocess.check_output(["git", "rev-parse", "HEAD^{tree}"], text=True).strip()

def api(path):
    return subprocess.check_output(["gh", "api", path], timeout=20)

def matching(receipt, tree):
    return (receipt.get("schema") == SCHEMA and receipt.get("tree") == tree
            and receipt.get("backend") is True
            and type(receipt.get("frontend")) is bool
            and type(receipt.get("full_browser")) is bool
            and isinstance(receipt.get("browser_specs"), list)
            and all(isinstance(name, str) and Path(name).name == name
                    and name.endswith(".spec.ts") for name in receipt["browser_specs"])
            and (receipt["frontend"] or not receipt["browser_specs"])
            and (not receipt["full_browser"] or receipt["frontend"]))

def find_evidence(tree, repo):
    deadline = time.monotonic() + 20
    artifacts = json.loads(api(
        f"repos/{repo}/actions/artifacts?name={ARTIFACT}&per_page=100"))["artifacts"]
    if not artifacts:
        return None, None
    # Only these workflows can supply evidence. No arbitrary check names/caches.
    for workflow in ("docker.yml", "ci.yml"):
        runs = json.loads(api(
            f"repos/{repo}/actions/workflows/{workflow}/runs?status=success&per_page=15"))
        for run in runs["workflow_runs"]:
            if time.monotonic() > deadline:
                return None, None
            if run["event"] not in ("pull_request", "push", "workflow_dispatch"):
                continue
            if run.get("head_repository", {}).get("full_name") != repo:
                continue
            # head_repository identifies the PR source, including after merge
            # (GitHub then returns an empty pull_requests array). Forks are
            # excluded by the repository identity check above.
            for artifact in artifacts:
                if artifact["workflow_run"]["id"] != run["id"]:
                    continue
                if artifact["name"] != ARTIFACT or artifact["expired"]:
                    continue
                if artifact["size_in_bytes"] > 65536:
                    continue
                archive = api(f"repos/{repo}/actions/artifacts/{artifact['id']}/zip")
                with zipfile.ZipFile(io.BytesIO(archive)) as bundle:
                    entry = bundle.getinfo("source-verification.json")
                    if entry.file_size > 4096:
                        continue
                    receipt = json.loads(bundle.read(entry))
                if matching(receipt, tree):
                    # Prefer complete evidence; partial PR evidence still saves
                    # race/vet/unit checks while the release fills browser coverage.
                    return receipt, run["html_url"]
    return None, None

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("mode", choices=("record", "find", "browser"))
    args = parser.parse_args()
    tree = git_tree()
    if args.mode == "browser":
        verified = json.loads(os.environ.get("VERIFIED_BROWSER_SPECS", "[]") or "[]")
        remaining = [path.name for path in sorted((ROOT / "web/e2e").glob("*.spec.ts"))
                     if path.name not in verified]
        if remaining:
            subprocess.run(["npm", "run", "test:e2e", "--", *remaining],
                           cwd=ROOT / "web", check=True)
        else:
            print("All browser specs already verified for this source tree")
        return
    if args.mode == "record":
        frontend = os.environ.get("VERIFIED_FRONTEND") == "true"
        full = frontend and (os.environ.get("GITHUB_EVENT_NAME") != "pull_request"
                             or os.environ.get("SENTE_FULL_VERIFICATION") == "true")
        if full:
            browser_specs = sorted(path.name for path in (ROOT / "web/e2e").glob("*.spec.ts"))
        elif frontend:
            suites = json.loads((ROOT / "scripts/browser-suites.json").read_text())
            changed = json.loads(os.environ.get("E2E_CHANGED_SPECS", "[]"))
            browser_specs = sorted(set(suites["pr"]) | {
                name for name in changed if (ROOT / "web/e2e" / name).is_file()})
        else:
            browser_specs = []
        Path("work/test-audit").mkdir(parents=True, exist_ok=True)
        Path("work/test-audit/source-verification.json").write_text(json.dumps({
            "schema": SCHEMA, "tree": tree, "backend": True,
            "frontend": frontend, "full_browser": full, "browser_specs": browser_specs,
        }) + "\n")
        return
    receipt, url = None, None
    try:
        receipt, url = find_evidence(tree, os.environ["GITHUB_REPOSITORY"])
    except (KeyError, ValueError, OSError, subprocess.SubprocessError,
            zipfile.BadZipFile) as error:
        print(f"Evidence unavailable ({type(error).__name__}); running source checks")
    values = {
        "backend": bool(receipt),
        "frontend": bool(receipt and receipt["frontend"]),
        "full_browser": bool(receipt and receipt["full_browser"]),
    }
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        for key, value in values.items():
            output.write(f"{key}={str(value).lower()}\n")
        output.write("browser_specs=" + json.dumps(receipt["browser_specs"] if receipt else []) + "\n")
    message = f"Reusing source checks for tree {tree}: {url}" if receipt else f"No successful evidence for tree {tree}; running source checks"
    print(message)
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as summary:
            summary.write(message + "\n")

if __name__ == "__main__":
    main()
