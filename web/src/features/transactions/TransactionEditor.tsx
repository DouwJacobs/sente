import { TransactionExtras } from "./TransactionExtras";
import { Allocations } from "./Allocations";
import { TransactionLabels } from "../../CoreWorkflows";
import { useEffect, useState, useRef } from "react";
import { Eye, EyeOff } from "lucide-react";
import { api, cents, decimal } from "../../api";
import { Button, Field, Badge, Modal, validateFields } from "../../ui";
import { ChoiceField } from "../../Choices";
import { moneyError } from "../../validation";
import { proposeRulePattern } from "../../ruleProposal";
import { useTask } from "../../shared/useTask";
import { type Row, type PageProps } from "../../shared/types";
export function TransactionEditor({
  transaction: t,
  data,
  refresh,
  notify,
  onClose,
  onSaved,
  reviewQuery,
  processed = [],
  onNavigate,
  onSaveNext,
}: {
  reviewQuery?: string;
  processed?: number[];
  onNavigate?: (direction: "previous" | "next") => Promise<void>;
  onSaveNext?: () => Promise<void>;
  transaction: Row;
  data: PageProps["data"];
  refresh: () => void;
  notify: PageProps["notify"];
  onClose: () => void;
  onSaved: () => void;
}) {
  const [note, setNote] = useState(t.note || ""),
    [merchant, setMerchant] = useState<number | null>(t.merchant_id || null),
    [tags, setTags] = useState<Row[]>(t.tags || []);
  const [nav, setNav] = useState<Row | null>(null),
    [leave, setLeave] = useState<null | (() => void)>(null);
  const draft = () =>
    JSON.stringify({
      date,
      amount,
      description,
      alloc,
      spendingGroup,
      transfer,
      assignment,
      period,
      note,
      merchant,
      tags,
      saveRule,
      pattern,
    });
  const baseline = useRef("");
  const [date, setDate] = useState(t.date),
    [amount, setAmount] = useState(decimal(t.amount_cents)),
    [description, setDescription] = useState(t.description);
  const [alloc, setAlloc] = useState<Row[]>(
    t.allocations.map((a: Row) => ({ ...a, amount: decimal(a.amount_cents) })),
  );
  const [spendingGroup, setSpendingGroup] = useState<number | null>(
    t.spending_group_id || null,
  );
  const [transfer, setTransfer] = useState(
      !!t.is_transfer ||
        String(t.spending_group_name || "")
          .trim()
          .toLowerCase() === "transfer",
    ),
    [assignment, setAssignment] = useState(t.assignment),
    [period, setPeriod] = useState(String(t.period_id || ""));
  const [seen, setSeen] = useState(!!t.seen);
  const changeSeen = async () => {
    if (
      await run(
        () =>
          api("/transactions/seen", "POST", {
            seen: !seen,
            items: [{ id: t.id, version: t.version }],
          }),
        seen ? "Transaction marked unseen" : "Transaction marked seen",
      )
    ) {
      setSeen(!seen);
      onSaved();
      refresh();
    }
  };
  const [saveRule, setSaveRule] = useState(false),
    [pattern, setPattern] = useState(proposeRulePattern(t.description));
  const { busy, run } = useTask(notify);
  const allocated = alloc.reduce((sum, a) => {
    try {
      return sum + cents(a.amount);
    } catch {
      return sum;
    }
  }, 0);
  let original = 0;
  try {
    original = cents(amount);
  } catch {}
  const update = (i: number, key: string, value: unknown) =>
    setAlloc((v) => v.map((a, n) => (n === i ? { ...a, [key]: value } : a)));
  const canSaveRule =
    alloc.length === 1 &&
    !!alloc[0]?.category_id &&
    !transfer &&
    original !== 0;
  if (!baseline.current) baseline.current = draft();
  const requestLeave = (action: () => void) => {
    if (t.can_edit && draft() !== baseline.current) setLeave(() => action);
    else action();
  };
  useEffect(() => {
    if (reviewQuery === undefined) return;
    let alive = true;
    const p = new URLSearchParams(reviewQuery);
    p.set("anchor_id", String(t.id));
    p.set("anchor_date", t.date);
    p.set("processed", processed.join(","));
    api("/transactions/navigation?" + p)
      .then((v) => alive && setNav(v))
      .catch((e) => notify(e.message, true));
    return () => {
      alive = false;
    };
  }, [reviewQuery, t.id, t.date, processed.join(",")]);
  const save = async (after?: () => void) => {
    const ok = await run(async () => {
      const parsed = cents(amount);
      const result = await api("/transactions/" + t.id, "PUT", {
        version: t.version,
        date,
        amount_cents: parsed,
        description,
        note,
        merchant_id: merchant,
        clear_merchant: merchant === null,
        tag_ids: tags.map((tag) => tag.id),
        allocations: alloc.map((a) => ({
          category_id: a.category_id ? Number(a.category_id) : null,
          amount_cents: cents(a.amount),
          note: a.note || "",
        })),
        is_transfer: transfer,
        spending_group_id: spendingGroup,
        assignment,
        period_id: assignment === "manual" ? Number(period) : null,
        rule: saveRule && canSaveRule ? { pattern } : null,
      });
      let message =
        transfer || alloc.every((a) => a.category_id)
          ? "Transaction saved and accepted"
          : "Transaction saved; missing categories still need review";
      const applied = result.pending_rule?.applied || 0,
        conflicts = result.pending_rule?.conflicts || 0;
      if (applied)
        message += `. Rule applied to ${applied} other uncategorized ${applied === 1 ? "transaction" : "transactions"}; they are accepted and unseen.`;
      if (conflicts)
        message += ` ${conflicts} matching ${conflicts === 1 ? "transaction needs" : "transactions need"} manual categorization because rules disagree.`;
      notify(message);
    });
    if (ok) {
      baseline.current = draft();
      setLeave(null);
      onSaved();
      refresh();
      if (after) after();
      else onClose();
    }
  };
  const submit = (container: HTMLElement) => {
    if (validateFields(container)) save();
  };
  return (
    <Modal
      size="wide"
      title={"Transaction #" + t.id}
      onClose={() => requestLeave(onClose)}
    >
      {leave && (
        <Modal title="Unsaved changes" onClose={() => setLeave(null)}>
          <p>Save your changes before leaving this transaction?</p>
          <div className="editor-actions">
            <Button
              variant="primary"
              loading={busy}
              onClick={(e) => {
                const parent = Array.from(
                  document.querySelectorAll("dialog[open]"),
                ).find(
                  (d) =>
                    d.getAttribute("aria-label") === "Transaction #" + t.id,
                );
                const action = leave;
                setLeave(null);
                requestAnimationFrame(() => {
                  if (
                    parent &&
                    validateFields(parent.querySelector("fieldset")!)
                  )
                    save(action);
                });
              }}
            >
              Save and continue
            </Button>
            <Button
              onClick={() => {
                const action = leave;
                setLeave(null);
                action();
              }}
            >
              Discard changes
            </Button>
            <Button onClick={() => setLeave(null)}>Stay here</Button>
          </div>
        </Modal>
      )}
      <div className="transaction-editor-context">
        {reviewQuery !== undefined && (
          <nav className="toolbar" aria-label="Review transactions">
            <Button
              disabled={busy || !nav?.previous}
              onClick={() => requestLeave(() => onNavigate?.("previous"))}
            >
              Previous
            </Button>
            <Button
              disabled={busy || !nav?.next}
              onClick={() => requestLeave(() => onNavigate?.("next"))}
            >
              Next
            </Button>
          </nav>
        )}
        <div className="editor-intro">
          <Badge tone={t.review_state === "approved" ? "good" : "pending"}>
            {t.review_state === "approved" ? "Accepted" : "Needs category"}
          </Badge>
          <span>{t.account_name}</span>
          <Badge tone={seen ? "good" : "pending"}>
            {seen ? "Seen by you" : "Unseen by you"}
          </Badge>
          {!t.can_edit && <Badge>View only</Badge>}
          <Button
            variant="quiet"
            loading={busy}
            disabled={busy}
            onClick={changeSeen}
          >
            {seen ? <EyeOff size={17} /> : <Eye size={17} />}Mark{" "}
            {seen ? "unseen" : "seen"}
          </Button>
        </div>
      </div>
      <fieldset
        className="transaction-editor-fields"
        disabled={!t.can_edit || busy}
      >
        <div className="transaction-editor-grid">
          <section
            className="transaction-editor-pane"
            aria-label="Transaction details"
          >
            <h3>Transaction details</h3>
            <div className="form-grid">
              <Field label="Date">
                <input
                  type="date"
                  required
                  value={date}
                  onChange={(e) => setDate(e.target.value)}
                />
              </Field>
              <Field
                label="Signed amount (ZAR)"
                validate={(value) => moneyError(value)}
                hint="Expenses are negative; income and refunds positive."
              >
                <input
                  inputMode="decimal"
                  required
                  value={amount}
                  onChange={(e) => {
                    setAmount(e.target.value);
                    if (alloc.length === 1) update(0, "amount", e.target.value);
                  }}
                />
              </Field>
            </div>
            <Field label="Description">
              <input
                required
                maxLength={1000}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </Field>
            <details
              className="details editor-section"
              open={!!t.note || !!t.merchant_id || !!t.tags?.length}
            >
              <summary>
                Notes, merchant and tags
                {note || merchant || tags.length ? " · Details added" : ""}
              </summary>
              <Field label="Transaction note">
                <textarea
                  maxLength={2000}
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                />
              </Field>
              <TransactionLabels
                globalMerchants={!!data.user.budget_member}
                account={t.account_id}
                merchant={merchant}
                tags={tags}
                onMerchant={(id, choice) => {
                  setMerchant(id);
                  if (
                    id &&
                    choice?.category_id &&
                    alloc.length === 1 &&
                    !alloc[0]?.category_id
                  ) {
                    update(0, "category_id", choice.category_id);
                    if (choice.spending_group_id && !spendingGroup) {
                      setSpendingGroup(choice.spending_group_id);
                    }
                  }
                }}
                onTags={setTags}
                notify={notify}
                disabled={!t.can_edit || busy}
              />
            </details>
            <details
              className="details editor-section"
              open={assignment !== "auto"}
            >
              <summary>
                Budget assignment
                {assignment === "manual"
                  ? " · Specific period"
                  : assignment === "outside"
                    ? " · Outside budgets"
                    : " · Automatic"}
              </summary>
              {data.user.budget_member && !!t.household && (
                <div className="form-grid">
                  <Field label="Budget assignment">
                    <select
                      value={assignment}
                      onChange={(e) => setAssignment(e.target.value)}
                    >
                      <option value="auto">
                        Match period dates automatically
                      </option>
                      <option value="manual">
                        Assign to a specific period
                      </option>
                      <option value="outside">Leave outside budgets</option>
                    </select>
                  </Field>
                  {assignment === "manual" && (
                    <Field label="Assigned period">
                      <select
                        required
                        value={period}
                        onChange={(e) => setPeriod(e.target.value)}
                      >
                        <option value="">Choose a period</option>
                        {data.periods.map((p) => (
                          <option value={p.id} key={p.id}>
                            {p.name} · {p.start_date} – {p.end_date}
                          </option>
                        ))}
                      </select>
                    </Field>
                  )}
                </div>
              )}
            </details>
          </section>
          <section
            className="transaction-editor-pane transaction-classification"
            aria-label="Classification"
          >
            <h3>Classification</h3>
            <ChoiceField
              source="/spending-groups"
              label="Spending group"
              value={spendingGroup}
              onChange={(id, group) => {
                const isTransfer =
                  group?.name.trim().toLowerCase() === "transfer";
                if (
                  (t.transfer_counterpart_id ||
                    t.transfer_counterpart_hidden) &&
                  !isTransfer
                ) {
                  notify(
                    "Unlink the transfer before changing its spending group.",
                    true,
                  );
                  return;
                }
                setSpendingGroup(id);
                setTransfer(isTransfer);
              }}
              options={data.spendingGroups.map((g) => ({
                id: g.id,
                name: g.name,
                color: g.color,
              }))}
              disabled={!t.can_edit || busy}
            />
            <Allocations
              transaction={t}
              data={data}
              refresh={refresh}
              notify={notify}
              alloc={alloc}
              setAlloc={setAlloc}
              update={update}
              original={original}
              allocated={allocated}
              busy={busy}
            />
            {transfer && (
              <p className="muted">
                Transfer excluded from income and spending. No category is
                required.
              </p>
            )}
            {canSaveRule && (
              <details className="details editor-section">
                <summary>
                  Automatically categorize similar transactions
                  {saveRule ? " · Enabled" : ""}
                </summary>
                <div className="rule-offer">
                  <h3>Proposed automatic rule</h3>
                  <p className="muted">
                    Uses this category and group for future matches in this
                    account, and fills eligible uncategorized entries already
                    imported. Existing categories, splits and groups are kept.
                    Rule-applied entries start unseen.
                  </p>
                  <label className="check">
                    <input
                      type="checkbox"
                      checked={saveRule}
                      onChange={(e) => setSaveRule(e.target.checked)}
                    />
                    Use this category and spending group for similar
                    transactions in this account
                  </label>
                  <Field
                    label="Description contains"
                    hint="Use the shop or service name, leaving out reference numbers."
                    validate={(value) =>
                      !saveRule
                        ? ""
                        : value.trim().length < 2
                          ? "Use at least 2 non-space characters."
                          : new TextEncoder().encode(value.trim()).length > 200
                            ? "Use a shorter description match."
                            : ""
                    }
                  >
                    <input
                      required={saveRule}
                      maxLength={200}
                      value={pattern}
                      onChange={(e) => setPattern(e.target.value)}
                    />
                  </Field>
                </div>
              </details>
            )}
          </section>
        </div>
      </fieldset>
      <TransactionExtras
        transaction={t}
        busy={busy}
        run={run}
        refresh={refresh}
        onClose={onClose}
      />
      <div className="transaction-editor-footer">
        <small className="muted">
          Saving marks this transaction seen by you.
        </small>
        <div className="editor-actions">
          {t.can_edit && (
            <Button
              variant="primary"
              loading={busy}
              disabled={busy}
              onClick={(e) =>
                submit(
                  e.currentTarget
                    .closest(".modal-body")!
                    .querySelector("fieldset")!,
                )
              }
            >
              Save changes
            </Button>
          )}
          {t.can_edit && reviewQuery !== undefined && (
            <Button
              loading={busy}
              onClick={(e) => {
                if (
                  validateFields(
                    e.currentTarget
                      .closest(".modal-body")!
                      .querySelector("fieldset")!,
                  )
                )
                  save(() => onSaveNext?.());
              }}
            >
              Save and next
            </Button>
          )}
          <Button disabled={busy} onClick={() => requestLeave(onClose)}>
            Close
          </Button>
        </div>
      </div>
    </Modal>
  );
}
