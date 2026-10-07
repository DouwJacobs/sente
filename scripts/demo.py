#!/usr/bin/env python3
"""Create only a new, isolated synthetic database; optionally run hot reload."""
from pathlib import Path
import argparse
import os
import shutil
import sqlite3
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
TARGET = ROOT / "data/demo/finance.sqlite"
PASSWORD = "sente-demo-password"

def seed():
    TARGET.parent.mkdir(parents=True, exist_ok=True)
    # Reserve exclusively, including against another concurrent seed. No reset flag.
    fd = os.open(TARGET, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    os.close(fd)
    try:
        go = os.environ.get("GO", "go")
        if not shutil.which(go):
            go = str(ROOT / "work/toolchain/go/bin/go")
        with tempfile.TemporaryDirectory(prefix="sente-demo-") as temporary:
            binary = Path(temporary) / "finance"
            subprocess.run([go, "build", "-o", str(binary), "./cmd/finance"], cwd=ROOT, check=True)
            env = {**os.environ, "DATABASE_PATH": str(TARGET),
                   "BACKUP_DIR": str(ROOT / "backups/demo"),
                   "PUBLIC_URL": "http://127.0.0.1:5174", "FINANCE_PASSWORD": PASSWORD}
            subprocess.run([str(binary), "create-admin", "demo"], env=env, check=True)
            subprocess.run([str(binary), "import-ruleset", str(ROOT / "internal/app/configuration/starter.json")], env={**env, "FINANCE_RULESET_USER":"demo"}, check=True)
        with sqlite3.connect(TARGET) as db:
            db.execute("PRAGMA foreign_keys=ON")
            db.executescript((ROOT / "fixtures/demo/household.sql").read_text())
            if db.execute("PRAGMA foreign_key_check").fetchall():
                raise RuntimeError("Demo foreign-key validation failed")
            if db.execute("PRAGMA integrity_check").fetchone()[0] != "ok":
                raise RuntimeError("Demo database integrity validation failed")
        print(f"Synthetic household ready: {TARGET}")
        print(f"Sign in as demo with password {PASSWORD}")
    except BaseException:
        # Only files belonging to our newly reserved database may be removed.
        for suffix in ("", "-wal", "-shm", ".lock"):
            Path(str(TARGET) + suffix).unlink(missing_ok=True)
        raise

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--serve", action="store_true", help="Run the existing demo with hot reload")
    args = parser.parse_args()
    if args.serve:
        if not TARGET.is_file():
            raise RuntimeError("Run make demo first")
        env = {**os.environ, "DATABASE_PATH": str(TARGET),
               "BACKUP_DIR": str(ROOT / "backups/demo"), "DEV_PORT": "5174",
               "DEV_API_PORT": "8082", "DEV_PUBLIC_URL": "http://127.0.0.1:5174"}
        subprocess.run([os.sys.executable, str(ROOT / "scripts/dev.py")], cwd=ROOT, env=env, check=True)
    else:
        seed()

if __name__ == "__main__":
    main()
