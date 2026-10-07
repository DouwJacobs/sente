import { useEffect, useState } from "react";
import { api } from "../../api";
import { Button, Field, Form, Loading } from "../../ui";
import { useTask } from "../../shared/useTask";
import type { PageProps } from "../../shared/types";

type SavedContext = { context: string; version: number };
export function SavedMCPContext({ notify }: Pick<PageProps, "notify">) {
  const [saved, setSaved] = useState<SavedContext | null>(null);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState("");
  const [failed, setFailed] = useState(false);
  const { busy, run } = useTask(notify);
  const accept = (value: SavedContext) => {
    setSaved(value);
    setDraft(value.context);
    setError("");
    setFailed(false);
  };
  useEffect(() => {
    let alive = true;
    api<SavedContext>("/mcp/context")
      .then((value) => {
        if (alive) accept(value);
      })
      .catch((e) => {
        if (alive) {
          setFailed(true);
          notify(e.message, true);
        }
      });
    return () => {
      alive = false;
    };
  }, []);
  const reload = () =>
    run(async () => accept(await api<SavedContext>("/mcp/context")));
  const save = () =>
    run(
      async () => {
        accept(
          await api<SavedContext>("/mcp/context", "PUT", {
            context: draft,
            version: saved?.version,
          }),
        );
      },
      "MCP context saved",
      (message) => {
        if (message.includes("6000 characters")) {
          setError(message);
          return true;
        }
        return false;
      },
    );
  const length = Array.from(draft).length;
  return (
    <section className="panel mcp-context-panel">
      <h2>Your MCP context</h2>
      <p className="muted">
        Save preferences and background you want your agents to know, such as
        your budget goals or how you use categories.
      </p>
      <p className="footnote">
        This belongs to your user account. Each agent needs permission to read
        it. Shared context is sent as written when it connects, and can be
        refreshed for later conversations.
      </p>
      {!saved ? (
        failed ? (
          <Button disabled={busy} onClick={reload}>
            Retry loading context
          </Button>
        ) : (
          <Loading>Loading your context</Loading>
        )
      ) : (
        <Form onSubmit={save}>
          <Field
            label="Context for your agents"
            hint="Include useful background, not passwords or banking credentials. Up to 6,000 characters."
            validate={() =>
              length > 6000 ? "Use no more than 6000 characters of context" : ""
            }
            serverError={error}
          >
            <textarea
              rows={6}
              value={draft}
              disabled={busy}
              placeholder="For example: Our budget starts on the 20th. Keep household groceries separate from eating out."
              onChange={(e) => {
                setDraft(e.target.value);
                setError("");
              }}
            />
          </Field>
          <p className="footnote" aria-live="polite">
            {length.toLocaleString("en-ZA")} / 6,000 characters
            {draft !== saved.context ? " · Unsaved changes" : ""}
          </p>
          <div className="editor-actions">
            <Button
              type="submit"
              variant="primary"
              disabled={busy || draft === saved.context}
              loading={busy}
            >
              Save context
            </Button>
            <Button disabled={busy} onClick={reload}>
              Reload saved context
            </Button>
          </div>
        </Form>
      )}
    </section>
  );
}
