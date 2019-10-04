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
echo "waiting on postgres ..."
until PGPASSWORD=dev-secret psql -h 127.0.0.1 -U dev-user -d my_postgres_db -c '\q'; do
  echo "waiting ..."
  sleep 2
done
echo "postgres is up!"

# =============================================================================================
echo "testing postgres integration ..."

sleep 5
go run main.go
sleep 10

curl http://john:doe@127.0.0.1:9990/api/v1/state/postgres/my_postgres_db
