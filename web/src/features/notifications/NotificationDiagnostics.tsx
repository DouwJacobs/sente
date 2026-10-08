import { useEffect, useState } from "react";
import { api } from "../../api";
import { Button, Field, Loading, Pagination } from "../../ui";
import type { PageProps } from "../../shared/types";
import { notificationLabels } from "./contracts";

type Diagnostic = { id: number; user_id: number; username: string; type: string; key_hash: string; evaluated_at: number; channel: string; status: string; error_code: string };
type Results = { items: Diagnostic[]; total: number };
const statuses = ["resolved", "delivered", "disabled", "deduplicated", "cooldown_suppressed", "coalesced", "no_longer_triggered", "pending", "sent", "channel_failure", "permission_revoked", "expired", "uncertain", "no_device"];
export function NotificationDiagnostics({ notify }: Pick<PageProps, "notify">) {
  const [kind, setKind] = useState(""), [status, setStatus] = useState(""), [user, setUser] = useState("");
  const [days, setDays] = useState("7"), [page, setPage] = useState(0), [revision, setRevision] = useState(0);
  const [result, setResult] = useState<Results | null>(null), [loading, setLoading] = useState(true), [failed, setFailed] = useState(false);
  useEffect(() => {
    let current = true;
    setLoading(true); setFailed(false); setResult(null);
    const params = new URLSearchParams({ type: kind, status, user, days, page: String(page), page_size: "20" });
    api<Results>("/notifications/diagnostics?" + params)
      .then(value => { if (current) setResult(value); })
      .catch(error => { if (current) { setFailed(true); notify(error.message, true); } })
      .finally(() => { if (current) setLoading(false); });
    return () => { current = false; };
  }, [kind, status, user, days, page, revision]);
  return <section className="panel notification-diagnostics" aria-label="Notification diagnostics" aria-busy={loading}>
    <div className="section-head"><h2>Notification diagnostics</h2><Button disabled={loading} onClick={() => setRevision(value => value + 1)}>Refresh diagnostics</Button></div>
    <p className="muted">Administrator metadata only. Financial messages and push secrets are omitted. Retained for 14 days, up to 10,000 entries. Routine checks are hidden; only alert changes and delivery attempts are recorded.</p>
    <div className="notification-diagnostic-filters">
      <Field label="Notification type"><select value={kind} onChange={event => { setKind(event.target.value); setPage(0); }}><option value="">All types</option>{Object.entries(notificationLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></Field>
      <Field label="Delivery status"><select value={status} onChange={event => { setStatus(event.target.value); setPage(0); }}><option value="">All statuses</option>{statuses.map(value => <option key={value} value={value}>{value.replaceAll("_", " ")}</option>)}</select></Field>
      <Field label="Recipient user ID"><input type="number" min="1" step="1" value={user} onChange={event => { const value = event.target.value; setUser(/^[1-9]\d*$/.test(value) ? value : ""); setPage(0); }} /></Field>
      <Field label="Recent history"><select value={days} onChange={event => { setDays(event.target.value); setPage(0); }}><option value="1">24 hours</option><option value="7">7 days</option><option value="14">14 days</option></select></Field>
    </div>
    {loading ? <Loading>Loading diagnostics</Loading> : failed ? <Button onClick={() => setRevision(value => value + 1)}>Retry diagnostics</Button> : result && <>
      <p role="status">{result.total} alert changes and delivery attempts</p>
      {!result.items.length ? <p>No diagnostics match this filter.</p> : <ul className="notification-diagnostic-list">{result.items.map(item => <li className="notification-diagnostic-row" key={item.id}>
        <div className="notification-diagnostic-summary"><strong>{notificationLabels[item.type]} · {item.status.replaceAll("_", " ")}</strong><span>{item.username} · {item.channel === "in_app" ? "In-app" : "Browser push"}</span></div>
        <div className="notification-diagnostic-meta muted"><time>{new Date(item.evaluated_at * 1000).toLocaleString()}</time>{item.error_code && ` · ${item.error_code.replaceAll("_", " ")}`}</div>
        <code className="notification-key muted" aria-label="Logical event fingerprint">{item.key_hash}</code>
      </li>)}</ul>}
      <Pagination total={result.total} page={page} loading={loading} onChange={setPage} />
    </>}
  </section>;
}
