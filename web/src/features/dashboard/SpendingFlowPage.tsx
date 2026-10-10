import { useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { api } from "../../api";
import { Button, Empty, Loading } from "../../ui";
import type { PageProps } from "../../shared/types";
import type { SpendingFlowEntry } from "./spendingFlow";
import { SpendingSankey } from "./SpendingSankey";

type Summary = { spent_cents: number; spending_breakdown: SpendingFlowEntry[] };

export function SpendingFlowPage({ period, account, revision, notify, refresh, onBack }: PageProps & {
  period: string;
  account: string;
  onBack: () => void;
}) {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    let alive = true;
    setLoading(true);
    setError("");
    setSummary(null);
    const query = new URLSearchParams({ period, account });
    api<Summary>("/dashboard?" + query)
      .then(value => { if (alive) setSummary(value); })
      .catch(error => {
        if (alive) {
          setError(error.message);
          notify(error.message, true);
        }
      })
      .finally(() => { if (alive) setLoading(false); });
    return () => { alive = false; };
  }, [period, account, revision]);

  return (
    <>
      <div className="toolbar">
        <Button variant="quiet" onClick={onBack} autoFocus>
          <ArrowLeft size={18} aria-hidden="true" />Back to dashboard
        </Button>
      </div>
      {loading ? <Loading>Loading this period</Loading> : error ? (
        <Empty kind="error" title="Spending flow unavailable">
          <p>{error}</p><Button variant="primary" onClick={refresh}>Retry spending flow</Button>
        </Empty>
      ) : summary && <SpendingSankey entries={summary.spending_breakdown} total={summary.spent_cents} />}
    </>
  );
}
