"""Evidence must match exact contents and come from successful trusted runs."""
import importlib.util
import io
import json
import os
import tempfile
from pathlib import Path
import unittest
from unittest.mock import patch
import zipfile

spec = importlib.util.spec_from_file_location("evidence", Path(__file__).with_name("ci-evidence.py"))
evidence = importlib.util.module_from_spec(spec)
spec.loader.exec_module(evidence)

class EvidenceTests(unittest.TestCase):
    def receipt(self, **values):
        return {"schema": 1, "tree": "tree", "backend": True,
                "frontend": True, "full_browser": False, "browser_specs": ["a.spec.ts"], **values}

    def test_exact_tree_and_completed_gates(self):
        self.assertTrue(evidence.matching(self.receipt(), "tree"))
        for values in [{"tree": "other"}, {"schema": 0}, {"backend": False},
                       {"frontend": "true"}, {"browser_specs": ["../unsafe.spec.ts"]}, {"full_browser": "false"},
                       {"frontend": False, "full_browser": True}]:
            with self.subTest(values=values):
                self.assertFalse(evidence.matching(self.receipt(**values), "tree"))

    def test_lookup_and_fork_exclusion(self):
        for fork in (False, True):
            with self.subTest(fork=fork):
                bundle = io.BytesIO()
                with zipfile.ZipFile(bundle, "w") as archive:
                    archive.writestr("source-verification.json", json.dumps(self.receipt()))
                run = {"id": 42, "event": "pull_request", "html_url": "run-url",
                       "head_repository": {"full_name": "fork/repo" if fork else "owner/repo"},
                       "pull_requests": []}
                def api(path):
                    if "/workflows/" in path:
                        return json.dumps({"workflow_runs": [run]}).encode()
                    if "/actions/artifacts?" in path:
                        return json.dumps({"artifacts": [{"id": 5, "name": evidence.ARTIFACT,
                                           "expired": False, "size_in_bytes": 500, "workflow_run": {"id": 42}}]}).encode()
                    return bundle.getvalue()
                with patch.object(evidence, "api", side_effect=api):
                    receipt, url = evidence.find_evidence("tree", "owner/repo")
                self.assertEqual(bool(receipt), not fork)
                self.assertEqual(url, None if fork else "run-url")

    def test_changed_tree_does_not_reuse(self):
        with patch.object(evidence, "api", return_value=b'{"artifacts": []}'):
            self.assertEqual(evidence.find_evidence("different", "owner/repo"), (None, None))


    def test_record_pr_and_complete_coverage(self):
        for event, full, frontend in [("pull_request", "false", "true"),
                                      ("push", "true", "true"),
                                      ("pull_request", "false", "false")]:
            with self.subTest(event=event, frontend=frontend), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                (root / "web/e2e").mkdir(parents=True)
                (root / "scripts").mkdir()
                for name in ["a.spec.ts", "b.spec.ts", "c.spec.ts"]:
                    (root / "web/e2e" / name).touch()
                (root / "scripts/browser-suites.json").write_text('{"pr":["a.spec.ts"]}')
                previous = Path.cwd()
                try:
                    os.chdir(root)
                    with patch.object(evidence, "ROOT", root), patch.object(evidence, "git_tree", return_value="tree"), patch(
                            "sys.argv", ["ci-evidence.py", "record"]), patch.dict(os.environ, {
                            "GITHUB_EVENT_NAME": event, "SENTE_FULL_VERIFICATION": full,
                            "VERIFIED_FRONTEND": frontend, "E2E_CHANGED_SPECS": '["b.spec.ts"]'}):
                        evidence.main()
                    receipt = json.loads((root / "work/test-audit/source-verification.json").read_text())
                    expected = [] if frontend == "false" else (
                        ["a.spec.ts", "b.spec.ts", "c.spec.ts"] if full == "true"
                        else ["a.spec.ts", "b.spec.ts"])
                    self.assertEqual(receipt["browser_specs"], expected)
                    self.assertTrue(evidence.matching(receipt, "tree"))
                finally:
                    os.chdir(previous)

    def test_release_runs_only_remaining_specs(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "web/e2e").mkdir(parents=True)
            for name in ["a.spec.ts", "b.spec.ts"]:
                (root / "web/e2e" / name).touch()
            for covered in [['a.spec.ts'], ['a.spec.ts', 'b.spec.ts']]:
                with patch.object(evidence, "ROOT", root), patch.object(evidence, "git_tree", return_value="tree"), patch(
                        "sys.argv", ["ci-evidence.py", "browser"]), patch.dict(
                        os.environ, {"VERIFIED_BROWSER_SPECS": json.dumps(covered)}), patch.object(
                        evidence.subprocess, "run") as runner:
                    evidence.main()
                    if len(covered) == 1:
                        runner.assert_called_once_with(
                            ["npm", "run", "test:e2e", "--", "b.spec.ts"], cwd=root / "web", check=True)
                    else:
                        runner.assert_not_called()

    def test_lookup_errors_fail_closed(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(evidence, "git_tree", return_value="tree"), patch(
                "sys.argv", ["ci-evidence.py", "find"]), patch.dict(
                os.environ, {"GITHUB_REPOSITORY": "owner/repo", "GITHUB_OUTPUT": str(Path(tmp) / "output"),
                             "GITHUB_STEP_SUMMARY": str(Path(tmp) / "summary")}), patch.object(
                evidence, "find_evidence", side_effect=ValueError("bad artifact")):
            evidence.main()
            output = (Path(tmp) / "output").read_text()
            self.assertIn("backend=false", output)
            self.assertIn("frontend=false", output)
            self.assertIn("full_browser=false", output)
            self.assertIn("browser_specs=[]", output)

    def test_nonmatching_and_expired_artifacts(self):
        for expired in (False, True):
            bundle = io.BytesIO()
            with zipfile.ZipFile(bundle, "w") as archive:
                archive.writestr("source-verification.json", json.dumps(self.receipt(tree="different")))
            def api(path):
                if "/workflows/" in path:
                    return json.dumps({"workflow_runs": [{"id": 42, "event": "push",
                        "html_url": "url", "head_repository": {"full_name": "owner/repo"}}]}).encode()
                if "/actions/artifacts?" in path:
                    return json.dumps({"artifacts": [{"id": 5, "name": evidence.ARTIFACT,
                        "expired": expired, "size_in_bytes": 500, "workflow_run": {"id": 42}}]}).encode()
                return bundle.getvalue()
            with patch.object(evidence, "api", side_effect=api):
                self.assertEqual(evidence.find_evidence("tree", "owner/repo"), (None, None))

if __name__ == "__main__":
    unittest.main()
