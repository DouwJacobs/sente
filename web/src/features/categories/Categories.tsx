import { SpendingGroupEditor } from "../../SpendingGroupEditor";
import { CategoryEditor, MerchantRules } from "../../Organisation";
import { useTransactionAccess } from "../../TransactionAccess";
import { usePagedList, ListStatus, ListNavigation } from "../../PagedList";
import { Rules } from "../../Rules";
import { GroupDot } from "../../Choices";
import { useEffect, useState } from "react";
import { Plus, Pencil, ChevronRight, CircleMinus, CirclePlus } from "lucide-react";
import { api } from "../../api";
import { Button, Field, Form, StatusIcon, Empty, Modal, Tabs } from "../../ui";
import { useTask } from "../../shared/useTask";
import { type PageProps, type Row } from "../../shared/types";
export function Categories({
  data,
  refresh,
  notify,
  revision,
  onAccounts,
  tab: externalTab,
  onTabChange,
}: PageProps & {
  onAccounts: () => void;
  tab?: string;
  onTabChange?: (tab: string) => void;
}) {
  const { viewTransactions } = useTransactionAccess();
  const [spendingEdit, setSpendingEdit] = useState<Row | null>(null);
  const [creating, setCreating] = useState(false),
    [name, setName] = useState(""),
    [kind, setKind] = useState("expense");
  const [categoryEdit, setCategoryEdit] = useState<Row | null>(null);
  const [internalTab, setInternalTab] = useState("categories");
  const categoryTab = externalTab || internalTab;
  const changeTab = (t: string) => {
    setInternalTab(t);
    onTabChange?.(t);
  };
  const categoryList = usePagedList("/categories", revision),
    groupList = usePagedList("/spending-groups", revision);
  const [nameError, setNameError] = useState("");
  useEffect(() => {
    setNameError("");
  }, [creating]);
  const { busy, run } = useTask(notify);
  return (
    <>
      <div className="classification-layout">
        <aside className="classification-nav">
          <Tabs
            id="classification"
            label="Categories and rules"
            items={[
              { id: "categories", label: "Categories" },
              { id: "groups", label: "Spending groups" },
              { id: "rules", label: "Automatic rules" },
              { id: "merchants", label: "Merchant rules" },
            ]}
            value={categoryTab}
            onChange={changeTab}
          />
        </aside>
        <div
          role="tabpanel"
          id={"classification-panel-" + categoryTab}
          aria-labelledby={"classification-tab-" + categoryTab}
        >
          {categoryTab === "merchants" && (
            <MerchantRules
              data={data}
              refresh={refresh}
              notify={notify}
              revision={revision}
            />
          )}
          {categoryEdit && (
            <CategoryEditor
              category={categoryEdit}
              notify={notify}
              onClose={() => setCategoryEdit(null)}
              onDone={() => {
                setCategoryEdit(null);
                refresh();
              }}
            />
          )}
          {categoryTab === "rules" && (
            <Rules
              onAccounts={onAccounts}
              onCreateCategory={() => setCreating(true)}
              data={data}
              refresh={refresh}
              notify={notify}
              revision={revision}
            />
          )}

          {categoryTab === "groups" && (
            <section className="panel">
              <div className="section-head">
                <div>
                  <h2>Spending groups</h2>
                  <p className="muted">
                    Organize transactions with an extra label, such as
                    Essentials or Leisure. This does not change their budget
                    category.
                  </p>
                </div>
                {data.user.budget_member && (
                  <Button
                    onClick={() => {
                      setSpendingEdit({});
                    }}
                  >
                    <Plus size={16} />
                    Add spending group
                  </Button>
                )}
              </div>
              <ListStatus list={groupList} />
              {!groupList.loading && !groupList.error && !groupList.items.length && <Empty kind="categories" title="Organize spending your way"><p>Use groups such as Essentials or Leisure to organize transactions without changing their categories.</p>{data.user.budget_member && <Button variant="primary" onClick={() => setSpendingEdit({})}>Create spending group</Button>}</Empty>}
              <div className="spending-group-grid">
                {groupList.items.map((g) => (
                  <div className="classification-group-entry" key={g.id}>
                    <button
                      type="button"
                      className="choice-row"
                      title={"View transactions for " + g.name}
                      onClick={() => viewTransactions({ group: String(g.id) })}
                    >
                      <GroupDot color={g.color} />
                      <span>{g.name}</span>
                      <ChevronRight size={16} aria-hidden="true" />
                    </button>
                    {data.user.budget_member && (
                      <Button
                        variant="quiet"
                        aria-label={"Edit " + g.name}
                        title={"Edit " + g.name}
                        onClick={() => {
                          setSpendingEdit(g);
                        }}
                      >
                        <Pencil size={16} aria-hidden="true" />
                      </Button>
                    )}
                  </div>
                ))}
              </div>
              <ListNavigation list={groupList} />
            </section>
          )}
          {categoryTab === "categories" && (
            <section className="panel">
              <div className="section-head">
                <div>
                  <h2>Categories</h2>
                  <p className="muted">
                    Limits are set on expense categories in Budgets.
                  </p>
                </div>
                {data.user.budget_member && (
                  <Button onClick={() => setCreating(true)}>
                    <Plus size={16} />
                    Add category
                  </Button>
                )}
              </div>
              <ListStatus list={categoryList} />
              {!categoryList.loading && !categoryList.error && !categoryList.items.length ? (
                <Empty kind="categories" title="Create your first categories">
                  <p>Give income and expenses a clear home, such as Salary, Groceries or Transport.</p>
                  {data.user.budget_member && <Button variant="primary" onClick={() => setCreating(true)}>Add category</Button>}
                </Empty>
              ) : (
                <div className="spending-group-grid category-list">
                  {categoryList.items.map((c) => (
                    <div className="classification-group-entry" key={c.id}>
                      <button
                        type="button"
                        className="choice-row classification-category-row"
                        key={c.id}
                        title={"View transactions for " + c.name}
                        onClick={() =>
                          viewTransactions({ category: String(c.id) })
                        }
                      >
                        <span>
                          {c.name}
                          {c.archived ? " · Archived" : ""}
                        </span>
                        <span className="toolbar-actions">
                          <StatusIcon label={c.kind === "expense" ? "Expense" : "Income"} icon={c.kind === "expense" ? CircleMinus : CirclePlus}/>
                          <ChevronRight size={16} aria-hidden="true" />
                        </span>
                      </button>
                      {data.user.budget_member && (
                        <Button
                          variant="quiet"
                          aria-label={"Edit " + c.name}
                          onClick={() => setCategoryEdit(c)}
                        >
                          <Pencil size={16} />
                        </Button>
                      )}
                    </div>
                  ))}
                </div>
              )}
              <ListNavigation list={categoryList} />
            </section>
          )}
        </div>
      </div>
      {spendingEdit && (
        <SpendingGroupEditor
          group={spendingEdit}
          notify={notify}
          onClose={() => setSpendingEdit(null)}
          onDone={() => {
            setSpendingEdit(null);
            refresh();
          }}
        />
      )}
      {creating && (
        <Modal title="Add category" onClose={() => setCreating(false)}>
          <Form
            onSubmit={async (e) => {
              e.preventDefault();
              const body: Record<string, unknown> = { name, kind };
              if (
                await run(
                  () => api("/categories", "POST", body),
                  "Category created",
                  (message) => {
                    if (message === "Category already exists") {
                      setNameError(message);
                      return true;
                    }
                    return false;
                  },
                )
              ) {
                setCreating(false);
                setName("");
                refresh();
              }
            }}
          >
            <Field label="Category name" serverError={nameError}>
              <input
                required
                maxLength={80}
                value={name}
                onChange={(e) => {
                  setName(e.target.value);
                  setNameError("");
                }}
              />
            </Field>
            <Field label="Type">
              <select
                value={kind}
                onChange={(e) => {
                  setKind(e.target.value);
                }}
              >
                <option value="expense">
                  Expense (refunds reduce spending)
                </option>
                <option value="income">Income</option>
              </select>
            </Field>
            <Button
              type="submit"
              variant="primary"
              loading={busy}
              disabled={busy}
            >
              Create category
            </Button>
          </Form>
        </Modal>
      )}
    </>
  );
}
