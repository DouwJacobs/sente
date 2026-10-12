package app

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestFNBPasswordRecoveryDisconnectsOnlyRecoveredUser(t *testing.T) {
	for _, mode := range []string{"admin", "offline", "self", "failed_admin", "failed_offline"} {
		t.Run(mode, func(t *testing.T) {
			e := setup(t)
			connectFNBTest(t, e)
			digest, err := passwordDigest("synthetic-old-password")
			if err != nil {
				t.Fatal(err)
			}
			securityExec(t, e, "UPDATE users SET password=? WHERE id=1", digest)
			securityExec(t, e, "UPDATE users SET admin=1 WHERE id=2")
			securityExec(t, e, "INSERT INTO fnb_connections(user_id,secret,interval_hours) VALUES(2,x'00',24)")
			securityExec(t, e, "INSERT INTO fnb_discoveries(user_id,bank_id,name,account_id,hidden) VALUES(1,'12345678901','Synthetic retained',1,1)")
			securityExec(t, e, "INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance) VALUES(1,1,'2026-10-12',-100,'Synthetic retained','2026-10-12',-100,'Synthetic retained','{}')")
			if mode == "failed_admin" || mode == "failed_offline" {
				securityExec(t, e, "CREATE TRIGGER deny_recovery_audit BEFORE INSERT ON audit BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END")
			}
			switch mode {
			case "admin", "failed_admin":
				expected := 200
				if mode == "failed_admin" {
					expected = 500
				}
				status(t, e.req(t, 2, "/api/users/1/password", "POST", map[string]any{"new_password": "synthetic-new-password", "version": 1}), expected)
			case "offline", "failed_offline":
				err = e.a.ResetPassword("owner", "synthetic-new-password")
				if (mode == "offline") != (err == nil) {
					t.Fatal("unexpected offline recovery outcome", err)
				}
			case "self":
				status(t, e.req(t, 1, "/api/password", "POST", map[string]string{"old_password": "synthetic-old-password", "new_password": "synthetic-new-password"}), 200)
			}
			disconnected := mode == "admin" || mode == "offline"
			want := int64(1)
			if disconnected {
				want = 0
			}
			if queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE user_id=1") != want {
				t.Fatal("wrong recovered-owner credential retention")
			}
			if queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE user_id=2 AND interval_hours=24") != 1 {
				t.Fatal("other owner's connection changed")
			}
			if queryInt(e.a.DB, "SELECT COUNT(*) FROM accounts") != 2 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE id=1 AND amount_cents=-100") != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_discoveries WHERE user_id=1 AND hidden=1") != 1 {
				t.Fatal("recovery changed accounts/history/discovery preferences")
			}
			key, err := e.a.fnbKey(false)
			clear(key)
			if err != nil {
				t.Fatal("shared key removed during recovery", err)
			}
			if disconnected {
				if queryInt(e.a.DB, "SELECT COUNT(*) FROM sessions WHERE user_id=1") != 0 {
					t.Fatal("recovered owner's sessions retained")
				}
				if _, err := e.a.runFNB(1, false, false); err == nil {
					t.Fatal("recovered owner could still use saved bank connection")
				}
			} else {
				status(t, e.req(t, 1, "/api/me", "GET", nil), 200)
			}
		})
	}
}

func TestFNBRecoveryCancelsActiveScheduledRunWithoutPersistence(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	securityExec(t, e, "UPDATE users SET admin=1 WHERE id=2")
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	e.a.fnbProvider = func(ctx context.Context, c fnbCredentials, visible bool) (fnbSnapshot, error) {
		close(started)
		select {
		case <-ctx.Done():
		case <-release:
		}
		balance := "123.45"
		return fnbSnapshot{Accounts: []fnbAccountSnapshot{{Name: "Synthetic revoked", BankID: "99999111", Balance: &balance}}}, nil
	}
	done := make(chan error, 1)
	go func() { done <- e.a.refreshFNB(1, false) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("synthetic provider did not start")
	}
	status(t, e.req(t, 2, "/api/users/1/password", "POST", map[string]any{"new_password": "synthetic-recovered-password", "version": 1}), 200)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled provider accepted")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("recovery did not cancel active provider")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM accounts WHERE bank_id='99999111'") != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE user_id=1") != 0 {
		t.Fatal("recovered connection persisted provider output")
	}
}

func TestFNBOldCredentialRecordCannotAuthorizePersistence(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	var original []byte
	if err := e.a.DB.QueryRow("SELECT secret FROM fnb_connections WHERE user_id=1").Scan(&original); err != nil {
		t.Fatal(err)
	}
	actor := e.owner
	actor.fnbCredentialHash = hash(string(original))
	securityExec(t, e, "DELETE FROM fnb_connections WHERE user_id=1")
	for _, replacement := range []bool{false, true} {
		if replacement {
			securityExec(t, e, "INSERT INTO fnb_connections(user_id,secret) VALUES(1,x'01')")
		}
		called := false
		err := e.a.fnbWrite(actor, func(tx *sql.Tx, u User) error { called = true; return nil })
		if err == nil || called {
			t.Fatal("removed/replaced credentials authorized old provider work")
		}
	}
}

func TestFNBFailedRecoveryDoesNotCancelActiveRun(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	securityExec(t, e, "UPDATE users SET admin=1 WHERE id=2")
	securityExec(t, e, "CREATE TRIGGER deny_recovery_audit BEFORE INSERT ON audit BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.a.fnbCancelOwner, e.a.fnbCancel = 1, cancel
	status(t, e.req(t, 2, "/api/users/1/password", "POST", map[string]any{"new_password": "synthetic-new-password", "version": 1}), 500)
	if ctx.Err() != nil || queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE user_id=1") != 1 {
		t.Fatal("failed recovery cancelled or disconnected banking")
	}
}
