import { useEffect, useState } from "react";
import { api } from "../api";

export type BuildInfo = {
  version: string;
  commit?: string;
  built_at?: string;
  modified: boolean;
};
export const repositoryURL = "https://github.com/DouwJacobs/sente";

export function buildLabel(info: BuildInfo): string {
  const commit = info.commit ? ` (${info.commit.slice(0, 12)})` : "";
  return `${info.version}${commit}${info.modified ? " · modified" : ""}`;
}

// Deliberately accept only build fields: never serialize workspace, user, account or browser data.
export function issueReportURL(info: BuildInfo | null): string {
  const details = info
    ? [`Version: ${info.version}`, `Commit: ${info.commit || "Unavailable"}`,
       `Build revision date: ${info.built_at || "Unavailable"}`, `Modified: ${info.modified ? "Yes" : "No"}`].join("\n")
    : "Build information unavailable";
  const body = `## What happened?\n\n\n## Steps to reproduce\n\n\n## Expected behavior\n\n\n## Sente build\n${details}\n\nPlease leave out financial, account and personal information.\n`;
  return `${repositoryURL}/issues/new?${new URLSearchParams({ body })}`;
}

export function useBuildInfo() {
  const [info, setInfo] = useState<BuildInfo | null>(null);
  const [error, setError] = useState("");
  const [attempt, retry] = useState(0);
  useEffect(() => {
    let alive = true;
    setError("");
    api<BuildInfo>("/build").then((value) => { if (alive) setInfo(value); })
      .catch(() => { if (alive) setError("Build information could not be loaded."); });
    return () => { alive = false; };
  }, [attempt]);
  return { info, error, retry: () => retry((value) => value + 1) };
}
