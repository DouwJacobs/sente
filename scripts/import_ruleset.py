#!/usr/bin/env python3
"""Delegate ruleset operations to the application's validated, audited CLI."""
import argparse
import os
from pathlib import Path
import subprocess
import sys

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--db", required=True, help="Explicit destination/source SQLite database")
    parser.add_argument("--user", required=True, help="Existing administrator with budget membership")
    parser.add_argument("--executable", default=os.environ.get("FINANCE_EXECUTABLE", "bin/finance"))
    parser.add_argument("file", help="Input JSON file, or - for stdin")
    args = parser.parse_args()
    if not Path(args.db).is_file():
        parser.error("The database must already exist; initialize it with the application first.")
    env = dict(os.environ, DATABASE_PATH=str(Path(args.db).resolve()), FINANCE_RULESET_USER=args.user)
    try:
        return subprocess.run([str(Path(args.executable).resolve()), "import-ruleset", args.file], env=env).returncode
    except OSError as exc:
        parser.error("Build the application CLI first: " + str(exc))
if __name__ == "__main__":
    sys.exit(main())
