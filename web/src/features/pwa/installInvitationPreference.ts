const storageKey = "sente-pwa-install-invitation";
let shownInPage = false;
export function wasInstallInvitationShown() {
  if (shownInPage) return true;
  try {
    return localStorage.getItem(storageKey) !== null;
  } catch {
    return false;
  }
}
export function rememberInstallInvitation() {
  shownInPage = true;
  try {
    localStorage.setItem(storageKey, "shown");
  } catch {
    /* Private storage can be unavailable. */
  }
}
