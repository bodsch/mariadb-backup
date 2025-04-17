#!/usr/bin/python3

import os
import sys
import time
import yaml
import logging
import argparse
import datetime
import shutil
import socket
from pathlib import Path
from subprocess import Popen, PIPE, list2cmdline

try:
    import MySQLdb as mysql_driver
    _mysql_cursor_param = 'cursorclass'
except ImportError:
    mysql_driver = None


class bcolors:
    HEADER = '\033[95m'
    OKBLUE = '\033[94m'
    OKCYAN = '\033[96m'
    OKGREEN = '\033[92m'
    DEBUG = '\033[1m\033[92m'
    WARNING = '\033[93m'
    FAIL = '\033[91m'
    ENDC = '\033[0m'
    BOLD = '\033[1m'
    UNDERLINE = '\033[4m'


class MemoryLogHandler(logging.Handler):
    """
        Speichert Log-Meldungen in einer internen Liste
    """

    def __init__(self):
        super().__init__()
        self.log_messages = []

    def emit(self, record):
        """Fügt eine formatierte Log-Nachricht der Liste hinzu"""
        if record.levelno >= logging.INFO:  # Speichert nur INFO und höher (kein DEBUG)
            # log_entry = self.format(record)
            log_entry = record.getMessage()  # Holt nur die reine Log-Nachricht
            self.log_messages.append(log_entry)

    def get_logs(self):
        """Gibt alle Logs als String zurück"""
        return "\n".join(self.log_messages)


class SMTPManager:
    """
    """

    def __init__(self, logging, subject, sender={}, recipient={}, smtp={}, body=None):
        """
        """
        self.logging = logging

        self.subject = subject
        self.body = body
        self.sender = sender
        self.recipient = recipient
        self.smtp = smtp

        self.init_smtp()

    def init_smtp(self):

        self.sender_email = self.sender.get("email", None)
        self.recipient_email = self.recipient.get("email", None)

        self.smtp_server = self.smtp.get("server_name", None)
        self.smtp_port = self.smtp.get("port", None)
        self.smtp_tls = self.smtp.get("tls", True)
        self.smtp_auth_username = self.smtp.get("auth", {}).get("username", None)
        self.smtp_auth_password = self.smtp.get("auth", {}).get("password", None)

        logging.debug("--------------------------------------------------")
        logging.debug(f" subject      : {self.subject}")
        logging.debug(f" sender       : {self.sender}")
        logging.debug(f" recipient    : {self.recipient}")
        logging.debug(f" smtp         : {self.smtp}")
        logging.debug("--------------------------------------------------")

        logging.debug("--------------------------------------------------")
        logging.debug("sending email")
        logging.debug(f"  - from       : {self.sender_email}")
        logging.debug(f"  - to         : {self.recipient_email}")
        logging.debug(f"  - subject    : {self.subject}")
        logging.debug("  - body       :")

        for line in self.body.splitlines():
            logging.debug(f"     {line}")

        logging.debug(f"  - smtp server: {self.smtp_server}:{self.smtp_port} {self.smtp_tls}")
        logging.debug("--------------------------------------------------")

    def send_email(self):
        """
            Sendet die gespeicherten Logs per E-Mail.
        """
        import smtplib
        from email.mime.multipart import MIMEMultipart
        from email.mime.text import MIMEText
        from ansi2html import Ansi2HTMLConverter

        # konvertiere Escpae Sequencen zu html tags
        conv = Ansi2HTMLConverter(inline=True)
        html_text = conv.convert(self.body, full=False)
        html_text = f"<pre style='font-family: monospace;'>{html_text}</pre>"

        Multipart-Mail vorbereiten (plain + html)
        msg = MIMEMultipart("alternative")

        part1 = MIMEText(self.remove_ansi_escape_sequences(self.body), "plain")
        part2 = MIMEText(html_text, "html")

        # email_body = self.log_memory_handler.get_logs()
        # subject = f"renew TLS certificates at {socket.getfqdn()} - {self.datetime_readable}"
        #
        # logging.debug("sending email")
        # logging.debug(f"  - from   : {self.notification_sender}")
        # logging.debug(f"  - to     : {self.notification_recipient}")
        # logging.debug(f"  - subject: {subject}")
        # logging.debug("  - body   :")
        # for line in email_body.splitlines():
        #     logging.debug(f"     {bcolors.FAIL}{line}{bcolors.ENDC}")

        if self.smtp_server and self.sender_email and self.recipient_email:
            """
            """
            # msg = MIMEText(self.remove_ansi_escape_sequences(self.body))

            msg.attach(part1)
            msg.attach(part2)

            msg["Subject"] = self.subject
            msg["From"] = self.sender_email
            msg["To"] = self.recipient_email

            if self.smtp_tls:
                # Create a secure SSL context
                context = ssl.create_default_context()

            try:
                server = smtplib.SMTP(host=self.smtp_server, port=int(self.smtp_port))
                """
                """
                logging.debug("smtp connected")

                # server.set_debuglevel(2)
                server.ehlo(name="boone-schulz.de")

                if self.smtp_tls:
                    logging.debug("smtp starttls")
                    server.starttls(context=context)
                    server.ehlo(name="boone-schulz.de")

                if self.smtp_auth_username and self.smtp_auth_password:
                    logging.debug("smtp auth")
                    try:
                        server.esmtp_features['auth'] = 'LOGIN PLAIN'
                        server.login(self.smtp_auth_username, self.smtp_auth_password)
                    except smtplib.SMTPHeloError as e:
                        logging.error(f"smtplib.SMTPHeloError: {e}")
                    except smtplib.SMTPAuthenticationError as e:
                        logging.error(f"smtplib.SMTPAuthenticationError: {e}")
                    except smtplib.SMTPNotSupportedError as e:
                        logging.error(f"smtplib.SMTPNotSupportedError: {e}")
                    except smtplib.SMTPException as e:
                        logging.error(f"smtplib.SMTPException: {e}")

                logging.debug("smtp sendmail")
                server.sendmail(
                    from_addr=self.sender_email,
                    to_addrs=self.recipient_email,
                    msg=msg.as_string()
                )
                server.quit()

                logging.info("email was successfully sent.")

            except smtplib.SMTPServerDisconnected:
                logging.error("smtplib.SMTPServerDisconnected")
            except smtplib.SMTPResponseException as e:
                logging.error("smtplib.SMTPResponseException:")
                logging.error(f"   {str(e.smtp_code)}  {str(e.smtp_error)}")
            except smtplib.SMTPSenderRefused:
                logging.error("smtplib.SMTPSenderRefused")
            except smtplib.SMTPRecipientsRefused:
                logging.error("smtplib.SMTPRecipientsRefused")
            except smtplib.SMTPDataError:
                logging.error("smtplib.SMTPDataError")
            except smtplib.SMTPConnectError:
                logging.error("smtplib.SMTPConnectError")
            except smtplib.SMTPHeloError:
                logging.error("smtplib.SMTPHeloError")
            except smtplib.SMTPAuthenticationError:
                logging.error("smtplib.SMTPAuthenticationError")

            except socket.error as e:
                logging.error("could not connect:")
                logging.error(f"  {e}")
            except Exception as e:
                logging.error("Fehler beim Senden der E-Mail:")
                logging.error(f"  {e}")
        else:
            logging.error("missing smtp server_nemr, or sender, or recipient.")

    def remove_ansi_escape_sequences(self, text):
        import re
        ansi_escape = re.compile(r'\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])')
        return ansi_escape.sub('', text)


class MariaDBBackup():

    def __init__(self):
        """
        """
        self.args = {}
        self.parse_args()

        self.backup_directory = self.args.directory
        self.config_file = self.args.config
        self.mycnf_file = self.args.mycnf
        self.dry_run = self.args.dry_run
        self.log_level = self.args.log_level

        self.log_memory_handler = MemoryLogHandler()
        self.setup_logging()

        self.datetime = time.strftime('%Y%m%d-%H%M')
        self.datetime_readable = time.strftime("%Y-%m-%d")

    def parse_args(self):
        """
            parse arguments
        """
        p = argparse.ArgumentParser(description='create mariadb backups')

        p.add_argument(
            "-d",
            "--directory",
            required=False,
            help="backup directory to store",
            default=os.getcwd()
        )
        p.add_argument(
            "-C",
            "--config",
            required=False,
            help="configuration file",
            default="config.yml"
        )
        p.add_argument(
            "--mycnf",
            required=False,
            help="my.cnf file",
            default=""
        )
        p.add_argument(
            "--dry-run",
            required=False,
            help="do nothing",
            default=False
        )
        p.add_argument(
            "--log-level",
            type=str,
            default="INFO",
            choices=["DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"],
            help="Setzt das Log-Level (default: INFO)"
        )

        self.args = p.parse_args()

    def setup_logging(self):
        """
            Konfiguriert das Logging mit dem gegebenen Log-Level.
        """
        log_level_numeric = getattr(
            logging, self.log_level)  # Umwandlung von Text in Level
        # DEBUG-Format (kurzer Zeitstempel)
        debug_formatter = logging.Formatter(
            "%(asctime)s - %(levelname)s - %(message)s", "%H:%M:%S")

        # Standard-Format für INFO+ (langer Zeitstempel)
        standard_formatter = logging.Formatter(
            "%(asctime)s - %(levelname)s - %(message)s", "%Y-%m-%d %H:%M:%S")

        # Datei-Logging (speichert ALLE Logs mit passendem Format)
        file_handler = logging.FileHandler("/var/log/mariadb-backup.log")
        file_handler.setLevel(log_level_numeric)
        file_handler.setFormatter(
            debug_formatter if log_level_numeric == logging.DEBUG else standard_formatter)

        # Konsolen-Logging (INFO und höher)
        console_handler = logging.StreamHandler(sys.stdout)
        console_handler.setLevel(logging.INFO)
        console_handler.setFormatter(standard_formatter)

        # Memory-Logging (für E-Mail, speichert NUR die reine Nachricht)
        self.log_memory_handler.setLevel(logging.INFO)

        # Logger abrufen und Handler hinzufügen
        logger = logging.getLogger()
        logger.setLevel(log_level_numeric)
        logger.addHandler(file_handler)
        logger.addHandler(console_handler)
        logger.addHandler(self.log_memory_handler)

    def run(self):
        """
        """
        logging.info(f"MariaDB Backup at {socket.getfqdn()} - {self.datetime_readable} ...")
        if os.path.exists(self.config_file):
            logging.debug(f"read config file: {self.config_file}")
            self.read_configuration(os.path.join(self.config_file))
            print("")

        """
            create output directory
        """
        self.output_directory = os.path.join(
            self.backup_directory, self.datetime)
        try:
            os.makedirs(self.output_directory, exist_ok=True)
        except FileExistsError:
            pass

        logging.debug(f"output directory: {self.output_directory}")
        logging.debug("list databases ...")
        # db_status = self.check_database()
        all_db_names = self.list_databases()

        db_names = self.reduce_databases(all_databases=all_db_names)

        logging.debug("dump databases ...")
        logging.debug(f"  {db_names}")

        # os.chdir(output_directory)
        self.dump_databases(db_names)

        self.rotate_directories(self.backup_directory)

        if self.notification_enabled:
            self.send_log_email()

        logging.info("done ...\n")

    def read_configuration(self, filename):
        """
        """
        self.notification_enabled = False
        self.notification_smtp_host = None
        self.notification_sender = None
        self.notification_recipient = None

        self.db_skip_ssl = True

        with open(filename) as file:
            content = yaml.load(file, Loader=yaml.FullLoader)

            if content:
                connection = content.get("connection", {})
                rotation = content.get("rotation", {})
                notification = content.get("notification", {})
                excludes = content.get("excludes", {})

                if connection:
                    self.db_username = connection.get("username", None)
                    self.db_password = connection.get("password", None)
                    self.db_host = connection.get("host", None)
                    self.db_port = connection.get("port", None)
                    self.db_socket = connection.get("socket", None)

                if rotation:
                    self.rotation_daily = rotation.get("daily", 3)
                    self.rotation_weekly = rotation.get("weekly", 2)

                if notification:
                    self.notification_enabled = notification.get(
                        "enabled", False)
                    self.notification_smtp_host = notification.get(
                        "smtp", {}).get("server_name", None)
                    self.notification_smtp_port = notification.get(
                        "smtp", {}).get("port", 587)
                    self.notification_smtp_tls = notification.get(
                        "smtp", {}).get("tls", True)
                    smtp_auth = notification.get("smtp", {}).get("auth", {})
                    self.notification_smtp_username = smtp_auth.get("username", None)
                    self.notification_smtp_password = smtp_auth.get("password", None)

                    self.notification_sender = notification.get("sender", None)
                    self.notification_recipient = notification.get(
                        "recipient", None)

                if excludes:
                    self.excludes_databases = excludes.get("databases", [])
                    self.excludes_tables = excludes.get("tables", [])

    def dump_options(self):
        """
        """
        opt = [
            '--quote-names',
            '--opt',
            '--events']

        opt_only_schema = [
            '--routines',
            '--single-transaction',
            '--no-data']

        opt_only_data = [
            '--routines',
            '--single-transaction',
            '--no-create-info']

        if len(self.db_username) != 0:
            opt.append('--user')
            opt.append(self.db_username)
            opt_only_schema.append('--user')
            opt_only_schema.append(self.db_username)
            opt_only_data.append('--user')
            opt_only_data.append(self.db_username)

        if len(self.db_password) != 0:
            opt.append(f'--password={self.db_password}')
            opt_only_schema.append(f'--password={self.db_password}')
            opt_only_data.append(f'--password={self.db_password}')

        if self.db_socket and len(self.db_socket) != 0:
            opt.append('--socket')
            opt.append(self.db_socket)
            opt_only_schema.append('--socket')
            opt_only_schema.append(self.db_socket)
            opt_only_data.append('--socket')
            opt_only_data.append(self.db_socket)

        if self.db_host and len(self.db_host) != 0:
            opt.append('--host')
            opt.append(self.db_host)
            opt_only_schema.append('--host')
            opt_only_schema.append(self.db_host)
            opt_only_data.append('--host')
            opt_only_data.append(self.db_host)
            if self.db_port is not None:
                opt.append('--port')
                opt.append(str(self.db_port))
                opt_only_schema.append('--port')
                opt_only_schema.append(str(self.db_port))
                opt_only_data.append('--port')
                opt_only_data.append(str(self.db_port))
            else:
                opt.append('--port')
                opt.append(str(3306))
                opt_only_schema.append('--port')
                opt_only_schema.append(str(3306))
                opt_only_data.append('--port')
                opt_only_data.append(str(3306))

        if self.db_skip_ssl:
            opt.append('--skip-ssl')
            opt_only_schema.append('--skip-ssl')
            opt_only_data.append('--skip-ssl')

        return (opt, opt_only_schema, opt_only_data)

    def list_databases(self):
        """ """
        all_db_names = []
        rows = None

        cursor, conn, error, message = self._mysql_connect()

        query = "show databases"

        try:
            cursor.execute(query)
            rows = cursor.fetchall()

        except Exception as e:
            logging.error(
                f"   {bcolors.FAIL}Cannot execute SQL {query}: {e}{bcolors.ENDC}")
            os._exit(1)

        if rows:
            for index in range(len(rows)):
                all_db_names.append(rows[index].get("Database", None))

        # logging.debug(f"   {bcolors.DEBUG}{all_db_names}{bcolors.ENDC}")
        return all_db_names

    def reduce_databases(self, all_databases=[]):
        """
        """
        dbs = [x for x in all_databases if x not in self.excludes_databases]

        return dbs

    def dump_databases(self, databases):
        """ """
        """
          -n, --no-create-db  Suppress the CREATE DATABASE ... IF EXISTS statement that
                              normally is output for each dumped database if
                              --all-databases or --databases is given.
          -t, --no-create-info
                              Don't write table creation info.
          -d, --no-data       No row information.

                --databases test
        """
        _output_directory = self.output_directory
        args = ["mariadb-dump"]

        (opt, opt_only_schema, opt_only_data) = self.dump_options()

        if len(databases):
            """ """
            for dba in databases:
                logging.info(
                    f"backup database {bcolors.DEBUG}{dba}{bcolors.ENDC}")

                _directory = os.path.join(self.output_directory, dba)
                try:
                    os.makedirs(_directory, exist_ok=True)
                    os.chdir(_directory)
                except FileExistsError:
                    pass

                _args = list(args)
                _args += list(opt_only_schema)
                _args.append('--databases')
                _args.append(dba)

                sql_file_name = "schema.sql"

                logging.debug(f"   {bcolors.DEBUG}schema{bcolors.ENDC}")

                self._dump(sql_file_name, _args)

                _args = None

                _args = list(args)
                _args += list(opt_only_data)
                _args.append('--databases')
                _args.append(dba)

                sql_file_name = "data.sql"

                logging.debug(f"   {bcolors.DEBUG}data{bcolors.ENDC}")

                self._dump(sql_file_name, _args)

                _args = None

                os.chdir(_output_directory)

                # create zip archive ...
                # shutil.make_archive(_directory, 'zip', _directory)

    def _dump(self, dump_file, args):

        cmdline = list2cmdline(args)

        stdout = open(dump_file, "w", 1)      # line-buffered

        try:
            process = Popen(args, stdout=stdout, stderr=PIPE, close_fds=True)
            _, stderr = process.communicate()
            rc = process.returncode

            if rc and rc > 1:
                logging.error(
                    f"   {bcolors.FAIL}{cmdline} failed. Code {rc}. Output follows below.{bcolors.ENDC}")

                _stderr = f"{stderr.rstrip()}"
                _stderr_lines = _stderr.split("\n")

                for _se in _stderr_lines:
                    logging.error(f"   {bcolors.FAIL}{_se}{bcolors.ENDC}")
                logging.error(f"   dump file: {dump_file}")
                logging.error(f"   cmd line : {cmdline}")

                for line in process.stdout:
                    sys.stdout.write(line)

            process.wait()
            stdout.close()

            # ret_code = process.wait()
        except OSError:
            logging.error("Failed to find mariadb-dump binary")
            sys.exit(1)

    def _mysql_connect(self):
        """
        """
        config = {}

        config_file = self.mycnf_file

        if config_file and os.path.exists(config_file):
            config['read_default_file'] = config_file

        # If dba_user or dba_password are given, they should override the
        # config file
        if self.db_username is not None:
            config['user'] = self.db_username
        if self.db_password is not None:
            config['passwd'] = self.db_password
        if self.db_host is not None and len(str(self.db_host)) > 0:
            config['host'] = self.db_host
            if self.db_port is not None and len(str(self.db_port)) > 0:
                config['port'] = self.db_port
            else:
                config['port'] = 3306

        if self.db_socket is not None:
            config['unix_socket'] = self.db_socket

        logging.debug(f"{bcolors.DEBUG}config : {config}{bcolors.ENDC}")

        if mysql_driver is None:
            logging.error(f"   {bcolors.FAIL}missing SQL driver{bcolors.ENDC}")
            sys.exit(1)

        try:
            db_connection = mysql_driver.connect(**config)
            cursor = db_connection.cursor(mysql_driver.cursors.DictCursor)

        except Exception as e:
            message = "unable to connect to database.\n"
            message += f"  Exception message: {e}\n"
            message += "    check 'login_host', 'login_user' and 'login_password' are correct \n"
            message += f"    or {config_file} has the credentials. "

            logging.error(f"  {bcolors.FAIL}{message}{bcolors.ENDC}")

            os._exit(1)
            return (None, None, True, message)
        # finally:
        #     #if db_connection.is_connected():
        #     #cursor.close()
        #     #db_connection.close()
        #     print("MySQL connection is closed")

        return (
            cursor,
            db_connection,
            False,
            "successful connected"
        )

    def get_creation_date(self, name):
        """
            Extrahiert das Erstellungsdatum aus dem Verzeichnisnamen im Format '%Y%m%d-%H%M'.
        """
        try:
            # Nur das Datum extrahieren
            return datetime.datetime.strptime(name[:11], "%Y%m%d-%H%M").date()
        except ValueError:
            return None

    def rotate_directories(self, base_dir):
        """
        """
        today = datetime.date.today()
        weekly_backups = {}  # Wöchentliche Backups sammeln
        daily_backups = {}   # Tägliche Backups gruppieren

        for entry in sorted(Path(base_dir).iterdir(), key=lambda e: e.name):
            if entry.is_dir():
                creation_date = self.get_creation_date(entry.name)

                if creation_date:
                    age_days = (today - creation_date).days
                    # week_number = creation_date.isocalendar()[1]
                    week_number = f"{creation_date.isocalendar()[1]:02d}"

                    # **1. Maximal 5 Backups pro Tag behalten**
                    daily_backups.setdefault(creation_date, []).append(entry)

                    # **2. Umbenennen, falls Sonntag (wöchentliches Backup)**
                    if creation_date.weekday() == 6:
                        new_name = f"KW{week_number}_{entry.name}"
                        new_path = entry.parent / new_name
                        if not new_path.exists():
                            entry.rename(new_path)
                            logging.info(
                                f"rename backup {entry.name} to {new_name}")

                    # **3. Löschen, wenn älter als rotation_daily Tage (außer Wochen-Backups)**
                    if age_days > self.rotation_daily and not entry.name.startswith("KW"):
                        self.delete_directory(entry)
                        logging.info(f"remove older backup {entry.name}.")

                    # **4. Sammeln der wöchentlichen Backups**
                    if entry.name.startswith("KW"):
                        weekly_backups.setdefault(
                            week_number, []).append(entry)

        # **5. Überzählige Backups pro Tag löschen (älteste zuerst)**
        for date, backups in daily_backups.items():
            if len(backups) > 5:
                # Sortieren nach Name (älteste zuerst löschen)
                backups_to_delete = sorted(backups, key=lambda e: e.name)[:-5]
                for old_backup in backups_to_delete:
                    self.delete_directory(old_backup)
                    logging.info(
                        f"delete excess daily backup {old_backup.name}")

        # **6. Maximal rotation_weekly wöchentliche Backups behalten**
        sorted_weeks = sorted(weekly_backups.keys(),
                              reverse=True)  # Neueste zuerst
        # Behalte die neuesten
        weeks_to_delete = sorted_weeks[self.rotation_weekly:]

        for week in weeks_to_delete:
            for entry in weekly_backups[week]:
                self.delete_directory(entry)
                logging.info(f"delete weekly backup {entry.name}")

    def delete_directory(self, directory):
        """
            Löscht ein Verzeichnis mit allen enthaltenen Dateien.
        """
        if os.path.exists(directory):
            shutil.rmtree(directory)

    def send_log_email(self):
        """
            Sendet die gespeicherten Logs per E-Mail.
        """
        if self.notification_smtp_host and self.notification_sender and self.notification_recipient:
            pass
        else:
            logging.error("missing smtp server_nemr, or sender, or recipient.")
            return

        smtp = SMTPManager(
            logging=logging,
            subject=f"Database Backup at {socket.getfqdn()} - {self.datetime_readable}",
            sender=dict(
                email=self.notification_sender
            ),
            recipient=dict(
                email=self.notification_recipient
            ),
            smtp=dict(
                server_name=self.notification_smtp_host,
                port=self.notification_smtp_port,
                tls=self.notification_smtp_tls,
                auth=dict(
                    username=self.notification_smtp_username,
                    password=self.notification_smtp_password
                )
            ),
            body=self.log_memory_handler.get_logs()
        )

        if self.dry_run:
            logging.info("send not email, we are in dry run ...")
            return

        smtp.send_email()


if __name__ == '__main__':
    """
    """
    r = MariaDBBackup()

    r.run()
