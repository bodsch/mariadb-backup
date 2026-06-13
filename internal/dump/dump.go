// Package dump runs mariadb-dump as a subprocess to produce a schema.sql and a
// data.sql per database. Each database is dumped by a self-contained Dumper
// call, which keeps the unit ready for a future worker-pool without
// restructuring. Output can optionally be gzip-compressed.
package dump

import (
	"os"
	"path/filepath"
	"strings"

	"bodsch.me/mariadb-backup/internal/logging"
)

// Dumper dumps individual databases via mariadb-dump.
type Dumper struct {
	DefaultsFile string   // path passed as --defaults-extra-file (may be empty)
	SkipSSL      bool     // append --skip-ssl
	Compress     bool     // gzip the output files
	IgnoreTables []string // fully-qualified db.table entries (excludes.tables)
	DryRun       bool     // log intended actions, change nothing
	Log          logging.Logger
	Runner       CommandRunner
}

// Result reports the outcome of dumping a single database.
type Result struct {
	DB string
	// Failed is true if schema or data dump exited with a code > 1 (the
	// threshold the Python script treated as a real failure).
	Failed bool
}

// DumpDatabase dumps schema and data for one database into
// <outputDir>/<db>/. A returned error is fatal for the whole run (the dump
// binary could not be started). Per-database dump failures (exit code > 1) are
// logged, recorded in Result.Failed and do NOT return an error, so the caller
// continues with the next database.
func (d *Dumper) DumpDatabase(outputDir, db string, mkdir func(string) error) (Result, error) {
	dir := filepath.Join(outputDir, db)
	res := Result{DB: db}

	d.Log.Info("backup database %s%s%s", logging.ColorDebug, db, logging.ColorReset)

	if d.DryRun {
		d.Log.Info("   would create %s", dir)
		d.Log.Info("   would dump schema -> %s", filepath.Join(dir, fileName(schemaOnly, d.Compress)))
		d.Log.Info("   would dump data   -> %s", filepath.Join(dir, fileName(dataOnly, d.Compress)))
		return res, nil
	}

	if err := mkdir(dir); err != nil {
		// Treat a directory creation failure like a per-DB failure: log and
		// continue, do not abort the whole run.
		d.Log.Error("   %scould not create %s: %v%s", logging.ColorFail, dir, err, logging.ColorReset)
		res.Failed = true
		return res, nil
	}

	for _, set := range []optionSet{schemaOnly, dataOnly} {
		d.Log.Debug("   %s%s%s", logging.ColorDebug, set.label(), logging.ColorReset)
		failed, err := d.dumpOne(dir, db, set)
		if err != nil {
			return res, err // fatal (binary not found)
		}
		if failed {
			res.Failed = true
		}
	}
	return res, nil
}

func (d *Dumper) dumpOne(dir, db string, set optionSet) (failed bool, fatal error) {
	args := buildArgs(set, db, d.DefaultsFile, d.SkipSSL, d.IgnoreTables)

	w, name, err := newWriter(dir, set, d.Compress)
	if err != nil {
		d.Log.Error("   %s%v%s", logging.ColorFail, err, logging.ColorReset)
		return true, nil
	}

	rc, stderr, runErr := d.Runner.Run(dumpBinary, args, w)
	closeErr := w.Close()

	if runErr != nil {
		// newWriter created (and Close just flushed) the output file before the
		// dump binary failed to start. Remove the empty/partial file so it is
		// not mistaken for a valid (empty) backup.
		if rmErr := os.Remove(filepath.Join(dir, name)); rmErr != nil && !os.IsNotExist(rmErr) {
			d.Log.Error("   %scould not remove incomplete dump %s: %v%s",
				logging.ColorFail, name, rmErr, logging.ColorReset)
		}
		d.Log.Error("Failed to find %s binary", dumpBinary)
		return false, runErr
	}
	if closeErr != nil {
		d.Log.Error("   %swriting %s failed: %v%s", logging.ColorFail, name, closeErr, logging.ColorReset)
		return true, nil
	}

	// Matches Python: only exit codes > 1 are treated as failures; rc == 1 is a
	// tolerated warning.
	if rc > 1 {
		line := cmdline(dumpBinary, args)
		d.Log.Error("   %s%s failed. Code %d. Output follows below.%s",
			logging.ColorFail, line, rc, logging.ColorReset)
		for _, se := range strings.Split(strings.TrimRight(stderr, "\n"), "\n") {
			if se == "" {
				continue
			}
			d.Log.Error("   %s%s%s", logging.ColorFail, se, logging.ColorReset)
		}
		d.Log.Error("   dump file: %s", name)
		d.Log.Error("   cmd line : %s", line)
		return true, nil
	}
	return false, nil
}
