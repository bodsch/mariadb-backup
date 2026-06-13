# mariadb backup

## config

```yaml
connection:
  encrypted_login: false
  username: 'backup'
  password: 'backUp'
  host: ''
  port: ''
  socket: /run/mysqld/mysqld.sock
  commpress_communication: true
  login_cnf_file: ''
  use_ssl: false
  max_allowed_packet: ''
  single_transaction: true

storage:
  # target directory for the backups.
  # overridden by the -d/--directory command line parameter.
  # defaults to the current working directory if unset.
  destination: /var/backup
  rotation:
    # Set rotation of daily backups. VALUE*24hours
    # If you want to keep only today's backups, you could choose 1,
    # i.e. everything older than 24hours will be removed.
    daily: 2
    weekly: 2

notification:
  enabled: false
  smtp:
    server_name: ""             # smtp.example.com
    port: ""                    # 587
    tls: false
    auth:
      username: ""
      password: ""
  sender: ""                    # backup@example.com
  recipient: ""                 # admin@foo.bar

excludes:
  databases:
    - performance_schema
    - information_schema
    - sys
  tables: []

includes:
  databases: []
  tables: []
```

### credentials

The credentials from the `connection` block are written to a temporary
option file (mode `0600`) which is shared by both the `mariadb-dump`
subprocess (`--defaults-extra-file`) and the Python driver
(`read_default_file`). This keeps the password out of the process list.
The file is removed automatically when the script exits.

An explicit `--mycnf` file is used as a fallback when no credentials are
configured.

## backup directory

The target directory is resolved in the following order:

1. `-d` / `--directory` command line parameter
2. `storage.destination` from the config
3. the current working directory

## rotation

* daily backups older than `rotation.daily` days are removed
* backups created on a Sunday are renamed to `KW<week>_<timestamp>` and kept
  as weekly backups; the newest `rotation.weekly` weekly backups are kept

## usage

```bash
mariadb_backup.py --help
usage: mariadb_backup.py [-h] [-d DIRECTORY] [-C CONFIG] [--mycnf MYCNF]
                         [--dry-run]
                         [--log-level {DEBUG,INFO,WARNING,ERROR,CRITICAL}]

create mariadb backups

options:
  -h, --help            show this help message and exit
  -d, --directory DIRECTORY
                        backup directory to store (overrides
                        storage.destination from config; defaults to
                        storage.destination or the current working directory)
  -C, --config CONFIG   configuration file
  --mycnf MYCNF         my.cnf file
  --dry-run             do nothing
  --log-level {DEBUG,INFO,WARNING,ERROR,CRITICAL}
                        Setzt das Log-Level (default: INFO)
```
