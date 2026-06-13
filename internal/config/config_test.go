package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bodsch.me/mariadb-backup/internal/logging"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadMissingReturnsDefaults(t *testing.T) {
	cfg, err := Load("/no/such/file.yml", logging.NewCapture())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.Rotation.Daily != 3 || cfg.Storage.Rotation.Weekly != 2 {
		t.Errorf("defaults not applied: %+v", cfg.Storage.Rotation)
	}
	if cfg.Storage.Compression != CompressionNone {
		t.Errorf("compression default = %q, want none", cfg.Storage.Compression)
	}
	if cfg.Notification.SMTP.Port != 587 {
		t.Errorf("smtp port default = %d, want 587", cfg.Notification.SMTP.Port)
	}
}

func TestExplicitRotationZeroPreserved(t *testing.T) {
	content := "storage:\n  rotation:\n    daily: 0\n    weekly: 0\n"
	cfg, err := Load(writeTemp(t, content), logging.NewCapture())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.Rotation.Daily != 0 || cfg.Storage.Rotation.Weekly != 0 {
		t.Errorf("explicit 0 must be preserved, got %+v", cfg.Storage.Rotation)
	}
}

func TestOmittedRotationGetsDefaults(t *testing.T) {
	// rotation present but daily omitted -> default; weekly explicit -> kept.
	content := "storage:\n  rotation:\n    weekly: 5\n"
	cfg, err := Load(writeTemp(t, content), logging.NewCapture())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.Rotation.Daily != 3 {
		t.Errorf("omitted daily should default to 3, got %d", cfg.Storage.Rotation.Daily)
	}
	if cfg.Storage.Rotation.Weekly != 5 {
		t.Errorf("explicit weekly should be kept, got %d", cfg.Storage.Rotation.Weekly)
	}
}

func TestLoadDeprecationWarning(t *testing.T) {
	content := `
connection:
  username: backup
  password: secret
  port: ''
  commpress_communication: true
  use_ssl: false
includes:
  databases: []
`
	log := logging.NewCapture()
	cfg, err := Load(writeTemp(t, content), log)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Connection.Username != "backup" {
		t.Errorf("username = %q", cfg.Connection.Username)
	}
	// A single deprecation warning listing the legacy keys.
	if !log.Contains("deprecated config keys") {
		t.Fatalf("no deprecation warning emitted; messages=%v", log.Messages())
	}
	for _, want := range []string{"connection.commpress_communication", "connection.use_ssl", "includes"} {
		if !log.Contains(want) {
			t.Errorf("deprecation warning missing %q; messages=%v", want, log.Messages())
		}
	}
}

func TestPortEmptyStringCoercion(t *testing.T) {
	content := "connection:\n  host: db.example.com\n  port: ''\n"
	cfg, err := Load(writeTemp(t, content), logging.NewCapture())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Connection.Port != 0 {
		t.Errorf("empty port should decode to 0, got %d", cfg.Connection.Port)
	}
	dsn := cfg.DSN()
	if !strings.Contains(dsn, "tcp(db.example.com:3306)") {
		t.Errorf("DSN should default port to 3306: %s", dsn)
	}
}

func TestInvalidCompressionRejected(t *testing.T) {
	content := "storage:\n  compression: zstd\n"
	_, err := Load(writeTemp(t, content), logging.NewCapture())
	if err == nil {
		t.Fatal("expected error for invalid compression")
	}
}

func TestDSNSocketTakesPrecedence(t *testing.T) {
	cfg := Default()
	cfg.Connection.Username = "u"
	cfg.Connection.Password = "p@ss:word"
	cfg.Connection.Socket = "/run/mysqld/mysqld.sock"
	cfg.Connection.Host = "ignored"
	dsn := cfg.DSN()
	if !strings.Contains(dsn, "unix(/run/mysqld/mysqld.sock)") {
		t.Errorf("socket should win: %s", dsn)
	}
	if !strings.Contains(dsn, "u:p%40ss%3Aword@") {
		t.Errorf("credentials should be URL-escaped: %s", dsn)
	}
	if !strings.Contains(dsn, "tls=false") {
		t.Errorf("skip-ssl should map to tls=false: %s", dsn)
	}
}

func TestDefaultsFileContent(t *testing.T) {
	cfg := Default()
	if got := cfg.DefaultsFileContent(); got != "" {
		t.Errorf("empty connection should yield no defaults file, got %q", got)
	}

	cfg.Connection.Username = "backup"
	cfg.Connection.Password = `pa"ss\word`
	cfg.Connection.Socket = "/run/mysqld/mysqld.sock"
	got := cfg.DefaultsFileContent()
	wantSubstr := `password="pa\"ss\\word"`
	if !strings.Contains(got, wantSubstr) {
		t.Errorf("password escaping wrong.\n got: %q\nwant substring: %q", got, wantSubstr)
	}
	if !strings.HasPrefix(got, "[client]\n") {
		t.Errorf("missing [client] header: %q", got)
	}
}

func TestWriteDefaultsFileMode(t *testing.T) {
	cfg := Default()
	cfg.Connection.Username = "backup"
	cfg.Connection.Password = "secret"
	path, cleanup, err := cfg.WriteDefaultsFile()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if path == "" {
		t.Fatal("expected a path")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("defaults file mode = %o, want 600", info.Mode().Perm())
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("cleanup should remove the file")
	}
}
