// Package app wires the configuration, database, dump, rotation and
// notification packages into the overall backup run, mirroring the Python
// MariaDBBackup.run orchestration.
package app

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"bodsch.me/mariadb-backup/internal/config"
	"bodsch.me/mariadb-backup/internal/dbx"
	"bodsch.me/mariadb-backup/internal/dump"
	"bodsch.me/mariadb-backup/internal/logging"
	"bodsch.me/mariadb-backup/internal/notify"
	"bodsch.me/mariadb-backup/internal/rotate"
)

// Exit codes.
const (
	exitOK            = 0
	exitFatal         = 1 // could not connect / list / dump binary missing
	exitPartialFailed = 2 // run completed but at least one dump failed
)

// now is indirection over time.Now for deterministic tests.
var now = time.Now

// Options carries the parsed CLI flags.
type Options struct {
	Directory  string // -d/--directory
	ConfigPath string // -C/--config
	DryRun     bool
	LogLevel   string
	Compress   bool // --compress (ORs with config)
}

// Dependencies are injectable seams; production code uses the defaults.
type Dependencies struct {
	// OpenDB connects and returns a DatabaseLister.
	OpenDB func(dsn string) (dbx.DatabaseLister, error)
	// NewSender builds a notify.Sender from the SMTP config.
	NewSender func(s config.SMTP) notify.Sender
	// Runner runs mariadb-dump.
	Runner dump.CommandRunner
	// Logger overrides the constructed logger (tests).
	Logger logging.Logger
}

func defaultDeps() Dependencies {
	return Dependencies{
		OpenDB: func(dsn string) (dbx.DatabaseLister, error) { return dbx.Open(dsn) },
		NewSender: func(s config.SMTP) notify.Sender {
			return &notify.SMTPSender{
				Host:     s.ServerName,
				Port:     int(s.Port),
				TLS:      s.TLS,
				Username: s.Auth.Username,
				Password: s.Auth.Password,
			}
		},
		Runner: dump.DefaultRunner(),
	}
}

// Run executes a backup run and returns the process exit code.
func Run(opts Options) int {
	return RunWith(opts, defaultDeps())
}

// RunWith is Run with injectable dependencies (used by tests).
func RunWith(opts Options, deps Dependencies) int {
	d := defaultDeps()
	if deps.OpenDB != nil {
		d.OpenDB = deps.OpenDB
	}
	if deps.NewSender != nil {
		d.NewSender = deps.NewSender
	}
	if deps.Runner != nil {
		d.Runner = deps.Runner
	}

	level, ok := logging.ParseLevel(opts.LogLevel)
	if !ok {
		level = logging.LevelInfo
	}

	var log logging.Logger
	if deps.Logger != nil {
		log = deps.Logger
	} else {
		log = logging.New(logging.Options{Level: level})
	}
	defer log.Close()

	n := now()
	datetimeReadable := n.Format("2006-01-02")
	datetime := n.Format("20060102-1504")

	log.Info("MariaDB Backup at %s - %s ...", notify.FQDN(), datetimeReadable)

	log.Debug("read config file: %s", opts.ConfigPath)
	cfg, err := config.Load(opts.ConfigPath, log)
	if err != nil {
		log.Error("%sconfiguration error: %v%s", logging.ColorFail, err, logging.ColorReset)
		return exitFatal
	}

	// Resolve backup directory: CLI -> storage.destination -> cwd.
	backupDir := opts.Directory
	if backupDir == "" {
		backupDir = cfg.Storage.Destination
	}
	if backupDir == "" {
		if cwd, err := os.Getwd(); err == nil {
			backupDir = cwd
		}
	}
	log.Debug("backup directory: %s", backupDir)

	compress := opts.Compress || cfg.Compress()

	// Connect and list databases (read-only; safe even in dry-run).
	client, err := d.OpenDB(cfg.DSN())
	if err != nil {
		log.Error("%sunable to connect to database: %v%s", logging.ColorFail, err, logging.ColorReset)
		return exitFatal
	}
	defer client.Close()

	log.Debug("list databases ...")
	allDBs, err := client.ListDatabases()
	if err != nil {
		log.Error("%scannot list databases: %v%s", logging.ColorFail, err, logging.ColorReset)
		return exitFatal
	}
	dbNames := reduceDatabases(allDBs, cfg.Excludes.Databases)
	ignoreTables := validIgnoreTables(cfg.Excludes.Tables, log)

	// Credentials file for mariadb-dump (skipped in dry-run, no dump runs).
	defaultsFile := ""
	if !opts.DryRun {
		path, cleanup, err := cfg.WriteDefaultsFile()
		if err != nil {
			log.Error("%s%v%s", logging.ColorFail, err, logging.ColorReset)
			return exitFatal
		}
		defaultsFile = path
		defer cleanup()
		if path != "" {
			log.Debug("wrote temporary defaults file: %s", path)
		}
	}

	outputDir := filepath.Join(backupDir, datetime)
	log.Debug("output directory: %s", outputDir)
	if !opts.DryRun {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			log.Error("%scould not create %s: %v%s", logging.ColorFail, outputDir, err, logging.ColorReset)
			return exitFatal
		}
	}

	dumper := &dump.Dumper{
		DefaultsFile: defaultsFile,
		SkipSSL:      config.SkipSSL,
		Compress:     compress,
		IgnoreTables: ignoreTables,
		DryRun:       opts.DryRun,
		Log:          log,
		Runner:       d.Runner,
	}

	log.Debug("dump databases ...")
	mkdir := func(dir string) error { return os.MkdirAll(dir, 0o755) }
	anyFailed := false
	for _, name := range dbNames {
		res, err := dumper.DumpDatabase(outputDir, name, mkdir)
		if err != nil {
			// Fatal: dump binary not found.
			log.Error("%s%v%s", logging.ColorFail, err, logging.ColorReset)
			return exitFatal
		}
		if res.Failed {
			anyFailed = true
		}
	}

	// Rotation: dry-run uses a FileSystem that only logs intended changes.
	var fs rotate.FileSystem = rotate.OSFileSystem{}
	if opts.DryRun {
		fs = rotate.DryRunFileSystem{Log: log}
	}
	if err := rotate.Rotate(backupDir, cfg.Storage.Rotation.Daily, cfg.Storage.Rotation.Weekly, n, fs, log); err != nil {
		log.Error("%srotation error: %v%s", logging.ColorFail, err, logging.ColorReset)
	}

	// Notification.
	if cfg.Notification.Enabled {
		sendEmail(cfg, datetimeReadable, opts.DryRun, d, log)
	}

	log.Info("done ...")

	if anyFailed {
		return exitPartialFailed
	}
	return exitOK
}

func reduceDatabases(all, excludes []string) []string {
	skip := make(map[string]bool, len(excludes))
	for _, e := range excludes {
		skip[e] = true
	}
	out := make([]string, 0, len(all))
	for _, db := range all {
		if !skip[db] {
			out = append(out, db)
		}
	}
	return out
}

// validIgnoreTables keeps only fully-qualified db.table entries; bare table
// names are reported and skipped (the agreed strict format).
func validIgnoreTables(tables []string, log logging.Logger) []string {
	var out []string
	for _, t := range tables {
		if strings.Contains(t, ".") {
			out = append(out, t)
		} else if t != "" {
			log.Warn("%signoring excludes.tables entry %q: expected db.table format%s",
				logging.ColorWarn, t, logging.ColorReset)
		}
	}
	return out
}

func sendEmail(cfg *config.Config, dateReadable string, dryRun bool, d Dependencies, log logging.Logger) {
	if cfg.Notification.SMTP.ServerName == "" || cfg.Notification.Sender == "" || cfg.Notification.Recipient == "" {
		log.Error("missing smtp server_name, or sender, or recipient.")
		return
	}
	if dryRun {
		log.Info("send no email, we are in dry run ...")
		return
	}

	sender := d.NewSender(cfg.Notification.SMTP)
	msg := notify.Message{
		From:    cfg.Notification.Sender,
		To:      cfg.Notification.Recipient,
		Subject: "Database Backup at " + notify.FQDN() + " - " + dateReadable,
		Body:    logging.StripANSI(log.Buffer()),
	}
	if err := sender.Send(msg); err != nil {
		log.Error("error sending email: %v", err)
		return
	}
	log.Info("email was successfully sent.")
}
