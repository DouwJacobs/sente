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
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [oldPasswordError, setOldPasswordError] = useState("");
  return (
    <>
      <section className="panel">
        <h2>Change your password</h2>
        <Form
          onSubmit={async (e) => {
            e.preventDefault();
            if (
              await run(
                () =>
                  api("/password", "POST", {
                    old_password: oldPassword,
                    new_password: newPassword,
                  }),
                "Password changed; other sessions signed out",
                (message) => {
                  if (message === "Current password is incorrect") {
                    setOldPasswordError(message);
                    return true;
                  }
                  return false;
                },
              )
            ) {
              setOldPassword("");
              setNewPassword("");
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
          <Field label="New password" validate={passwordError}>
            <input
              type="password"
              autoComplete="new-password"
              required
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
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
