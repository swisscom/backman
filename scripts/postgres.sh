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
echo "waiting on postgres ..."
export PGPASSWORD=dev-secret
retry 10 psql -h 127.0.0.1 -U dev-user -d my_postgres_db -c '\q'
echo "postgres is up!"

# =============================================================================================
echo "testing postgres integration ..."

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

if [ $(curl -s -o /dev/null -w "%{http_code}" http://john:doe@127.0.0.1:9990/api/v1/state/postgres/my_postgres_db) != "200" ]; then
	echo "Failed to query state"
	exit 1
fi
curl -s http://john:doe@127.0.0.1:9990/api/v1/state/postgres/my_postgres_db | grep '"Status":"idle"'

# write to postgres
psql -h 127.0.0.1 -U dev-user -d my_postgres_db <<EOF
CREATE TABLE test_example (my_column text);
INSERT INTO test_example (my_column) VALUES ('my_backup_value');
EOF

# trigger new backup
curl -X POST http://john:doe@127.0.0.1:9990/api/v1/backup/postgres/my_postgres_db
curl -s http://john:doe@127.0.0.1:9990/api/v1/state/postgres/my_postgres_db | grep '"Operation":"backup"' | grep '"Status":"running"'
sleep 15
curl -s http://john:doe@127.0.0.1:9990/api/v1/state/postgres/my_postgres_db | grep '"Operation":"backup"' | grep '"Status":"success"'

# read from postgres
psql -h 127.0.0.1 -U dev-user -d my_postgres_db -c 'select my_column from test_example' | grep 'my_backup_value'

# delete from postgres
psql -h 127.0.0.1 -U dev-user -d my_postgres_db -c 'delete from test_example'
psql -h 127.0.0.1 -U dev-user -d my_postgres_db -c 'select my_column from test_example' | grep -v 'my_backup_value'

# trigger restore
FILENAME=$(curl -s http://john:doe@127.0.0.1:9990/api/v1/backup/postgres/my_postgres_db | jq -r .Files[0].Filename)
curl -X POST http://john:doe@127.0.0.1:9990/api/v1/restore/postgres/my_postgres_db/${FILENAME}
curl -s http://john:doe@127.0.0.1:9990/api/v1/state/postgres/my_postgres_db | grep '"Operation":"restore"' | grep '"Status":"running"'
sleep 15
curl -s http://john:doe@127.0.0.1:9990/api/v1/state/postgres/my_postgres_db | grep '"Operation":"restore"' | grep '"Status":"success"'

# read from postgres
psql -h 127.0.0.1 -U dev-user -d my_postgres_db -c 'select my_column from test_example' | grep 'my_backup_value'

# delete backup
curl -X DELETE http://john:doe@127.0.0.1:9990/api/v1/backup/postgres/my_postgres_db/${FILENAME}
sleep 10
curl -s http://john:doe@127.0.0.1:9990/api/v1/backup/postgres/my_postgres_db | grep -v 'Filename'
