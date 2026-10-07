package main

import (
	"bufio"
	"context"
	"finance-tracker/internal/app"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
func password() (string, error) {
	if v := os.Getenv("FINANCE_PASSWORD"); v != "" {
		return v, nil
	}
	fmt.Fprintln(os.Stderr, "Read password from stdin (12–72 bytes):")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return "", fmt.Errorf("password required")
	}
	return strings.TrimSuffix(scanner.Text(), "\r"), nil
}

var errRestart = fmt.Errorf("restart requested")

func main() {
	for {
		err := run()
		if err == errRestart {
			if os.Getenv("DEV_RESTART_MANAGED") == "1" {
				os.Exit(75)
			}
			continue
		}
		if err != nil {
			log.Fatal(err)
		}
		return
	}
}
func run() error {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	dbPath := env("DATABASE_PATH", "data/finance.sqlite")
	if command == "export-ruleset" || command == "import-ruleset" {
		if len(os.Args) > 3 {
			return fmt.Errorf("usage: finance %s [FILE|-]", command)
		}
		if os.Getenv("DATABASE_PATH") == "" || os.Getenv("FINANCE_RULESET_USER") == "" {
			return fmt.Errorf("ruleset commands require explicit DATABASE_PATH and FINANCE_RULESET_USER")
		}
		if info, err := os.Stat(dbPath); err != nil || info.IsDir() {
			return fmt.Errorf("ruleset database must already exist")
		}
		if command == "export-ruleset" && len(os.Args) == 3 && os.Args[2] != "-" {
			source, err := filepath.Abs(dbPath)
			if err != nil {
				return err
			}
			target, err := filepath.Abs(os.Args[2])
			if err != nil {
				return err
			}
			if source == target {
				return fmt.Errorf("ruleset export must not overwrite the database")
			}
			if sourceInfo, err := os.Stat(source); err == nil {
				if targetInfo, err := os.Stat(target); err == nil && os.SameFile(sourceInfo, targetInfo) {
					return fmt.Errorf("ruleset export must not overwrite the database")
				}
			}
		}
	}
	backupDir := env("BACKUP_DIR", "backups")
	if command == "restore" {
		if len(os.Args) != 3 {
			return fmt.Errorf("usage: finance restore SNAPSHOT")
		}
		return app.Restore(dbPath, os.Args[2])
	}
	a, err := app.Open(dbPath, env("PUBLIC_URL", "http://localhost:8080"), backupDir)
	if err != nil {
		return err
	}
	defer a.Close()
	if err := a.Initialize(); err != nil {
		return err
	}
	switch command {
	case "create-admin":
		if len(os.Args) != 3 {
			return fmt.Errorf("usage: finance create-admin USERNAME")
		}
		var n int
		a.DB.QueryRow("SELECT COUNT(*) FROM users WHERE admin=1").Scan(&n)
		if n > 0 {
			return fmt.Errorf("administrator already exists; use reset-password")
		}
		p, err := password()
		if err != nil {
			return err
		}
		_, err = a.CreateUser(os.Args[2], p, true, true)
		return err
	case "reset-password":
		if len(os.Args) != 3 {
			return fmt.Errorf("usage: finance reset-password USERNAME")
		}
		p, err := password()
		if err != nil {
			return err
		}
		return a.ResetPassword(os.Args[2], p)
	case "reset-network":
		return a.ResetNetworkSettings()
	case "export-ruleset":
		target := "-"
		if len(os.Args) > 2 {
			target = os.Args[2]
		}
		return a.ExportRulesetToFile(target)
	case "import-ruleset":
		source := "-"
		if len(os.Args) > 2 {
			source = os.Args[2]
		}
		summary, err := a.ImportRulesetFromFile(source)
		if err != nil {
			return err
		}
		fmt.Println(summary)
		return nil
	case "backup":
		path, err := a.Backup()
		if err == nil {
			fmt.Println(path)
		}
		return err
	case "serve":
		if err := a.ApplyNetworkConfig(os.Getenv("TRUSTED_PROXIES")); err != nil {
			return err
		}
		port, err := strconv.Atoi(env("PORT", "8080"))
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("PORT must be 1–65535")
		}
		server := &http.Server{Addr: net.JoinHostPort(os.Getenv("LISTEN_ADDRESS"), strconv.Itoa(port)), Handler: a.Handler(env("STATIC_DIR", "web/dist")), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 60 * time.Second, WriteTimeout: 240 * time.Second, IdleTimeout: 120 * time.Second, MaxHeaderBytes: 1 << 20}
		restart := make(chan struct{}, 1)
		a.RequestRestart = func() {
			select {
			case restart <- struct{}{}:
			default:
			}
		}
		restarting := false
		shutdownDone := make(chan struct{})
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(signals)
		go func() {
			defer close(shutdownDone)
			select {
			case <-signals:
			case <-restart:
				restarting = true
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			server.Shutdown(ctx)
		}()
		a.StartBackups()
		a.StartFNBScheduler()
		log.Printf("Sente listening on port %d", port)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			return err
		}
		<-shutdownDone
		if restarting {
			return errRestart
		}
		return nil
	default:
		return fmt.Errorf("commands: serve, create-admin, reset-password, backup, restore, reset-network, export-ruleset, import-ruleset")
	}
}
