import { useEffect, useState } from "react";
import { api, setCSRF } from "../../api";
import type { Row, PageProps } from "../../shared/types";
export function useSession({
  notify,
  onExpired,
}: {
  notify: PageProps["notify"];
  onExpired: () => void;
}) {
  const [user, setUser] = useState<Row | null>(null),
    [ready, setReady] = useState(false);
  const [setup, setSetup] = useState<Row | null>(null),
    [startupError, setStartupError] = useState(""),
    [startupRetry, setStartupRetry] = useState(0);
  useEffect(() => {
    let alive = true;
    setReady(false);
    setStartupError("");
    api("/setup")
      .then(async (status) => {
        if (!alive) return;
        if (status.required) {
          setSetup(status);
          setCSRF(status.csrf);
          return;
        }
        setSetup(null);
        try {
          const v = await api("/me");
          if (alive) {
            setUser(v.user);
            setCSRF(v.csrf);
          }
        } catch {}
      })
      .catch((e) => alive && setStartupError(e.message))
      .finally(() => alive && setReady(true));
    return () => {
      alive = false;
    };
  }, [startupRetry]);
  useEffect(() => {
    const handler = (event: Event) =>
      notify((event as CustomEvent<string>).detail, true);
    window.addEventListener("finance-request-error", handler);
    return () => window.removeEventListener("finance-request-error", handler);
  }, []);
  useEffect(() => {
    const expired = () => {
      setUser(null);
      onExpired();
    };
    window.addEventListener("session-expired", expired);
    return () => window.removeEventListener("session-expired", expired);
  }, []);
  return {
    user,
    setUser,
    ready,
    setup,
    setSetup,
    startupError,
    setStartupRetry,
  };
}
