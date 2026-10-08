import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { Button, Toast } from "../../ui";
import { rememberInstallInvitation } from "./installInvitationPreference";
export type InstallEvent = Event & {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: string }>;
};
type PWAState = {
  installed: boolean;
  installable: boolean;
  updateAvailable: boolean;
  install: () => Promise<void>;
  update: () => void;
  check: () => Promise<void>;
};
const Context = createContext<PWAState>({
  installed: false,
  installable: false,
  updateAvailable: false,
  install: async () => {},
  update: () => {},
  check: async () => {},
});
export const usePWA = () => useContext(Context);
export function PWAProvider({ children }: { children: ReactNode }) {
  const [installEvent, setInstallEvent] = useState<InstallEvent | null>(null);
  const [installed, setInstalled] = useState(
    matchMedia("(display-mode: standalone)").matches ||
      !!(navigator as Navigator & { standalone?: boolean }).standalone,
  );
  const [registration, setRegistration] =
    useState<ServiceWorkerRegistration | null>(null);
  const [waiting, setWaiting] = useState<ServiceWorker | null>(null);
  const [dismissed, setDismissed] = useState(false);
  const updating = useRef(false);
  useEffect(() => {
    let alive = true;
    const install = (event: Event) => {
      event.preventDefault();
      setInstallEvent(event as InstallEvent);
    };
    const done = () => {
      setInstalled(true);
      setInstallEvent(null);
    };
    window.addEventListener("beforeinstallprompt", install);
    window.addEventListener("appinstalled", done);
    if (window.isSecureContext && "serviceWorker" in navigator) {
      navigator.serviceWorker
        .register("/push-sw.js", { scope: "/", updateViaCache: "none" })
        .then((reg) => {
          if (!alive) return;
          setRegistration(reg);
          const inspect = () => {
            if (alive && reg.waiting && navigator.serviceWorker.controller) {
              setWaiting(reg.waiting);
              setDismissed(false);
            }
          };
          inspect();
          reg.addEventListener?.("updatefound", () => {
            reg.installing?.addEventListener("statechange", inspect);
          });
        })
        .catch(() => {});
    }
    return () => {
      alive = false;
      window.removeEventListener("beforeinstallprompt", install);
      window.removeEventListener("appinstalled", done);
    };
  }, []);
  useEffect(() => {
    if (!registration) return;
    const check = () => {
      if (document.visibilityState === "visible" && navigator.onLine)
        registration.update?.().catch(() => {});
    };
    window.addEventListener("online", check);
    document.addEventListener("visibilitychange", check);
    const timer = window.setInterval(check, 60 * 60 * 1000);
    return () => {
      clearInterval(timer);
      window.removeEventListener("online", check);
      document.removeEventListener("visibilitychange", check);
    };
  }, [registration]);
  const update = () => {
    if (!waiting || updating.current) return;
    updating.current = true;
    if (waiting.state === "activated") {
      window.location.reload();
      return;
    }
    // Reload only the consenting tab; other tabs keep their mounted drafts.
    const reload = () => window.location.reload();
    navigator.serviceWorker.addEventListener("controllerchange", reload, {
      once: true,
    });
    waiting.postMessage({ type: "SKIP_WAITING" });
  };
  return (
    <Context.Provider
      value={{
        installed,
        installable: !!installEvent,
        updateAvailable: !!waiting,
        update,
        install: async () => {
          if (installEvent) {
            rememberInstallInvitation();
            try {
              await installEvent.prompt();
              await installEvent.userChoice;
            } finally {
              setInstallEvent(null);
            }
          }
        },
        check: async () => {
          if (!registration) {
            throw new Error(
              "App updates are not ready in this browser. Use HTTPS or localhost, then reload and try again.",
            );
          }
          await registration.update();
        },
      }}
    >
      {children}
      {waiting && !dismissed && (
        <Toast
          message="A Sente update is available. Reloading clears unsaved changes in this tab."
          error={false}
          autoDismiss={false}
          onDismiss={() => setDismissed(true)}
          action={<Button onClick={update}>Update and reload</Button>}
        />
      )}
    </Context.Provider>
  );
}
