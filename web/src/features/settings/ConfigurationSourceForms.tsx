import { useEffect, useState } from "react";
import { api } from "../../api";
import { Button, Field, Form } from "../../ui";
import { useTask } from "../../shared/useTask";
import type { Source, Preview } from "./configurationTypes";
type Props = {
  busy: boolean;
  run: ReturnType<typeof useTask>["run"];
  onPreview: (preview: Preview) => void;
};
function repositoryError(value: string) {
  try {
    const url = new URL(value);
    if (
      url.protocol !== "https:" ||
      url.username ||
      url.password ||
      url.search ||
      url.hash ||
      !url.hostname ||
      (url.port && url.port !== "443")
    )
      throw new Error();
    return "";
  } catch {
    return "Use a public HTTPS Git repository URL without credentials";
  }
}

export function RepositorySourceForm({ busy, run, onPreview }: Props) {
  const [name, setName] = useState("");
  const [repository, setRepository] = useState("");
  const [ref, setRef] = useState("");
  const [path, setPath] = useState("sente.json");
  const [repositoryServerError, setRepositoryServerError] = useState("");
  return (
    <section className="panel">
      <h2>Add a repository</h2>
      <Form
        onSubmit={() =>
          run(
            async () =>
              onPreview(
                await api("/configuration/preview", "POST", {
                  source: {
                    name,
                    kind: "repository",
                    repository_url: repository,
                    ref,
                    path,
                  },
                }),
              ),
            undefined,
            (message) => {
              if (
                message.startsWith("Repository") ||
                message.startsWith("Use a public")
              ) {
                setRepositoryServerError(message);
                return true;
              }
              return false;
            },
          )
        }
      >
        <Field label="Source name">
          <input
            required
            maxLength={200}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </Field>
        <Field
          label="Repository URL"
          validate={repositoryError}
          serverError={repositoryServerError}
          hint="Public HTTPS repositories. For a private repository, upload its configuration file instead."
        >
          <input
            required
            type="url"
            value={repository}
            onChange={(e) => {
              setRepository(e.target.value);
              setRepositoryServerError("");
            }}
          />
        </Field>
        <Field
          label="Branch, tag or commit"
          hint="Leave blank to use the repository’s default branch."
          validate={(value) =>
            value &&
            (!/^[A-Za-z0-9][A-Za-z0-9._/-]{0,199}$/.test(value) ||
              value.includes("..") ||
              value.includes("//"))
              ? "Use a branch, tag or commit reference"
              : ""
          }
        >
          <input value={ref} onChange={(e) => setRef(e.target.value)} />
        </Field>
        <Field
          label="Configuration file path"
          validate={(value) =>
            !value.endsWith(".json") ||
            value.startsWith("/") ||
            value.includes("\\") ||
            value
              .split("/")
              .some((part) => !part || part === "." || part === "..")
              ? "Use a relative JSON file path"
              : ""
          }
        >
          <input
            required
            value={path}
            onChange={(e) => setPath(e.target.value)}
          />
        </Field>
        <Button type="submit" disabled={busy} loading={busy}>
          Pull and preview
        </Button>
      </Form>
    </section>
  );
}

export function FileSourceForm({
  busy,
  run,
  onPreview,
  sources,
  importRevision,
}: Props & { sources: Source[]; importRevision: number }) {
  const [file, setFile] = useState<File | null>(null);
  const [fileSource, setFileSource] = useState<Source | null>(null);
  const [fileKey, setFileKey] = useState(0);
  const [fileError, setFileError] = useState("");
  useEffect(() => {
    setFile(null);
    setFileSource(null);
    setFileKey((key) => key + 1);
    setFileError("");
  }, [importRevision]);
  useEffect(() => {
    if (fileSource && !sources.some((source) => source.id === fileSource.id))
      setFileSource(null);
  }, [sources, fileSource]);
  return (
    <section className="panel">
      <h2>Import a file</h2>
      <Form
        onSubmit={() =>
          run(
            async () => {
              if (!file) return;
              const body = new FormData();
              body.append("file", file);
              if (fileSource) body.append("source_id", String(fileSource.id));
              onPreview(await api("/configuration/preview", "POST", body));
            },
            undefined,
            (message) => {
              if (
                /^(Please sign in|Session expired|Invalid security token|Origin not allowed|Administrator access|Account editor access|This record changed|Configuration or access changed|Request could not be completed|Request failed|The server returned)/.test(
                  message,
                ) ||
                /fetch|network|load failed/i.test(message)
              )
                return false;
              setFileError(message);
              return true;
            },
          )
        }
      >
        <Field label="File source">
          <select
            value={fileSource?.id || ""}
            onChange={(e) => {
              setFileSource(
                sources.find(
                  (source) => source.id === Number(e.target.value),
                ) || null,
              );
              setFileError("");
            }}
          >
            <option value="">Add a new file source</option>
            {sources
              .filter((source) => source.kind === "file")
              .map((source) => (
                <option key={source.id} value={source.id}>
                  {source.name}
                </option>
              ))}
          </select>
        </Field>
        <Field
          label="Configuration JSON file"
          hint="Up to 32 MiB and 10,000 entries. Export from Sente or follow the format in the starter repository."
          serverError={fileError}
        >
          <input
            key={fileKey}
            type="file"
            accept=".json,application/json"
            required
            onChange={(e) => {
              const selected = e.target.files?.[0] || null;
              setFile(selected);
              setFileError(
                selected && selected.size > 32 * 1024 * 1024
                  ? "Choose a file up to 32 MiB"
                  : "",
              );
            }}
          />
        </Field>
        <Button type="submit" disabled={busy || !!fileError} loading={busy}>
          Validate and preview file
        </Button>
      </Form>
    </section>
  );
}
