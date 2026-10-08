import { useState } from "react";
import { api } from "../../api";
import { Button, Field, Form } from "../../ui";
import { useTask } from "../../shared/useTask";
import type { PageProps } from "../../shared/types";
import { IconField } from "./IconField";
export function BrandingSettings({ data, refresh, notify }: PageProps) {
  const [saved, setSaved] = useState(data.branding);
  const [name, setName] = useState(data.branding.display_name);
  const [logo, setLogo] = useState(data.branding.logo || "");
  const [error, setError] = useState("");
  const [iconError, setIconError] = useState("");
  const { busy, run } = useTask(notify);
  const apply = (b: typeof saved) => {
    setSaved(b);
    setName(b.display_name);
    setLogo(b.logo || "");
    setError("");
    setIconError("");
  };
  return (
    <section className="panel">
      <h2>Branding</h2>
      <p className="muted">
        The workspace name appears after sign-in. The logo appears on sign-in,
        navigation and browser tabs. Upload only a logo you are happy to make
        public.
      </p>
      <Form
        onSubmit={async () => {
          if (
            await run(
              async () =>
                apply(
                  await api("/branding", "PUT", {
                    display_name: name,
                    logo,
                    version: saved.version,
                  }),
                ),
              "Branding saved",
              (message) => {
                if (message.startsWith("Use a workspace name")) {
                  setError(message);
                  return true;
                }
                if (message.startsWith("Use a square PNG icon")) {
                  setIconError(message);
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
          serverError={error}
          validate={(value) =>
            Array.from(value.trim()).length < 2 ||
            Array.from(value.trim()).length > 60
              ? "Use a workspace name of 2–60 characters"
              : ""
          }
        >
          <input
            required
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              setError("");
            }}
          />
        </Field>
        <IconField
          label="Branding logo"
          value={logo}
          onChange={(value) => {
            setLogo(value);
            setIconError("");
          }}
          serverError={iconError}
          onClearError={() => setIconError("")}
        />
        <div className="editor-actions settings-save-actions">
          <Button
            type="submit"
            variant="primary"
            loading={busy}
            disabled={
              busy ||
              (name === saved.display_name && logo === (saved.logo || ""))
            }
          >
            Save branding
          </Button>
          <Button
            disabled={busy}
            variant="quiet"
            onClick={() => run(async () => apply(await api("/branding")))}
          >
            Reload saved branding
          </Button>
        </div>
      </Form>
    </section>
  );
}
