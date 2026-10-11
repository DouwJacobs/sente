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
                            "NOTICE", "scripts/test_demo.py", "scripts/demo.py", "scripts/ci-scope.test.py"})
    return any(not backend_only(path) for path in paths)

def needs_source(paths):
    return any(not (path.startswith("docs/") or path in
                    {"README.md", "AGENTS.md", "LICENSE", "NOTICE"}) for path in paths)

UI_AUDITS = {"audit-followup.spec.ts", "dashboard-buckets.spec.ts", "ui-backlog.spec.ts"}
RELATED_BROWSER_SPECS = {
    "web/src/Rules.tsx": {"audit-followup.spec.ts"},
    "web/src/features/categories/Categories.tsx": {"audit-followup.spec.ts"},
    "web/src/features/accounts/Accounts.tsx": {"ui-backlog.spec.ts"},
    "web/src/features/dashboard/Dashboard.tsx": {"dashboard-buckets.spec.ts"},
    "web/src/features/budgets/Budgets.tsx": {"dashboard-buckets.spec.ts"},
    "web/src/features/budgets/BudgetGroups.tsx": {"dashboard-buckets.spec.ts"},
    "web/src/ui.tsx": UI_AUDITS,
    "web/src/styles.css": UI_AUDITS,
    "web/src/finance-theme.css": UI_AUDITS,
    "web/src/App.tsx": UI_AUDITS,
}

def changed_browser_specs(paths):
    specs = {Path(path).name for path in paths
             if Path(path).parent == Path("web/e2e") and path.endswith(".spec.ts")}
    for path in paths:
        specs.update(RELATED_BROWSER_SPECS.get(path, set()))
    return sorted(specs)

def main():
    required = True
    source = True
    paths = []
    if os.environ.get("GITHUB_EVENT_NAME") == "pull_request" and os.environ.get("SENTE_FULL_VERIFICATION") != "true":
        try:
            event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
            pr = event["pull_request"]
            base, head = pr["base"]["sha"], pr["head"]["sha"]
            output = subprocess.check_output(
                ["git", "diff", "--name-only", "-z", "--no-renames", f"{base}...{head}"])
            paths = output.decode().rstrip("\0").split("\0") if output else []
            required = needs_frontend(paths)
            source = needs_source(paths)
        except (KeyError, OSError, ValueError, subprocess.CalledProcessError) as error:
            print(f"Scope detection unavailable; running frontend checks ({type(error).__name__})")
            paths = ["web/e2e/" + path.name for path in
                     (Path(__file__).resolve().parents[1] / "web/e2e").glob("*.spec.ts")]
    value = str(required).lower()
    print(f"Frontend checks: {value}")
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write(f"source={str(source).lower()}\n")
        output.write(f"frontend={value}\n")
        output.write("browser_specs=" + json.dumps(changed_browser_specs(paths)) + "\n")

if __name__ == "__main__":
    main()
