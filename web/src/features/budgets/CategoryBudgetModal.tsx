import { useEffect, useState } from "react";
import { Plus } from "lucide-react";
import { api, cents, decimal } from "../../api";
import { CreateCategory } from "../../Choices";
import { PagedSelect } from "../../PagedList";
import { Button, Field, Form, Loading, Modal } from "../../ui";
import { moneyError } from "../../validation";
import { useTask } from "../../shared/useTask";
import type { PageProps, Row } from "../../shared/types";
import { CategoryBudgetAlert, useCategoryBudgetAlert } from "../notifications/CategoryBudgetAlert";

export function CategoryBudgetModal({ category, group, period, periodName, onClose, onSaved, notify, alertAvailable = true }: {
  category?: Row; group: Row; period: string; periodName?: string; onClose: () => void;
  onSaved: (amount: number) => void; notify: PageProps["notify"]; alertAvailable?: boolean;
}) {
  const [selected, setSelected] = useState<Row | undefined>(category);
  const [categoryID, setCategoryID] = useState(category ? String(category.id) : "");
  const [creating, setCreating] = useState(false);
  const [amount, setAmount] = useState("0.00");
  const [future, setFuture] = useState(false);
  const [included, setIncluded] = useState(false);
  const [version, setVersion] = useState<number | null>(null);
  const [loading, setLoading] = useState(!!category);
  const [loadFailed, setLoadFailed] = useState(false);
  const [saveFailed, setSaveFailed] = useState(false);
  const [reload, setReload] = useState(0);
  const { busy, run } = useTask(notify);
  const alert = useCategoryBudgetAlert(Number(categoryID), group.id || 0, alertAvailable && !!categoryID, notify);
  useEffect(() => {
    if (!categoryID) return;
    let current = true;
    setLoading(true);
    setLoadFailed(false);
    setVersion(null);
    Promise.all([
      api("/periods?page=0&id=" + period),
      api("/periods/" + period + "/targets?group=" + (group.id || 0) + "&budget_only=1&page=0&id=" + categoryID),
    ]).then(([periods, targets]) => {
      if (!current) return;
      const savedPeriod = periods.items?.[0];
      if (!savedPeriod) throw new Error("Budget period unavailable. Reload the budget.");
      const item = targets.items?.[0];
      setAmount(decimal(item?.amount_cents || 0));
      setFuture(!!item?.carry_forward);
      setIncluded(!!item);
      setVersion(savedPeriod.version);
    }).catch(error => {
      if (current) { setLoadFailed(true); notify(error.message, true); }
    }).finally(() => { if (current) setLoading(false); });
    return () => { current = false; };
  }, [period, group.id, categoryID, reload]);
  const ready = !!categoryID && version !== null && !loading && !loadFailed && (!alertAvailable || (!alert.loading && !alert.failed && alert.draft?.category_id === Number(categoryID)));
  const save = async (remove = false) => {
    if (!ready) return;
    const targetCents = remove ? 0 : cents(amount);
    if (await run(() => api("/periods/" + period + "/budget", "PUT", {
      version,
      ...(!remove && alertAvailable && alert.changed && alert.draft ? {
        alert_preferences: [{ ...alert.draft, threshold: alert.draft.threshold === "" ? null : Number(alert.draft.threshold) }],
      } : {}),
      groups: [{ group_id: group.id || 0, remove: false, targets: [{
        category_id: Number(categoryID), amount_cents: targetCents,
        carry_forward: future, apply_upcoming: future, remove,
      }] }],
    }), remove ? "Budget limit removed" : "Changes saved", () => { setSaveFailed(true); return false; })) {
      onSaved(targetCents);
      onClose();
    }
  };
  const chooseCategory = (id: string) => {
    setCategoryID(id); setVersion(null); setLoading(!!id); setLoadFailed(false); setSaveFailed(false);
    setAmount("0.00"); setFuture(false); setIncluded(false);
  };
  return <Modal size="compact" title={category ? "Edit budget · " + category.name : "Add category · " + group.name} onClose={() => { if (!busy && !creating) onClose(); }}>
    <Form onSubmit={() => void save()}>
      <p className="muted">{periodName || "Selected budget period"} · {group.name || "No spending group"}</p>
      {!category && <>
        <PagedSelect url="/categories?kind=expense&active=1" label="Category" required value={categoryID}
          onChange={chooseCategory} onSelectRow={setSelected} options={selected ? [selected] : []} />
        <Button className="category-budget-create" disabled={busy} onClick={() => setCreating(true)}><Plus size={16} />Create expense category</Button>
      </>}
      {category && <p className="muted">Set the budget limit for <strong>{category.name}</strong>.</p>}
      {loading && <Loading>Loading category budget</Loading>}
      {loadFailed && <Button onClick={() => setReload(value => value + 1)} disabled={busy}>Reload budget</Button>}
      <Field label="Budget amount" validate={value => moneyError(value, true)}>
        <input autoFocus={!!category} inputMode="decimal" required value={amount} disabled={busy || loading || loadFailed || !categoryID} onChange={event => setAmount(event.target.value)} />
      </Field>
      <Field label="Use this category in">
        <select value={future ? "future" : "once"} disabled={busy || loading || loadFailed || !categoryID} onChange={event => setFuture(event.target.value === "future")}>
          <option value="once">This budget only</option>
          <option value="future">This and upcoming budgets</option>
        </select>
      </Field>
      {future && <p className="footnote">Use this amount in new periods and upcoming budgets where this category isn't already included. Existing amounts stay as set.</p>}
      {alertAvailable && <CategoryBudgetAlert value={alert.draft?.category_id === Number(categoryID) ? alert.draft : null}
        onChange={alert.setDraft} loading={alert.loading} failed={alert.failed} onReload={alert.reload} busy={busy} saveFailed={saveFailed} />}
      {saveFailed && <Button variant="quiet" disabled={busy || loading} onClick={() => setReload(value => value + 1)}>Reload budget</Button>}
      <div className="editor-actions">
        <Button type="submit" variant="primary" loading={busy} disabled={busy || (!!categoryID && !ready)}>Save changes</Button>
        {included && <Button variant="quiet" disabled={busy || !ready} onClick={() => void save(true)}>Remove limit</Button>}
        <Button onClick={onClose} disabled={busy || creating}>Cancel</Button>
      </div>
    </Form>
    {creating && <Modal size="compact" title="Create expense category" onClose={() => setCreating(false)}>
      <CreateCategory name="" expenseOnly notify={notify} done={(id, name) => {
        chooseCategory(String(id)); setSelected({ id, name: name || "New category" }); setCreating(false);
      }} />
    </Modal>}
  </Modal>;
}
