import { useState } from "react";
import { Button, Field, Form } from "../../ui";
import type { Change, Preview } from "./configurationTypes";
const entityNames: Record<string, string> = {
  spending_groups: "Spending group",
  categories: "Category",
  merchants: "Merchant",
  merchant_rules: "Merchant naming rule",
  rules: "Account rule",
  builtin_rules: "Global fallback rule",
};
function ChangeDetails({ change }: { change: Change }) {
  const keys = Object.keys(change.after).filter(
    (key) =>
      JSON.stringify(change.before?.[key]) !==
      JSON.stringify(change.after[key]),
  );
  const show = (value: unknown, key: string) =>
    key === "logo_data"
      ? value
        ? "Logo supplied"
        : "No logo"
      : value == null || value === ""
        ? "Not set"
        : String(value);
  return (
    <details>
      <summary>
        {change.action === "create" ? "Add" : "Replace"}: {change.name}{" "}
        <span className="muted">({entityNames[change.entity]})</span>
      </summary>
      <dl>
        {keys.map((key) => (
          <div key={key}>
            <dt>{key.replaceAll("_", " ")}</dt>
            <dd>
              {change.before && (
                <>
                  <span>{show(change.before[key], key)}</span> →{" "}
                </>
              )}
              <strong>{show(change.after[key], key)}</strong>
            </dd>
          </div>
        ))}
      </dl>
    </details>
  );
}

export function ConfigurationPreview({
  preview,
  busy,
  onApply,
  onCancel,
}: {
  preview: Preview;
  busy: boolean;
  onApply: (allowUpdates: boolean) => Promise<unknown>;
  onCancel: () => void;
}) {
  const [allowUpdates, setAllowUpdates] = useState(false);
  const replacements = preview.changes.filter(
    (change) => change.action === "update",
  ).length;
  return (
    <section className="panel" aria-label="Configuration import preview">
      <h2>Preview: {preview.source.name}</h2>
      <p>
        {preview.changes.length} changes, including {replacements} replacements.
        Nothing has been imported yet.
      </p>
      <p className="muted">
        Omitted entries stay in place. For matching entries, this import takes
        precedence only after you confirm it. Local edits may be replaced. This
        preview expires after 15 minutes.
      </p>
      {preview.changes.length ? (
        preview.changes.map((change, index) => (
          <ChangeDetails key={index} change={change} />
        ))
      ) : (
        <p>
          The configuration already matches. Import to record this source and
          revision.
        </p>
      )}
      <Form onSubmit={() => onApply(allowUpdates)}>
        {replacements > 0 && (
          <Field label="Replace the existing configuration shown above, including any local edits.">
            <input
              type="checkbox"
              required
              checked={allowUpdates}
              onChange={(e) => setAllowUpdates(e.target.checked)}
            />
          </Field>
        )}
        <div className="editor-actions settings-save-actions">
          <Button
            type="submit"
            variant="primary"
            disabled={busy}
            loading={busy}
          >
            Import configuration
          </Button>
          <Button disabled={busy} onClick={onCancel}>
            Cancel preview
          </Button>
        </div>
      </Form>
    </section>
  );
}
