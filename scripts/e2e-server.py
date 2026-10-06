"""Run an isolated service for browser tests. No production database is touched."""
from pathlib import Path
import os, subprocess, sqlite3, tempfile, signal, sys
root = Path(__file__).resolve().parents[1]
go = os.environ.get("GO", str(root / "work/toolchain/go/bin/go"))
if not Path(go).exists():
    go = "go"
work = Path(tempfile.mkdtemp(prefix="finance-e2e-"))
binary = work / "finance"
subprocess.run([go, "build", "-o", str(binary), "./cmd/finance"], cwd=root, check=True)
port = os.environ.get("E2E_PORT", "18080")
env = {**os.environ, "DATABASE_PATH": str(work/"finance.sqlite"), "BACKUP_DIR": str(work/"backups"),
       "PUBLIC_URL": os.environ.get("E2E_PUBLIC_URL", f"http://127.0.0.1:{port}"), "PORT": port, "LISTEN_ADDRESS": "127.0.0.1", "STATIC_DIR": str(root/"web/dist"),
       "FINANCE_PASSWORD": "synthetic-browser-password"}
if os.environ.get("E2E_EMPTY") != "1":
    subprocess.run([str(binary), "create-admin", "demo"], env=env, check=True)
    db = sqlite3.connect(work/"finance.sqlite")
    db.executescript("""
    DELETE FROM builtin_rules;
    DELETE FROM categories;
    INSERT INTO users(id,username,password,admin,budget_member) VALUES(2,'partner','unused',0,1),(3,'restricted','unused',0,0);
    INSERT INTO accounts(id,name,bank_id,household,balance_cents,balance_date) VALUES
    (1,'Everyday account','12345678901',1,1452500,'2026-10-26'),
    (2,'Savings','22222222222',1,3200000,'2026-10-26'),
    (3,'Private account','33333333333',0,120000,'2026-10-26');
    INSERT INTO grants VALUES(1,1,'editor'),(1,2,'editor'),(1,3,'editor');
    INSERT INTO categories(id,name,group_name,kind) VALUES
    (1,'Groceries','Living costs','expense'),(2,'Transport','Living costs','expense'),
    (3,'Eating out','Everyday','expense'),(4,'Salary','Income','income');
    DELETE FROM periods;
    INSERT INTO periods(id,name,start_date,end_date) VALUES(1,'October 2026','2026-10-20','2026-11-19');
    INSERT INTO targets VALUES(1,1,450000),(1,2,200000),(1,3,120000);
    INSERT INTO group_targets(period_id,category_id,amount_cents) VALUES(1,1,450000),(1,2,200000),(1,3,120000);
    INSERT INTO budget_groups VALUES(1,NULL);
    INSERT INTO rules(user_id,account_id,pattern,category_id,priority) VALUES(1,1,'Market',1,0);
    """)
    rows=[("2026-10-20",3500000,"Salary payment",4,"approved"),
          ("2026-10-21",-84650,"Market groceries",1,"approved"),
          ("2026-10-22",-62000,"Fuel station",2,"approved"),
          ("2026-10-24",-28500,"Corner restaurant",3,"approved"),
          ("2026-10-25",-125000,"Synthetic long description for a purchase that needs categorization and review on mobile",None,"pending_review")]
    for date,amount,description,category,state in rows:
        cur=db.execute("INSERT INTO transactions(account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance,review_state,period_id) VALUES(1,?,?,?,?,?,?,'{}',?,1)",(date,amount,description,date,amount,description,state))
        db.execute("INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(?,?,?)",(cur.lastrowid,category,amount))
    if os.environ.get("E2E_EMPTY_ACCOUNTS") == "1":
        for table in ("allocations", "transactions", "rules", "grants", "targets", "categories", "accounts"):
            db.execute("DELETE FROM " + table)
    db.commit(); db.close()
proc=subprocess.Popen([str(binary),"serve"],env=env)
def stop(*_):
    proc.terminate()
    try: proc.wait(timeout=15)
    except subprocess.TimeoutExpired: proc.kill(); proc.wait()
    import shutil
    shutil.rmtree(work)
    sys.exit(0)
signal.signal(signal.SIGTERM,stop); signal.signal(signal.SIGINT,stop)
try: sys.exit(proc.wait())
finally:
    if proc.poll() is None: stop()
