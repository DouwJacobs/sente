import { MCPConnectionCard } from "./features/mcp/MCPConnectionCard";
import { MCPProposalCard } from "./features/mcp/MCPProposalCard";
import { SavedMCPContext } from "./features/mcp/SavedMCPContext";
import { useEffect, useState } from "react";
import { api } from "./api";
import { Button, Field, Loading, Empty, Form, Modal } from "./ui";
import { useTask } from "./shared/useTask";
import { type PageProps, type Row } from "./shared/types";
import { MCPPermissionFields } from "./MCPPermissions";

export function MCPSettings({ notify }: Pick<PageProps, "notify" | "refresh">) {
  const [state, setState] = useState<Row | null>(null),
    [editing, setEditing] = useState<Row | null>(null),
    [draft, setDraft] = useState<Row | null>(null),
    [consent, setConsent] = useState(false),
    [selected, setSelected] = useState<string[]>([]);
  const { busy, run } = useTask(notify);
  const load = async () => {
    const next = await api("/mcp/settings");
    setState(next);
    setSelected((ids) =>
      ids.filter((id) =>
        next.proposals.some((p: Row) => p.id === id && p.status === "pending"),
      ),
    );
  };
  useEffect(() => {
    let alive = true;
    api("/mcp/settings")
      .then((s) => alive && setState(s))
      .catch((e) => alive && notify(e.message, true));
    return () => {
      alive = false;
    };
  }, []);
  const endpoint = state?.endpoint || "";
  const copy = (value: string) =>
    run(async () => {
      await navigator.clipboard.writeText(value);
    }, "Copied");
  const pending: Row[] =
    state?.proposals.filter((p: Row) => p.status === "pending") || [];
  const approveBatch = (ids: string[]) =>
    run(async () => {
      await api("/mcp/proposals/batch", "POST", {
        proposal_ids: ids,
        approve: true,
      });
      await load();
    }, `${ids.length} proposals approved; your agents can now apply them`);
  return (
    <>
      <section className="panel mcp-connection-panel">
        <h2>Connect an agent</h2>
        <p className="muted">
          Give an agent access to Sente, with permissions you choose for each
          connection.
        </p>
        <ol className="mcp-connect-steps">
          <li>
            Copy the endpoint into an agent that supports remote MCP and OAuth.
          </li>
          <li>
            Sign in to Sente and choose what that agent can read or propose.
          </li>
          <li>Review its connection and proposed changes below.</li>
        </ol>
        <details className="mcp-privacy">
          <summary>What information is shared?</summary>
          <p>
            Accessible transactions, categories, rules and budgets include
            financial amounts and dates. Account names use generic labels;
            account numbers, bank login details, original import details and
            notes are excluded. Long numbers, email addresses and URLs are
            removed from descriptions, but names and other personal text may
            remain.
          </p>
          <p>
            Merchant details and your saved context are shared only when you
            enable them for that agent. Context is shared as written.
          </p>
        </details>
        {!state ? (
          <Loading>Loading MCP settings</Loading>
        ) : (
          <>
            <Field label="MCP server endpoint">
              <input readOnly value={endpoint} />
            </Field>
            <div className="editor-actions">
              <Button onClick={() => copy(endpoint)}>Copy endpoint</Button>
            </div>
            <p className="footnote">
              The endpoint also provides setup instructions for your agent.
            </p>
          </>
        )}
      </section>
      <SavedMCPContext notify={notify} />
      <section className="panel">
        <div className="section-head">
          <h2>Connected agents</h2>
          <Button disabled={busy} onClick={() => run(load)}>
            Refresh connections
          </Button>
        </div>
        <p className="muted">
          Manage each agent’s access and approval settings. Revoking disconnects
          the agent and removes its pending proposals.
        </p>
        {state && !state.connections.length && (
          <Empty title="No connected agents">
            Connect using the endpoint above, then approve the browser request.
          </Empty>
        )}
        {state?.connections.map((connection: Row) => (
          <MCPConnectionCard
            key={connection.id}
            connection={connection}
            accounts={state.accounts}
            busy={busy}
            onEdit={(t) => {
              setEditing(t);
              setDraft({
                ...t.permissions,
                preset:
                  t.permissions.preset === "legacy_finance"
                    ? "custom"
                    : t.permissions.preset,
              });
              setConsent(false);
            }}
            onRevoke={(t) =>
              run(async () => {
                await api("/mcp/connections/" + t.id, "DELETE");
                await load();
              }, "MCP connection revoked")
            }
          />
        ))}
      </section>
      <section className="panel">
        <div className="section-head">
          <h2>Agent change proposals</h2>
          <Button disabled={busy} loading={busy} onClick={() => run(load)}>
            Refresh proposals
          </Button>
        </div>
        <p className="muted">
          Review the exact changes before approving. Approval lets the agent
          apply only those changes; it does not apply them immediately.
          Proposals expire after one hour. Automatic approval covers only the
          change types you enabled for that agent.
        </p>
        {state && !state.proposals.length && (
          <Empty title="No change proposals">
            Ask your agent to prepare a change, then refresh this list. Approved
            proposals remain here until applied or expired.
          </Empty>
        )}
        {pending.length > 0 && (
          <div className="editor-actions">
            <label className="check">
              <input
                type="checkbox"
                aria-label="Select all pending proposals"
                disabled={busy}
                checked={pending.every((p) => selected.includes(p.id))}
                onChange={(e) =>
                  setSelected(e.target.checked ? pending.map((p) => p.id) : [])
                }
              />
              Select all pending proposals
            </label>
            <span>{selected.length} selected</span>
            <Button
              variant="primary"
              disabled={busy || !selected.length}
              onClick={() => approveBatch(selected)}
            >
              Approve selected ({selected.length})
            </Button>
            <Button
              disabled={busy}
              onClick={() => approveBatch(pending.map((p) => p.id))}
            >
              Approve all shown ({pending.length})
            </Button>
          </div>
        )}
        {state?.proposals.map((proposal: Row) => (
          <MCPProposalCard
            key={proposal.id}
            proposal={proposal}
            busy={busy}
            selected={selected.includes(proposal.id)}
            onSelect={(checked) =>
              setSelected((ids) =>
                checked
                  ? [...ids, proposal.id]
                  : ids.filter((id) => id !== proposal.id),
              )
            }
            onDecide={(approve) =>
              run(
                async () => {
                  await api("/mcp/proposals/" + proposal.id, "POST", {
                    approve,
                  });
                  await load();
                },
                approve
                  ? "Proposal approved; your agent can now apply it"
                  : "Proposal rejected",
              )
            }
          />
        ))}
      </section>
      {editing && draft && (
        <Modal
          size="wide"
          title={"Permissions for " + editing.name}
          onClose={() => {
            if (!busy) setEditing(null);
          }}
        >
          <Form
            className="mcp-permission-form"
            onSubmit={() =>
              run(async () => {
                await api(
                  "/mcp/connections/" + editing.id + "/permissions",
                  "PUT",
                  {
                    version: editing.permission_version,
                    permissions: draft,
                    consent,
                  },
                );
                await load();
                setEditing(null);
              }, "Connection permissions updated")
            }
          >
            <MCPPermissionFields
              value={draft}
              onChange={(v) => {
                setDraft(v);
                setConsent(false);
              }}
              accounts={state?.accounts || []}
              canPropose={!!editing.can_write}
            />
            {!editing.can_write && (
              <p className="footnote">
                To allow proposals, start a new connection from the agent and
                approve proposal access.
              </p>
            )}
            <Field
              label="Confirm permissions"
              validate={() =>
                consent ? "" : "Confirm the permissions shown above."
              }
            >
              <select
                value={consent ? "confirmed" : ""}
                onChange={(e) => setConsent(e.target.value === "confirmed")}
              >
                <option value="">Review permissions above</option>
                <option value="confirmed">
                  I approve these connection permissions
                </option>
              </select>
            </Field>
            <div className="editor-actions">
              <Button
                type="submit"
                variant="primary"
                disabled={busy}
                loading={busy}
              >
                Save permissions
              </Button>
              <Button disabled={busy} onClick={() => setEditing(null)}>
                Cancel
              </Button>
            </div>
          </Form>
        </Modal>
      )}
    </>
  );
}
