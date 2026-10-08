import { useId, useState } from "react";
import { ChevronDown, ChevronRight, Pencil, Plus } from "lucide-react";
import { api, money } from "../../api";
import { GroupDot } from "../../Choices";
import { ListNavigation, ListStatus, PagedSelect, usePagedList } from "../../PagedList";
import { ActionMenu, Button, Empty, Form, Modal } from "../../ui";
import { useTask } from "../../shared/useTask";
import type { PageProps, Row } from "../../shared/types";
import { CategoryBudgetModal } from "./CategoryBudgetModal";

type Props = { period: Row; revision: number; notify: PageProps["notify"]; refresh: () => void };
export function BudgetGroups({ period, revision, notify, refresh }: Props) {
  const list = usePagedList("/periods/" + period.id + "/budget-groups", revision);
  const [adding, setAdding] = useState(false);
  const [groupID, setGroupID] = useState("");
  const [removing, setRemoving] = useState<Row | null>(null);
  const { busy, run } = useTask(notify);
  const changeGroup = async (id: number, remove: boolean) => {
    // The list's version belongs to the displayed budget snapshot.
    if (await run(() => api("/periods/" + period.id + "/budget", "PUT", {
      version: list.items[0]?.version ?? period.version,
      groups: [{ group_id: id, remove, targets: [] }],
    }), remove ? "Budget group removed" : "Budget group added")) {
      setAdding(false); setRemoving(null); refresh(); list.retry();
    }
  };
  return <div className="budget-groups-view" aria-label={"Category budgets for " + period.name}>
    <div className="section-head">
      <h3>Spending groups</h3>
      <Button disabled={busy || list.loading || !!list.error} onClick={() => { setGroupID(""); setAdding(true); }}><Plus size={16} />Add group</Button>
    </div>
    <ListStatus list={list} />
    {!list.loading && !list.error && !list.items.length && <Empty title="Start building your budget">Add a group, then choose its categories and amounts.</Empty>}
    {list.items.map(group => <BudgetGroup key={group.id} group={group} period={period} revision={revision} notify={notify} refresh={refresh}
      onRemove={() => setRemoving(group)} />)}
    <ListNavigation list={list} />
    {adding && <Modal size="compact" title="Add budget group" onClose={() => { if (!busy) setAdding(false); }}>
      <Form onSubmit={() => void changeGroup(Number(groupID), false)}>
        <PagedSelect url="/spending-groups" label="Spending group" required value={groupID} onChange={setGroupID}
          specialOptions={[{ value: "0", label: "No spending group" }]} />
        <div className="editor-actions">
          <Button type="submit" variant="primary" loading={busy}>Add group to budget</Button>
          <Button disabled={busy} onClick={() => setAdding(false)}>Cancel</Button>
        </div>
      </Form>
    </Modal>}
    {removing && <Modal size="compact" title={"Remove budget group · " + removing.name} onClose={() => { if (!busy) setRemoving(null); }}>
      <p>Remove {removing.name} and its category limits from {period.name}? Transaction and category history will remain available.</p>
      <div className="editor-actions">
        <Button variant="danger" loading={busy} onClick={() => void changeGroup(removing.id, true)}>Remove group</Button>
        <Button disabled={busy} onClick={() => setRemoving(null)}>Cancel</Button>
      </div>
    </Modal>}
  </div>;
}
function BudgetGroup({ group, period, revision, notify, refresh, onRemove }: Props & { group: Row; onRemove: () => void }) {
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<{ category?: Row } | null>(null);
  const id = useId();
  const Icon = open ? ChevronDown : ChevronRight;
  return <section className="budget-group" aria-label={group.name + " budget"}>
    <div className="budget-group-header">
      <button type="button" className="budget-group-summary" aria-expanded={open} aria-controls={id} onClick={() => setOpen(value => !value)}>
        <Icon size={16} aria-hidden="true" /><GroupDot color={group.color} />
        <strong className="budget-group-name">{group.name}</strong>
        <span className="budget-group-amount">{money(group.target_cents || 0)}</span>
      </button>
      <Button variant="quiet" className="budget-icon-action" aria-label={"Add category to " + group.name} title={"Add category to " + group.name}
        onClick={() => { setOpen(true); setEditing({}); }}><Plus size={18} aria-hidden="true" /></Button>
      <ActionMenu label={"Budget actions for " + group.name}><Button variant="quiet" onClick={onRemove}>Remove group</Button></ActionMenu>
    </div>
    <div id={id} hidden={!open}>
      {open && <BudgetCategories group={group} period={period} revision={revision} onEdit={category => setEditing({ category })} />}
    </div>
    {editing && <CategoryBudgetModal category={editing.category} group={group} period={String(period.id)} periodName={period.name}
      notify={notify} onClose={() => setEditing(null)} onSaved={() => refresh()} />}
  </section>;
}
function BudgetCategories({ group, period, revision, onEdit }: { group: Row; period: Row; revision: number; onEdit: (category: Row) => void }) {
  const list = usePagedList("/periods/" + period.id + "/targets?group=" + group.id + "&budget_only=1", revision);
  return <div className="budget-group-categories">
    <ListStatus list={list} />
    {!list.loading && !list.error && !list.items.length && <p className="muted">Add the categories you want to budget for in this group.</p>}
    {list.items.map(category => <div className="budget-category-row" key={category.id}>
      <div className="budget-category-name"><strong>{category.name}</strong><small>{category.carry_forward ? "This and upcoming budgets" : "This budget only"}</small></div>
      <strong className="budget-category-amount">{money(category.amount_cents)}</strong>
      <Button variant="quiet" className="budget-icon-action" aria-label={"Edit budget for " + category.name + " in " + group.name}
        title={"Edit budget for " + category.name} onClick={() => onEdit(category)}><Pencil size={16} aria-hidden="true" /></Button>
    </div>)}
    <ListNavigation list={list} />
  </div>;
}
