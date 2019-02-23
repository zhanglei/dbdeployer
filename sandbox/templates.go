// DBDeployer - The MySQL Sandbox
// Copyright © 2006-2019 Giuseppe Maxia
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sandbox

import (
	"fmt"
	"os"
)

type TemplateDesc struct {
	TemplateInFile bool
	Description    string
	Notes          string
	Contents       string
}

type TemplateCollection map[string]TemplateDesc
type AllTemplateCollection map[string]TemplateCollection

// templates for single sandbox

var (
	Copyright string = `
#    DBDeployer - The MySQL Sandbox
#    Copyright (C) 2006-2018 Giuseppe Maxia
#
#    Licensed under the Apache License, Version 2.0 (the "License");
#    you may not use this file except in compliance with the License.
#    You may obtain a copy of the License at
#
#        http://www.apache.org/licenses/LICENSE-2.0
#
#    Unless required by applicable law or agreed to in writing, software
#    distributed under the License is distributed on an "AS IS" BASIS,
#    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#    See the License for the specific language governing permissions and
#    limitations under the License.
`
	initDbTemplate string = `#!/bin/bash
		{{.Copyright}}
		# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
		export SBDIR="{{.SandboxDir}}"
		export BASEDIR={{.Basedir}}
		export DATADIR=$SBDIR/data
		export LD_LIBRARY_PATH=$BASEDIR/lib:$BASEDIR/lib/mysql:$LD_LIBRARY_PATH
		export DYLD_LIBRARY_PATH=$BASEDIR/lib:$BASEDIR/lib/mysql:$DYLD_LIBRARY_PATH

		cd $SBDIR
		if [ -d $DATADIR/mysql ]
		then
			echo "Initialization already done."
			echo "This script should run only once."
			exit 0
		fi
		
		{{.InitScript}} \
		    {{.InitDefaults}} \
		    --user={{.OsUser}} \
		    --basedir=$BASEDIR \
		    --datadir=$DATADIR \
		    --tmpdir=$SBDIR/tmp {{.ExtraInitFlags}}
		exit_code=$?
		if [ "$exit_code" == "0" ]
		then
			echo "Database installed in $SBDIR"
		else
			echo "Error installing database in $SBDIR"
		fi
		{{.FixUuidFile1}}
		{{.FixUuidFile2}}
`

	startTemplate string = `#!/bin/bash
		{{.Copyright}}
		# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
		source {{.SandboxDir}}/sb_include
		MYSQLD_SAFE="bin/mysqld_safe"
		CUSTOM_MYSQLD={{.CustomMysqld}}
		if [ -n "$CUSTOM_MYSQLD" ]
		then
    		CUSTOM_MYSQLD="--mysqld=$CUSTOM_MYSQLD"
		fi
		if [ ! -f $BASEDIR/$MYSQLD_SAFE ]
		then
			echo "mysqld_safe not found in $BASEDIR/bin/"
			exit 1
		fi
		MYSQLD_SAFE_OK=$(sh -n $BASEDIR/$MYSQLD_SAFE 2>&1)
		if [ "$MYSQLD_SAFE_OK" == "" ]
		then
			if [ "$SBDEBUG" == "2" ]
			then
				echo "$MYSQLD_SAFE OK"
			fi
		else
			echo "$MYSQLD_SAFE has errors"
			echo "((( $MYSQLD_SAFE_OK )))"
			exit 1
		fi

		TIMEOUT=180
		if [ -n "$(is_running)" ]
		then
			echo "sandbox server already started (found pid file $PIDFILE)"
		else
			if [ -f $PIDFILE ]
			then
				# Server is not running. Removing stale pid-file
				rm -f $PIDFILE
			fi
			CURDIR=$(pwd)
			cd $BASEDIR
			if [ "$SBDEBUG" = "" ]
			then
				$MYSQLD_SAFE --defaults-file=$SBDIR/my.sandbox.cnf $CUSTOM_MYSQLD $@ > /dev/null 2>&1 &
			else
				$MYSQLD_SAFE --defaults-file=$SBDIR/my.sandbox.cnf $CUSTOM_MYSQLD $@ > "$SBDIR/start.log" 2>&1 &
			fi
			cd $CURDIR
			ATTEMPTS=1
			while [ ! -f $PIDFILE ]
			do
				ATTEMPTS=$(( $ATTEMPTS + 1 ))
				echo -n "."
				if [ $ATTEMPTS = $TIMEOUT ]
				then
					break
				fi
				sleep $SLEEP_TIME
			done
		fi

		if [ -f $PIDFILE ]
		then
			echo " sandbox server started"
		else
			echo " sandbox server not started yet"
			exit 1
		fi
`
	useTemplate string = `#!/bin/bash
		{{.Copyright}}
		# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
		source {{.SandboxDir}}/sb_include
		export LD_LIBRARY_PATH=$CLIENT_LD_LIBRARY_PATH
		[ -n "$TEST_REPL_DELAY" -a -f $SBDIR/data/mysql-relay.index ] && sleep $TEST_REPL_DELAY
		[ -z "$MYSQL_EDITOR" ] && MYSQL_EDITOR="$CLIENT_BASEDIR/bin/mysql"
		if [ ! -x $MYSQL_EDITOR ]
		then
			if [ -x $SBDIR/$MYSQL_EDITOR ]
			then
				MYSQL_EDITOR=$SBDIR/$MYSQL_EDITOR
			else
				echo "MYSQL_EDITOR '$MYSQL_EDITOR' not found or not executable"
				exit 1
			fi
		fi
		HISTDIR={{.HistoryDir}}
		[ -z "$HISTDIR" ] && export HISTDIR=$SBDIR
		[ -z "$MYSQL_HISTFILE" ] && export MYSQL_HISTFILE="$HISTDIR/.mysql_history"
		MY_CNF=$SBDIR/my.sandbox.cnf
		MY_CNF_NO_PASSWORD=$SBDIR/my.sandbox_np.cnf
		if [ -n "$NOPASSWORD" ]
		then
			grep -v '^password' < $MY_CNF > $MY_CNF_NO_PASSWORD
			MY_CNF=$MY_CNF_NO_PASSWORD
		fi
		if [ -f $PIDFILE ]
		then
			$MYSQL_EDITOR --defaults-file=$MY_CNF $MYCLIENT_OPTIONS "$@"
		else
			exit 1
		fi
`
	mysqlshTemplate string = `#!/bin/bash
{{.Copyright}}
# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
source {{.SandboxDir}}/sb_include
export LD_LIBRARY_PATH=$CLIENT_LD_LIBRARY_PATH
[ -z "$MYSQL_SHELL" ] && MYSQL_SHELL="{{.MysqlShell}}"

[ -z "$URI" ] && URI="root:{{.DbPassword}}@127.0.0.1:{{.MysqlXPort}}"

if [ -f $PIDFILE ]
then
    if [ "$1" != "" ]
	then
	    $MYSQL_SHELL --uri="$URI" "$*"
	else
	    $MYSQL_SHELL --uri="$URI"
	fi
else
	echo "# $0 pidfile not found"
	exit 1
fi
`

	stopTemplate string = `#!/bin/bash
		{{.Copyright}}
		# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
		source {{.SandboxDir}}/sb_include
		export LD_LIBRARY_PATH=$CLIENT_LD_LIBRARY_PATH

		MYSQL_ADMIN="$CLIENT_BASEDIR/bin/mysqladmin"

		if [ -n "$(is_running)" ]
		then
			echo "stop $SBDIR"
			# echo "$MYSQL_ADMIN --defaults-file=$SBDIR/my.sandbox.cnf $MYCLIENT_OPTIONS shutdown"
			$MYSQL_ADMIN --defaults-file=$SBDIR/my.sandbox.cnf $MYCLIENT_OPTIONS shutdown
			sleep $SLEEP_TIME
		else
			if [ -f $PIDFILE ]
			then
				rm -f $PIDFILE
			fi
		fi

		if [ -n "$(is_running)" ]
		then
			# use the send_kill script if the server is not responsive
			$SBDIR/send_kill
		fi
`
	clearTemplate string = `#!/bin/bash
		{{.Copyright}}
		# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
		source {{.SandboxDir}}/sb_include
		export LD_LIBRARY_PATH=$CLIENT_LD_LIBRARY_PATH

		cd $SBDIR

		#
		# attempt to drop databases gracefully
		#

		if [ -n "$(is_running)" ]
		then
			for D in $(echo "show databases " | ./use -B -N | grep -v "^mysql$" | grep -iv "^information_schema$" | grep -iv "^performance_schema" | grep -ivw "^sys")
			do
				echo "set sql_mode=ansi_quotes;drop database \"$D\"" | ./use
			done
			VERSION={{.Version}}
			is_slave=$(ls data | grep relay)
			if [ -n "$is_slave" ]
			then
				./use -e "stop slave; reset slave;"
			fi
			if [[ "$VERSION" > "5.1" ]]
			then
				for T in general_log slow_log plugin
				do
					exists_table=$(./use -e "show tables from mysql like '$T'")
					if [ -n "$exists_table" ]
					then
						./use -e "truncate mysql.$T"
					fi
				done
			fi
		fi

		is_master=$(ls data | grep 'mysql-bin')
		if [ -n "$is_master" ]
		then
			./use -e 'reset master'
		fi

		./stop
		rm -f data/$(hostname)*
		rm -f data/log.0*
		rm -f data/*.log

		#
		# remove all databases if any (up to 8.0)
		#
		if [[ "$VERSION" < "8.0" ]]
		then
			for D in $(ls -d data/*/ | grep -w -v mysql | grep -iv performance_schema | grep -ivw sys)
			do
				rm -rf $D
			done
			mkdir data/test
		fi
`

	myCnfTemplate string = `
		{{.Copyright}}
		# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
[mysql]
prompt='{{.Prompt}} [\h:{{.Port}}] {\u} (\d) > '
#

[client]
user               = {{.DbUser}}
password           = {{.DbPassword}}
port               = {{.Port}}
socket             = {{.GlobalTmpDir}}/mysql_sandbox{{.Port}}.sock

[mysqld]
user               = {{.OsUser}}
port               = {{.Port}}
socket             = {{.GlobalTmpDir}}/mysql_sandbox{{.Port}}.sock
basedir            = {{.Basedir}}
datadir            = {{.Datadir}}
tmpdir             = {{.Tmpdir}}
pid-file           = {{.Datadir}}/mysql_sandbox{{.Port}}.pid
bind-address       = {{.BindAddress}}
{{.ReportHost}}
{{.ReportPort}}
log-error={{.Datadir}}/msandbox.err
{{.ServerId}}
{{.ReplOptions}}
{{.GtidOptions}}
{{.ReplCrashSafeOptions}}
{{.SemiSyncOptions}}
{{.ReadOnlyOptions}}

{{.ExtraOptions}}
`
	sendKillTemplate string = `#!/bin/bash
		{{.Copyright}}
		# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
		source {{.SandboxDir}}/sb_include

		TIMEOUT=30

		mysqld_safe_pid=$(ps auxw | grep mysqld_safe | grep "defaults-file=$SBDIR" | awk '{print $2}')
		if [ -n "$(is_running)" ]
		then
			MYPID=$(cat $PIDFILE)
			kill -9 $mysqld_safe_pid
			echo "Attempting normal termination --- kill -15 $MYPID"
			kill -15 $MYPID
			# give it a chance to exit peacefully
			ATTEMPTS=1
			while [ -f $PIDFILE ]
			do
				ATTEMPTS=$(( $ATTEMPTS + 1 ))
				if [ $ATTEMPTS = $TIMEOUT ]
				then
					break
				fi
				sleep $SLEEP_TIME
			done
			if [ -f $PIDFILE ]
			then
				echo "SERVER UNRESPONSIVE --- kill -9 $MYPID"
				kill -9 $MYPID
				rm -f $PIDFILE
			fi
		else
			# server not running - removing stale pid-file
			if [ -f $PIDFILE ]
			then
				rm -f $PIDFILE
			fi
		fi
`
	statusTemplate string = `#!/bin/bash
		{{.Copyright}}
		# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
		source {{.SandboxDir}}/sb_include
		export LD_LIBRARY_PATH=$CLIENT_LD_LIBRARY_PATH

		baredir=$(basename $SBDIR)

		node_status=off
		exit_code=0
		if [ -f $PIDFILE ]
		then
			MYPID=$(cat $PIDFILE)
			running=$(ps -p $MYPID | grep $MYPID)
			if [ -n "$running" ]
			then
				node_status=on
				exit_code=0
			fi
		fi
		echo "$baredir $node_status"
		exit $exit_code
`
	restartTemplate string = `#!/bin/bash
		{{.Copyright}}
		# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
		source {{.SandboxDir}}/sb_include

		$SBDIR/stop
		$SBDIR/start $@
`
	loadGrantsTemplate string = `#!/bin/bash
		{{.Copyright}}
		# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
		source {{.SandboxDir}}/sb_include
		export LD_LIBRARY_PATH=$CLIENT_LD_LIBRARY_PATH

		SOURCE_SCRIPT=$1
		if [ -z "$SOURCE_SCRIPT" ]
		then
			SOURCE_SCRIPT=grants.mysql
		fi
		PRE_GRANT_SCRIPTS="grants.mysql pre_grants.sql"
		if [ -n "$(echo $PRE_GRANT_SCRIPTS | grep $SOURCE_SCRIPT)" ]
		then
			export NOPASSWORD=1
		fi
		if [ -n "$NOPASSWORD" ]
		then
			MYSQL="$CLIENT_BASEDIR/bin/mysql --no-defaults --socket={{.GlobalTmpDir}}/mysql_sandbox{{.Port}}.sock --port={{.Port}}"
		else
			MYSQL="$CLIENT_BASEDIR/bin/mysql --defaults-file=$SBDIR/my.sandbox.cnf"
		fi
		VERBOSE_SQL=''
		[ -n "$SBDEBUG" ] && VERBOSE_SQL=-v
		if [ ! -f $SBDIR/$SOURCE_SCRIPT ]
		then
			[ -n "$VERBOSE_SQL" ] && echo "$SBDIR/$SOURCE_SCRIPT not found"
			exit 0
		fi
		# echo "$MYSQL -u root -t $VERBOSE_SQL < $SBDIR/$SOURCE_SCRIPT"
		$MYSQL -u root -t $VERBOSE_SQL < $SBDIR/$SOURCE_SCRIPT
`
	grantsTemplate5x string = `
# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
use mysql;
set password=password('{{.DbPassword}}');
grant all on *.* to {{.DbUser}}@'{{.RemoteAccess}}' identified by '{{.DbPassword}}';
grant all on *.* to {{.DbUser}}@'localhost' identified by '{{.DbPassword}}';
grant SELECT,INSERT,UPDATE,DELETE,CREATE,DROP,INDEX,ALTER,
    SHOW DATABASES,CREATE TEMPORARY TABLES,LOCK TABLES, EXECUTE
    on *.* to msandbox_rw@'localhost' identified by '{{.DbPassword}}';
grant SELECT,INSERT,UPDATE,DELETE,CREATE,DROP,INDEX,ALTER,
    SHOW DATABASES,CREATE TEMPORARY TABLES,LOCK TABLES, EXECUTE
    on *.* to msandbox_rw@'{{.RemoteAccess}}' identified by '{{.DbPassword}}';
grant SELECT,EXECUTE on *.* to msandbox_ro@'{{.RemoteAccess}}' identified by '{{.DbPassword}}';
grant SELECT,EXECUTE on *.* to msandbox_ro@'localhost' identified by '{{.DbPassword}}';
grant REPLICATION SLAVE on *.* to {{.RplUser}}@'{{.RemoteAccess}}' identified by '{{.RplPassword}}';
delete from user where password='';
delete from db where user='';
flush privileges;
create database if not exists test;
`
	grantsTemplate57 string = `
# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
use mysql;
set password='{{.DbPassword}}';

create user {{.DbUser}}@'{{.RemoteAccess}}' identified by '{{.DbPassword}}';
grant all on *.* to {{.DbUser}}@'{{.RemoteAccess}}' ;

create user {{.DbUser}}@'localhost' identified by '{{.DbPassword}}';
grant all on *.* to {{.DbUser}}@'localhost';

create user msandbox_rw@'localhost' identified by '{{.DbPassword}}';
grant SELECT,INSERT,UPDATE,DELETE,CREATE,DROP,INDEX,ALTER,
    SHOW DATABASES,CREATE TEMPORARY TABLES,LOCK TABLES, EXECUTE
    on *.* to msandbox_rw@'localhost';

create user msandbox_rw@'{{.RemoteAccess}}' identified by '{{.DbPassword}}';
grant SELECT,INSERT,UPDATE,DELETE,CREATE,DROP,INDEX,ALTER,
    SHOW DATABASES,CREATE TEMPORARY TABLES,LOCK TABLES, EXECUTE
    on *.* to msandbox_rw@'{{.RemoteAccess}}';

create user msandbox_ro@'{{.RemoteAccess}}' identified by '{{.DbPassword}}';
create user msandbox_ro@'localhost' identified by '{{.DbPassword}}';
create user {{.RplUser}}@'{{.RemoteAccess}}' identified by '{{.RplPassword}}';
grant SELECT,EXECUTE on *.* to msandbox_ro@'{{.RemoteAccess}}';
grant SELECT,EXECUTE on *.* to msandbox_ro@'localhost';
grant REPLICATION SLAVE on *.* to {{.RplUser}}@'{{.RemoteAccess}}';
create schema if not exists test;
`
	grantsTemplate8x string = `
# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
use mysql;
set password='{{.DbPassword}}';

create role R_DO_IT_ALL;
create role R_READ_WRITE;
create role R_READ_ONLY;
create role R_REPLICATION;

grant all on *.* to R_DO_IT_ALL;
grant SELECT,INSERT,UPDATE,DELETE,CREATE,DROP,INDEX,ALTER,
    SHOW DATABASES,CREATE TEMPORARY TABLES,LOCK TABLES, EXECUTE
    on *.* to R_READ_WRITE;
grant SELECT,EXECUTE on *.* to R_READ_ONLY;
grant REPLICATION SLAVE on *.* to R_REPLICATION;

create user {{.DbUser}}@'{{.RemoteAccess}}' identified by '{{.DbPassword}}';
create user {{.DbUser}}@'localhost' identified by '{{.DbPassword}}';

grant R_DO_IT_ALL to {{.DbUser}}@'{{.RemoteAccess}}' ;
set default role R_DO_IT_ALL to {{.DbUser}}@'{{.RemoteAccess}}';

grant R_DO_IT_ALL to {{.DbUser}}@'localhost' ;
set default role R_DO_IT_ALL to {{.DbUser}}@'localhost';

create user msandbox_rw@'localhost' identified by '{{.DbPassword}}';
create user msandbox_rw@'{{.RemoteAccess}}' identified by '{{.DbPassword}}';

grant R_READ_WRITE to msandbox_rw@'localhost';
set default role R_READ_WRITE to msandbox_rw@'localhost';
grant R_READ_WRITE to msandbox_rw@'{{.RemoteAccess}}';
set default role R_READ_WRITE to msandbox_rw@'{{.RemoteAccess}}';

create user msandbox_ro@'{{.RemoteAccess}}' identified by '{{.DbPassword}}';
create user msandbox_ro@'localhost' identified by '{{.DbPassword}}';
create user {{.RplUser}}@'{{.RemoteAccess}}' identified by '{{.RplPassword}}';

grant R_READ_ONLY to msandbox_ro@'{{.RemoteAccess}}';
set default role R_READ_ONLY to msandbox_ro@'{{.RemoteAccess}}';

grant R_READ_ONLY to msandbox_ro@'localhost';
set default role R_READ_ONLY to msandbox_ro@'localhost';

grant R_REPLICATION to {{.RplUser}}@'{{.RemoteAccess}}';
set default role R_REPLICATION to {{.RplUser}}@'{{.RemoteAccess}}';

create schema if not exists test;
`

	addOptionTemplate string = `#!/bin/bash
{{.Copyright}}
# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
source {{.SandboxDir}}/sb_include
export LD_LIBRARY_PATH=$CLIENT_LD_LIBRARY_PATH

curdir=$SBDIR
cd $curdir

if [ -z "$*" ]
then
    echo "# Syntax $0 options-for-my.cnf [more options] "
    exit
fi

CHANGED=''
NO_RESTART=''

for OPTION in $@
do
    # Users can choose to skip restart if they use one of the
    # following keywords on the command line
    if [ "$OPTION" == "NORESTART" -o "$OPTION" == "NO_RESTART" -o "$OPTION" == "SKIP_RESTART" ]
    then
        NO_RESTART=1
        continue
    fi
    option_exists=$(grep $OPTION ./my.sandbox.cnf)
    if [ -z "$option_exists" ]
    then
        echo "$OPTION" >> my.sandbox.cnf
        echo "# option '$OPTION' added to configuration file"
        CHANGED=1
    else
        echo "# option '$OPTION' already exists in configuration file"
    fi
done

if [ -n "$CHANGED" -a -z "$NO_RESTART" ]
then
    ./restart
fi
`
	showLogTemplate string = `#!/bin/bash
{{.Copyright}}
# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
source {{.SandboxDir}}/sb_include

cd $SBDIR

if [ ! -d ./data ]
then
    echo "$SBDIR/data not found"
    exit 1
fi

log=$1
[ -z "$log" ] && log='err'
function get_help {
    exit_code=$1
    [ -z "$exit_code" ] && exit_code=0
    echo "# Usage: $0 [log] "
    echo "# Where 'log' is one of 'err' (error log),  'gen' (general log)"
	echo "# Or it can be a variable name that identifies a log"
    echo "# (The default is 'err')"
    exit $exit_code
}

if [ "$log" == "-h" -o "$log" == "--help" -o "$log" == "-help" -o "$log" == "help" ]
then
    get_help 0
fi

check_output

case $log in
    err)
        target=$SBDIR/data/msandbox.err
        ;;
    gen)
        target=$($SBDIR/use -BN -e "show variables like 'general_log_file'" | awk '{print $2}')
        ;;
    slow)
        target=$SBDIR/data/slow_log.data
        ;;
    *)
        target=$($SBDIR/use -BN -e "show variables like '$log'" | awk '{print $2}')
        ;;
esac

if [ -z "$target" ]
then
    echo "target not set"
    exit 1
fi
if [ ! -f $target ]
then
    echo "Log file '$target' not found"
    exit 1
fi

if [ -n "$pager" ]
then
    (printf "#\n# Showing $target\n#\n" ; cat $target ) | $pager
else
    (printf "#\n# Showing $target\n#\n" ; cat $target )
fi
`
	showBinlogTemplate string = `#!/bin/bash
{{.Copyright}}
# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
source {{.SandboxDir}}/sb_include
export LD_LIBRARY_PATH=$CLIENT_LD_LIBRARY_PATH

curdir=$SBDIR
cd $curdir

if [ ! -d ./data ]
then
    echo "$curdir/data not found"
    exit 1
fi

pattern=$1
[ -z "$pattern" ] && pattern='[0-9]*'
function get_help {
    exit_code=$1
    [ -z "$exit_code" ] && exit_code=0
    echo "# Usage: $0 [BINLOG_PATTERN] "
    echo "# Where BINLOG_PATTERN is a number, or part of a number used after 'mysql-bin'"
    echo "# (The default is '[0-9]*]')"
    echo "# examples:"
    echo "#          ./show_binlog 000001 | less "
    echo "#          ./show_binlog 000012 | vim - "
    echo "#          ./show_binlog  | grep -i 'CREATE TABLE'"
    exit $exit_code
}

if [ "$pattern" == "-h" -o "$pattern" == "--help" -o "$pattern" == "-help" -o "$pattern" == "help" ]
then
    get_help 0
fi
# set -x
last_binlog=$(ls -lotr data/mysql-bin.$pattern | tail -n 1 | awk '{print $NF}')

if [ -z "$last_binlog" ]
then
    echo "No binlog found in $curdir/data"
    get_help 1
fi

check_output

if [ -n "$pager" ]
then
    (printf "#\n# Showing $last_binlog\n#\n" ; ./my sqlbinlog --verbose $last_binlog ) | $pager
else
    (printf "#\n# Showing $last_binlog\n#\n" ; ./my sqlbinlog --verbose $last_binlog )
fi
`
	myTemplate string = `#!/bin/bash
{{.Copyright}}
# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
source {{.SandboxDir}}/sb_include
export LD_LIBRARY_PATH=$CLIENT_LD_LIBRARY_PATH

if [ "$1" = "" ]
then
    echo "syntax my sql{dump|binlog|admin} arguments"
    exit
fi

MYSQL=$CLIENT_BASEDIR/bin/mysql

SUFFIX=$1
shift

MYSQLCMD="$CLIENT_BASEDIR/bin/my$SUFFIX"

NODEFAULT=(myisam_ftdump
myisamlog
mysql_config
mysql_convert_table_format
mysql_find_rows
mysql_fix_extensions
mysql_fix_privilege_tables
mysql_secure_installation
mysql_setpermission
mysql_tzinfo_to_sql
mysql_config_editor
mysql_waitpid
mysql_zap
mysqlaccess
mysqlbinlog
mysqlbug
mysqldumpslow
mysqlhotcopy
mysqltest
mysqlsh
mysqltest_embedded)

DEFAULTSFILE="--defaults-file=$SBDIR/my.sandbox.cnf"

for NAME in ${NODEFAULT[@]}
do
    if [ "my$SUFFIX" = "$NAME" ]
    then
        DEFAULTSFILE=""
        break
    fi
done

if [ -f $MYSQLCMD ]
then
    $MYSQLCMD $DEFAULTSFILE "$@"
else
    echo "$MYSQLCMD not found "
fi
`
	showRelaylogTemplate string = `#!/bin/bash
{{.Copyright}}
# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
source {{.SandboxDir}}/sb_include
export LD_LIBRARY_PATH=$CLIENT_LD_LIBRARY_PATH
curdir=$SBDIR
cd $curdir

if [ ! -d ./data ]
then
    echo "$curdir/data not found"
    exit 1
fi
relay_basename=$1
[ -z "$relay_basename" ] && relay_basename='mysql-relay'
pattern=$2
[ -z "$pattern" ] && pattern='[0-9]*'
function get_help {
    exit_code=$1
    [ -z "$exit_code" ] && exit_code=0
    echo "# Usage: $0 [ relay-base-name [BINLOG_PATTERN]] "
    echo "# Where relay-basename is the initial part of the relay ('$relay_basename')"
    echo "# and BINLOG_PATTERN is a number, or part of a number used after '$relay_basename'"
    echo "# (The default is '[0-9]*]')"
    echo "# examples:"
    echo "#          ./show_relaylog relay-log-alpha 000001 | less "
    echo "#          ./show_relaylog relay-log 000012 | vim - "
    echo "#          ./show_relaylog  | grep -i 'CREATE TABLE'"
    exit $exit_code
}

if [ "$pattern" == "-h" -o "$pattern" == "--help" -o "$pattern" == "-help" -o "$pattern" == "help" ]
then
    get_help 0
fi
# set -x
last_relaylog=$(ls -lotr data/$relay_basename.$pattern | tail -n 1 | awk '{print $NF}')

if [ -z "$last_relaylog" ]
then
    echo "No relay log found in $curdir/data"
    get_help 1
fi

check_output

if [ -n "$pager" ]
then
    (printf "#\n# Showing $last_relaylog\n#\n" ; ./my sqlbinlog --verbose $last_relaylog ) | $pager
else
   (printf "#\n# Showing $last_relaylog\n#\n" ; ./my sqlbinlog --verbose $last_relaylog )
fi
`
	testSbTemplate string = `#!/bin/bash
{{.Copyright}}
# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
source {{.SandboxDir}}/sb_include
export LD_LIBRARY_PATH=$CLIENT_LD_LIBRARY_PATH
cd $SBDIR

fail=0
pass=0
TIMEOUT=180
expected_port={{.Port}}
expected_version=$(echo "{{.Version}}" | tr -d 'A-Z,a-z,_-')


if [ -f sbdescription.json ]
then
	sb_single=$(grep 'type' sbdescription.json| grep 'single')
fi

function test_query {
    user=$1
    query="$2"
    expected=$3
	./use -BN -u $user -e "$query" > /dev/null 2>&1
    exit_code=$?
    if [ "$exit_code" == "$expected" ]
    then
		msg="was successful"
		if [ "$expected" != "0" ]
		then
			msg="failed as expected"
		fi
        echo "ok - query $msg for user $user: '$query'"
        pass=$((pass+1))
    else
        echo "not ok - query failed for user $user: '$query'"
        fail=$((fail+1))
    fi
}

if [ -n "$CHECK_LOGS" ]
then
    log_has_errors=$(grep ERROR $SBDIR/data/msandbox.err)
    if [ -z "$log_has_errors" ]
    then
	    echo "ok - no errors in log"
        pass=$((pass+1))
    else
        echo "not ok - errors found in log"
        fail=$((fail+1))
    fi
fi

if [ -z "$(is_running)" ]
then
	echo "not ok - server stopped"
    fail=$((fail+1))
else
    version=$(./use -BN -e "select version()")
    port=$(./use -BN -e "show variables like 'port'" | awk '{print $2}')
    if [ -n "$version" ]
    then
        echo "ok - version '$version'"
        pass=$((pass+1))
    else
        echo "not ok - no version detected"
        fail=$((fail+1))
    fi
    if [ -n "$port" ]
    then
        echo "ok - port detected: $port"
        pass=$((pass+1))
    else
        echo "not ok - no port detected"
        fail=$((fail+1))
    fi
    
    if [ -n "$( echo $version| grep $expected_version)" ]
    then
        echo "ok - version is $version as expected"
        pass=$((pass+1))
    else
        echo "not ok - version detected ($version) but expected was $expected_version"
        fail=$((fail+1))
    fi
    if [ "$port" == "$expected_port" ]
    then
        echo "ok - port is $port as expected"
        pass=$((pass+1))
    else
        echo "not ok - port detected ($port) but expected was $expected_port"
        fail=$((fail+1))
    fi
	if [[ $MYSQL_VERSION_MAJOR -ge 5 ]]
    then
	    ro_query='use mysql; select count(*) from information_schema.tables where table_schema=schema()'
    else
	    ro_query='show tables from mysql'
    fi
    create_query='create table if not exists test.txyz(i int)'
    drop_query='drop table if exists test.txyz'
    test_query msandbox_ro 'select 1' 0
    test_query msandbox_rw 'select 1' 0
    test_query msandbox_ro "$ro_query" 0
    test_query msandbox_rw "$ro_query" 0
	if [ -n "$sb_single" ]
	then
        test_query msandbox_ro "$create_query" 1
        test_query msandbox_rw "$create_query" 0
        test_query msandbox_rw "$drop_query" 0
	fi
fi
fail_label="fail"
pass_label="PASS"
exit_code=0
tests=$(($pass+$fail))
if [ "$fail" != "0" ]
then
	fail_label="FAIL"
	pass_label="pass"
	exit_code=1
fi
printf "# Tests : %5d\n" $tests
printf "# $pass_label  : %5d \n" $pass
printf "# $fail_label  : %5d \n" $fail
exit $exit_code
`
	replicationOptions string = `
# basic replication options
relay-log-index=mysql-relay
relay-log=mysql-relay
log-bin=mysql-bin
`
	semisyncMasterOptions string = `
# semi-synchronous replication options for master
plugin-load=rpl_semi_sync_master=semisync_master.so
#rpl_semi_sync_master_enabled=1
`
	semisyncSlaveOptions string = `
# semi-synchronous replication options for slave
plugin-load=rpl_semi_sync_slave=semisync_slave.so
#rpl_semi_sync_slave_enabled=1
`
	replCrashSafeOptions string = `
# replication crash-safe options
master-info-repository=table
relay-log-info-repository=table
relay-log-recovery=on
`
	gtidOptions56 string = `
# GTID options for 5.6
gtid_mode=ON
log-slave-updates
enforce-gtid-consistency
`
	gtidOptions57 string = `
# GTID options for 5.7 +
gtid_mode=ON
enforce-gtid-consistency
`
	exposeDdTables string = `
set persist debug='+d,skip_dd_table_access_check';
set @col_type=(select c.type from mysql.columns c inner join mysql.tables t where t.id=table_id and t.name='tables' and c.name='hidden');
set @visible=(if(@col_type = 'MYSQL_TYPE_ENUM', 'Visible', '0'));
set @hidden=(if(@col_type = 'MYSQL_TYPE_ENUM', 'System', '1'));
create table sys.dd_hidden_tables (id bigint unsigned not null primary key, name varchar(64), schema_id bigint unsigned);
insert into sys.dd_hidden_tables select id, name, schema_id from mysql.tables where hidden=@hidden;
update mysql.tables set hidden=@visible where hidden=@hidden and schema_id = 1
`

	sbLockedTemplate string = `#!/bin/bash
{{.Copyright}}
# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
echo "This sandbox is locked."
echo "The '{{.ClearCmd}}' command has been disabled."
echo "The contents of the old '{{.ClearCmd}}' command are in the '{{.NoClearCmd}}' file"
echo 'To remove the lock, run "dbdeployer admin unlock {{.SandboxDir}}"'
`
	noOpMockTemplate string = `#!/bin/bash
# The purpose of this script is to run mock tests with a
# command that returns a wanted exit code
exit_code=0
 
# The calling procedure can set FAILMOCK to
# force a failing result.
if [ -n "$FAILMOCK" ]
then
    exit_code=$FAILMOCK
fi
# If MOCKMSG is set, the script will display its contents
if [ -n "$MOCKMSG" ]
then
	echo $MOCKMSG
fi

# If MOCKARGS is set, the script will display its arguments
if [ -n "$MOCKARGS" ]
then
	echo "[$exit_code] $0 $@"
fi
exit $exit_code`

	mysqldSafeMockTemplate string = `#!/bin/bash
# This script mimics the minimal behavior of mysqld_safe
# so that we can run tests for dbdeployer without using the real
# MySQL binaries.
defaults_file=$1
if [ -z "$defaults_file" ]
then
    echo "No defaults file provided: use --defaults-file=filename"
    exit 1
fi
valid_defaults=$(echo $defaults_file | grep '--defaults-file')
if [ -z "$defaults_file" ]
then
    echo "Not a valid defaults-file spec"
    exit 1
fi
defaults_file=$(echo $defaults_file| sed 's/--defaults-file=//')

if [ ! -f "$defaults_file" ]
then
    echo "defaults file $defaults_file not found"
    exit 1
fi

pid_file=$(grep pid-file $defaults_file | awk '{print $3}')

if [ -z "$pid_file" ]
then
    echo "PID file not found in  $defaults_file"
    exit 1
fi

touch $pid_file

exit 0
`

	tidbMockTemplate string = `#!/bin/bash
# This script mimics the minimal behavior of tidb-server
# so that we can run tests for dbdeployer without using the real
# TiDB binaries.
config=$1
if [ -z "$config" ]
then
    echo "No defaults file provided: use -config filename"
    exit 1
fi
valid_config=$(echo $config | grep '\-config')
if [ -z "$valid_config" ]
then
    echo "Not a valid config spec"
    exit 1
fi

config_file=$2

if [ -z "$config_file" ]
then
    echo "No configuration file provided"
    exit 1
fi

if [ ! -f "$config_file" ]
then
    echo "config file $config_file not found"
    exit 1
fi

socket_file=$(grep "socket\s*=" $config_file | awk '{print $3}' | tr -d '"')

if [ -z "$socket_file" ]
then
    echo "socket file not found in  $config_file"
    exit 1
fi

touch $socket_file
sleep 1
exit 0
`

	afterStartTemplate string = `#!/bin/bash
{{.Copyright}}
# Generated by dbdeployer {{.AppVersion}} using {{.TemplateName}} on {{.DateTime}}
source {{.SandboxDir}}/sb_include

# Modify this template to run commands that you want to execute
# after the database is started
exit 0
`

	sbIncludeTemplate string = `
export SBDIR="{{.SandboxDir}}"
export BASEDIR={{.Basedir}}
export CLIENT_BASEDIR={{.ClientBasedir}}
export MYSQL_VERSION={{.Version}}
export MYSQL_VERSION_MAJOR={{.VersionMajor}}
export MYSQL_VERSION_MINOR={{.VersionMinor}}
export MYSQL_VERSION_REV={{.VersionRev}}
export DATADIR=$SBDIR/data
export LD_LIBRARY_PATH=$BASEDIR/lib:$BASEDIR/lib/mysql:$LD_LIBRARY_PATH
export CLIENT_LD_LIBRARY_PATH=$CLIENT_BASEDIR/lib:$CLIENT_BASEDIR/lib/mysql:$LD_LIBRARY_PATH
export DYLD_LIBRARY_PATH=$BASEDIR/lib:$BASEDIR/lib/mysql:$DYLD_LIBRARY_PATH
export CLIENT_DYLD_LIBRARY_PATH=$CLIENT_BASEDIR/lib:$CLIENT_BASEDIR/lib/mysql:$DYLD_LIBRARY_PATH
export PIDFILE=$SBDIR/data/mysql_sandbox{{.Port}}.pid
[ -z "$SLEEP_TIME" ] && export SLEEP_TIME=1

# dbdeployer is not compatible with .mylogin.cnf,
# as it bypasses --defaults-file and --no-defaults.
# See: https://dev.mysql.com/doc/refman/8.0/en/mysql-config-editor.html
# The following statement disables .mylogin.cnf
export MYSQL_TEST_LOGIN_FILE=/tmp/dont_break_my_sandboxes$RANDOM

function is_running
{
    if [ -f $PIDFILE ]
    then
        MYPID=$(cat $PIDFILE)
        ps -p $MYPID | grep $MYPID
    fi
}

function check_output
{
    # Checks if the output is a terminal or a pipe
    if [  -t 1 ]
    then
        echo "###################### WARNING ####################################"
        echo "# You are not using a pager."
        echo "# The output of this script can be quite large."
        echo "# Please pipe this script with a pager, such as 'less' or 'vim -'"
        echo "# Choose one of the following:"
        echo "#     * simply RETURN to continue without a pager"
        echo "#     * 'q' to exit "
        echo "#     * enter the name of the pager to use"
        read answer
        case $answer in
            q)
            exit
            ;;
            *)
            unset pager
            [ -n "$answer" ] && pager=$answer
            ;;
        esac
    fi
}
`

	SingleTemplates = TemplateCollection{
		"Copyright": TemplateDesc{
			Description: "Copyright for every sandbox script",
			Notes:       "",
			Contents:    Copyright,
		},
		"replication_options": TemplateDesc{
			Description: "Replication options for my.cnf",
			Notes:       "",
			Contents:    replicationOptions,
		},
		"semisync_master_options": TemplateDesc{
			Description: "master semi-synch options for my.cnf",
			Notes:       "",
			Contents:    semisyncMasterOptions,
		},
		"semisync_slave_options": TemplateDesc{
			Description: "slave semi-synch options for my.cnf",
			Notes:       "",
			Contents:    semisyncSlaveOptions,
		},
		"gtid_options_56": TemplateDesc{
			Description: "GTID options for my.cnf 5.6.x",
			Notes:       "",
			Contents:    gtidOptions56,
		},
		"gtid_options_57": TemplateDesc{
			Description: "GTID options for my.cnf 5.7.x and 8.0",
			Notes:       "",
			Contents:    gtidOptions57,
		},
		"repl_crash_safe_options": TemplateDesc{
			Description: "Replication crash safe options",
			Notes:       "",
			Contents:    replCrashSafeOptions,
		},
		"expose_dd_tables": TemplateDesc{
			Description: "Commands needed to enable data dictionary table usage",
			Notes:       "",
			Contents:    exposeDdTables,
		},
		"init_db_template": TemplateDesc{
			Description: "Initialization template for the database",
			Notes:       "This should normally run only once",
			Contents:    initDbTemplate,
		},
		"start_template": TemplateDesc{
			Description: "starts the database in a single sandbox (with optional mysqld arguments)",
			Notes:       "",
			Contents:    startTemplate,
		},
		"use_template": TemplateDesc{
			Description: "Invokes the MySQL client with the appropriate options",
			Notes:       "",
			Contents:    useTemplate,
		},
		"mysqlsh_template": TemplateDesc{
			Description: "Invokes the MySQL shell with an appropriate URI",
			Notes:       "",
			Contents:    mysqlshTemplate,
		},
		"stop_template": TemplateDesc{
			Description: "Stops a database in a single sandbox",
			Notes:       "",
			Contents:    stopTemplate,
		},
		"clear_template": TemplateDesc{
			Description: "Remove all data from a single sandbox",
			Notes:       "",
			Contents:    clearTemplate,
		},
		"my_cnf_template": TemplateDesc{
			Description: "Default options file for a sandbox",
			Notes:       "",
			Contents:    myCnfTemplate,
		},
		"status_template": TemplateDesc{
			Description: "Shows the status of a single sandbox",
			Notes:       "",
			Contents:    statusTemplate,
		},
		"restart_template": TemplateDesc{
			Description: "Restarts the database (with optional mysqld arguments)",
			Notes:       "",
			Contents:    restartTemplate,
		},
		"send_kill_template": TemplateDesc{
			Description: "Sends a kill signal to the database",
			Notes:       "",
			Contents:    sendKillTemplate,
		},
		"load_grants_template": TemplateDesc{
			Description: "Loads the grants defined for the sandbox",
			Notes:       "",
			Contents:    loadGrantsTemplate,
		},
		"grants_template5x": TemplateDesc{
			Description: "Grants for sandboxes up to 5.6",
			Notes:       "",
			Contents:    grantsTemplate5x,
		},
		"grants_template57": TemplateDesc{
			Description: "Grants for sandboxes from 5.7+",
			Notes:       "",
			Contents:    grantsTemplate57,
		},
		"grants_template8x": TemplateDesc{
			Description: "Grants for sandboxes from 8.0+",
			Notes:       "",
			Contents:    grantsTemplate8x,
		},
		"my_template": TemplateDesc{
			Description: "Prefix script to run every my* command line tool",
			Notes:       "",
			Contents:    myTemplate,
		},
		"add_option_template": TemplateDesc{
			Description: "Adds options to the my.sandbox.cnf file and restarts",
			Notes:       "",
			Contents:    addOptionTemplate,
		},
		"show_log_template": TemplateDesc{
			Description: "Shows error log or custom log",
			Notes:       "",
			Contents:    showLogTemplate,
		},
		"show_binlog_template": TemplateDesc{
			Description: "Shows a binlog for a single sandbox",
			Notes:       "",
			Contents:    showBinlogTemplate,
		},
		"show_relaylog_template": TemplateDesc{
			Description: "Show the relaylog for a single sandbox",
			Notes:       "",
			Contents:    showRelaylogTemplate,
		},
		"test_sb_template": TemplateDesc{
			Description: "Tests basic sandbox functionality",
			Notes:       "",
			Contents:    testSbTemplate,
		},
		"sb_locked_template": TemplateDesc{
			Description: "locked sandbox script",
			Notes:       "This script is replacing 'clear' when the sandbox is locked",
			Contents:    sbLockedTemplate,
		},
		"after_start_template": TemplateDesc{
			Description: "commands to run after the database started",
			Notes:       "This script does nothing. You can change it and reuse through --use-template",
			Contents:    afterStartTemplate,
		},
		"sb_include_template": TemplateDesc{
			Description: "Common variables and routines for sandboxes scripts",
			Notes:       "",
			Contents:    sbIncludeTemplate,
		},
	}
	MockTemplates = TemplateCollection{
		"no_op_mock_template": TemplateDesc{
			Description: "mock script that does nothing",
			Notes:       "Used for internal tests",
			Contents:    noOpMockTemplate,
		},
		"mysqld_safe_mock_template": TemplateDesc{
			Description: "mock script for mysqld_safe",
			Notes:       "Used for internal tests",
			Contents:    mysqldSafeMockTemplate,
		},
		"tidb_mock_template": TemplateDesc{
			Description: "mock script for tidb-server",
			Notes:       "Used for internal tests",
			Contents:    tidbMockTemplate,
		},
	}

	AllTemplates = AllTemplateCollection{
		"mock":        MockTemplates,
		"single":      SingleTemplates,
		"tidb":        TidbTemplates,
		"multiple":    MultipleTemplates,
		"replication": ReplicationTemplates,
		"group":       GroupTemplates,
	}
)

func init() {
	// The command dbdeployer defaults template show templateName
	// depends on the template names being unique across all collections.
	// This initialisation routine will ensure that there are no duplicates.
	var seen = make(map[string]bool)
	for collName, coll := range AllTemplates {
		for name, _ := range coll {
			_, ok := seen[name]
			if ok {
				// name already exists:
				fmt.Printf("Duplicate template %s found in %s\n", name, collName)
				os.Exit(1)
			}
			seen[name] = true
		}
	}
}
