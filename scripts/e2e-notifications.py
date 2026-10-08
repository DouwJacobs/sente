"""Optional synthetic inbox fixture; no production data or producer is used."""
import time

def seed(db):
    now = int(time.time())
    def add(title, kind="system", source=0, recipient=1, accounts=(), dismissible=1):
        alert_type = {"budget": "budget_threshold", "account": "unusual_spending", "transaction": "unusual_spending"}.get(kind, "system")
        receipt = db.execute("INSERT INTO notification_receipts(user_id,type,dedupe_hash,event_hash,created_at) VALUES(?,?,?,?,?)", (recipient, alert_type, title, "synthetic", now)).lastrowid
        item = db.execute("INSERT INTO notifications(receipt_id,user_id,type,severity,title,message,source_kind,source_id,occurred_at,created_at,dismissible) VALUES(?,?,?,'info',?,?,?,?,?,?,?)", (receipt, recipient, alert_type, title, "Synthetic message with a long explanation " + "readable content " * 20, kind, source, now, now, dismissible)).lastrowid
        for account in accounts:
            db.execute("INSERT INTO notification_accounts VALUES(?,?)", (item, account))
    for number in range(22):
        add(f"Synthetic message {number}")
    add("Another user's private message", recipient=2)
    add("Inaccessible private details", "account", 999, accounts=(999,))
    add("Budget message", "budget", 1, accounts=(1,))
    add("Account message", "account", 1, accounts=(1,))
    add("Transaction message", "transaction", 1, accounts=(1,))
    add("System message cannot dismiss", dismissible=0)
