# mariadb-backup

A small Go tool that backs up all (non-excluded) MariaDB databases by dumping
each one to a `schema.sql` and a `data.sql`, rotating old backups and optionally
emailing the run log.

It is a Go reimplementation of the original Python script (preserved under
[`legacy/`](legacy/)). The actual dump is still produced by the external
`mariadb-dump` binary; everything else (config, connection, rotation,
notification) is pure Go with no other external processes.

## Build

Requires Go 1.22+ and produces a single static binary.

```bash
make build            # -> dist/mariadb-backup (host platform)
make build-all        # -> dist/mariadb-backup-linux-amd64, -linux-arm64
make test             # run the unit tests
```

`mariadb-dump` must be installed on the host at runtime (resolved via `$PATH`).

## Usage

```text
mariadb-backup [flags]

  -d, --directory string   backup directory (overrides storage.destination;
                           defaults to storage.destination or the cwd)
  -C, --config string      configuration file (default "config.yml")
      --compress           gzip-compress the dump files (overrides storage.compression)
      --dry-run            do nothing (no dumps, no rotation, no email)
      --log-level string   DEBUG | INFO | WARNING | ERROR | CRITICAL (default "INFO")
      --version            print version information and exit
  -h, --help               show help
```

The backup directory is resolved in order: `-d/--directory` →
`storage.destination` → current working directory.

## Configuration

See [`config.yml`](config.yml) for the full annotated schema. Top-level blocks:

- `connection` — `username`, `password`, `host`, `port`, `socket`.
- `storage` — `destination`, `compression` (`none`|`gzip`), `rotation.daily`,
  `rotation.weekly`.
- `notification` — `enabled`, `smtp.{server_name,port,tls,auth}`, `sender`,
  `recipient`.
- `excludes` — `databases` (names) and `tables` (fully-qualified `db.table`).

### Credentials

The credentials from the `connection` block are written to a temporary options
file (mode `0600`) passed to `mariadb-dump` via `--defaults-extra-file`, so the
password never appears on the process list. The file is removed automatically
when the run finishes. The database connection used for listing databases is
built from the same configuration values.

### Compression

When `storage.compression: gzip` (or `--compress` is given) each dump is written
streaming-compressed as `schema.sql.gz` / `data.sql.gz`. Default is uncompressed.

## Rotation

The retention behaviour follows the original script, with a few corrections to
quirks of the Python version (see below):

- Backups created on a **Sunday** are renamed to `KW<week>_<timestamp>` and kept
  as weekly backups. Directories that are *already* weekly backups are left
  untouched (the Python version re-prefixed them into `KW20_KW20_…` on every
  run).
- The newest `rotation.weekly` weeks are kept; older weekly backups are removed.
  Weeks are counted per **ISO year + week**, so the same week number in two
  different years (e.g. `KW52` of 2024 and 2025) is never merged into one bucket.
- Non-weekly backups older than `rotation.daily` **calendar days** are removed.
  The age is computed in whole calendar days, so a daylight-saving transition
  cannot shift it by a day.
- At most **5** backups are kept per calendar day (a fixed cap, independent of
  `rotation.daily`).

`rotation.daily`/`rotation.weekly` default to `3`/`2` when the key is omitted; an
explicit `0` is honoured (e.g. `weekly: 0` keeps no weekly backups).

Backup directory names use the local-time format `YYYYMMDD-HHMM`.

## Notification

When `notification.enabled` is true and SMTP server/sender/recipient are set, the
run log is emailed as plain text (STARTTLS and LOGIN/PLAIN auth supported). No
email is sent in `--dry-run`.

## Exit codes

- `0` — success.
- `1` — fatal error (cannot connect, cannot list databases, `mariadb-dump` not
  found).
- `2` — the run completed but at least one database dump failed.

A single database dump failing does **not** abort the whole run; the error is
logged (and emailed) and the next database is attempted.

## Logging

Logs go to three places: the file `/var/log/mariadb-backup.log` (hardcoded), the
console (INFO and above, with colour), and an in-memory buffer used as the email
body. If the log file is not writable (e.g. running as non-root) the file sink is
disabled with a warning and the run continues.

## Restore

Each database directory contains `schema.sql` (structure, routines) and
`data.sql` (rows). To restore, load the schema first, then the data, e.g.:

```bash
mariadb < schema.sql
mariadb < data.sql
# gzip variants:
zcat schema.sql.gz | mariadb
zcat data.sql.gz   | mariadb
```
