package config

import (
	"fmt"
	"os"
	"strings"
)

// DefaultsFileContent renders the [client] section of a my.cnf-style options
// file from the connection settings, mirroring the Python create_defaults_file.
// It returns an empty string when no credential information is present (only the
// section header would be written), signalling that no file is needed.
func (c *Config) DefaultsFileContent() string {
	conn := c.Connection
	lines := []string{"[client]"}

	if conn.Username != "" {
		lines = append(lines, "user="+conn.Username)
	}
	if conn.Password != "" {
		// Quotes allow special characters; escape backslash and quote exactly
		// as the Python original did.
		escaped := strings.ReplaceAll(conn.Password, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		lines = append(lines, fmt.Sprintf(`password="%s"`, escaped))
	}
	if conn.Socket != "" {
		lines = append(lines, "socket="+conn.Socket)
	}
	if conn.Host != "" {
		lines = append(lines, "host="+conn.Host)
		port := int(conn.Port)
		if port == 0 {
			port = defaultMySQLPort
		}
		lines = append(lines, fmt.Sprintf("port=%d", port))
	}

	if len(lines) == 1 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// WriteDefaultsFile writes the credentials to a temporary options file with mode
// 0600 (so the password never appears on the mariadb-dump process list) and
// returns its path plus a cleanup function. If no credentials are configured it
// returns an empty path and a no-op cleanup.
func (c *Config) WriteDefaultsFile() (path string, cleanup func() error, err error) {
	content := c.DefaultsFileContent()
	if content == "" {
		return "", func() error { return nil }, nil
	}

	f, err := os.CreateTemp("", "mariadb-backup-*.cnf")
	if err != nil {
		return "", func() error { return nil }, fmt.Errorf("create defaults file: %w", err)
	}
	// CreateTemp already uses 0600, but be explicit.
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", func() error { return nil }, fmt.Errorf("chmod defaults file: %w", err)
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", func() error { return nil }, fmt.Errorf("write defaults file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", func() error { return nil }, fmt.Errorf("close defaults file: %w", err)
	}

	p := f.Name()
	return p, func() error { return os.Remove(p) }, nil
}
