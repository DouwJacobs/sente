"""Safety and financial invariants for the publishable synthetic household."""
import hashlib
from pathlib import Path
import sqlite3
import tempfile
import unittest
from unittest.mock import patch
import demo

class DemoTests(unittest.TestCase):
    def test_new_database_and_overwrite_refusal(self):
        with tempfile.TemporaryDirectory(prefix="sente-demo-test-") as temporary:
            target = Path(temporary) / "demo/finance.sqlite"
            with patch.object(demo, "TARGET", target):
                demo.seed()
                with sqlite3.connect(target) as db:
                    self.assertEqual(db.execute("PRAGMA integrity_check").fetchone()[0], "ok")
                    self.assertEqual(db.execute("PRAGMA foreign_key_check").fetchall(), [])
                    self.assertEqual(db.execute("SELECT COUNT(*) FROM transactions").fetchone()[0], 19)
                    self.assertEqual(db.execute("SELECT COUNT(*) FROM transactions t WHERE amount_cents != (SELECT SUM(amount_cents) FROM allocations WHERE transaction_id=t.id)").fetchone()[0], 0)
                    self.assertEqual(db.execute("SELECT SUM(amount_cents) FROM transactions WHERE is_transfer=1").fetchone()[0], 0)
                    self.assertEqual(db.execute("SELECT COUNT(*) FROM transactions WHERE review_state='pending_review'").fetchone()[0], 1)
                    self.assertEqual(db.execute("SELECT COUNT(*) FROM fnb_connections").fetchone()[0], 0)
                    self.assertEqual(db.execute("SELECT COUNT(*) FROM mcp_tokens").fetchone()[0], 0)
                    self.assertEqual(db.execute("SELECT COUNT(*) FROM grants WHERE account_id=3 AND user_id=1 AND role='editor'").fetchone()[0], 1)
                    self.assertEqual(db.execute("SELECT COUNT(*) FROM targets t WHERE amount_cents != (SELECT SUM(amount_cents) FROM group_targets WHERE period_id=t.period_id AND category_id=t.category_id)").fetchone()[0], 0)
                before = hashlib.sha256(target.read_bytes()).digest()
                with self.assertRaises(FileExistsError):
                    demo.seed()
                self.assertEqual(hashlib.sha256(target.read_bytes()).digest(), before)

if __name__ == "__main__":
    unittest.main()
