# Portable configuration

The browser and offline CLI export/import classification configuration through the same application services. Configuration does not export ledger entries, accounts, grants, bank identifiers, credentials or private notes, and importing never applies rules to existing transactions. For offline commands, stop the application using the database first; the existing database lock prevents concurrent offline writes.

Both CLI commands require explicit DATABASE_PATH and FINANCE_RULESET_USER (an enabled existing administrator with budget membership). The database must already exist. Account-level entries require the operator's current editor access to the named enabled account; exports include only accessible account scopes. A private-account grant is not implied by administrator status. Account names appear in the portable file because import maps to an existing account; protect configuration exports as private.

```sh
DATABASE_PATH=/path/to/finance.sqlite FINANCE_RULESET_USER=operator bin/finance export-ruleset /path/to/ruleset.json
DATABASE_PATH=/path/to/finance.sqlite FINANCE_RULESET_USER=operator bin/finance import-ruleset /path/to/ruleset.json
```

The Python scripts are wrappers around the same built CLI, with required --db and --user, optional --executable (default bin/finance), export --out, and an import file argument. They contain no SQL or separate mutation implementation. Build the CLI with the repository Go toolchain before using them. No operation defaults to the production database.

Exports use version 2 and include groups, flat categories (including archived state and historical identity keys), global/account-scoped merchants with optional local logos and independent category/group defaults, global/account-scoped merchant rules, custom account categorization rules and built-in rules. Priorities including zero/negative values, directions and enabled states are exact. Optional category_group_name disambiguates historical category IDs with the same display name. merchant_account_name identifies the merchant catalogue's scope separately from the naming rule's scope. builtin=true identifies rules applying to all enabled accounts. An empty account name means global only for merchant/catalogue scopes; custom category rules must name an account.

Import accepts versions 1 and 2, rejects unsupported versions/unknown fields/trailing documents, and bounds input to 32 MiB and 10,000 entries. Every reference must resolve uniquely; there is no first-account fallback or silent skipping. Resolve ambiguous existing same-scope/pattern/direction rule matches before importing. Duplicate entries in the same imported configuration reject. Category kind cannot change; legacy category spending-group fields from version 1 are validated but never create ownership. Historical group_name is an identity compatibility key, not a current group relationship.

Imports merge atomically. Existing group/merchant/rule changes increment versions and record audits through shared writers; unchanged entries do not rewrite versions. Rule identity includes direction. Omitted entities are retained. Merchant defaults/logo in supplied entries are exact, including clearing empty values. Categories can restore before enabled rules are written, and archive after imported rules are paused; active rule/carry-forward dependencies still block archive. Invalid references, logos, permissions, type changes or dependencies roll back the entire import, including audits. Import summaries count created/updated entities, not merely processed input rows.

Export queries share one consistent snapshot, propagate failures and encode fully before writing. Files are atomically replaced and restricted to owner permissions. No MCP import/export tool is added: browser configuration imports require an enabled administrator with budget membership; offline commands require host/operator authority. Existing agent connection consent/proposal capabilities remain separate.

## Settings → Configuration

Fresh installations start with no classification configuration. Upgrades preserve existing categories, groups, merchants and rules; the historical migration catalogue remains frozen for old database upgrades. No personal rules or EFT patterns are installed on a fresh database.

An enabled administrator with budget membership can export a JSON snapshot, import a JSON file, add a public HTTPS Git repository, or import the optional Sente starter. Several sources coexist (maximum 50). Imported data stays local: there is no background pull and no running connection to a repository. Files are uploaded explicitly and need a new upload for updates. Repository sources record URL, relative JSON path, branch/tag/commit selection, last successful applied revision and UTC time. Pull again fetches only when requested. Forget source removes the tracking record, retaining all imported configuration.

The default source is [DouwJacobs/sente-config](https://github.com/DouwJacobs/sente-config), file `sente.json`, on its default branch. `SENTE_DEFAULT_CONFIG_REPOSITORY`, `SENTE_DEFAULT_CONFIG_REF` and `SENTE_DEFAULT_CONFIG_PATH` override that source at startup. A tag or commit pins a compatible configuration release. Preview bundled starter is a separate offline choice using the application's release snapshot; later Pull again reads its public starter repository. Each source's versioned format is checked before import, so incompatible versions reject instead of applying partially.

Repository access uses the host's Git executable (included in the Docker image), HTTPS only, a 45-second deadline, isolated temporary storage and no host Git credentials, hooks, filters, submodules or redirects. DNS must resolve entirely to public addresses, and Git pins a validated address while preserving TLS verification. The selected path must be a regular JSON blob. Private repositories/authentication are not supported by the browser pull: upload their file instead. Remote content is configuration data, never code. Git fetches a shallow filtered snapshot; the final JSON is bounded to 32 MiB. A fetch failure leaves the prior source's last successful import unchanged.

Every pull/upload runs a complete import simulation inside a rolled-back SQL savepoint. The preview shows created/replaced configuration, named references and before/after values; previewing creates no configuration or audit writes. A one-use, 15-minute preview is bound to the user, payload, source, configuration versions and current access. Any relevant intervening edit or permission change requires another preview. Applying rechecks authorization and commits entities, source history and audits atomically.

Source precedence follows explicit application order, never an automatic repository ranking. Matching identities with different values are shown as replacements and require the Replace existing configuration confirmation, including during a repeat pull. This protects local edits from silent overwrites. Identical entries do not rewrite versions or duplicate rules. Omitted entries remain; removing an entry from a remote file never deletes local configuration. There is no automatic deletion or renaming through sync. To preserve a local edit, cancel that preview and adjust the source or edit locally after importing.

Rule identity uses normalized pattern, direction and scope; merchant naming rules identify scope/pattern/direction and may replace their merchant target only after confirmation. Duplicate identities within a file reject, even if their casing/spacing differs. Referenced categories/groups/merchants/accounts must resolve unambiguously, with current account editor access for scoped writes. Generic global category rules use `builtin: true` and remain fallback rules; explicit account rules take precedence. Existing classification conflict handling still withholds category assignment for incompatible overlapping matches. Importing configuration never rewrites transactions.

## Version 2 authoring

The JSON Schema is in [configuration/schema.json](../internal/app/configuration/schema.json). Runtime authorization, combined 10,000-entry bounds, reference resolution, duplicates and dependency validation supplement the schema. Missing collections are empty; `enabled` is explicit on rules, `priority` defaults to zero, and omitted direction means any. `builtin: true` means all enabled accounts; otherwise a category rule must specify `account_name`. Global merchant records/naming rules omit `account_name`. Do not add `$schema` to a configuration file; the strict importer rejects unknown fields.

```json
{
  "version": 2,
  "categories": [{"name": "Supplies", "group_name": "", "kind": "expense"}],
  "merchants": [{"name": "Example Shop", "category": "Supplies"}],
  "merchant_rules": [{"merchant": "Example Shop", "pattern": "EXAMPLE SHOP", "direction": "debit", "priority": 0, "enabled": true}],
  "rules": [{"pattern": "EXAMPLE SHOP", "category": "Supplies", "builtin": true, "direction": "debit", "priority": 0, "enabled": true}]
}
```

The export includes all five configuration collections, accessible scoped rules/merchants and optional logos. It excludes budgets, account definitions, grants, transactions, bank identities, credentials, notes and source-tracking records. Preserve historical category `group_name` keys when round-tripping existing configurations. Map account names deliberately when transferring personal configuration to a differently named installation. Account names, merchant names, personal patterns and logos may be sensitive even though credentials are excluded.
