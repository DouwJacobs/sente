import { useEffect, useState } from "react";
import { api, download } from "../../api";
import { Button, Loading } from "../../ui";
import { useTask } from "../../shared/useTask";
import {
  RepositorySourceForm,
  FileSourceForm,
} from "./ConfigurationSourceForms";
import { ConfigurationPreview } from "./ConfigurationPreview";
import type { Source, Preview } from "./configurationTypes";
import type { PageProps } from "../../shared/types";

export function ConfigurationSettings({
  notify,
  refresh,
}: Pick<PageProps, "notify" | "refresh">) {
  const [sources, setSources] = useState<Source[]>([]);
  const [starter, setStarter] = useState<Source | null>(null);
  const [loading, setLoading] = useState(true);
  const [importRevision, setImportRevision] = useState(0);
  const [preview, setPreview] = useState<Preview | null>(null);
  const { busy, run } = useTask(notify);
  const load = async () => {
    const result = await api<{ sources: Source[]; default_source: Source }>(
      "/configuration",
    );
    setSources(result.sources);
    setStarter(result.default_source);
  };
  useEffect(() => {
    let alive = true;
    api<{ sources: Source[]; default_source: Source }>("/configuration")
      .then((result) => {
        if (alive) {
          setSources(result.sources);
          setStarter(result.default_source);
        }
      })
      .catch((e) => {
        if (alive) notify(e.message, true);
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, []);
  const showPreview = (result: Preview) => {
    setPreview(result);
  };
  const existingStarter = sources.find((source) => source.kind === "starter");
  return (
    <>
      <section className="panel">
        <div className="section-head">
          <h2>Portable configuration</h2>
          <Button
            disabled={busy}
            loading={busy}
            onClick={() =>
              run(() =>
                download("/configuration/export", "sente-configuration.json"),
              )
            }
          >
            Export configuration
          </Button>
        </div>
        <p>
          Import categories, spending groups, merchants and rules from JSON
          files or public Git repositories. You can keep several sources
          together.
        </p>
        <p className="muted">
          Sources are pulled only when you ask. Imports affect future
          transactions. Exports include accessible account names and merchant
          logos; keep personal files private.
        </p>
        {loading && <Loading>Loading configuration sources</Loading>}
        {starter && (
          <div>
            <h3>Sente starter configuration</h3>
            <p className="muted">
              Optional general categories and basic rules. Repository:{" "}
              {starter.repository_url}
            </p>
            <Button
              disabled={busy}
              loading={busy}
              onClick={() =>
                run(async () =>
                  showPreview(
                    await api(
                      "/configuration/preview",
                      "POST",
                      existingStarter
                        ? { source_id: existingStarter.id }
                        : { starter: true },
                    ),
                  ),
                )
              }
            >
              {existingStarter
                ? "Pull starter again"
                : "Pull starter and preview"}
            </Button>
            {!existingStarter && (
              <Button
                variant="quiet"
                disabled={busy}
                onClick={() =>
                  run(async () =>
                    showPreview(
                      await api("/configuration/preview", "POST", {
                        bundled: true,
                      }),
                    ),
                  )
                }
              >
                Preview bundled starter
              </Button>
            )}
            {!existingStarter && (
              <p className="footnote">
                The bundled snapshot can be imported without network access.
                Pull from the repository for its current configuration.
              </p>
            )}
          </div>
        )}
      </section>
      <div className="general-settings-grid configuration-source-forms">
        <RepositorySourceForm busy={busy} run={run} onPreview={showPreview} />
        <FileSourceForm
          busy={busy}
          run={run}
          onPreview={showPreview}
          sources={sources}
          importRevision={importRevision}
        />
      </div>
      {preview && (
        <ConfigurationPreview
          key={preview.id}
          preview={preview}
          busy={busy}
          onCancel={() => setPreview(null)}
          onApply={(allowUpdates) =>
            run(async () => {
              await api("/configuration/apply", "POST", {
                preview_id: preview.id,
                allow_updates: allowUpdates,
              });
              setPreview(null);
              setImportRevision((revision) => revision + 1);
              await load();
              refresh();
            }, "Configuration imported")
          }
        />
      )}
      <section className="panel">
        <h2>Configuration sources</h2>
        {!loading && !sources.length && (
          <p>
            No sources imported yet. Add a repository or file, or try the
            starter configuration.
          </p>
        )}
        {sources.map((source) => (
          <div className="configuration-source" key={source.id}>
            <h3>{source.name}</h3>
            <p className="muted">
              {source.kind === "file"
                ? "Uploaded file"
                : `${source.repository_url} · ${source.ref || "Default branch"} · ${source.path}`}
            </p>
            <p className="footnote">
              Last imported: {source.last_sync || "Never"} · Revision:{" "}
              {source.last_revision}
            </p>
            <div className="editor-actions">
              {source.kind !== "file" && (
                <Button
                  disabled={busy}
                  loading={busy}
                  onClick={() =>
                    run(async () =>
                      showPreview(
                        await api("/configuration/preview", "POST", {
                          source_id: source.id,
                        }),
                      ),
                    )
                  }
                >
                  Pull again
                </Button>
              )}
              <Button
                variant="quiet"
                disabled={busy}
                onClick={() =>
                  run(async () => {
                    await api(`/configuration/sources/${source.id}`, "DELETE", {
                      version: source.version,
                    });
                    await load();
                    setPreview(null);
                  }, "Source removed; imported configuration kept")
                }
              >
                Forget source
              </Button>
            </div>
          </div>
        ))}
        {!!sources.length && (
          <p className="muted">
            For file updates, choose the file source above and upload its
            replacement. Forgetting a source keeps the imported categories,
            merchants and rules.
          </p>
        )}
      </section>
    </>
  );
}
