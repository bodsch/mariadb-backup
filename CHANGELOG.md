# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Complete rewrite of the tool from Python to Go. The core behaviour is preserved;
a few long-standing bugs were fixed deliberately (see Changed/Fixed).

### Added

- Optional gzip compression of dumps via `storage.compression: gzip` and the
  `--compress` flag (output as `schema.sql.gz` / `data.sql.gz`).
- `--version` flag printing version, commit and build date.
- Distinct exit code `2` when a run completes but at least one database dump
  failed (better for cron/monitoring); fatal errors still exit `1`.
- Startup deprecation warning listing any obsolete/unknown config keys.
- Graceful degradation when `/var/log/mariadb-backup.log` is not writable
  (warn and continue instead of crashing).

### Changed

- Reimplemented in Go as a single static binary (`bodsch.me/mariadb-backup`),
  targeting linux/amd64 and linux/arm64. The database connection now uses the
  pure-Go `go-sql-driver/mysql`; `mariadb-dump` is still invoked for the dump.
- `--dry-run` is now fully side-effect-free: no dumps, directory creation,
  rotation, renames, deletions or email (previously it only suppressed email).
- Notification emails are now plain text only (the HTML/ANSI-to-HTML variant was
  removed).
- Configuration schema cleaned up: unused/typo'd keys removed (kept readable via
  the deprecation warning).

### Fixed

- `excludes.tables` is now actually applied (via `mariadb-dump --ignore-table`);
  previously the setting was read but ignored. Entries must be fully-qualified
  `db.table`.
- Events are now included in dumps. The Python original built the global option
  group `--quote-names --opt --events` but never passed it to `mariadb-dump`, so
  events were silently omitted; the options are now applied.

### Removed

- `--mycnf` flag and the `read_default_file` credential fallback; credentials now
  come solely from the configuration.
- Legacy `connection` keys (`encrypted_login`, `commpress_communication`,
  `login_cnf_file`, `use_ssl`, `max_allowed_packet`, `single_transaction`) and
  the unused `includes` block.

[Unreleased]: https://bodsch.me/mariadb-backup
