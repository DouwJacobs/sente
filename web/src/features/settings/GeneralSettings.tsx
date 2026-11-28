import { api } from "../../api";
import { Button, Field, Form, Loading } from "../../ui";
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
  loading,
  failed,
  onRetry,
  onSaved,
}: {
  data: PageProps["data"];
  refresh: () => void;
  theme: string;
  onTheme: (theme: string) => void;
  startDay: string;
  setStartDay: (day: string) => void;
  loading: boolean;
  failed: boolean;
  onRetry: () => void;
  onSaved: () => void;
  busy: boolean;
  run: ReturnType<typeof useTask>["run"];
}) {
  return (
    <>
      <div className="general-settings-grid general-preferences-grid">
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
                if (loading || failed) return;
                if (
                  await run(
                    () =>
                      api("/settings", "PUT", {
                        start_day: Number(startDay),
                      }),
                    "Default updated; existing periods are unchanged",
                  )
                ) {
                  onSaved();
                  refresh();
                }
              }}
            >
              {loading && <Loading>Loading settings</Loading>}
              {failed && <Button onClick={onRetry}>Retry settings</Button>}
              <Field
                label="Default period start day"
                hint="Used for new periods. Days beyond a month’s length use its last day."
              >
                <input
                  disabled={busy || loading || failed}
                  type="number"
                  min={1}
                  max={31}
                  required
                  value={startDay}
                  onChange={(e) => setStartDay(e.target.value)}
                />
              </Field>
              <div className="form-actions">
                <Button variant="primary" type="submit" loading={busy} disabled={busy || loading || failed}>Save default</Button>
              </div>
            </Form>
          )}
        </section>
      </div>
    </>
  );
}
