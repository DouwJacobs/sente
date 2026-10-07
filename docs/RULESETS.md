# Portable classification rulesets

The offline CLI exports/imports classification configuration through the application services. It does not export ledger entries, accounts, grants, bank identifiers, credentials or private notes, and importing never applies rules to existing transactions. Stop the application using the database first; the existing database lock prevents concurrent offline writes.

Both CLI commands require explicit DATABASE_PATH and FINANCE_RULESET_USER (an enabled existing administrator with budget membership). The database must already exist. Account-level entries require the operator's current editor access to the named enabled account; exports include only accessible account scopes. A private-account grant is not implied by administrator status. Account names appear in the portable file because import maps to an existing account; protect configuration exports as private.

```sh
DATABASE_PATH=/path/to/finance.sqlite FINANCE_RULESET_USER=operator bin/finance export-ruleset /path/to/ruleset.json
DATABASE_PATH=/path/to/finance.sqlite FINANCE_RULESET_USER=operator bin/finance import-ruleset /path/to/ruleset.json
```

The Python scripts are wrappers around the same built CLI, with required --db and --user, optional --executable (default bin/finance), export --out, and an import file argument. They contain no SQL or separate mutation implementation. Build the CLI with the repository Go toolchain before using them. No operation defaults to the production database.

Exports use version 2 and include groups, flat categories (including archived state and historical identity keys), global/account-scoped merchants with optional local logos and independent category/group defaults, global/account-scoped merchant rules, custom account categorization rules and built-in rules. Priorities including zero/negative values, directions and enabled states are exact. Optional category_group_name disambiguates historical category IDs with the same display name. merchant_account_name identifies the merchant catalogue's scope separately from the naming rule's scope. builtin=true identifies rules applying to all enabled accounts. An empty account name means global only for merchant/catalogue scopes; custom category rules must name an account.

Import accepts versions 1 and 2, rejects unsupported versions/unknown fields/trailing documents, and bounds input to 32 MiB and 10,000 entries. Every reference must resolve uniquely; there is no first-account fallback or silent skipping. Resolve ambiguous existing same-scope/pattern/direction rule matches before importing. Duplicate entries in the same imported configuration reject. Category kind cannot change; legacy category spending-group fields from version 1 are validated but never create ownership. Historical group_name is an identity compatibility key, not a current group relationship.

Imports merge atomically. Existing group/merchant/rule changes increment versions and record audits through shared writers; unchanged entries do not rewrite versions. Rule identity includes direction. Omitted entities are retained. Merchant defaults/logo in supplied entries are exact, including clearing empty values. Categories can restore before enabled rules are written, and archive after imported rules are paused; active rule/carry-forward dependencies still block archive. Invalid references, logos, permissions, type changes or dependencies roll back the entire import, including audits. Import summaries count created/updated entities, not merely processed input rows.

Export queries share one consistent snapshot, propagate failures and encode fully before writing. Files are atomically replaced and restricted to owner permissions. No MCP import/export tool is added: bulk offline configuration operations require host/operator authority and remain separate from existing connection consent/proposal capabilities.
