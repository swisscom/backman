#!/bin/bash

# fail on error
set -e

# =============================================================================================
if [[ "$(basename $PWD)" == "scripts" ]]; then
	cd ..
fi
echo $PWD

# =============================================================================================
source .env

# =============================================================================================
retry() {
    local -r -i max_attempts="$1"; shift
    local -r cmd="$@"
    local -i attempt_num=1

    until $cmd
    do
        if (( attempt_num == max_attempts ))
        then
            echo "Attempt $attempt_num failed and there are no more attempts left!"
            return 1
        else
            echo "Attempt $attempt_num failed! Trying again in $attempt_num seconds..."
            sleep $(( attempt_num++ ))
        fi
    done
}

# =============================================================================================
echo "waiting on mysql ..."
export MYSQL_PWD=my-secret-pw
retry 10 mysql -h 127.0.0.1 -u root -D mysql -e '\q'
echo "mysql is up!"

# =============================================================================================
echo "testing mysql integration ..."

sleep 5
# starting backman
killall backman || true
./backman 2>&1 &
sleep 5

set -x
if [ $(curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:9990) != "401" ]; then
	echo "Should be Unauthorized"
	exit 1
fi

if [ $(curl -s -o /dev/null -w "%{http_code}" http://john:doe@127.0.0.1:9990) != "200" ]; then
	echo "Should be authorized"
	exit 1
fi

if [ $(curl -s -o /dev/null -w "%{http_code}" http://john:doe@127.0.0.1:9990/api/v1/state/mysql/my_mysql_db) != "200" ]; then
	echo "Failed to query state"
	exit 1
fi
curl -s http://john:doe@127.0.0.1:9990/api/v1/state/mysql/my_mysql_db | grep '"Status":"idle"'

# write to mysql
mysql -h 127.0.0.1 -u root -D mysql <<EOF
CREATE TABLE test_example (my_column text);
INSERT INTO test_example (my_column) VALUES ('my_backup_value');
EOF

# trigger new backup
curl -X POST http://john:doe@127.0.0.1:9990/api/v1/backup/mysql/my_mysql_db
curl -s http://john:doe@127.0.0.1:9990/api/v1/state/mysql/my_mysql_db | grep '"Operation":"backup"' | grep '"Status":"running"'
sleep 15
curl -s http://john:doe@127.0.0.1:9990/api/v1/state/mysql/my_mysql_db | grep '"Operation":"backup"' | grep '"Status":"success"'

# read from mysql
mysql -h 127.0.0.1 -u root -D mysql -e 'select my_column from test_example' | grep 'my_backup_value'

# delete from mysql
mysql -h 127.0.0.1 -u root -D mysql -e 'delete from test_example'
mysql -h 127.0.0.1 -u root -D mysql -e "insert into test_example (my_column) values ('backup_different')"
mysql -h 127.0.0.1 -u root -D mysql -e 'select my_column from test_example' | grep -v 'my_backup_value'

# trigger restore
FILENAME=$(curl -s http://john:doe@127.0.0.1:9990/api/v1/backup/mysql/my_mysql_db | jq -r .Files[0].Filename)
curl -X POST http://john:doe@127.0.0.1:9990/api/v1/restore/mysql/my_mysql_db/${FILENAME}
curl -s http://john:doe@127.0.0.1:9990/api/v1/state/mysql/my_mysql_db | grep '"Operation":"restore"' | grep '"Status":"running"'
sleep 15
curl -s http://john:doe@127.0.0.1:9990/api/v1/state/mysql/my_mysql_db | grep '"Operation":"restore"' | grep '"Status":"success"'

# read from mysql
mysql -h 127.0.0.1 -u root -D mysql -e 'select my_column from test_example' | grep -v 'backup_different'
mysql -h 127.0.0.1 -u root -D mysql -e 'select my_column from test_example' | grep 'my_backup_value'

# delete backup
curl -X DELETE http://john:doe@127.0.0.1:9990/api/v1/backup/mysql/my_mysql_db/${FILENAME}
sleep 10
curl -s http://john:doe@127.0.0.1:9990/api/v1/backup/mysql/my_mysql_db | grep -v 'Filename'
