#!/usr/bin/env python3
"""Conservative frontend PR gate; unknown build inputs run the frontend suite."""
import json
import os
from pathlib import Path
import subprocess

def needs_frontend(paths):
    # Renames include both names; unknown paths run full frontend verification.
    def backend_only(path):
        return (path.startswith(("internal/", "cmd/", "docs/", "fixtures/demo/"))
                or path in {"go.mod", "go.sum", "AGENTS.md", "README.md", "LICENSE",
                            "scripts/test_demo.py", "scripts/demo.py", "scripts/ci-scope.test.py"})
    return any(not backend_only(path) for path in paths)

def changed_browser_specs(paths):
    return sorted({Path(path).name for path in paths
                   if Path(path).parent == Path("web/e2e") and path.endswith(".spec.ts")})

def main():
    required = True
    paths = []
    if os.environ.get("GITHUB_EVENT_NAME") == "pull_request":
        try:
            event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
            pr = event["pull_request"]
            base, head = pr["base"]["sha"], pr["head"]["sha"]
            output = subprocess.check_output(
                ["git", "diff", "--name-only", "-z", "--no-renames", f"{base}...{head}"])
            paths = output.decode().rstrip("\0").split("\0") if output else []
            required = needs_frontend(paths)
        except (KeyError, OSError, ValueError, subprocess.CalledProcessError) as error:
            print(f"Scope detection unavailable; running frontend checks ({type(error).__name__})")
            paths = ["web/e2e/" + path.name for path in
                     (Path(__file__).resolve().parents[1] / "web/e2e").glob("*.spec.ts")]
    value = str(required).lower()
    print(f"Frontend checks: {value}")
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write(f"frontend={value}\n")
        output.write("browser_specs=" + json.dumps(changed_browser_specs(paths)) + "\n")

if __name__ == "__main__":
    main()
