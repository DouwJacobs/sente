-- Frozen historical schema from 13e9ed8; synthetic data is supplied by tests.
CREATE TABLE IF NOT EXISTS workspace_branding (id INTEGER PRIMARY KEY CHECK(id=1), display_name TEXT NOT NULL DEFAULT 'Household', version INTEGER NOT NULL DEFAULT 1);
INSERT OR IGNORE INTO workspace_branding(id) VALUES(1);
CREATE TABLE IF NOT EXISTS migrations(version INTEGER PRIMARY KEY);
CREATE TABLE IF NOT EXISTS users(id INTEGER PRIMARY KEY,username TEXT NOT NULL UNIQUE,password TEXT NOT NULL,admin INTEGER NOT NULL DEFAULT 0,budget_member INTEGER NOT NULL DEFAULT 1,disabled INTEGER NOT NULL DEFAULT 0,version INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS sessions(token TEXT PRIMARY KEY,user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,csrf TEXT NOT NULL,expires_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS accounts(id INTEGER PRIMARY KEY,name TEXT NOT NULL,bank_id TEXT NOT NULL UNIQUE,household INTEGER NOT NULL DEFAULT 0,version INTEGER NOT NULL DEFAULT 1,balance_cents INTEGER,balance_date TEXT,sync_hidden INTEGER NOT NULL DEFAULT 0);
CREATE TABLE IF NOT EXISTS grants(user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,account_id INTEGER REFERENCES accounts(id) ON DELETE CASCADE,role TEXT NOT NULL CHECK(role IN ('viewer','editor')),PRIMARY KEY(user_id,account_id));
CREATE TABLE IF NOT EXISTS categories(id INTEGER PRIMARY KEY,name TEXT NOT NULL,group_name TEXT NOT NULL,kind TEXT NOT NULL CHECK(kind IN ('expense','income')),spending_group_id INTEGER REFERENCES spending_groups(id),archived INTEGER NOT NULL DEFAULT 0,version INTEGER NOT NULL DEFAULT 1,UNIQUE(group_name,name));
CREATE TABLE IF NOT EXISTS rules(id INTEGER PRIMARY KEY,user_id INTEGER NOT NULL REFERENCES users(id),account_id INTEGER NOT NULL REFERENCES accounts(id),pattern TEXT NOT NULL,category_id INTEGER NOT NULL REFERENCES categories(id),priority INTEGER NOT NULL DEFAULT 0,direction TEXT NOT NULL DEFAULT 'any' CHECK(direction IN ('any','debit','credit')),spending_group_id INTEGER REFERENCES spending_groups(id),enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),version INTEGER NOT NULL DEFAULT 1);
CREATE TABLE IF NOT EXISTS settings(id INTEGER PRIMARY KEY CHECK(id=1),start_day INTEGER NOT NULL CHECK(start_day BETWEEN 1 AND 31));
INSERT OR IGNORE INTO settings VALUES(1,20);
CREATE TABLE IF NOT EXISTS periods(id INTEGER PRIMARY KEY,name TEXT NOT NULL,start_date TEXT NOT NULL,end_date TEXT NOT NULL,version INTEGER NOT NULL DEFAULT 1,CHECK(start_date<=end_date));
CREATE TABLE IF NOT EXISTS targets(period_id INTEGER REFERENCES periods(id) ON DELETE CASCADE,category_id INTEGER REFERENCES categories(id),amount_cents INTEGER NOT NULL CHECK(amount_cents>=0),PRIMARY KEY(period_id,category_id));
CREATE TABLE IF NOT EXISTS imports(id INTEGER PRIMARY KEY,user_id INTEGER NOT NULL REFERENCES users(id),account_id INTEGER NOT NULL REFERENCES accounts(id),name TEXT NOT NULL,hash TEXT NOT NULL,format TEXT NOT NULL,committed_at TEXT,status TEXT NOT NULL DEFAULT 'staged',data TEXT NOT NULL,created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE UNIQUE INDEX IF NOT EXISTS completed_file ON imports(account_id,hash) WHERE status='committed';
CREATE TABLE IF NOT EXISTS spending_groups(id INTEGER PRIMARY KEY,name TEXT NOT NULL COLLATE NOCASE UNIQUE,color TEXT NOT NULL,version INTEGER NOT NULL DEFAULT 1);
CREATE TABLE IF NOT EXISTS merchants(id INTEGER PRIMARY KEY,account_id INTEGER REFERENCES accounts(id),name TEXT NOT NULL COLLATE NOCASE,logo_data TEXT NOT NULL DEFAULT '',category_id INTEGER REFERENCES categories(id),spending_group_id INTEGER REFERENCES spending_groups(id),version INTEGER NOT NULL DEFAULT 1,UNIQUE(account_id,name));
CREATE TABLE IF NOT EXISTS tags(id INTEGER PRIMARY KEY,account_id INTEGER NOT NULL REFERENCES accounts(id),name TEXT NOT NULL COLLATE NOCASE,UNIQUE(account_id,name));
CREATE TABLE IF NOT EXISTS transactions(id INTEGER PRIMARY KEY,account_id INTEGER NOT NULL REFERENCES accounts(id),date TEXT NOT NULL,amount_cents INTEGER NOT NULL,description TEXT NOT NULL,note TEXT NOT NULL DEFAULT '',merchant_id INTEGER REFERENCES merchants(id),fitid TEXT,source_date TEXT NOT NULL,source_amount INTEGER NOT NULL,source_description TEXT NOT NULL,source_key TEXT NOT NULL DEFAULT '',provenance TEXT NOT NULL,import_id INTEGER REFERENCES imports(id),review_state TEXT NOT NULL DEFAULT 'pending_review',reviewed_by INTEGER REFERENCES users(id),reviewed_at TEXT,version INTEGER NOT NULL DEFAULT 1,is_transfer INTEGER NOT NULL DEFAULT 0,spending_group_id INTEGER REFERENCES spending_groups(id),period_id INTEGER REFERENCES periods(id),assignment TEXT NOT NULL DEFAULT 'auto',created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE UNIQUE INDEX IF NOT EXISTS transaction_fitid ON transactions(account_id,fitid) WHERE fitid IS NOT NULL AND fitid<>'';
CREATE INDEX IF NOT EXISTS transaction_source_match ON transactions(account_id,source_key);
CREATE TABLE IF NOT EXISTS allocations(id INTEGER PRIMARY KEY,transaction_id INTEGER NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,category_id INTEGER REFERENCES categories(id),amount_cents INTEGER NOT NULL,note TEXT NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS transfer_links(left_id INTEGER NOT NULL UNIQUE REFERENCES transactions(id),right_id INTEGER NOT NULL UNIQUE REFERENCES transactions(id),CHECK(left_id<>right_id));
CREATE TABLE IF NOT EXISTS audit(id INTEGER PRIMARY KEY,user_id INTEGER REFERENCES users(id),account_id INTEGER REFERENCES accounts(id),entity TEXT NOT NULL,entity_id INTEGER,action TEXT NOT NULL,details TEXT NOT NULL,created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS backups(id INTEGER PRIMARY KEY,path TEXT NOT NULL,created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
INSERT OR IGNORE INTO migrations VALUES(1);

CREATE TABLE IF NOT EXISTS fnb_connections(user_id INTEGER PRIMARY KEY REFERENCES users(id),secret BLOB NOT NULL,interval_hours INTEGER NOT NULL DEFAULT 0,state TEXT NOT NULL DEFAULT 'ready',last_attempt TEXT,last_success TEXT,next_due INTEGER,last_error TEXT NOT NULL DEFAULT '',last_skipped INTEGER NOT NULL DEFAULT 0,debug_browser INTEGER NOT NULL DEFAULT 0,last_diagnostics TEXT NOT NULL DEFAULT '{}',version INTEGER NOT NULL DEFAULT 1);
CREATE TABLE IF NOT EXISTS fnb_discoveries(user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,bank_id TEXT NOT NULL,name TEXT NOT NULL,balance_cents INTEGER,balance_date TEXT,hidden INTEGER NOT NULL DEFAULT 0,account_id INTEGER REFERENCES accounts(id),PRIMARY KEY(user_id,bank_id));

CREATE TABLE IF NOT EXISTS network_settings(id INTEGER PRIMARY KEY CHECK(id=1),enabled INTEGER NOT NULL DEFAULT 0,public_url TEXT NOT NULL DEFAULT '',trusted_proxies TEXT NOT NULL DEFAULT '',version INTEGER NOT NULL DEFAULT 1);
INSERT OR IGNORE INTO network_settings(id) VALUES(1);

CREATE TABLE IF NOT EXISTS builtin_rules(id INTEGER PRIMARY KEY,pattern TEXT NOT NULL,category_id INTEGER NOT NULL REFERENCES categories(id),spending_group_id INTEGER REFERENCES spending_groups(id),direction TEXT NOT NULL CHECK(direction IN ('any','debit','credit')),priority INTEGER NOT NULL DEFAULT -1000,enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),version INTEGER NOT NULL DEFAULT 1);

CREATE INDEX IF NOT EXISTS allocation_transaction ON allocations(transaction_id);
CREATE TABLE IF NOT EXISTS transaction_seen(user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,transaction_id INTEGER NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,transaction_version INTEGER NOT NULL,seen_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,PRIMARY KEY(user_id,transaction_id));

CREATE TABLE IF NOT EXISTS mcp_tokens(id INTEGER PRIMARY KEY,user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,name TEXT NOT NULL,token_hash TEXT NOT NULL UNIQUE,
 client_id TEXT NOT NULL DEFAULT '',
 last_used_at INTEGER,permissions TEXT NOT NULL DEFAULT '{}',permission_version INTEGER NOT NULL DEFAULT 1,can_write INTEGER NOT NULL DEFAULT 0,expires_at INTEGER NOT NULL,created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS mcp_proposals(id TEXT PRIMARY KEY,token_id INTEGER NOT NULL REFERENCES mcp_tokens(id) ON DELETE CASCADE,user_id INTEGER NOT NULL REFERENCES users(id),operation TEXT NOT NULL,payload TEXT NOT NULL,status TEXT NOT NULL DEFAULT 'pending',result TEXT NOT NULL DEFAULT '{}',expires_at INTEGER NOT NULL,created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);

CREATE TABLE IF NOT EXISTS mcp_oauth_clients(id TEXT PRIMARY KEY,name TEXT NOT NULL,redirect_uris TEXT NOT NULL,expires_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS mcp_oauth_requests(id TEXT PRIMARY KEY,client_id TEXT NOT NULL REFERENCES mcp_oauth_clients(id) ON DELETE CASCADE,redirect_uri TEXT NOT NULL,state TEXT NOT NULL,challenge TEXT NOT NULL,scope TEXT NOT NULL,expires_at INTEGER NOT NULL,user_id INTEGER,session_hash TEXT NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS mcp_oauth_codes(token_hash TEXT PRIMARY KEY,connection_id INTEGER NOT NULL REFERENCES mcp_tokens(id) ON DELETE CASCADE,client_id TEXT NOT NULL,redirect_uri TEXT NOT NULL,challenge TEXT NOT NULL,resource TEXT NOT NULL,expires_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS mcp_access_tokens(token_hash TEXT PRIMARY KEY,connection_id INTEGER NOT NULL REFERENCES mcp_tokens(id) ON DELETE CASCADE,resource TEXT NOT NULL,expires_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS mcp_refresh_tokens(token_hash TEXT PRIMARY KEY,connection_id INTEGER NOT NULL REFERENCES mcp_tokens(id) ON DELETE CASCADE,client_id TEXT NOT NULL,resource TEXT NOT NULL,expires_at INTEGER NOT NULL,used INTEGER NOT NULL DEFAULT 0);

-- Authorized ledger paging and category ID-set filtering. Additive on startup.
CREATE INDEX IF NOT EXISTS transaction_account_date_id ON transactions(account_id,date DESC,id DESC);
CREATE INDEX IF NOT EXISTS allocation_category_transaction ON allocations(category_id,transaction_id);

CREATE TABLE IF NOT EXISTS group_targets(carry_forward INTEGER NOT NULL DEFAULT 1 CHECK(carry_forward IN (0,1)),included INTEGER NOT NULL DEFAULT 1 CHECK(included IN (0,1)),period_id INTEGER NOT NULL REFERENCES periods(id) ON DELETE CASCADE,category_id INTEGER NOT NULL REFERENCES categories(id),spending_group_id INTEGER REFERENCES spending_groups(id),amount_cents INTEGER NOT NULL CHECK(amount_cents>=0));
CREATE UNIQUE INDEX IF NOT EXISTS group_target_scope ON group_targets(period_id,category_id,COALESCE(spending_group_id,0));

CREATE TABLE IF NOT EXISTS budget_groups(period_id INTEGER NOT NULL REFERENCES periods(id) ON DELETE CASCADE,spending_group_id INTEGER REFERENCES spending_groups(id));
CREATE UNIQUE INDEX IF NOT EXISTS budget_group_scope ON budget_groups(period_id,COALESCE(spending_group_id,0));

CREATE TABLE IF NOT EXISTS transaction_tags(transaction_id INTEGER NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,tag_id INTEGER NOT NULL REFERENCES tags(id),PRIMARY KEY(transaction_id,tag_id));
CREATE TABLE IF NOT EXISTS merchant_rules(id INTEGER PRIMARY KEY,account_id INTEGER REFERENCES accounts(id),merchant_id INTEGER NOT NULL REFERENCES merchants(id),pattern TEXT NOT NULL,direction TEXT NOT NULL DEFAULT 'any' CHECK(direction IN ('any','debit','credit')),priority INTEGER NOT NULL DEFAULT 0,enabled INTEGER NOT NULL DEFAULT 1,version INTEGER NOT NULL DEFAULT 1);
CREATE INDEX IF NOT EXISTS tag_transaction ON transaction_tags(tag_id,transaction_id);

CREATE TABLE IF NOT EXISTS account_import_checks(account_id INTEGER PRIMARY KEY REFERENCES accounts(id),last_checked TEXT NOT NULL,import_id INTEGER REFERENCES imports(id));

CREATE UNIQUE INDEX IF NOT EXISTS global_merchant_name ON merchants(name COLLATE NOCASE) WHERE account_id IS NULL;

INSERT OR IGNORE INTO migrations VALUES(19);
