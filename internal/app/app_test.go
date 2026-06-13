package app

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bodsch.me/mariadb-backup/internal/config"
	"bodsch.me/mariadb-backup/internal/dbx"
	"bodsch.me/mariadb-backup/internal/logging"
	"bodsch.me/mariadb-backup/internal/notify"
)

type mockLister struct {
	dbs []string
	err error
}

func (m *mockLister) ListDatabases() ([]string, error) { return m.dbs, m.err }
func (m *mockLister) Close() error                     { return nil }

type fakeRunner struct{ rc int }

func (f fakeRunner) Run(_ string, _ []string, out io.Writer) (int, string, error) {
	io.WriteString(out, "-- sql\n")
	return f.rc, "", nil
}

type captureSender struct {
	msg    notify.Message
	called bool
}

func (c *captureSender) Send(m notify.Message) error { c.msg = m; c.called = true; return nil }

func baseDeps(lister dbx.DatabaseLister, rc int, sender notify.Sender) Dependencies {
	return Dependencies{
		OpenDB:    func(string) (dbx.DatabaseLister, error) { return lister, nil },
		NewSender: func(config.SMTP) notify.Sender { return sender },
		Runner:    fakeRunner{rc: rc},
		Logger:    logging.NewCapture(),
	}
}

func TestRunSuccess(t *testing.T) {
	dir := t.TempDir()
	code := RunWith(Options{Directory: dir, LogLevel: "INFO"}, baseDeps(&mockLister{dbs: []string{"db1"}}, 0, &captureSender{}))
	if code != exitOK {
		t.Fatalf("exit = %d, want %d", code, exitOK)
	}
	// One dated subdir with the database inside.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("want 1 dated dir, got %v", entries)
	}
	if _, err := os.Stat(filepath.Join(dir, entries[0].Name(), "db1", "schema.sql")); err != nil {
		t.Errorf("schema.sql missing: %v", err)
	}
}

func TestRunPartialFailureExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	code := RunWith(Options{Directory: dir, LogLevel: "INFO"}, baseDeps(&mockLister{dbs: []string{"db1"}}, 2, &captureSender{}))
	if code != exitPartialFailed {
		t.Fatalf("exit = %d, want %d (partial failure)", code, exitPartialFailed)
	}
}

func TestRunConnectErrorIsFatal(t *testing.T) {
	deps := baseDeps(&mockLister{}, 0, &captureSender{})
	deps.OpenDB = func(string) (dbx.DatabaseLister, error) { return nil, errors.New("connection refused") }
	code := RunWith(Options{Directory: t.TempDir(), LogLevel: "INFO"}, deps)
	if code != exitFatal {
		t.Fatalf("exit = %d, want %d", code, exitFatal)
	}
}

func TestRunListErrorIsFatal(t *testing.T) {
	deps := baseDeps(&mockLister{err: errors.New("denied")}, 0, &captureSender{})
	code := RunWith(Options{Directory: t.TempDir(), LogLevel: "INFO"}, deps)
	if code != exitFatal {
		t.Fatalf("exit = %d, want %d", code, exitFatal)
	}
}

func TestRunSendsEmailWithStrippedBody(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yml")
	content := `
notification:
  enabled: true
  smtp:
    server_name: smtp.example.com
    port: 25
    tls: false
  sender: backup@example.com
  recipient: admin@example.com
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sender := &captureSender{}
	code := RunWith(
		Options{Directory: t.TempDir(), ConfigPath: cfgPath, LogLevel: "INFO"},
		baseDeps(&mockLister{dbs: []string{"db1"}}, 0, sender),
	)
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if !sender.called {
		t.Fatal("email should have been sent")
	}
	if !strings.HasPrefix(sender.msg.Subject, "Database Backup at ") {
		t.Errorf("subject = %q", sender.msg.Subject)
	}
	if sender.msg.Body == "" {
		t.Error("email body should not be empty")
	}
	if strings.Contains(sender.msg.Body, "\033[") {
		t.Errorf("email body must be ANSI-stripped: %q", sender.msg.Body)
	}
}

func TestRunDryRunSendsNoEmailAndWritesNothing(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yml")
	content := `
notification:
  enabled: true
  smtp:
    server_name: smtp.example.com
    port: 25
  sender: backup@example.com
  recipient: admin@example.com
`
	os.WriteFile(cfgPath, []byte(content), 0o644)
	dir := t.TempDir()
	sender := &captureSender{}
	code := RunWith(
		Options{Directory: dir, ConfigPath: cfgPath, DryRun: true, LogLevel: "INFO"},
		baseDeps(&mockLister{dbs: []string{"db1"}}, 0, sender),
	)
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if sender.called {
		t.Error("dry-run must not send email")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("dry-run must not write backups, found %v", entries)
	}
}
