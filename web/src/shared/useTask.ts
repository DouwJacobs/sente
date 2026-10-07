import { useState, useRef } from "react";
import type { PageProps } from "./types";
export function useTask(notify: PageProps["notify"]) {
  const [busy, setBusy] = useState(false),
    pending = useRef(false);
  const run = async (
    fn: () => Promise<unknown>,
    message?: string,
    onError?: (message: string) => boolean,
  ) => {
    if (pending.current) return false;
    pending.current = true;
    setBusy(true);
    try {
      await fn();
      if (message) notify(message);
      return true;
    } catch (e) {
      const message = (e as Error).message;
      if (!onError?.(message)) notify(message, true);
      return false;
    } finally {
      pending.current = false;
      setBusy(false);
    }
  };
  return { busy, run };
}
