import type { Dispatch, SetStateAction, RefObject } from "react";
import { Button, Loading, Toast } from "../../ui";
import { setCSRF } from "../../api";
import { Login, Setup } from "./Auth";
import type { useSession } from "./useSession";
import type { PageProps } from "../../shared/types";
type Notice = {
  id: number;
  message: string;
  error: boolean;
};
export function SessionScreen({
  session,
  notice,
  setNotice,
  notify,
  noticeSequence,
}: {
  session: ReturnType<typeof useSession>;
  notice: Notice | null;
  setNotice: Dispatch<SetStateAction<Notice | null>>;
  notify: PageProps["notify"];
  noticeSequence: RefObject<number>;
}) {
  const {
    user,
    setUser,
    ready,
    setup,
    setSetup,
    startupError,
    setStartupRetry,
  } = session;
  if (!ready)
    return (
      <div className="initial">
        <Loading>Loading your workspace</Loading>
      </div>
    );
  if (startupError)
    return (
      <main className="login">
        <div className="login-panel">
          <h1>Unable to load your workspace</h1>
          <p role="alert" className="error-text">
            {startupError}
          </p>
          <Button onClick={() => setStartupRetry((v) => v + 1)}>Retry</Button>
        </div>
      </main>
    );
  if (setup)
    return (
      <Setup
        onComplete={(u) => {
          setSetup(null);
          setUser(u);
          setNotice(null);
        }}
        onClosed={() => {
          setSetup(null);
          setCSRF("");
          setNotice({
            id: ++noticeSequence.current,
            message:
              "Setup is already complete. Sign in with your administrator account.",
            error: false,
          });
        }}
      />
    );
  if (!user)
    return (
      <>
        <Login
          onAttempt={() => setNotice(null)}
          onError={(message) => notify(message, true)}
          onLogin={(u) => {
            setUser(u);
            setNotice(null);
          }}
        />
        {notice && (
          <Toast
            key={notice.id}
            message={notice.message}
            error={notice.error}
            autoDismiss={false}
            onDismiss={() => setNotice(null)}
          />
        )}
      </>
    );
  return null;
}
