import { useEffect, useRef, useState } from "react";
import { api } from "../../api";
import { Button, Field, Form, Modal, validateFields } from "../../ui";
import { CategoryChoice, ChoiceField } from "../../Choices";
import { MerchantLogoField } from "../../MerchantAvatar";
import { PagedSelect } from "../../PagedList";
import { useTask } from "../../shared/useTask";
import type { PageProps, Row } from "../../shared/types";
import { ruleRefs, type RuleRef } from "./types";
import { RuleBackfill } from "./RuleBackfill";

export function UnifiedRuleEditor({
  rule,
  data,
  notify,
  refresh,
  onClose,
  onDone,
}: {
  rule?: Row;
  revision?: number;
  data: PageProps["data"];
  notify: PageProps["notify"];
  refresh: () => void;
  onClose: () => void;
  onDone?: () => void;
}) {
  const saved = !!rule?.id,
    initialKind =
      rule?.rule_kind || (rule?.merchant_id ? "merchant" : "category");
  const [kind, setKind] = useState(initialKind),
    [pattern, setPattern] = useState(rule?.pattern || "");
  const [category, setCategory] = useState<number | null>(
      rule?.category_id || null,
    ),
    [group, setGroup] = useState<number | null>(
      rule?.spending_group_id || null,
    );
  const [merchant, setMerchant] = useState<number | null>(
      rule?.merchant_id || null,
    ),
    [name, setName] = useState("");
  const [scope, setScope] = useState(
    saved
      ? rule?.builtin || rule?.account_id == null
        ? "global"
        : String(rule?.account_id)
      : data.user.budget_member
        ? "current"
        : String(data.accounts.find((a) => a.role === "editor")?.id || ""),
  );
  const [direction, setDirection] = useState(rule?.direction || "any"),
    [priority, setPriority] = useState(String(rule?.priority ?? 0)),
    [enabled, setEnabled] = useState(rule ? !!rule.enabled : true);
  const [logo, setLogo] = useState(rule?.merchant_logo || ""),
    [logoVersion, setLogoVersion] = useState(rule?.merchant_version || 1),
    [logoDirty, setLogoDirty] = useState(false),
    [logoBusy, setLogoBusy] = useState(false);
  const [loading, setLoading] = useState(false),
    [fieldError, setFieldError] = useState(""),
    [example, setExample] = useState(""),
    [exampleDirection, setExampleDirection] = useState("debit"),
    [results, setResults] = useState<Row[] | null>(null);
  const [created, setCreated] = useState<RuleRef[] | null>(null);
  const { busy, run } = useTask(notify),
    request = useRef(0);
  useEffect(
    () => () => {
      request.current++;
    },
    [],
  );
  useEffect(() => {
    setResults(null);
    setFieldError("");
  }, [
    pattern,
    category,
    group,
    merchant,
    name,
    scope,
    direction,
    priority,
    enabled,
    kind,
    example,
    exampleDirection,
  ]);
  const form = useRef<HTMLFormElement>(null);
  const categoryError = /category/i.test(fieldError) ? fieldError : "";
  const groupError =
    !categoryError && /group/i.test(fieldError) ? fieldError : "";
  const merchantError = !categoryError && !groupError ? fieldError : "";
  const focusAssignment = () =>
    requestAnimationFrame(() =>
      form.current
        ?.querySelector<HTMLButtonElement>(".choice-control")
        ?.focus(),
    );
  const finish = () => {
    (onDone || onClose)();
  };
  const account = scope === "current" || scope === "global" ? 0 : Number(scope);
  const payload = (merchantID = merchant, version = logoVersion) => ({
    kind,
    scope,
    refs: saved ? ruleRefs(rule!) : [],
    rule: {
      account_id: account,
      pattern,
      category_id: category || 0,
      spending_group_id: group,
      direction,
      priority: Number(priority),
      enabled,
    },
    merchant_rule: {
      account_id: account,
      merchant_id: merchantID || 0,
      pattern,
      category_id: category,
      spending_group_id: group,
      clear_category: !category,
      clear_group: !group,
      direction,
      priority: Number(priority),
      enabled,
      ...(logoDirty ? { merchant_logo: logo, merchant_version: version } : {}),
    },
  });
  const selectMerchant = async (id: number | null) => {
    const revision = ++request.current;
    setMerchant(id);
    setLogoDirty(false);
    setLogo("");
    setName("");
    if (!id) {
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const selected = (await api("/labels?kind=merchant&id=" + id)).items[0];
      if (request.current !== revision) return;
      setLogo(selected?.logo_data || "");
      setLogoVersion(selected?.logo_version || 1);
      // User assignments belong to the rule. Defaults are only offered when all outputs are empty.
      if (!saved && !category && !group) {
        setCategory(selected?.category_id ?? null);
        setGroup(selected?.spending_group_id ?? null);
      }
    } catch (e) {
      if (request.current === revision) notify((e as Error).message, true);
    } finally {
      if (request.current === revision) setLoading(false);
    }
  };
  if (created)
    return (
      <RuleBackfill
        canSelectPeriod={!!data.user.budget_member}
        refs={created}
        notify={notify}
        refresh={refresh}
        offered
        onClose={finish}
      />
    );
  return (
    <Modal
      title={saved ? "Edit rule" : "Add rule"}
      onClose={() => {
        if (!busy) onClose();
      }}
    >
      <Form
        ref={form}
        onSubmit={() =>
          run(
            async () => {
              if (kind === "category" && !category && !group) {
                setFieldError("Choose a merchant, category or spending group.");
                focusAssignment();
                return;
              }
              let id = merchant,
                version = logoVersion;
              if (kind === "merchant" && !id) {
                if (!name.trim()) {
                  setFieldError(
                    "Choose a merchant or enter a new merchant name.",
                  );
                  return;
                }
                const result = await api("/labels", "POST", {
                  kind: "merchant",
                  account_id: account,
                  name,
                });
                id = result.id;
                version = result.logo_version || 1;
                setMerchant(id);
                setLogoVersion(version);
              }
              const result = await api(
                "/unified-rules",
                "POST",
                payload(id, version),
              );
              notify("Rule saved");
              refresh();
              if (!saved && enabled) setCreated(result.refs);
              else finish();
            },
            undefined,
            (message) => {
              if (
                message.startsWith("Choose ") ||
                message.includes("category") ||
                message.toLowerCase().includes("merchant name")
              ) {
                setFieldError(message);
                return true;
              }
              return false;
            },
          )
        }
      >
        <fieldset disabled={busy} className="rule-fields">
          <Field
            label="Description contains"
            hint={
              kind === "merchant"
                ? "Plain text or advanced pattern. Use | for alternatives."
                : "Literal text; matching ignores case and repeated spaces."
            }
            validate={(value) =>
              value.trim().length < 2
                ? "Use at least 2 non-space characters."
                : new TextEncoder().encode(value.trim()).length > 200
                  ? "Use a shorter description match."
                  : ""
            }
          >
            <input
              autoFocus
              required
              minLength={2}
              maxLength={200}
              value={pattern}
              onChange={(e) => setPattern(e.target.value)}
            />
          </Field>
          {!saved && (
            <label className="check">
              <input
                type="checkbox"
                checked={kind === "merchant"}
                onChange={(e) =>
                  setKind(e.target.checked ? "merchant" : "category")
                }
              />
              Assign a merchant
            </label>
          )}
          {kind === "merchant" && (
            <>
              <ChoiceField
                label="Merchant"
                value={merchant}
                options={[]}
                source={
                  "/labels?kind=merchant&" +
                  (account ? "account=" + account : "scope=global")
                }
                onChange={selectMerchant}
                onError={(message) => notify(message, true)}
                validationError={merchantError}
              />
              <Field
                label="New merchant name"
                hint="Leave empty when choosing an existing merchant."
                serverError={merchantError}
              >
                <input
                  required={!merchant}
                  minLength={2}
                  maxLength={100}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
              </Field>
              <MerchantLogoField
                name={name || rule?.merchant_name || "Merchant"}
                value={logo}
                onChange={(value) => {
                  setLogo(value);
                  setLogoDirty(true);
                }}
                onBusy={setLogoBusy}
              />
            </>
          )}
          <CategoryChoice
            label="Category"
            data={data}
            value={category}
            disabled={loading}
            validationError={categoryError}
            onChange={setCategory}
            refresh={refresh}
            notify={notify}
          />
          <ChoiceField
            label="Spending group"
            validationError={groupError}
            source="/spending-groups"
            options={[]}
            value={group}
            empty="Leave unassigned"
            disabled={loading}
            onChange={setGroup}
            onError={(message) => notify(message, true)}
          />
          <details className="details">
            <summary>More options</summary>
            {saved ? (
              <p>
                Applies to{" "}
                {rule?.builtin || rule?.account_id == null
                  ? "all accounts, including future accounts"
                  : rule?.rows?.length > 1
                    ? rule.rows.length + " current accounts"
                    : rule?.account_name ||
                      data.accounts.find((a) => a.id === rule?.account_id)
                        ?.name ||
                      "the saved account"}
                . Account scope is preserved.
              </p>
            ) : (
              <PagedSelect
                url="/accounts?role=editor"
                label="Applies to"
                value={scope}
                required
                options={data.accounts.filter((a) => a.role === "editor")}
                onChange={(value) => {
                  setScope(value);
                  if (kind === "merchant") {
                    setMerchant(null);
                    setLogo("");
                    setLogoDirty(false);
                  }
                }}
                specialOptions={
                  data.user.budget_member
                    ? [
                        ...(!saved
                          ? [
                              {
                                value: "current",
                                label: "All current accounts you can edit",
                              },
                            ]
                          : []),
                        {
                          value: "global",
                          label: "All accounts, including future accounts",
                        },
                      ]
                    : []
                }
              />
            )}
            <Field label="Payment direction">
              <select
                value={direction}
                onChange={(e) => setDirection(e.target.value)}
              >
                <option value="any">Money in or out</option>
                <option value="debit">Money out</option>
                <option value="credit">Money in</option>
              </select>
            </Field>
            <Field label="Priority">
              <input
                type="number"
                min={-1000000}
                max={1000000}
                required
                value={priority}
                onChange={(e) => setPriority(e.target.value)}
              />
            </Field>
            <label className="check">
              <input
                type="checkbox"
                checked={enabled}
                onChange={(e) => setEnabled(e.target.checked)}
              />
              Active
            </label>
          </details>
          <details className="details">
            <summary>Check an example</summary>
            <Field label="Example description">
              <input
                maxLength={1000}
                value={example}
                onChange={(e) => setExample(e.target.value)}
              />
            </Field>
            <Field label="Example direction">
              <select
                value={exampleDirection}
                onChange={(e) => setExampleDirection(e.target.value)}
              >
                <option value="debit">Money out</option>
                <option value="credit">Money in</option>
              </select>
            </Field>
            <Button
              type="button"
              loading={busy}
              disabled={
                !example.trim() ||
                (kind === "merchant" && !merchant) ||
                logoBusy ||
                loading
              }
              onClick={(event) => {
                if (
                  event.currentTarget.form &&
                  !validateFields(event.currentTarget.form)
                )
                  return;
                run(async () => {
                  const result = await api("/unified-rules/test", "POST", {
                    draft: payload(),
                    description: example,
                    direction: exampleDirection,
                  });
                  setResults(result.items);
                });
              }}
            >
              Check rule
            </Button>
            {results && (
              <div role="status">
                {results.map((result: Row) => (
                  <p key={result.account}>
                    <strong>{result.account}</strong>:{" "}
                    {result.conflict
                      ? "Conflicting rules: needs review"
                      : [result.merchant, result.category, result.group]
                          .filter(Boolean)
                          .join(" · ") || "No matching assignment"}
                    {result.explanation && <small>{result.explanation}</small>}
                  </p>
                ))}
              </div>
            )}
          </details>
          <p className="footnote">
            {kind === "merchant"
              ? "Merchant rules take precedence. Highest merchant priority wins; conflicting outputs at the same priority need review."
              : "Category and group rules apply when merchant rules do not supply a category. Conflicting matching outputs need review."}{" "}
            Saving affects future imports only.
          </p>
          <div className="editor-actions">
            <Button
              type="submit"
              variant="primary"
              loading={busy}
              disabled={logoBusy || loading}
            >
              Save rule
            </Button>
            <Button disabled={busy} onClick={onClose}>
              Cancel
            </Button>
          </div>
        </fieldset>
      </Form>
    </Modal>
  );
}
