import type { Row } from "../../shared/types";

const statusLabels: Record<string, string> = {
  queued: "Request accepted · waiting to start",
  running: "Bank refresh in progress",
  succeeded: "Bank refresh completed",
  needs_review: "Fetched · import exceptions need review",
  partial: "Partly completed · check the saved results",
  failed: "Bank refresh failed",
  cancelled: "Bank refresh cancelled",
  interrupted: "Bank refresh interrupted",
};

export function BankRefreshJobs({ jobs }: { jobs: Row[] }) {
  if (!jobs.length) return null;
  return <section className="panel">
    <h2>Agent bank refreshes</h2>
    <p className="muted">Accepted requests run in the background. Use Refresh proposals to check their latest results.</p>
    {jobs.map(job => <article className="mcp-proposal" key={job.job_id}>
      <h3>{statusLabels[job.status] || "Result unavailable"}</h3>
      <p>{job.mode.replaceAll("_", " ")} · {job.accounts.map((account: Row) => account.label).join(", ")}</p>
      <p>{job.totals.balances_updated} balances updated · {job.totals.inserted} transactions imported · {job.totals.duplicates} duplicates recognised · {job.totals.exceptions} import exceptions</p>
      {job.totals.possible_gap && <p className="footnote">Recent bank history may be incomplete. A completed refresh does not guarantee full coverage.</p>}
      {job.recovery && <p>{job.recovery}</p>}
      <small>Requested {new Date(job.created_at * 1000).toLocaleString("en-ZA")} · Job {job.job_id.slice(0, 8)}</small>
    </article>)}
  </section>;
}
