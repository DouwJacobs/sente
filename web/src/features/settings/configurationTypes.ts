export type Source = {
  id: number;
  name: string;
  kind: "repository" | "file" | "starter";
  repository_url: string;
  ref: string;
  path: string;
  version: number;
  last_revision?: string;
  last_sync?: string;
};
export type Change = {
  entity: string;
  name: string;
  action: "create" | "update";
  before: Record<string, unknown> | null;
  after: Record<string, unknown>;
};
export type Preview = { id: string; source: Source; changes: Change[] };
