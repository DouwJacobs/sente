import { useEffect, useState } from "react";
import { api } from "../../api";
import { Button, Field, Form, Loading } from "../../ui";
import { useTask } from "../../shared/useTask";
import type { PageProps } from "../../shared/types";
import { usePWA } from "../pwa/PWAProvider";
import { IconField } from "./IconField";
type Identity = {
  name: string;
  logo: string;
  use_branding_name: boolean;
  use_branding_logo: boolean;
  version: number;
};
export function PWASettings({ data, notify }: PageProps) {
  const [iconError, setIconError] = useState("");
  const pwa = usePWA();
  const { busy, run } = useTask(notify);
  const [draft, setDraft] = useState<Identity | null>(null),
    [saved, setSaved] = useState<Identity | null>(null),
    [nameError, setNameError] = useState("");
  const apply = (value: Identity) => {
    setSaved(value);
    setDraft(value);
    setNameError("");
    setIconError("");
  };
  useEffect(() => {
    let alive = true;
    if (data.user.admin)
      api<Identity>("/pwa")
        .then((value) => {
          if (alive) apply(value);
        })
        .catch((error) => {
          if (alive) notify(error.message, true);
        });
    return () => {
      alive = false;
    };
  }, []);
  const edit = (patch: Partial<Identity>) => {
    setDraft((value) => (value ? { ...value, ...patch } : value));
    setNameError("");
    setIconError("");
  };
  return (
    <div className="general-settings-grid">
      <section className="panel">
        <h2>Install Sente</h2>
        <p className="muted">
          Open Sente from your home screen or desktop. Financial data requires a
          connection and a current sign-in.
        </p>
        <p>
          {pwa.installed
            ? "Running as an installed app."
            : pwa.installable
              ? "This browser can install Sente."
              : "Use your browser’s Install app menu. On iPhone or iPad, use Safari → Share → Add to Home Screen. Installation requires HTTPS or localhost."}
        </p>
        <div className="editor-actions settings-save-actions">
          {pwa.installable && (
            <Button onClick={() => run(pwa.install)}>Install app</Button>
          )}
          {pwa.updateAvailable && (
            <Button onClick={pwa.update}>Update and reload</Button>
          )}
          <Button
            disabled={busy}
            onClick={() => run(pwa.check, "Update check completed")}
          >
            Check for updates
          </Button>
        </div>
        <p className="footnote">
          An update reloads this tab. Save any open edits first. Existing
          installations may take time to refresh their name and icon; reinstall
          if your browser keeps the old identity.
        </p>
      </section>
      {data.user.admin && (
        <section className="panel">
          <h2>PWA identity</h2>
          <p className="muted">
            The installed app’s name and icons are public, including before
            sign-in. Choose whether to use Branding or a separate identity.
          </p>
          {!draft ? (
            <>
              <Loading>Loading PWA settings</Loading>
              <Button onClick={() => run(async () => apply(await api("/pwa")))}>
                Reload saved PWA settings
              </Button>
            </>
          ) : (
            <Form
              onSubmit={async () => {
                await run(
                  async () => apply(await api("/pwa", "PUT", draft)),
                  "PWA settings saved",
                  (message) => {
                    if (message.startsWith("Use a PWA name")) {
                      setNameError(message);
                      return true;
                    }
                    if (message.startsWith("Use a square PNG icon")) {
                      setIconError(message);
                      return true;
                    }
                    return false;
                  },
                );
              }}
            >
              <Field label="PWA name source">
                <select
                  value={draft.use_branding_name ? "branding" : "custom"}
                  onChange={(e) =>
                    edit({ use_branding_name: e.target.value === "branding" })
                  }
                >
                  <option value="branding">Use workspace name</option>
                  <option value="custom">Custom PWA name</option>
                </select>
              </Field>
              {draft.use_branding_name ? (
                <p className="muted">
                  Public name: {data.branding.display_name}
                </p>
              ) : (
                <Field
                  label="PWA name"
                  serverError={nameError}
                  validate={(value) =>
                    Array.from(value.trim()).length < 2 ||
                    Array.from(value.trim()).length > 60 ||
                    /[\u0000-\u001f\u007f]/.test(value)
                      ? "Use a PWA name of 2–60 characters without control characters"
                      : ""
                  }
                >
                  <input
                    required
                    value={draft.name}
                    onChange={(e) => edit({ name: e.target.value })}
                  />
                </Field>
              )}
              <Field label="PWA icon source">
                <select
                  value={draft.use_branding_logo ? "branding" : "custom"}
                  onChange={(e) =>
                    edit({ use_branding_logo: e.target.value === "branding" })
                  }
                >
                  <option value="branding">Use branding logo</option>
                  <option value="custom">Custom PWA icon</option>
                </select>
              </Field>
              {draft.use_branding_logo ? (
                <div className="branding-preview-frame">
                  <img
                    width={80}
                    height={80}
                    className="branding-preview"
                    src={data.branding.logo || "/sente.svg"}
                    alt="PWA icon preview"
                  />
                </div>
              ) : (
                <IconField
                  label="PWA icon"
                  value={draft.logo}
                  onChange={(logo) => edit({ logo })}
                  serverError={iconError}
                  onClearError={() => setIconError("")}
                />
              )}
              <div className="editor-actions settings-save-actions">
                <Button
                  type="submit"
                  variant="primary"
                  loading={busy}
                  disabled={
                    busy || JSON.stringify(draft) === JSON.stringify(saved)
                  }
                >
                  Save PWA settings
                </Button>
                <Button
                  variant="quiet"
                  disabled={busy}
                  onClick={() => run(async () => apply(await api("/pwa")))}
                >
                  Reload saved PWA settings
                </Button>
              </div>
            </Form>
          )}
        </section>
      )}
    </div>
  );
}
