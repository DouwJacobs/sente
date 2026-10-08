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
  return (
    <>
      <div className="general-settings-grid">
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
