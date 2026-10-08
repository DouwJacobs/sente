package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func enqueueNotificationPushTx(tx *sql.Tx, e notificationEvent, id int64, now time.Time) error {
	if queryInt(tx, "SELECT COUNT(*) FROM notification_push_preferences WHERE user_id=? AND type=? AND enabled=1", e.RecipientID, e.Type) == 0 {
		return notificationDiagnosticTx(tx, e.RecipientID, e.Type, hash(e.DedupeKey), "push", "disabled", "", now)
	}
	result, err := tx.Exec(`INSERT OR IGNORE INTO notification_push_outbox(subscription_id,notification_id,type,key_hash,next_attempt,created_at)
 SELECT id,?,?,?,?,? FROM notification_push_subscriptions WHERE user_id=? AND enabled=1`, id, e.Type, hash(e.DedupeKey), now.Unix(), now.Unix(), e.RecipientID)
	if err != nil {
		return err
	}
	status := "pending"
	if count, _ := result.RowsAffected(); count == 0 {
		status = "no_device"
	}
	return notificationDiagnosticTx(tx, e.RecipientID, e.Type, hash(e.DedupeKey), "push", status, "", now)
}

// Pin a checked public IP at the actual dial; reject redirects and untrusted
// provider hosts. No user URL, key or provider error is logged.
func pushHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errors.New("invalid push address")
		}
		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, errors.New("push DNS failed")
		}
		for _, ip := range ips {
			ip = ip.Unmap()
			if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || netip.MustParsePrefix("100.64.0.0/10").Contains(ip) {
				return nil, errors.New("private push address")
			}
		}
		if len(ips) == 0 {
			return nil, errors.New("push DNS empty")
		}
		dialer := net.Dialer{Timeout: 10 * time.Second}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}
	return &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func sendBrowserPush(ctx context.Context, sub webpush.Subscription, private, public, subscriber, topic string, messageID int64) (int, error) {
	if err := validatePushSubscription(sub); err != nil {
		return 0, err
	}
	// Financial fields never leave the server or appear on the lock screen.
	link := "/?notifications=1"
	if messageID > 0 {
		link += fmt.Sprintf("&message=%d", messageID)
	}
	payload, _ := json.Marshal(map[string]string{"title": "Sente", "body": "You have a new notification. Open Sente to view it.", "tag": topic, "url": link})
	response, err := webpush.SendNotificationWithContext(ctx, payload, &sub, &webpush.Options{HTTPClient: pushHTTPClient(), Subscriber: subscriber, VAPIDPrivateKey: private, VAPIDPublicKey: public, TTL: 300, Topic: topic, Urgency: webpush.UrgencyNormal})
	if err != nil {
		return 0, errors.New("push transport failed")
	}
	defer response.Body.Close()
	return response.StatusCode, nil
}

type pushJob struct {
	id, uid, nid, subscription, attempts int64
	kind, key, private, public           string
	sub                                  webpush.Subscription
}

func (a *App) dispatchNotificationPush(now time.Time) error {
	for count := 0; count < 50; count++ {
		select {
		case <-a.stop:
			return nil
		default:
		}
		var job pushJob
		err := a.writeQuiet(func(tx *sql.Tx) error {
			err := tx.QueryRow(`SELECT o.id,s.user_id,COALESCE(o.notification_id,0),s.id,o.attempts,o.type,o.key_hash,s.endpoint,s.p256dh,s.auth
   FROM notification_push_outbox o JOIN notification_push_subscriptions s ON s.id=o.subscription_id
   WHERE o.status='pending' AND o.next_attempt<=? ORDER BY o.id LIMIT 1`, now.Unix()).Scan(&job.id, &job.uid, &job.nid, &job.subscription, &job.attempts, &job.kind, &job.key, &job.sub.Endpoint, &job.sub.Keys.P256dh, &job.sub.Keys.Auth)
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			status := ""
			if queryInt(tx, "SELECT COUNT(*) FROM notification_push_subscriptions s JOIN users u ON u.id=s.user_id WHERE s.id=? AND s.enabled=1 AND u.disabled=0 AND u.deleted_at IS NULL", job.subscription) == 0 {
				status = "permission_revoked"
			}
			if job.nid != 0 {
				if queryInt(tx, "SELECT COUNT(*) FROM notifications n WHERE n.id=? AND "+notificationPushVisibilitySQL(), append([]any{job.nid}, notificationReadArgs(job.uid)...)...) == 0 {
					status = "permission_revoked"
				}
				if queryInt(tx, "SELECT COUNT(*) FROM notification_push_preferences WHERE user_id=? AND type=? AND enabled=1", job.uid, job.kind) == 0 {
					status = "disabled"
				}
			}
			if now.Unix()-queryInt(tx, "SELECT created_at FROM notification_push_outbox WHERE id=?", job.id) > 300 {
				status = "expired"
			}
			if status != "" {
				if _, err := tx.Exec("UPDATE notification_push_outbox SET status=? WHERE id=?", status, job.id); err != nil {
					return err
				}
				return notificationDiagnosticTx(tx, job.uid, job.kind, job.key, "push", status, "", now)
			}
			job.private, job.public, err = pushKeysTx(tx)
			if err != nil {
				return err
			}
			_, err = tx.Exec("UPDATE notification_push_outbox SET status='sending',attempts=attempts+1 WHERE id=?", job.id)
			return err
		})
		if err != nil {
			return err
		}
		if job.id == 0 {
			return nil
		}
		// A suppressed job was completed above; never send it outside the lock.
		if queryInt(a.DB, "SELECT COUNT(*) FROM notification_push_outbox WHERE id=? AND status='sending'", job.id) == 0 {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		status, sendErr := a.pushSender(ctx, job.sub, job.private, job.public, a.PublicURL, job.key[:32], job.nid)
		cancel()
		err = a.writeQuiet(func(tx *sql.Tx) error {
			result, code := "sent", ""
			if sendErr != nil {
				result = "uncertain"
				code = "transport_failure"
			} else if status == 404 || status == 410 {
				result = "channel_failure"
				code = "subscription_expired"
				if _, err := tx.Exec("DELETE FROM notification_push_subscriptions WHERE id=?", job.subscription); err != nil {
					return err
				}
			} else if status < 200 || status >= 300 {
				result = "channel_failure"
				code = "provider_rejected"
				if (status == 429 || status >= 500) && job.attempts < 2 {
					result = "pending"
					code = "provider_retry"
				}
			}
			if _, err := tx.Exec("UPDATE notification_push_outbox SET status=?,next_attempt=? WHERE id=?", result, now.Add(time.Duration(job.attempts+1)*time.Minute).Unix(), job.id); err != nil {
				return err
			}
			diagnostic := result
			if result == "pending" {
				diagnostic = "channel_failure"
			}
			return notificationDiagnosticTx(tx, job.uid, job.kind, job.key, "push", diagnostic, code, now)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func recoverUncertainPushTx(tx *sql.Tx, now time.Time) error {
	rows, err := data(tx, `SELECT o.id,s.user_id,o.type,o.key_hash FROM notification_push_outbox o JOIN notification_push_subscriptions s ON s.id=o.subscription_id WHERE o.status='sending'`)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if err := notificationDiagnosticTx(tx, num(r["user_id"]), r["type"].(string), r["key_hash"].(string), "push", "uncertain", "dispatcher_restarted", now); err != nil {
			return err
		}
	}
	_, err = tx.Exec("UPDATE notification_push_outbox SET status='uncertain' WHERE status='sending'")
	return err
}
