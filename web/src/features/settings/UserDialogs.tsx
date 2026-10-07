import { api } from "../../api";
import { Button, Field, Form, Modal } from "../../ui";
import { passwordError, usernameError } from "../../validation";
import { useTask } from "../../shared/useTask";
import type { useUserAccess } from "./useUserAccess";
export function UserDialogs({
  access,
  refresh,
  busy,
  run,
}: {
  access: ReturnType<typeof useUserAccess>;
  refresh: () => void;
  busy: boolean;
  run: ReturnType<typeof useTask>["run"];
}) {
  const {
    userList,
    grantList,
    editUser,
    setEditUser,
    addUser,
    setAddUser,
    username,
    setUsername,
    password,
    setPassword,
    member,
    setMember,
    admin,
    setAdmin,
    grantUser,
    setGrantUser,
    grantAccount,
    setGrantAccount,
    role,
    setRole,
    userNameError,
    setUserNameError,
  } = access;
  return (
    <>
      {editUser && (
        <Modal title="Edit user" onClose={() => setEditUser(null)}>
          <Form
            onSubmit={async (e) => {
              e.preventDefault();
              if (
                await run(
                  () =>
                    api("/users/" + editUser.id, "PUT", {
                      username: editUser.username,
                      admin: !!editUser.admin,
                      budget_member: !!editUser.budget_member,
                      disabled: !!editUser.disabled,
                      version: editUser.version,
                    }),
                  "User updated",
                  (message) => {
                    if (message === "Username already exists") {
                      setUserNameError(message);
                      return true;
                    }
                    return false;
                  },
                )
              ) {
                setEditUser(null);
                refresh();
              }
            }}
          >
            <Field
              label="Username"
              validate={usernameError}
              serverError={userNameError}
            >
              <input
                required
                minLength={2}
                maxLength={80}
                value={editUser.username}
                onChange={(e) => {
                  setEditUser({ ...editUser, username: e.target.value });
                  setUserNameError("");
                }}
              />
            </Field>
            <label className="check">
              <input
                type="checkbox"
                checked={!!editUser.budget_member}
                onChange={(e) =>
                  setEditUser({ ...editUser, budget_member: e.target.checked })
                }
              />
              Household budget member
            </label>
            <label className="check">
              <input
                type="checkbox"
                checked={!!editUser.admin}
                onChange={(e) =>
                  setEditUser({ ...editUser, admin: e.target.checked })
                }
              />
              Administrator
            </label>
            <label className="check">
              <input
                type="checkbox"
                checked={!!editUser.disabled}
                onChange={(e) =>
                  setEditUser({ ...editUser, disabled: e.target.checked })
                }
              />
              Disable sign-in and sign out this user
            </label>
            <Button
              variant="primary"
              type="submit"
              loading={busy}
              disabled={busy}
            >
              Save user
            </Button>
          </Form>
        </Modal>
      )}
      {addUser && (
        <Modal title="Add user" onClose={() => setAddUser(false)}>
          <Form
            onSubmit={async (e) => {
              e.preventDefault();
              if (
                await run(
                  () =>
                    api("/users", "POST", {
                      username,
                      password,
                      admin,
                      budget_member: member,
                    }),
                  "User created",
                  (message) => {
                    if (message === "Username already exists") {
                      setUserNameError(message);
                      return true;
                    }
                    return false;
                  },
                )
              ) {
                setAddUser(false);
                setUsername("");
                setPassword("");
                refresh();
              }
            }}
          >
            <Field
              label="Username"
              validate={usernameError}
              serverError={userNameError}
            >
              <input
                required
                minLength={2}
                maxLength={80}
                value={username}
                onChange={(e) => {
                  setUsername(e.target.value);
                  setUserNameError("");
                }}
              />
            </Field>
            <Field label="Initial password" validate={passwordError}>
              <input
                type="password"
                autoComplete="new-password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </Field>
            <label className="check">
              <input
                type="checkbox"
                checked={member}
                onChange={(e) => setMember(e.target.checked)}
              />
              Household budget member
            </label>
            <label className="check">
              <input
                type="checkbox"
                checked={admin}
                onChange={(e) => setAdmin(e.target.checked)}
              />
              Administrator
            </label>
            <Button
              variant="primary"
              loading={busy}
              disabled={busy}
              type="submit"
            >
              Create user
            </Button>
          </Form>
        </Modal>
      )}
    </>
  );
}
