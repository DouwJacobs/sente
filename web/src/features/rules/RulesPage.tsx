import { useState } from "react";
import { api } from "../../api";
import {
  Button,
  Field,
  Form,
  ActionMenu,
  Empty,
  StatusIcon,
  Modal,
} from "../../ui";
import { CircleCheck, Pause, Globe } from "lucide-react";
import { MerchantAvatar } from "../../MerchantAvatar";
import { usePagedList, ListStatus, ListNavigation } from "../../PagedList";
import { useTask } from "../../shared/useTask";
import type { PageProps, Row } from "../../shared/types";
import { rulePayload, ruleRefs } from "./types";
import { UnifiedRuleEditor } from "./UnifiedRuleEditor";
import { RuleBackfill } from "./RuleBackfill";

export function RulesPage({
  data,
  revision,
  notify,
  refresh,
}: Pick<PageProps, "data" | "revision" | "notify" | "refresh">) {
  const [status, setStatus] = useState("all"),
    [kind, setKind] = useState("all"),
    [direction, setDirection] = useState("all"),
    [account, setAccount] = useState("");
  const params = new URLSearchParams({
    status,
    kind,
    direction,
    ...(account ? { account } : {}),
  });
  const list = usePagedList("/unified-rules?" + params, revision);
  const [edit, setEdit] = useState<Row | null | undefined>(undefined),
    [backfill, setBackfill] = useState<Row | null>(null),
    [deleting, setDeleting] = useState<Row | null>(null);
  const { busy, run } = useTask(notify);
  const filtered =
    !!list.query ||
    status !== "all" ||
    kind !== "all" ||
    direction !== "all" ||
    !!account;
  const clear = () => {
    list.setQuery("");
    setStatus("all");
    setKind("all");
    setDirection("all");
    setAccount("");
  };
  const canCreate =
    data.user.budget_member || data.accounts.some((a) => a.role === "editor");
  return (
    <section className="panel">
      <div className="section-head">
        <div>
          <h2>Rules</h2>
          <p className="muted">
            Match descriptions to merchants, categories and spending groups.
            Merchant rules take precedence.
          </p>
        </div>
        {canCreate &&
          (list.loading || list.error || list.items.length > 0 || filtered) && (
            <Button onClick={() => setEdit(null)}>Add rule</Button>
          )}
      </div>
      <Field label="Search rules">
        <input
          type="search"
          placeholder="Description, merchant, category or group"
          value={list.query}
          onChange={(e) => list.setQuery(e.target.value)}
        />
      </Field>
      <details className="details">
        <summary>Filters{filtered ? " · Active" : ""}</summary>
        <div className="form-grid">
          <Field label="Rule status">
            <select
              value={status}
              onChange={(e) => {
                setStatus(e.target.value);
                list.setPage(0);
              }}
            >
              <option value="all">Active and paused</option>
              <option value="active">Active</option>
              <option value="paused">Paused</option>
            </select>
          </Field>
          <Field label="Assignments">
            <select
              value={kind}
              onChange={(e) => {
                setKind(e.target.value);
                list.setPage(0);
              }}
            >
              <option value="all">All assignments</option>
              <option value="merchant">With a merchant</option>
              <option value="category">Category or group only</option>
            </select>
          </Field>
          <Field label="Direction">
            <select
              value={direction}
              onChange={(e) => {
                setDirection(e.target.value);
                list.setPage(0);
              }}
            >
              <option value="all">All directions</option>
              <option value="any">Money in or out</option>
              <option value="debit">Money out</option>
              <option value="credit">Money in</option>
            </select>
          </Field>
          <Field label="Account">
            <select
              value={account}
              onChange={(e) => {
                setAccount(e.target.value);
                list.setPage(0);
              }}
            >
              <option value="">All accessible accounts</option>
              {data.accounts.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </select>
          </Field>
        </div>
      </details>
      {filtered && (
        <Button variant="quiet" onClick={clear}>
          Clear filters
        </Button>
      )}
      <ListStatus list={list} />
      {!list.loading && !list.error && (
        <p className="footnote" role="status">
          {list.total} matching {list.total === 1 ? "rule" : "rules"}
        </p>
      )}
      {!list.loading && !list.error && !list.items.length && (
        <Empty
          kind={filtered ? "filtered" : "categories"}
          title={filtered ? "No rules found" : "No rules yet"}
        >
          <p>
            {filtered
              ? "Try another search or clear filters."
              : "Create a rule to recognize merchants or categorize future transactions."}
          </p>
          {filtered ? (
            <Button onClick={clear}>Clear filters</Button>
          ) : (
            canCreate && (
              <Button variant="primary" onClick={() => setEdit(null)}>
                Add rule
              </Button>
            )
          )}
        </Empty>
      )}
      {list.items.map((rule: Row) => (
        <div
          className={
            "rule-row rule-list-row" +
            (rule.merchant_id ? " merchant-rule-row" : "")
          }
          key={rule.rule_kind + ":" + rule.id}
        >
          {rule.merchant_id && (
            <MerchantAvatar
              name={rule.merchant_name}
              logo={rule.merchant_logo}
            />
          )}
          <div className="rule-list-content">
            <strong>{rule.pattern}</strong>
            <small>
              {[
                rule.merchant_name,
                rule.category_name,
                rule.spending_group_name,
              ]
                .filter(Boolean)
                .join(" · ")}
            </small>
            <small>
              {rule.rows.length > 1
                ? rule.rows.length + " current accounts"
                : rule.account_name}
              {rule.direction !== "any"
                ? " · " +
                  (rule.direction === "debit" ? "Money out" : "Money in")
                : ""}
            </small>
          </div>
          <div className="rule-list-actions">
            <StatusIcon
              label={rule.enabled ? "Active" : "Paused"}
              icon={rule.enabled ? CircleCheck : Pause}
              tone={rule.enabled ? "good" : "neutral"}
            />
            {!!rule.builtin && (
              <StatusIcon label="Global fallback" icon={Globe} />
            )}
            {rule.editable && (
              <ActionMenu label={"Actions for rule " + rule.pattern}>
                <Button
                  variant="quiet"
                  disabled={busy}
                  onClick={() => setEdit(rule)}
                >
                  Edit
                </Button>
                <Button
                  variant="quiet"
                  disabled={busy}
                  onClick={() =>
                    run(
                      async () => {
                        const payload = rulePayload(rule);
                        payload.rule.enabled = !rule.enabled;
                        payload.merchant_rule.enabled = !rule.enabled;
                        await api("/unified-rules", "POST", payload);
                        refresh();
                      },
                      rule.enabled ? "Rule paused" : "Rule resumed",
                    )
                  }
                >
                  {rule.enabled ? "Pause" : "Resume"}
                </Button>
                <Button
                  variant="quiet"
                  disabled={busy || !rule.enabled}
                  onClick={() => setBackfill(rule)}
                >
                  Apply to previous transactions
                </Button>
                <Button
                  variant="quiet"
                  disabled={busy}
                  onClick={() => setDeleting(rule)}
                >
                  Delete
                </Button>
              </ActionMenu>
            )}
          </div>
        </div>
      ))}
      <ListNavigation list={list} />
      {edit !== undefined && (
        <UnifiedRuleEditor
          rule={edit || undefined}
          data={data}
          notify={notify}
          refresh={refresh}
          onClose={() => setEdit(undefined)}
        />
      )}
      {backfill && (
        <RuleBackfill
          canSelectPeriod={!!data.user.budget_member}
          refs={ruleRefs(backfill)}
          notify={notify}
          refresh={refresh}
          onClose={() => setBackfill(null)}
        />
      )}
      {deleting && (
        <Modal
          title="Delete rule"
          onClose={() => {
            if (!busy) setDeleting(null);
          }}
        >
          <Form
            onSubmit={() =>
              run(async () => {
                await api("/unified-rules", "POST", {
                  ...rulePayload(deleting),
                  delete: true,
                });
                refresh();
                setDeleting(null);
              }, "Rule deleted")
            }
          >
            <p>
              Delete the rule matching “{deleting.pattern}”
              {deleting.rows.length > 1
                ? " from all " + deleting.rows.length + " listed accounts"
                : ""}
              ? Existing transactions keep their assignments.
            </p>
            <div className="editor-actions">
              <Button type="submit" loading={busy}>
                Delete rule
              </Button>
              <Button disabled={busy} onClick={() => setDeleting(null)}>
                Cancel
              </Button>
            </div>
          </Form>
        </Modal>
      )}
    </section>
  );
}
