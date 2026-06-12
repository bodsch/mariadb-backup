package config

import (
	"fmt"
	"net/url"
)

// SkipSSL mirrors the Python `db_skip_ssl` flag, which was hardcoded to true.
// It is exposed so the dump layer and the DSN stay consistent.
const SkipSSL = true

// DSN builds a go-sql-driver/mysql DSN from the connection settings. It mirrors
// the credential resolution of the Python _mysql_connect, minus the removed
// read_default_file fallback: credentials come solely from the config.
//
// A configured socket takes precedence over host/port (matching the driver's
// own precedence and the typical local-backup setup).
func (c *Config) DSN() string {
	conn := c.Connection
	auth := ""
	if conn.Username != "" {
		auth = url.QueryEscape(conn.Username)
		if conn.Password != "" {
			auth += ":" + url.QueryEscape(conn.Password)
		}
		auth += "@"
	}

	var netPart string
	if conn.Socket != "" {
		netPart = fmt.Sprintf("unix(%s)", conn.Socket)
	} else {
		host := conn.Host
		if host == "" {
			host = "localhost"
		}
		port := int(conn.Port)
		if port == 0 {
			port = defaultMySQLPort
		}
		netPart = fmt.Sprintf("tcp(%s:%d)", host, port)
	}

	// No database is selected; we only run SHOW DATABASES.
	dsn := fmt.Sprintf("%s%s/", auth, netPart)

	// --skip-ssl equivalent for the driver.
	if SkipSSL {
		dsn += "?tls=false"
	}
	return dsn
}
