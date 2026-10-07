import { useState } from "react";
import { api } from "../../api";
import { Button, Field, Form, Modal } from "../../ui";
import { passwordError } from "../../validation";
import type { Row } from "../../shared/types";
import type { useTask } from "../../shared/useTask";

export function UserSecurityDialog({ user, action, busy, run, refresh, onClose }: {
  user: Row;
  action: "password" | "delete";
  busy: boolean;
  run: ReturnType<typeof useTask>["run"];
  refresh: () => void;
  onClose: () => void;
}) {
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [serverError, setServerError] = useState("");
  const deleting = action === "delete";
  const title = deleting ? "Delete user" : "Reset password";
  return (
    <Modal title={title} onClose={() => { if (!busy) onClose(); }} size="compact">
      <p><strong>{user.username}</strong></p>
      <p className="muted">
        {deleting
          ? "Permanently remove this user’s access, sessions, agent connections and saved banking credentials. Transactions, rules, imports and historical attribution remain. Their username stays reserved."
          : "Set a new password for this user. All their sessions and agent connections will be revoked. Their disabled status stays unchanged."}
      </p>
      <Form onSubmit={async (event) => {
        event.preventDefault();
        const changed = await run(
          () => deleting
            ? api("/users/" + user.id, "DELETE", {
                version: user.version,
                confirm_username: confirm,
              })
            : api("/users/" + user.id + "/password", "POST", {
                version: user.version,
                new_password: password,
              }),
          deleting ? "User deleted" : "Password reset; user signed out",
          (message) => {
            if (message === "Type the username exactly to confirm deletion" ||
                message === "New password must be 12–72 bytes") {
              setServerError(message);
              return true;
            }
            return false;
          },
        );
        if (changed) {
          setPassword("");
          onClose();
          refresh();
        }
      }}>
        {deleting ? (
          <Field
            label="Confirm username"
            serverError={serverError}
            hint={"Type " + user.username + " to confirm permanent deletion."}
            validate={(value) => value === user.username
              ? "" : "Type the username exactly to confirm deletion"}
          >
            <input
              autoFocus required autoComplete="off" value={confirm}
              onChange={(event) => {
                setConfirm(event.target.value);
                setServerError("");
              }}
            />
          </Field>
        ) : (
          <>
            <Field label="New password" validate={passwordError} serverError={serverError}>
              <input
                autoFocus required type="password" autoComplete="new-password"
                value={password}
                onChange={(event) => {
                  setPassword(event.target.value);
                  setServerError("");
                }}
              />
            </Field>
            <Field
              label="Confirm new password"
              validate={(value) => value === password ? "" : "Passwords do not match."}
            >
              <input
                required type="password" autoComplete="new-password"
                value={confirm} onChange={(event) => setConfirm(event.target.value)}
              />
            </Field>
          </>
        )}
        <div className="actions">
          <Button type="submit" variant={deleting ? "danger" : "primary"} loading={busy} disabled={busy}>
            {title}
          </Button>
          <Button disabled={busy} onClick={onClose}>Cancel</Button>
        </div>
      </Form>
    </Modal>
  );
}
