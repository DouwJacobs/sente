"""Gate regressions: mixed changes, renames, uncertainty and main/manual runs."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("ci_scope", Path(__file__).with_name("ci-scope.py"))
scope = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scope)

class ScopeTests(unittest.TestCase):
    def run_scope(self, event_name="pull_request", changed=b"", error=None):
        with tempfile.TemporaryDirectory() as directory:
            event, output = Path(directory)/"event.json", Path(directory)/"output"
            event.write_text(json.dumps({"pull_request":{"base":{"sha":"base"},"head":{"sha":"head"}}}))
            with patch.dict(os.environ, {"GITHUB_EVENT_NAME":event_name,
                           "GITHUB_EVENT_PATH":str(event), "GITHUB_OUTPUT":str(output)}):
                with patch.object(subprocess, "check_output", return_value=changed, side_effect=error) as diff:
                    scope.main()
                    if event_name == "pull_request" and not error:
                        diff.assert_called_once_with(["git","diff","--name-only","-z","--no-renames","base...head"])
            return next(line+"\n" for line in output.read_text().splitlines() if line.startswith("frontend="))

    def test_documentation_only_needs_no_source_checks(self):
        self.assertFalse(scope.needs_source(["docs/PLAN.md", "README.md", "NOTICE"]))
        self.assertTrue(scope.needs_source(["docs/PLAN.md", "internal/app/security.go"]))
        self.assertTrue(scope.needs_source([".github/workflows/ci.yml"]))

    def test_backend_and_docs_only_skip_frontend(self):
        self.assertEqual(self.run_scope(changed=b"internal/app/security.go\\0docs/PLAN.md\\0".replace(b"\\0",b"\0")), "frontend=false\n")

    def test_mixed_backend_and_frontend_runs_frontend(self):
        self.assertEqual(self.run_scope(changed=b"internal/app/security.go\0web/src/App.tsx\0"), "frontend=true\n")

    def test_shared_inputs_and_unknown_paths_run_frontend(self):
        for path in ["web/e2e/a.spec.ts", "connectors/fnb/owner/transactions.mjs",
                     "scripts/test-browser.py", ".github/workflows/ci.yml",
                     "Makefile", "Dockerfile", "new-build-input"]:
            with self.subTest(path=path):
                self.assertTrue(scope.needs_frontend([path]))

    def test_deleted_or_renamed_frontend_path_runs_frontend(self):
        self.assertEqual(self.run_scope(changed=b"web/src/old.ts\0internal/new.ts\0"), "frontend=true\n")

    def test_uncertain_diff_runs_frontend(self):
        self.assertEqual(self.run_scope(error=subprocess.CalledProcessError(1, ["git"])), "frontend=true\n")

    def test_changed_browser_specs_use_only_root_spec_names(self):
        self.assertEqual(scope.changed_browser_specs(["web/e2e/about.spec.ts",
                         "web/e2e/about.spec.ts","web/src/shared/buildInfo.ts","docs/not.spec.ts"]),
                         ["about.spec.ts"])

    def test_related_ui_specs_join_only_affected_prs(self):
        self.assertEqual(scope.changed_browser_specs(["web/src/Rules.tsx"]),
                         ["audit-followup.spec.ts"])
        self.assertEqual(scope.changed_browser_specs(["web/src/features/budgets/Budgets.tsx"]),
                         ["dashboard-buckets.spec.ts"])
        self.assertEqual(scope.changed_browser_specs(["web/src/features/accounts/Accounts.tsx"]),
                         ["ui-backlog.spec.ts"])
        self.assertEqual(set(scope.changed_browser_specs(["web/src/ui.tsx"])), scope.UI_AUDITS)
        self.assertEqual(scope.changed_browser_specs(["internal/app/accounts.go",
                          "web/src/shared/buildInfo.ts", "docs/UI.md"]), [])
        self.assertEqual(scope.changed_browser_specs(["web/src/Rules.tsx",
                          "web/e2e/audit-followup.spec.ts"]), ["audit-followup.spec.ts"])

    def test_release_call_forces_full_source_verification(self):
        with tempfile.TemporaryDirectory() as directory:
            output=Path(directory)/'output'
            with patch.dict(os.environ, {'GITHUB_EVENT_NAME':'pull_request',
                        'SENTE_FULL_VERIFICATION':'true','GITHUB_OUTPUT':str(output)}):
                with patch.object(subprocess,'check_output') as diff:
                    scope.main()
                    diff.assert_not_called()
            self.assertIn('frontend=true',output.read_text())

    def test_main_and_manual_runs_always_run_frontend(self):
        for event in ["push", "workflow_dispatch"]:
            with self.subTest(event=event):
                self.assertEqual(self.run_scope(event), "frontend=true\n")

if __name__ == "__main__":
    unittest.main()
