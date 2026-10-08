import { useEffect, useState } from "react";
import { Bell, BellOff, RotateCcw } from "lucide-react";
import { api } from "../../api";
import { Button, Field } from "../../ui";
import type { PageProps } from "../../shared/types";
import type { BudgetAlertPreference } from "./contracts";

export type BudgetAlertDraft = {
  category_id: number; group_id: number; enabled: boolean; threshold: string; version: number;
};
const draftOf = (item: BudgetAlertPreference): BudgetAlertDraft => ({
  category_id: item.category_id, group_id: item.group_id, enabled: item.enabled,
  threshold: item.threshold === null ? "" : String(item.threshold), version: item.version,
});
export function useCategoryBudgetAlert(categoryID: number, groupID: number, available: boolean, notify: PageProps["notify"]) {
  const [draft, setDraft] = useState<BudgetAlertDraft | null>(null);
  const [saved, setSaved] = useState<BudgetAlertDraft | null>(null);
  const [loading, setLoading] = useState(available);
  const [failed, setFailed] = useState(false);
  const [reload, setReload] = useState(0);
  useEffect(() => {
    if (!available) return;
    let current = true;
    setLoading(true);
    setFailed(false);
    api<{ budget_items: BudgetAlertPreference[] }>("/notifications/preferences?channels=all")
      .then(result => {
        if (!current) return;
        const item = result.budget_items.find(item => item.category_id === categoryID && item.group_id === groupID);
        const value = item ? draftOf(item) : { category_id: categoryID, group_id: groupID, enabled: true, threshold: "", version: 0 };
        setSaved(value);
        setDraft(value);
      })
      .catch(error => { if (current) { setFailed(true); notify(error.message, true); } })
      .finally(() => { if (current) setLoading(false); });
    return () => { current = false; };
  }, [categoryID, groupID, available, reload]);
  const changed = draft !== null && saved !== null && (draft.enabled !== saved.enabled || draft.threshold !== saved.threshold);
  return { draft, setDraft, loading, failed, changed, reload: () => setReload(value => value + 1) };
}
export function CategoryBudgetAlert({ value, onChange, loading, failed, onReload, busy, saveFailed }: {
  value: BudgetAlertDraft | null; onChange: (value: BudgetAlertDraft) => void;
  loading: boolean; failed: boolean; onReload: () => void; busy: boolean; saveFailed: boolean;
}) {
  if (failed) return <div><Button variant="quiet" onClick={onReload} disabled={busy}>Reload alert settings</Button></div>;
  const ToggleIcon = value?.enabled ? Bell : BellOff;
  return <div className="category-alert-form">
    <div className="category-alert-heading">
      <span>Budget alerts</span>
      <div className="category-alert-heading-actions">
        <Button variant="quiet" className="category-alert-toggle" disabled={busy || loading || !value || (value.threshold === "" && value.enabled)}
          aria-label="Reset alert to defaults" title="Reset alert to defaults" onClick={() => value && onChange({ ...value, enabled: true, threshold: "" })}>
          <RotateCcw size={16} aria-hidden="true" />
        </Button>
        <Button variant="quiet" className="category-alert-toggle" aria-label={value?.enabled ? "Mute budget alerts" : "Enable budget alerts"}
          title={value?.enabled ? "Mute budget alerts" : "Enable budget alerts"} aria-pressed={value?.enabled || false}
          disabled={loading || busy || !value} onClick={() => value && onChange({ ...value, enabled: !value.enabled })}>
          <ToggleIcon size={18} aria-hidden="true" />
        </Button>
      </div>
    </div>
    <Field label="Alert threshold (%)" hint="Leave blank for 75%, 90% and 100%." validate={value => value === "" || (/^\d+$/.test(value) && Number(value) >= 1 && Number(value) <= 100) ? "" : "Use a whole percentage from 1 to 100."}>
      <input inputMode="numeric" value={value?.threshold || ""} disabled={loading || busy || !value}
        onChange={event => value && onChange({ ...value, threshold: event.target.value })} />
    </Field>
    {saveFailed && <Button variant="quiet" disabled={busy || loading} onClick={onReload}>Reload alert</Button>}
  </div>;
}
