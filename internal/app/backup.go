package app

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

func lockDB(path string) (*os.File, error) {
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("database is in use; stop the service before running this command")
	}
	return f, nil
}
func (a *App) Backup() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := os.MkdirAll(a.BackupDir, 0700); err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(filepath.Join(a.BackupDir, "finance-"+time.Now().UTC().Format("20060102T150405.000000000")+"-"+randomToken()[:6]+".sqlite"))
	if err != nil {
		return "", err
	}
	if _, err := a.DB.Exec("VACUUM INTO ?", absolute); err != nil {
		return "", err
	}
	if err := os.Chmod(absolute, 0600); err != nil {
		return "", err
	}
	db, err := sql.Open("sqlite", "file:"+absolute+"?mode=ro")
	if err != nil {
		return "", err
	}
	var check string
	err = db.QueryRow("PRAGMA integrity_check").Scan(&check)
	db.Close()
	if err != nil || check != "ok" {
		os.Remove(absolute)
		return "", fmt.Errorf("snapshot integrity check failed")
	}
	if _, err := a.DB.Exec("INSERT INTO backups(path) VALUES(?)", absolute); err != nil {
		return "", err
	}
	entries, err := filepath.Glob(filepath.Join(a.BackupDir, "finance-*.sqlite"))
	if err != nil {
		return "", err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(entries)))
	for i := 14; i < len(entries); i++ {
		if err := os.Remove(entries[i]); err != nil {
			return "", err
		}
	}
	return absolute, nil
}
func (a *App) StartBackups() {
	a.backupWG.Add(1)
	go func() {
		defer a.backupWG.Done()
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			var last string
			a.DB.QueryRow("SELECT created_at FROM backups ORDER BY id DESC LIMIT 1").Scan(&last)
			t, _ := time.Parse("2006-01-02 15:04:05", last)
			if time.Since(t) >= 24*time.Hour {
				_, err := a.Backup()
				a.mu.Lock()
				if err != nil {
					a.backupError = "Automatic backup failed; check the backup directory"
				} else {
					a.backupError = ""
				}
				a.mu.Unlock()
			}
			select {
			case <-a.stop:
				return
			case <-ticker.C:
			}
		}
	}()
}
func (a *App) backups(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(Current(r)); err != nil {
		return err
	}
	rows, err := data(a.DB, "SELECT id,created_at FROM backups ORDER BY id DESC LIMIT 14")
	if err != nil {
		return err
	}
	a.mu.Lock()
	message := a.backupError
	a.mu.Unlock()
	send(w, map[string]any{"items": rows, "error": message, "retention": 14})
	return nil
}
func (a *App) backupNow(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(Current(r)); err != nil {
		return err
	}
	if _, err := a.Backup(); err != nil {
		return err
	}
	success(w)
	return nil
}
func Restore(target, snapshot string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	lock, err := lockDB(target)
	if err != nil {
		return err
	}
	defer lock.Close()
	source, err := filepath.Abs(snapshot)
	if err != nil {
		return err
	}
	destination, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if source == destination {
		return fmt.Errorf("snapshot and destination must differ")
	}
	db, err := sql.Open("sqlite", "file:"+source+"?mode=ro")
	if err != nil {
		return err
	}
	var check string
	err = db.QueryRow("PRAGMA integrity_check").Scan(&check)
	var version int
	verErr := db.QueryRow("SELECT MAX(version) FROM migrations").Scan(&version)
	db.Close()
	if err != nil || check != "ok" || verErr != nil || (version < 1 || version > schemaVersion) {
		return fmt.Errorf("not a valid supported finance snapshot")
	}
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	tmp, err := os.CreateTemp(filepath.Dir(target), ".restore-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	// Invalidate sessions in the prepared database before the atomic replacement.
	prepared, err := sql.Open("sqlite", tmpName)
	if err != nil {
		return err
	}
	if _, err := prepared.Exec("PRAGMA journal_mode=DELETE; DELETE FROM sessions;"); err != nil {
		prepared.Close()
		return err
	}
	// Older supported snapshots predate MCP; only clear tables they contain.
	for _, table := range []string{"mcp_oauth_codes", "mcp_access_tokens", "mcp_refresh_tokens", "mcp_oauth_requests", "mcp_oauth_clients", "mcp_proposals", "mcp_tokens"} {
		if queryInt(prepared, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table) == 1 {
			if _, err := prepared.Exec("DELETE FROM " + table); err != nil {
				prepared.Close()
				return err
			}
		}
	}
	if err := prepared.Close(); err != nil {
		return err
	}
	ready, err := os.Open(tmpName)
	if err != nil {
		return err
	}
	err = ready.Sync()
	ready.Close()
	if err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		old, err := sql.Open("sqlite", target)
		if err != nil {
			return err
		}
		var busy, logPages, checkpointed int
		err = old.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logPages, &checkpointed)
		old.Close()
		if err != nil || busy != 0 {
			return fmt.Errorf("could not checkpoint previous database")
		}
		previous := target + ".before-restore-" + time.Now().UTC().Format("20060102T150405.000000000")
		in, err := os.Open(target)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(previous, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		in.Close()
		syncErr := out.Sync()
		out.Close()
		if copyErr != nil {
			os.Remove(previous)
			return copyErr
		}
		if syncErr != nil {
			os.Remove(previous)
			return syncErr
		}
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(target + suffix); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.Rename(tmpName, target); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(target))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func safePath(s string) bool { return !strings.ContainsRune(s, 0) }
