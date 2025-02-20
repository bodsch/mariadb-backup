#!/usr/bin/python3

import os
import sys
import time
import yaml
# import traceback
import argparse
from subprocess import PIPE, STDOUT, Popen, list2cmdline

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
        self.debug = self.args.debug

        self.datetime = time.strftime('%Y%m%d-%H%M')

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
            "--debug",
            required=False,
            help="say everything",
            default=False
        )

        self.args = p.parse_args()

    def run(self):
        """
        """
        if os.path.exists(self.config_file):
            if self.debug:
                print(f"read config file: {self.config_file}")
            self.read_configuration(os.path.join(self.config_file))
            print("")

        """
            create output directory
        """
        self.output_directory = os.path.join(self.backup_directory, self.datetime)
        try:
            os.makedirs(self.output_directory, exist_ok=True)
        except FileExistsError:
            pass
        if self.debug:
            print(f"output directory: {self.output_directory}")
            print("list databases ...")
        # db_status = self.check_database()
        all_db_names = self.list_databases()

        db_names = self.reduce_databases(all_databases=all_db_names)

        if self.debug:
            print("dump databases ...")
            print(f"  {db_names}")
        # os.chdir(output_directory)
        self.dump_databases(db_names)

    def read_configuration(self, filename):
        """
        """
        with open(filename) as file:
            content = yaml.load(file, Loader=yaml.FullLoader)

            if content:
                connection = content.get("connection", {})
                dump = content.get("dump", {})
                rotation = content.get("rotation", {})
                excludes = content.get("excludes", {})

                if connection:
                    self.db_username = connection.get("username", None)
                    self.db_password = connection.get("password", None)
                    self.db_host = connection.get("host", None)
                    self.db_socket = connection.get("socket", None)

                # if dump:
                #     self.dump_create_database = dump.get("create_database", False)
                #     self.dump_full_schema = dump.get("full_schema", False)
                #     # self.dump_dbstatus = dump.get("dbstatus", False)
                #     # self.dump_use_separate_dirs = dump.get("use_separate_dirs", False)
                #     # self.dump_host_friendly = dump.get("host_friendly", False)
                #     # self.dump_single_transaction = dump.get("single_transaction", False)

                if rotation:
                    self.rotation_daily = rotation.get("daily", 6)

                if excludes:
                    self.excludes_databases = excludes.get("databases", [])
                    self.excludes_tables = excludes.get("tables", [])

    def dump_options(self):
        """
        """
        opt=[
            '--quote-names',
            '--opt',
            '--events']

        opt_only_schema=[
            '--routines',
            '--single-transaction',
            '--no-data']

        opt_only_data=[
            '--routines',
            '--single-transaction',
            '--no-create-info']

        opt_dbstatus=[
            '--status']

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

        if len(self.db_socket) != 0:
            opt.append('--socket')
            opt.append(self.db_socket)
            opt_only_schema.append('--socket')
            opt_only_schema.append(self.db_socket)
            opt_only_data.append('--socket')
            opt_only_data.append(self.db_socket)

        if self.debug:
            print(f"opt: {opt}")
            print(f"opt_only_schema: {opt_only_schema}")
            print(f"opt_only_data: {opt_only_data}")

        return (opt, opt_only_schema, opt_only_data)

    def list_databases(self):
        """ """
        all_db_names=[]
        rows = None

        cursor, conn, error, message = self._mysql_connect()

        query = "show databases"

        if self.debug:
            print(f"   {bcolors.DEBUG}query: {query}{bcolors.ENDC}")

        try:
            cursor.execute(query)
            rows = cursor.fetchall()

        except Exception as e:
            print(f"   {bcolors.FAIL}Cannot execute SQL {query}: {e}{bcolors.ENDC}")
            os._exit(1)

        # print(f"   {bcolors.DEBUG}{rows} {len(rows)}{bcolors.ENDC}")

        if rows:
            for index in range(len(rows)):
                all_db_names.append(rows[index].get("Database", None))

        if self.debug:
            print(f"   {bcolors.DEBUG}{all_db_names}{bcolors.ENDC}")

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
                if self.debug:
                    print(f"   {bcolors.DEBUG}dba: {dba}{bcolors.ENDC}")

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

                if self.debug:
                    print(f"   {bcolors.DEBUG}schema{bcolors.ENDC}")
                    print(f"   {bcolors.DEBUG}{_args}{bcolors.ENDC}")
                self._dump(sql_file_name, _args)

                _args = None

                _args = list(args)
                _args += list(opt_only_data)
                _args.append('--databases')
                _args.append(dba)

                sql_file_name = "data.sql"

                if self.debug:
                    print(f"   {bcolors.DEBUG}data{bcolors.ENDC}")
                    print(f"   {bcolors.DEBUG}{_args}{bcolors.ENDC}")
                self._dump(sql_file_name, _args)

                _args = None

                os.chdir(_output_directory)
        # else:
        #     _args = []
        #     _args = args
        #     _args.append('--all-databases')
        #
        #     sql_file_name = "database.sql"
        #
        #     self._dump(sql_file_name, _args)


    def _dump(self, dump_file, args):

        cmdline = list2cmdline(args)

        if self.debug:
            print(f"args: {cmdline}")

        stdout = open(dump_file, "w", 1)  # line-buffered

        try:
            process = Popen(args, stdout=stdout, stderr=PIPE, close_fds=True)

            if process.returncode and process.returncode > 1:
                print(f"   {bcolors.FAIL}{cmdline} failed. Code {process.returncode}. Output follows below.{bcolors.ENDC}")
                print(f"   {bcolors.FAIL}{stderr}{bcolors.ENDC}")

                for line in process.stdout:
                    sys.stdout.write(line)
                    # log_file.write(line)
            process.wait()
            stdout.close()
            # ret_code = process.wait()
        except OSError:
            print("Failed to find mariadb-dump binary")
            sys.exit(1)

            #for line in stderr.splitlines():
            #    print(f"   {bcolors.FAIL}{line}{bcolors.ENDC}")


    def _mysql_connect(self):
        """
        """
        config = {}

        config_file = self.mycnf_file

        if config_file and os.path.exists(config_file):
            config['read_default_file'] = config_file

        # config['host'] = "127.0.0.1"

        # If dba_user or dba_password are given, they should override the
        # config file
        if self.db_username is not None:
            config['user'] = self.db_username
        if self.db_password is not None:
            config['passwd'] = self.db_password
        if self.db_socket is not None:
            config['unix_socket'] = self.db_socket

        if self.debug:
            print(f"{bcolors.DEBUG}config : {config}{bcolors.ENDC}")

        if mysql_driver is None:
            print(f"   {bcolors.FAIL}missing SQL driver{bcolors.ENDC}")
            sys.exit(1)

        try:
            db_connection = mysql_driver.connect(**config)
            cursor = db_connection.cursor(mysql_driver.cursors.DictCursor)

        except Exception as e:
            message = "unable to connect to database.\n"
            message += f"  Exception message: {e}\n"
            message += "    check login_host, login_user and login_password are correct \n"
            message += f"    or {config_file} has the credentials. "

            print(f"  {bcolors.FAIL}{message}{bcolors.ENDC}")

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


if __name__ == '__main__':
    """
    """
    r = MariaDBBackup()

    r.run()
