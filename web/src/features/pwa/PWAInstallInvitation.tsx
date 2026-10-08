import { useEffect, useState } from "react";
import { Button, Toast } from "../../ui";
import { usePWA } from "./PWAProvider";

import {
  wasInstallInvitationShown,
  rememberInstallInvitation,
} from "./installInvitationPreference";

// Signed-in composition owns the invitation: do not interrupt onboarding/login/OAuth.
export function PWAInstallInvitation() {
  const pwa = usePWA();
  const [visible, setVisible] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const ios =
    /iPad|iPhone|iPod/.test(navigator.userAgent) ||
    (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
  const eligible =
    window.isSecureContext &&
    (pwa.installable || ios) &&
    !pwa.installed &&
    !pwa.updateAvailable;
  useEffect(() => {
    if (!eligible || wasInstallInvitationShown()) return;
    // Defer until the workspace settles and no modal is open. No recurring reminder.
    const timer = window.setInterval(() => {
      if (wasInstallInvitationShown()) {
        clearInterval(timer);
        return;
      }
      if (
        document.visibilityState !== "visible" ||
        document.querySelector("dialog[open]")
      )
        return;
      rememberInstallInvitation();
      setVisible(true);
      clearInterval(timer);
    }, 4000);
    return () => clearInterval(timer);
  }, [eligible]);
  useEffect(() => {
    if (pwa.installed || pwa.updateAvailable) setVisible(false);
  }, [pwa.installed, pwa.updateAvailable]);
  if (!visible) return null;
  return (
    <Toast
      message={
        error ||
        (pwa.installable
          ? "Keep Sente handy: install it on your home screen or desktop. You can also install later in Settings → PWA."
          : "Add Sente to your home screen: in Safari, choose Share → Add to Home Screen. You can find this again in Settings → PWA.")
      }
      error={!!error}
      autoDismiss={false}
      onDismiss={() => setVisible(false)}
      action={
        pwa.installable ? (
          <Button
            loading={busy}
            disabled={busy}
            onClick={async () => {
              if (busy) return;
              setBusy(true);
              try {
                await pwa.install();
                setVisible(false);
              } catch (error) {
                setError((error as Error).message);
              } finally {
                setBusy(false);
              }
            }}
          >
            Install now
          </Button>
        ) : undefined
      }
    />
  );
}
