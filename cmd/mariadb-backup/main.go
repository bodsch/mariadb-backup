// Command mariadb-backup creates MariaDB backups by dumping each non-excluded
// database to schema.sql and data.sql, rotating old backups and optionally
// emailing the run log. It is a Go reimplementation of the original Python
// script; see the repository README for behaviour notes.
package main

import (
	"fmt"
	"os"
	"strings"

	"bodsch.me/mariadb-backup/internal/app"
	flag "github.com/spf13/pflag"
)

// Build information, stamped via -ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var logLevels = []string{"DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"}

func main() {
	var (
		directory   = flag.StringP("directory", "d", "", "backup directory to store (overrides storage.destination from config; defaults to storage.destination or the current working directory)")
		configFile  = flag.StringP("config", "C", "config.yml", "configuration file")
		dryRun      = flag.Bool("dry-run", false, "do nothing (no dumps, no rotation, no email)")
		logLevel    = flag.String("log-level", "INFO", "log level: "+strings.Join(logLevels, ", "))
		compress    = flag.Bool("compress", false, "gzip-compress the dump files (overrides storage.compression)")
		showVersion = flag.Bool("version", false, "print version information and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("mariadb-backup %s (commit %s, built %s)\n", version, commit, date)
		os.Exit(0)
	}

	if !validLogLevel(*logLevel) {
		fmt.Fprintf(os.Stderr, "invalid --log-level %q (want one of %s)\n", *logLevel, strings.Join(logLevels, ", "))
		flag.Usage()
		os.Exit(2)
	}

	os.Exit(app.Run(app.Options{
		Directory:  *directory,
		ConfigPath: *configFile,
		DryRun:     *dryRun,
		LogLevel:   *logLevel,
		Compress:   *compress,
	}))
}

func validLogLevel(level string) bool {
	level = strings.ToUpper(level)
	for _, l := range logLevels {
		if l == level {
			return true
		}
	}
	return false
}
