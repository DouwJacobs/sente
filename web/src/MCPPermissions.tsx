import { Field } from "./ui";
import type { Row } from "./shared/types";
export const capabilityLabels: Record<string, string> = {
  manage_merchants: "Create/edit merchants and logos",
  manage_merchant_rules: "Create/edit merchant naming rules",
  delete_merchant_rule: "Delete merchant naming rules",
  assign_merchants: "Assign or clear transaction merchants",
  assign_missing: "Fill missing categories",
  recategorize: "Change assigned categories",
  financial_edit:
    "Edit amounts, dates, descriptions, notes, splits, groups and periods",
  create_category: "Create categories",
  create_rule: "Create rules",
  update_rule: "Edit rules",
  delete_rule: "Delete rules",
  manage_budget_alerts: "Change your personal budget alert switches and thresholds",
  update_budget: "Edit shared budget limits",
  change_seen: "Mark your transactions seen or unseen",
};
export const reviewPermissions = () => ({
  schema_version: 1,
  preset: "review",
  read_budget_alerts: false,
  read_context: false,
  read_merchants: false,
  automatic_approval: {},
  capabilities: Object.fromEntries(
    Object.keys(capabilityLabels).map((k) => [k, false]),
  ),
  constraints: {
    account_ids: [] as number[],
    selected_accounts: false,
    missing_categories_only: false,
    constrained_rules: false,
  },
});
export function selectPreset(p: Row, preset: string): Row {
  const capabilities = Object.fromEntries(
    Object.keys(capabilityLabels).map((k) => [
      k,
      (preset === "finance" &&
        ![
          "manage_budget_alerts",
          "change_seen",
          "manage_merchants",
          "manage_merchant_rules",
          "delete_merchant_rule",
          "assign_merchants",
        ].includes(k)) ||
        (preset === "categorization" &&
          ["assign_missing", "create_rule"].includes(k)),
    ]),
  );
  return {
    ...p,
    preset,
    automatic_approval: {},
    capabilities: preset === "custom" ? p.capabilities : capabilities,
    constraints: {
      ...p.constraints,
      selected_accounts:
        preset === "categorization" || p.constraints.selected_accounts,
      missing_categories_only:
        preset === "custom"
          ? p.constraints.missing_categories_only
          : preset === "categorization",
      constrained_rules:
        preset === "custom"
          ? p.constraints.constrained_rules
          : preset === "categorization",
    },
  };
}
export function PermissionSummary({
  value,
  accounts,
}: {
  value: Row;
  accounts: Row[];
}) {
  const ids: number[] = value.constraints.account_ids || [];
  const granted = Object.keys(capabilityLabels).filter(
    (key) => value.capabilities[key],
  );
  const automatic = granted.filter((key) => value.automatic_approval?.[key]);
  return (
    <div className="mcp-permission-summary">
      <p>
        {ids.length
          ? "Reads and proposals limited to: " +
            ids
              .map(
                (id) =>
                  accounts.find((account) => account.id === id)?.name ||
                  "Account #" + id,
              )
              .join(", ") +
            "."
          : "The agent can read all accounts you currently have access to."}
      </p>
      <ul>
        <li>
          {value.read_context
            ? "Your saved context is shared as written."
            : "Your saved context is not shared."}
        </li>
        <li>
          {value.read_merchants
            ? "Merchant names, rules and requested logos can be read."
            : "Merchant names, rules and logos are not shared."}
        </li>
        <li>
          {value.read_budget_alerts
            ? "Your personal budget alert switches and thresholds can be read."
            : "Your personal budget alert settings are not shared."}
        </li>
      </ul>
      {granted.length ? (
        <>
          <p>Can propose:</p>
          <ul>
            {granted.map((key) => (
              <li key={key}>{capabilityLabels[key]}</li>
            ))}
          </ul>
        </>
      ) : (
        <p>Read-only: no change proposals.</p>
      )}
      {value.constraints.missing_categories_only && (
        <p className="footnote">
          Category changes can only fill missing categories.
        </p>
      )}
      {value.constraints.constrained_rules && (
        <p className="footnote">
          New rules are limited to selected accounts and match part of the
          transaction description.
        </p>
      )}
      {automatic.length ? (
        <>
          <p>Automatically approves:</p>
          <ul>
            {automatic.map((key) => (
              <li key={key}>{capabilityLabels[key]}</li>
            ))}
          </ul>
          <p className="footnote">
            Other change types require your approval in Sente.
          </p>
        </>
      ) : (
        <p className="footnote">
          {granted.length
            ? "Proposed changes require your approval in Sente before the agent can apply them."
            : "The agent can answer questions but cannot change your data."}
        </p>
      )}
    </div>
  );
}
export function MCPPermissionFields({
  value,
  onChange,
  accounts,
  canPropose,
}: {
  value: Row;
  onChange: (p: Row) => void;
  accounts: Row[];
  canPropose: boolean;
}) {
  const ids: number[] = value.constraints.account_ids || [],
    scopeError =
      value.constraints.selected_accounts && !ids.length
        ? "Select at least one account."
        : value.constraints.constrained_rules &&
            value.capabilities.create_rule &&
            !ids.length
          ? "Select at least one account for new rules."
          : (ids.length || value.constraints.selected_accounts) &&
              (value.capabilities.update_budget || value.read_budget_alerts || value.capabilities.manage_budget_alerts)
            ? "Shared budgets and personal budget alerts require all accessible accounts."
            : "";
  const merchantError =
    [
      "manage_merchants",
      "manage_merchant_rules",
      "delete_merchant_rule",
      "assign_merchants",
    ].some((k) => value.capabilities[k]) && !value.read_merchants
      ? "Allow the agent to read merchant details before enabling merchant changes."
      : "";
  const constraintError =
    merchantError ||
    (value.capabilities.manage_budget_alerts && !value.read_budget_alerts
      ? "Allow personal budget alert reads before enabling alert changes."
      : "") ||
    (value.constraints.missing_categories_only &&
    (value.capabilities.financial_edit || value.capabilities.recategorize)
      ? "Missing-category-only access cannot include financial edits or recategorization."
      : value.constraints.constrained_rules &&
          (value.capabilities.update_rule || value.capabilities.delete_rule)
        ? "Limited rule access allows new rules only."
        : "");
  const constraint = (key: string, v: boolean) =>
    onChange({ ...value, constraints: { ...value.constraints, [key]: v } });
  return (
    <>
      <Field
        label="Permission level"
        validate={() => constraintError}
        hint="Read-only lets the agent answer questions. Proposal permissions let it request changes; approval is separate."
      >
        <select
          value={value.preset}
          onChange={(e) => onChange(selectPreset(value, e.target.value))}
        >
          <option value="review">Read-only</option>
          {canPropose && (
            <>
              <option value="categorization">Categorisation proposals</option>
              <option value="finance">Finance editing proposals</option>
              <option value="custom">Custom permissions</option>
            </>
          )}
          {value.preset === "legacy_finance" && (
            <option value="legacy_finance" disabled>
              Existing finance permissions
            </option>
          )}
        </select>
      </Field>
      <section className="permission-scope">
        <h3>What the agent can read</h3>
        <p className="footnote">
          Transactions, categories, rules and budgets within your access are
          available by default. Banking credentials, account numbers, notes and
          import source details are excluded.
        </p>
        <label className="check">
          <input
            type="checkbox"
            checked={!!value.read_context}
            onChange={(e) =>
              onChange({ ...value, read_context: e.target.checked })
            }
          />
          Read your saved MCP context
        </label>
        <p className="footnote mcp-context-sharing-hint">
          Includes the full text you saved in Settings. You can turn sharing off
          at any time.
        </p>
        <label className="check">
          <input
            type="checkbox"
            checked={!!value.read_merchants}
            onChange={(e) =>
              onChange({ ...value, read_merchants: e.target.checked })
            }
          />
          Read merchant names, naming rules and logos
        </label>
        <label className="check">
          <input
            type="checkbox"
            checked={!!value.read_budget_alerts}
            onChange={(e) => onChange({ ...value, read_budget_alerts: e.target.checked })}
          />
          Read your personal budget alert switches and thresholds
        </label>
        <p className="footnote">
          Requires household access and all accessible accounts. Does not share
          notification messages, global channel preferences or push devices.
        </p>
        <Field
          label="Account access"
          validate={() => scopeError}
          hint="Selected accounts limit both reads and proposals. Current account permissions still apply. Global merchant changes require all accessible accounts."
        >
          <select
            value={
              value.constraints.selected_accounts || ids.length
                ? "selected"
                : "all"
            }
            onChange={(e) =>
              onChange({
                ...value,
                constraints: {
                  ...value.constraints,
                  selected_accounts: e.target.value === "selected",
                  account_ids: e.target.value === "all" ? [] : ids,
                },
              })
            }
          >
            <option value="all">All accessible accounts</option>
            <option value="selected">Selected accounts</option>
          </select>
        </Field>
        {(value.constraints.selected_accounts || ids.length > 0) && (
          <fieldset>
            <legend>Selected accounts</legend>
            {accounts.map((a) => (
              <label className="check" key={a.id}>
                <input
                  type="checkbox"
                  checked={ids.includes(a.id)}
                  onChange={(e) =>
                    onChange({
                      ...value,
                      constraints: {
                        ...value.constraints,
                        account_ids: e.target.checked
                          ? [...ids, a.id]
                          : ids.filter((id) => id !== a.id),
                      },
                    })
                  }
                />
                {a.name}
                {!a.can_edit ? " (view only)" : ""}
              </label>
            ))}
          </fieldset>
        )}
      </section>
      {value.preset === "custom" && canPropose && (
        <section className="permission-changes">
          <h3>Changes the agent can propose</h3>
          <p className="footnote">
            Choose specific change types. Each proposal needs your approval
            unless you enable automatic approval below.
          </p>
          <div className="permission-groups">
            {[
              [
                "Categorisation",
                ["assign_missing", "recategorize", "create_category"],
              ],
              [
                "Merchants",
                [
                  "manage_merchants",
                  "manage_merchant_rules",
                  "delete_merchant_rule",
                  "assign_merchants",
                ],
              ],
              [
                "Rules and budgets",
                ["create_rule", "update_rule", "delete_rule", "update_budget", "manage_budget_alerts"],
              ],
              ["Transaction details", ["financial_edit", "change_seen"]],
            ].map(([title, keys]) => (
              <fieldset key={title as string}>
                <legend>{title}</legend>
                {(keys as string[]).map((key) => (
                  <label className="check" key={key}>
                    <input
                      type="checkbox"
                      checked={!!value.capabilities[key]}
                      onChange={(e) =>
                        onChange({
                          ...value,
                          capabilities: {
                            ...value.capabilities,
                            [key]: e.target.checked,
                          },
                          automatic_approval: {
                            ...value.automatic_approval,
                            [key]: e.target.checked
                              ? !!value.automatic_approval?.[key]
                              : false,
                          },
                        })
                      }
                    />
                    {capabilityLabels[key]}
                  </label>
                ))}
              </fieldset>
            ))}
          </div>
        </section>
      )}
      {value.preset === "custom" && (
        <section className="permission-scope">
          <h3>Restrictions</h3>
          <label className="check">
            <input
              type="checkbox"
              checked={value.constraints.missing_categories_only}
              onChange={(e) =>
                constraint("missing_categories_only", e.target.checked)
              }
            />
            Only fill missing categories
          </label>
          <label className="check">
            <input
              type="checkbox"
              checked={value.constraints.constrained_rules}
              onChange={(e) =>
                constraint("constrained_rules", e.target.checked)
              }
            />
            Only create description rules in selected accounts
          </label>
        </section>
      )}
      {canPropose && Object.values(value.capabilities).some(Boolean) && (
        <details className="details permission-automatic">
          <summary>
            Optional automatic approval ·{" "}
            {
              Object.values(value.automatic_approval || {}).filter(Boolean)
                .length
            }{" "}
            enabled
          </summary>
          <fieldset>
            <legend>Automatically approve selected changes</legend>
            <p className="footnote">
              Enable only the change types this agent may approve without asking
              you. You can inspect every proposal. Changes apply only while
              access is still allowed and the affected records are unchanged. A
              proposal with several change types needs approval for each type.
            </p>
            {Object.entries(capabilityLabels)
              .filter(([key]) => value.capabilities[key])
              .map(([key, label]) => (
                <label className="check" key={key}>
                  <input
                    type="checkbox"
                    aria-label={"Automatically approve: " + label}
                    checked={!!value.automatic_approval?.[key]}
                    onChange={(e) =>
                      onChange({
                        ...value,
                        automatic_approval: {
                          ...value.automatic_approval,
                          [key]: e.target.checked,
                        },
                      })
                    }
                  />
                  {label}
                </label>
              ))}
          </fieldset>
        </details>
      )}
      <details className="permission-summary">
        <summary>Review permission summary</summary>
        <PermissionSummary value={value} accounts={accounts} />
      </details>
    </>
  );
}
