package dump

// dumpBinary is the external dump tool. Held as a single constant so it is
// trivial to change in one place; resolved via $PATH (matches Python).
const dumpBinary = "mariadb-dump"

// optionSet selects the schema-only or data-only option group.
type optionSet int

const (
	schemaOnly optionSet = iota // structure, routines, events; no rows
	dataOnly                    // rows only; no CREATE statements
)

// buildArgs assembles the mariadb-dump argument list for a single database.
//
// The Python `dump_options` built a global option group
// (--quote-names --opt --events) that it never actually applied (a bug). It is
// applied here so events are included in the backup, consistent with --routines
// already being part of both option sets.
func buildArgs(set optionSet, db, defaultsFile string, skipSSL bool, ignoreTables []string) []string {
	var args []string

	// --defaults-extra-file must come first.
	if defaultsFile != "" {
		args = append(args, "--defaults-extra-file="+defaultsFile)
	}

	// Global options (formerly dead code in the Python original).
	args = append(args, "--quote-names", "--opt", "--events")

	args = append(args, "--routines", "--single-transaction")
	switch set {
	case schemaOnly:
		args = append(args, "--no-data")
	case dataOnly:
		args = append(args, "--no-create-info")
	}

	if skipSSL {
		args = append(args, "--skip-ssl")
	}

	// Deliberate fix: apply excludes.tables (the Python read these but never
	// used them). Entries must be fully qualified `db.table`.
	for _, t := range ignoreTables {
		args = append(args, "--ignore-table="+t)
	}

	args = append(args, "--databases", db)
	return args
}

// fileName returns the output file name for the option set and compression.
func fileName(set optionSet, compress bool) string {
	base := "data.sql"
	if set == schemaOnly {
		base = "schema.sql"
	}
	if compress {
		return base + ".gz"
	}
	return base
}

func (s optionSet) label() string {
	if s == schemaOnly {
		return "schema"
	}
	return "data"
}

// redactArgs masks any --password=... argument so it never reaches logs. With
// the defaults-file approach the password is not on the command line, but the
// redactor is kept as a safety net (matches Python _redact).
func redactArgs(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		if len(a) >= len("--password=") && a[:len("--password=")] == "--password=" {
			out[i] = "--password=***"
		} else {
			out[i] = a
		}
	}
	return out
}

func cmdline(name string, args []string) string {
	out := name
	for _, a := range redactArgs(args) {
		out += " " + a
	}
	return out
}
