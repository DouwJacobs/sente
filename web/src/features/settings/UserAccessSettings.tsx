import { ListStatus, ListNavigation, PagedSelect } from "../../PagedList";
import { Plus } from "lucide-react";
import { api } from "../../api";
import { ActionMenu, Button, Field, Form, Badge } from "../../ui";
import { useTask } from "../../shared/useTask";
import { type Row } from "../../shared/types";
import type { useUserAccess } from "./useUserAccess";
export function UserAccessSettings({
  access,
  users,
  manageAccounts,
  revision,
  refresh,
  busy,
  run,
}: {
  access: ReturnType<typeof useUserAccess>;
  users: Row[];
  manageAccounts: Row[];
  revision: number;
  refresh: () => void;
  busy: boolean;
  run: ReturnType<typeof useTask>["run"];
}) {
  const { userList, grantList, setEditUser, setAddUser,
    grantUser, setGrantUser, grantAccount, setGrantAccount, role, setRole,
    currentUserID, setSecurityUser } = access;
  return (
    <>
      <section className="panel">
        <div className="section-head">
          <h2>Users</h2>
          <Button onClick={() => setAddUser(true)}>
            <Plus size={16} />
            Add user
          </Button>
        </div>
        <ListStatus list={userList} />
        {userList.items.map((u) => (
          <div className="line user-row" key={u.id}>
            <strong>{u.username}</strong>
            <div className="row-meta">
              {u.admin === 1 && <Badge>Administrator</Badge>}
              {u.budget_member === 1 && <Badge>Household member</Badge>}
              {u.disabled === 1 && <Badge tone="bad">Disabled</Badge>}
              <ActionMenu label={"Actions for user " + u.username}>
                <Button variant="quiet" disabled={busy} onClick={() => setEditUser({ ...u })}>
                  Edit
                </Button>
                {u.id !== currentUserID && (
                  <>
                    <Button
                      variant="quiet" disabled={busy}
                      onClick={() => setSecurityUser({ user: { ...u }, action: "password" })}
                    >
                      Reset password
                    </Button>
                    <Button
                      variant="danger" disabled={busy}
                      onClick={() => setSecurityUser({ user: { ...u }, action: "delete" })}
                    >
                      Delete user
                    </Button>
                  </>
                )}
              </ActionMenu>
            </div>
          </div>
        ))}
        <ListNavigation list={userList} />
        <p className="footnote">
          Household members can edit shared accounts and budgets. Use a user’s
          actions to reset their password or delete their access. Change your own
          password in Security.
        </p>
      </section>
      <section className="panel">
        <h2>Account access</h2>
        <p className="muted">
          Household members automatically have editor access to household
          accounts. Use the settings below to give users access to individual
          accounts.
        </p>
        <Form
          onSubmit={async (e) => {
            e.preventDefault();
            if (
              await run(
                () =>
                  api("/grants", "PUT", {
                    user_id: Number(grantUser),
                    account_id: Number(grantAccount),
                    role,
                  }),
                "Account access updated",
              )
            )
              refresh();
          }}
        >
          <div className="form-grid">
            <PagedSelect
              url="/users"
              label="User"
              nameKey="username"
              required
              value={grantUser}
              options={users}
              onChange={setGrantUser}
              revision={revision}
            />
            <PagedSelect
              url="/accounts/manage"
              label="Account"
              optionLabel={(a) =>
                a.name + (a.household ? " · Household" : " · Private")
              }
              required
              value={grantAccount}
              options={manageAccounts}
              onChange={setGrantAccount}
              revision={revision}
            />
          </div>
          <Field label="Permission">
            <select value={role} onChange={(e) => setRole(e.target.value)}>
              <option value="viewer">Viewer</option>
              <option value="editor">Editor (can update transactions)</option>
              <option value="">Remove individual access</option>
            </select>
          </Field>
          <Button loading={busy} disabled={busy} type="submit">
            Update access
          </Button>
        </Form>
        <ListStatus list={grantList} />
        {grantList.items.map((g) => (
          <div className="line" key={g.user_id + "-" + g.account_id}>
            <span>
              {g.username} · {g.account_name}
            </span>
            <Badge>{g.role}</Badge>
          </div>
        ))}
        <ListNavigation list={grantList} />
      </section>
    </>
  );
}
