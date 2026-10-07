import { useState } from "react";
import { api } from "../../api";
import { Button, Field, Form } from "../../ui";
import { useTask } from "../../shared/useTask";
import { type PageProps } from "../../shared/types";
export function GeneralSettings({
  data,
  refresh,
  theme,
  onTheme,
  startDay,
  setStartDay,
  busy,
  run,
}: {
  data: PageProps["data"];
  refresh: () => void;
  theme: string;
  onTheme: (theme: string) => void;
  startDay: string;
  setStartDay: (day: string) => void;
  busy: boolean;
  run: ReturnType<typeof useTask>["run"];
}) {
  const [workspaceName, setWorkspaceName] = useState(
    data.branding.display_name,
  );
  const [brandingVersion, setBrandingVersion] = useState(data.branding.version);
  const [brandingError, setBrandingError] = useState("");
  return (
    <>
      <div className="general-settings-grid">
        {data.user.admin && (
          <section className="panel">
            <h2>Workspace name</h2>
            <Form
              onSubmit={async () => {
                if (
                  await run(
                    async () => {
                      const result = await api("/branding", "PUT", {
                        display_name: workspaceName,
                        version: brandingVersion,
                      });
                      setWorkspaceName(result.display_name);
                      setBrandingVersion(result.version);
                    },
                    "Workspace name saved",
                    (message) => {
                      if (message.startsWith("Use a workspace name")) {
                        setBrandingError(message);
                        return true;
                      }
                      return false;
                    },
                  )
                )
                  refresh();
              }}
            >
              <Field
                label="Display name"
                hint="Shown to signed-in users. The sign-in screen keeps a generic name."
                serverError={brandingError}
                validate={(value) =>
                  Array.from(value.trim()).length < 2 ||
                  Array.from(value.trim()).length > 60
                    ? "Use a workspace name of 2–60 characters"
                    : ""
                }
              >
                <input
                  required
                  value={workspaceName}
                  onChange={(e) => {
                    setWorkspaceName(e.target.value);
                    setBrandingError("");
                  }}
                />
              </Field>
              <div className="editor-actions settings-save-actions">
                <Button
                  type="submit"
                  variant="primary"
                  loading={busy}
                  disabled={busy}
                >
                  Save workspace name
                </Button>
                <Button
                  variant="quiet"
                  loading={busy}
                  disabled={busy}
                  onClick={async () => {
                    if (
                      await run(async () => {
                        const b = await api("/branding");
                        setWorkspaceName(b.display_name);
                        setBrandingVersion(b.version);
                        setBrandingError("");
                      })
                    )
                      refresh();
                  }}
                >
                  Reload saved name
                </Button>
              </div>
            </Form>
          </section>
        )}
        <section className="panel">
          <h2>Preferences</h2>
          <Field label="Appearance">
            <select value={theme} onChange={(e) => onTheme(e.target.value)}>
              <option value="system">Follow device</option>
              <option value="light">Light</option>
              <option value="dark">Dark</option>
            </select>
          </Field>
          <div className="line">
            <span>Currency</span>
            <strong>South African rand (ZAR)</strong>
          </div>
          <div className="line">
            <span>Budget timezone</span>
            <strong>Africa/Johannesburg</strong>
          </div>
          {data.user.budget_member && (
            <Form
              onSubmit={async (e) => {
                e.preventDefault();
                if (
                  await run(
                    () =>
                      api("/settings", "PUT", {
                        start_day: Number(startDay),
                      }),
                    "Default updated; existing periods are unchanged",
                  )
                )
                  refresh();
              }}
            >
              <Field
                label="Default period start day"
                hint="Used for new periods. Days beyond a month’s length use its last day."
              >
                <input
                  type="number"
                  min={1}
                  max={31}
                  required
                  value={startDay}
                  onChange={(e) => setStartDay(e.target.value)}
                />
              </Field>
              <Button type="submit" loading={busy} disabled={busy}>
                Save default
              </Button>
            </Form>
          )}
        </section>
      </div>
    </>
  );
}
