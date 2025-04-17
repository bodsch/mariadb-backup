# mariadb backup

python-ansi2html

## config

```ỳaml
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

rotation:
  # Set rotation of daily backups. VALUE*24hours
  # If you want to keep only today's backups, you could choose 1,
  # i.e. everything older than 24hours will be removed.
  daily: 2
  weekly: 2

notification:
  enabled: true
  smtp:
    server_name: ""             # smtp.example.com
    port: ""                    # 587
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

## usage

```bash
mariadb_backup.py --help
usage: mariadb_backup.py [-h] [-d DIRECTORY] [-C CONFIG] [--mycnf MYCNF] [--dry-run DRY_RUN] [--log-level {DEBUG,INFO,WARNING,ERROR,CRITICAL}]

create mariadb backups

options:
  -h, --help            show this help message and exit
  -d, --directory DIRECTORY
                        backup directory to store
  -C, --config CONFIG   configuration file
  --mycnf MYCNF         my.cnf file
  --dry-run DRY_RUN     do nothing
  --log-level {DEBUG,INFO,WARNING,ERROR,CRITICAL}
                        Setzt das Log-Level (default: INFO)
```
