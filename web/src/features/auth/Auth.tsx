import type { Row } from "../../shared/types";
import { useState, useRef } from "react";
import { api, setCSRF } from "../../api";
import { Button, Field, Form, Toast } from "../../ui";
import { passwordError, usernameError } from "../../validation";
export function Setup({
  onComplete,
  onClosed,
}: {
  onComplete: (user: Row) => void;
  onClosed: () => void;
}) {
  const [usernameServerError, setUsernameServerError] = useState("");
  const [username, setUsername] = useState(""),
    [password, setPassword] = useState(""),
    [confirmation, setConfirmation] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  return (
    <main className="login">
      <div className="login-brand">
        <span className="brand-mark">
          <img width={32} height={32} className="brand-icon" src="/branding/icon.png" alt="" />
        </span>
        Sente
      </div>
      <div className="login-panel">
        <h1>Create admin account</h1>
        <p className="muted">Set up Sente for your household.</p>
        <Form
          onSubmit={async (e) => {
            e.preventDefault();
            setError("");
            setBusy(true);
            try {
              const v = await api("/setup", "POST", { username, password });
              setCSRF(v.csrf);
              onComplete(v.user);
            } catch (e) {
              const message = (e as Error).message;
              if (message.toLowerCase().includes("username already exists"))
                setUsernameServerError(message);
              else setError(message);
              try {
                const status = await api("/setup");
                if (!status.required) onClosed();
                else setCSRF(status.csrf);
              } catch {}
            } finally {
              setBusy(false);
            }
          }}
        >
          <Field
            label="Username"
            validate={usernameError}
            serverError={usernameServerError}
          >
            <input
              autoFocus
              autoComplete="username"
              required
              minLength={2}
              maxLength={80}
              value={username}
              onChange={(e) => {
                setUsername(e.target.value);
                setUsernameServerError("");
              }}
            />
          </Field>
          <Field label="Password" validate={passwordError}>
            <input
              type="password"
              autoComplete="new-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </Field>
          <Field
            label="Confirm password"
            validate={(value) =>
              value !== password ? "Passwords do not match." : ""
            }
          >
            <input
              type="password"
              autoComplete="new-password"
              required
              value={confirmation}
              onChange={(e) => setConfirmation(e.target.value)}
            />
          </Field>
          {error && (
            <Toast message={error} error onDismiss={() => setError("")} />
          )}
          <Button
            variant="primary"
            loading={busy}
            disabled={busy}
            type="submit"
          >
            Create account
          </Button>
        </Form>
      </div>
    </main>
  );
}
export function Login({
  onLogin,
  onAttempt,
  onError,
}: {
  onLogin: (user: Row) => void;
  onAttempt: () => void;
  onError: (message: string) => void;
}) {
  const [username, setUsername] = useState(""),
    [password, setPassword] = useState(""),
    [busy, setBusy] = useState(false),
    pending = useRef(false);
  return (
    <main className="login">
      <div className="login-brand">
        <span className="brand-mark">
          <img width={32} height={32} className="brand-icon" src="/branding/icon.png" alt="" />
        </span>
        Sente
      </div>
      <div className="login-panel">
        <h1>Welcome back</h1>
        <p className="muted">Sign in to Sente.</p>
        <Form
          onSubmit={async (e) => {
            e.preventDefault();
            if (pending.current) return;
            pending.current = true;
            setBusy(true);
            onAttempt();
            try {
              const v = await api("/login", "POST", { username, password });
              setCSRF(v.csrf);
              onLogin(v.user);
            } catch (e) {
              onError((e as Error).message);
            } finally {
              pending.current = false;
              setBusy(false);
            }
          }}
        >
          <Field label="Username">
            <input
              autoComplete="username"
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
          </Field>
          <Field label="Password">
            <input
              type="password"
              autoComplete="current-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </Field>
          <Button
            variant="primary"
            loading={busy}
            disabled={busy}
            type="submit"
          >
            Sign in
          </Button>
        </Form>
      </div>
    </main>
  );
}
