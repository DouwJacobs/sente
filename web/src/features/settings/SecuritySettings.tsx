import { useState } from "react";
import { api } from "../../api";
import { Button, Field, Form } from "../../ui";
import { passwordError } from "../../validation";
import { useTask } from "../../shared/useTask";
export function SecuritySettings({
  busy,
  run,
}: {
  busy: boolean;
  run: ReturnType<typeof useTask>["run"];
}) {
  const [formVersion, setFormVersion] = useState(0);
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [newPasswordError, setNewPasswordError] = useState("");
  const [oldPasswordError, setOldPasswordError] = useState("");
  return (
    <>
      <section className="panel">
        <h2>Change your password</h2>
        <p className="muted">This session stays signed in. Other sessions and all agent connections will be revoked.</p>
        <Form
          key={formVersion}
          onSubmit={async (e) => {
            e.preventDefault();
            if (
              await run(
                () =>
                  api("/password", "POST", {
                    old_password: oldPassword,
                    new_password: newPassword,
                  }),
                "Password changed; other sessions and agent connections revoked",
                (message) => {
                  if (message === "Current password is incorrect") {
                    setOldPasswordError(message);
                    return true;
                  }
                  if (message === "New password must be 12–72 bytes") {
                    setNewPasswordError(message);
                    return true;
                  }
                  return false;
                },
              )
            ) {
              setOldPassword("");
              setNewPassword("");
              setConfirmation("");
              setFormVersion((version) => version + 1);
            }
          }}
        >
          <Field label="Current password" serverError={oldPasswordError}>
            <input
              type="password"
              autoComplete="current-password"
              required
              value={oldPassword}
              onChange={(e) => {
                setOldPassword(e.target.value);
                setOldPasswordError("");
              }}
            />
          </Field>
          <Field label="New password" validate={passwordError} serverError={newPasswordError}>
            <input
              type="password"
              autoComplete="new-password"
              required
              value={newPassword}
              onChange={(e) => {
                setNewPassword(e.target.value);
                setNewPasswordError("");
              }}
            />
          </Field>
          <Field label="Confirm new password" validate={(value) => value === newPassword ? "" : "Passwords do not match."}>
            <input
              required type="password" autoComplete="new-password"
              value={confirmation} onChange={(e) => setConfirmation(e.target.value)}
            />
          </Field>
          <Button type="submit" loading={busy} disabled={busy}>
            Change password
          </Button>
        </Form>
      </section>
    </>
  );
}
