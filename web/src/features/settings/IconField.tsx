import { useEffect, useRef, useState } from "react";
import { Button, Field } from "../../ui";
export function IconField({
  label,
  value,
  onChange,
  serverError = "",
  onClearError,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  serverError?: string;
  onClearError?: () => void;
}) {
  const [error, setError] = useState("");
  const revision = useRef(0);
  useEffect(
    () => () => {
      revision.current++;
    },
    [],
  );
  return (
    <div className="branding-icon-field">
      <div className="branding-preview-frame">
        <img
          width={80}
          height={80}
          className="branding-preview"
          src={value || "/sente.svg"}
          alt={label + " preview"}
        />
      </div>
      <Field
        label={label}
        hint="Square PNG, 192–1024 pixels, up to 256 KiB. Icons are public."
        serverError={error || serverError}
      >
        <input
          type="file"
          accept="image/png"
          onChange={async (event) => {
            const current = ++revision.current;
            const file = event.target.files?.[0];
            event.target.value = "";
            setError("");
            onClearError?.();
            if (!file) return;
            if (file.type !== "image/png" || file.size > 256 * 1024) {
              setError(
                "Use a square PNG icon, 192–1024 pixels and at most 256 KiB",
              );
              return;
            }
            try {
              const bitmap = await createImageBitmap(file);
              const valid =
                bitmap.width === bitmap.height &&
                bitmap.width >= 192 &&
                bitmap.width <= 1024;
              bitmap.close();
              if (current !== revision.current) return;
              if (!valid)
                throw new Error(
                  "Use a square PNG icon, 192–1024 pixels and at most 256 KiB",
                );
              const reader = new FileReader();
              reader.onload = () => {
                if (current === revision.current)
                  onChange(String(reader.result));
              };
              reader.onerror = () => {
                if (current === revision.current)
                  setError("The icon could not be read");
              };
              reader.readAsDataURL(file);
            } catch (error) {
              if (current === revision.current)
                setError((error as Error).message);
            }
          }}
        />
      </Field>
      {value && (
        <Button
          variant="quiet"
          onClick={() => {
            revision.current++;
            onChange("");
            setError("");
            onClearError?.();
          }}
        >
          Remove {label.toLowerCase()}
        </Button>
      )}
    </div>
  );
}
