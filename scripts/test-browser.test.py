"""Fixture isolation and smoke selection boundaries for the browser runner."""
import importlib.util
from pathlib import Path
import unittest

spec=importlib.util.spec_from_file_location("browser_runner",Path(__file__).with_name("test-browser.py"))
runner=importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)

class BrowserSuiteTests(unittest.TestCase):
    def test_unknown_specs_preserve_all_fixtures(self):
        self.assertEqual(runner.fixture_offsets("new-feature.spec.ts"),[0,1,2])

    def test_empty_installation_specs_keep_distinct_services(self):
        self.assertEqual(runner.fixture_offsets("onboarding.spec.ts"),[0,1])
        self.assertEqual(runner.fixture_offsets("classification-defaults.spec.ts"),[0,2])

    def test_reviewed_primary_only_specs_do_not_spawn_unused_empty_services(self):
        self.assertEqual(runner.fixture_offsets("pwa.spec.ts"),[0])
        self.assertEqual(runner.fixture_offsets("settings.spec.ts"),[0])

    def test_pr_suite_has_existing_unique_specs_and_critical_workflows(self):
        paths=runner.SUITES["pr"]
        self.assertEqual(len(paths),len(set(paths)))
        for name in paths:
            self.assertTrue((runner.WEB/"e2e"/name).is_file(),name)
        critical={"onboarding.spec.ts","seen.spec.ts","mcp.spec.ts","configuration.spec.ts",
                  "transaction-editor.spec.ts","transaction-filters.spec.ts","user-security.spec.ts",
                  "notification-delivery.spec.ts","pwa.spec.ts","responsive-geometry.spec.ts"}
        self.assertTrue(critical.issubset(paths))

    def test_changed_specs_join_pr_suite_without_duplicates(self):
        selected=runner.pr_specs(["about.spec.ts","seen.spec.ts","removed.spec.ts"])
        self.assertIn("about.spec.ts",selected)
        self.assertEqual(selected.count("seen.spec.ts"),1)
        self.assertNotIn("removed.spec.ts",selected)
        with self.assertRaises(ValueError):
            runner.pr_specs(["../outside.spec.ts"])
        with self.assertRaises(ValueError):
            runner.pr_specs("about.spec.ts")

    def test_fixture_registry_has_valid_offsets_and_no_stale_specs(self):
        for name,offsets in runner.SUITES["fixture_offsets"].items():
            self.assertTrue((runner.WEB/"e2e"/name).is_file(),name)
            self.assertEqual(sorted(set(offsets)),offsets)
            self.assertEqual(offsets[0],0)
            self.assertTrue(set(offsets).issubset({0,1,2}))

if __name__=="__main__":
    unittest.main()
