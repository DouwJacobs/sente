import { useEffect, useSyncExternalStore } from "react";
import { api } from "../api";

export type ApplicationUpdate = {
  state: "available" | "current" | "unsupported" | "unknown" | "unavailable";
  channel?: "stable" | "beta";
  available_version?: string;
  release_url?: string;
  checked_at?: string;
  attempted_at?: string;
  retry_at?: string;
  stale: boolean;
  message?: string;
};
type Snapshot = { status: ApplicationUpdate | null; pending: boolean };
let snapshot: Snapshot = { status: null, pending: false };
const listeners = new Set<() => void>();
function publish(value: Snapshot) {
  snapshot = value;
  listeners.forEach(listener => listener());
}
async function check(force = false) {
  if (snapshot.pending) return;
  publish({ ...snapshot, pending: true });
  try {
    const status = await api<ApplicationUpdate>("/build/update", force ? "POST" : "GET");
    publish({ status, pending: false });
  } catch {
    const previous = snapshot.status;
    publish({ pending: false, status: {
      state: "unavailable", message: "Release check could not be loaded. Try again when your connection is available.",
      stale: Boolean(previous?.checked_at), checked_at: previous?.checked_at,
      available_version: previous?.available_version, release_url: previous?.release_url,
    } });
  }
}
function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}
export function useApplicationUpdate() {
  const value = useSyncExternalStore(subscribe, () => snapshot);
  useEffect(() => {
    if (!snapshot.status) void check();
    const timer = window.setInterval(() => { if (!document.hidden) void check(); }, 6 * 60 * 60 * 1000);
    return () => window.clearInterval(timer);
  }, []);
  return { ...value, check: () => check(true) };
}
export function updateLabel(status: ApplicationUpdate | null): string {
  if (!status) return "Checking application releases…";
  if (status.state === "available") return `Update available: ${status.available_version}`;
  if (status.state === "current") return "Up to date for this release channel";
  if (status.state === "unsupported") return "Release checks unsupported for this build";
  if (status.state === "unknown") return "Update status unknown";
  return "Release check unavailable";
}
