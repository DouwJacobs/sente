import { useEffect, useState } from "react";
import { Button } from "../../ui";
import { repositoryURL } from "../../shared/buildInfo";
import { updateLabel, useApplicationUpdate } from "../../shared/applicationUpdate";

export function ApplicationUpdateStatus() {
  const { status, pending, check } = useApplicationUpdate();
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);
  const retryAt = status?.retry_at ? Date.parse(status.retry_at) : 0;
  const coolingDown = retryAt > now;
  return (
    <section className="application-update" aria-labelledby="application-update-title">
      <h3 id="application-update-title">Application updates</h3>
      <div role="status" aria-live="polite" aria-atomic="true">
        <p><strong>{updateLabel(status)}</strong></p>
        {status?.message && <p className="muted">{status.message}</p>}
        {status?.channel && <p className="footnote">Release channel: {status.channel}</p>}
        {status?.checked_at && <p className="footnote">{status.stale ? "Last successful check (outdated): " : "Checked: "}
          <time dateTime={status.checked_at}>{new Date(status.checked_at).toLocaleString()}</time></p>}
      </div>
      {status?.release_url && <p>
        <a href={status.release_url} target="_blank" rel="noopener noreferrer">
          {status.stale ? "Previously found release" : "Release notes"}: {status.available_version}
        </a>
      </p>}
      <p className="footnote">An administrator can upgrade the Docker installation manually. Before upgrading, review release notes, stop Sente and take a backup with the installed image. Migrations can prevent an image rollback; keep a pre-upgrade snapshot and a separate backup of any FNB connector key.</p>
      <p><a href={`${repositoryURL}/blob/main/docs/RELEASES.md#upgrade-with-a-pre-upgrade-backup`} target="_blank" rel="noopener noreferrer">Upgrade and backup guide</a></p>
      {status?.state !== "unsupported" && <>
        <Button variant="secondary" loading={pending} disabled={pending || coolingDown} onClick={() => void check()}>Check for updates</Button>
        {coolingDown && <p className="footnote">You can check again in {Math.ceil((retryAt - now) / 1000)} seconds.</p>}
      </>}
    </section>
  );
}
