// Package dbx wraps the pure-Go MySQL driver for the read-only queries the
// backup needs (currently just listing databases). The DatabaseLister interface
// keeps the rest of the application testable with a mock.
package dbx

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

// DatabaseLister lists the databases available on the server.
type DatabaseLister interface {
	ListDatabases() ([]string, error)
	Close() error
}

// SQLClient is a DatabaseLister backed by go-sql-driver/mysql.
type SQLClient struct {
	db *sql.DB
}

// Open connects using the given DSN. The connection is lazy; ListDatabases (or
// Ping) triggers the actual handshake.
func Open(dsn string) (*SQLClient, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return &SQLClient{db: db}, nil
}

// ListDatabases runs SHOW DATABASES and returns the database names.
func (c *SQLClient) ListDatabases() ([]string, error) {
	rows, err := c.db.Query("SHOW DATABASES")
	if err != nil {
		return nil, fmt.Errorf("show databases: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan database name: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate databases: %w", err)
	}
	return names, nil
}

// Close releases the underlying connection pool.
func (c *SQLClient) Close() error {
	return c.db.Close()
}
