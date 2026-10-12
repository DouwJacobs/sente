package app

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type fnbCredentials struct {
	Username            string   `json:"username"`
	Password            string   `json:"password"`
	Hidden              []string `json:"hidden,omitempty"`
	TransactionAccounts []string `json:"transaction_accounts,omitempty"`
	RunID               string   `json:"run_id,omitempty"`
}
type fnbAccountSnapshot struct {
	AccountType string  `json:"account_type,omitempty"`
	Name        string  `json:"name"`
	BankID      string  `json:"bank_id"`
	Balance     *string `json:"balance_decimal"`
}
type fnbSnapshot struct {
	Accounts    []fnbAccountSnapshot `json:"accounts"`
	Reports     []FNBReport          `json:"reports,omitempty"`
	Skipped     int                  `json:"skipped"`
	Error       string               `json:"error,omitempty"`
	Diagnostics map[string]int       `json:"diagnostics,omitempty"`
}

func (a *App) fnbKey(create bool) ([]byte, error) {
	path := a.fnbKeyPath
	if path == "" {
		path = os.Getenv("FNB_KEY_FILE")
	}
	if path == "" {
		home, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, "finance-tracker", "fnb.key")
	}
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("invalid key location")
	}
	if create {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	path = filepath.Join(parent, filepath.Base(path))
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for _, directory := range []string{cwd, a.BackupDir} {
		if directory == "" {
			continue
		}
		absolute, err := filepath.Abs(directory)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(absolute, path)
		if err != nil {
			return nil, err
		}
		if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("key must be outside workspace and backups")
		}
	}

	// Pin the private directory and validate the opened file, not a separate
	// pathname lookup. Shared writable directories permit key substitution.
	dirFD, err := syscall.Open(parent, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	directory := os.NewFile(uintptr(dirFD), parent)
	defer directory.Close()
	directoryInfo, err := directory.Stat()
	if err != nil {
		return nil, err
	}
	owner, ok := directoryInfo.Sys().(*syscall.Stat_t)
	if !ok || int(owner.Uid) != os.Geteuid() || directoryInfo.Mode().Perm()&0022 != 0 {
		return nil, fmt.Errorf("key directory must be runtime-owned and not group or other writable")
	}
	name := filepath.Base(path)
	fd, err := syscall.Openat(dirFD, name, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err == nil {
		file := os.NewFile(uintptr(fd), path)
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			return nil, err
		}
		owner, ok := info.Sys().(*syscall.Stat_t)
		if !ok || int(owner.Uid) != os.Geteuid() || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return nil, fmt.Errorf("invalid key permissions")
		}
		key, err := io.ReadAll(io.LimitReader(file, 33))
		if err != nil || len(key) != 32 {
			clear(key)
			return nil, fmt.Errorf("invalid key")
		}
		return key, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if !create {
		return nil, fmt.Errorf("key unavailable")
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	fd, err = syscall.Openat(dirFD, name, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	if _, err = file.Write(key); err != nil {
		return nil, err
	}
	if err = file.Sync(); err != nil {
		return nil, err
	}
	if err = directory.Sync(); err != nil {
		clear(key)
		return nil, err
	}
	return key, nil
}
func fnbSeal(key, plain []byte, owner int64) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return g.Seal(nonce, nonce, plain, []byte(fmt.Sprintf("fnb-v1:%d", owner))), nil
}
func fnbUnseal(key, secret []byte, owner int64) (fnbCredentials, error) {
	var out fnbCredentials
	block, err := aes.NewCipher(key)
	if err != nil {
		return out, err
	}
	g, err := cipher.NewGCM(block)
	if err != nil || len(secret) < g.NonceSize() {
		return out, fmt.Errorf("invalid secret")
	}
	plain, err := g.Open(nil, secret[:g.NonceSize()], secret[g.NonceSize():], []byte(fmt.Sprintf("fnb-v1:%d", owner)))
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(plain, &out)
	clear(plain)
	return out, err
}
