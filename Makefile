.PHONY: run gin build test swagger elasticsearch elasticsearch-network elasticsearch-stop elasticsearch-start elasticsearch-data minio minio-stop minio-start mysql mysql-network mysql-stop mysql-start mysql-client mysql-test postgres postgres-network postgres-stop postgres-start postgres-client postgres-test mongodb mongodb-network mongodb-stop mongodb-start mongodb-client cleanup
SHELL := /bin/bash

all: run

run:
	go run main.go

gin:
	gin --all --immediate run main.go

build:
	rm -f backman
	go build -o backman

install:
	go install

test:
	source .env && GOARCH=amd64 GOOS=linux go test -v ./...

swagger:
	swagger generate spec -o ./swagger.yml
	swagger serve -F=swagger swagger.yml

elasticsearch: elasticsearch-network elasticsearch-stop elasticsearch-start elasticsearch-data
	docker logs elasticsearch -f

elasticsearch-network:
	docker network create elasticsearch-network --driver bridge || true

elasticsearch-stop:
	docker rm -f elasticsearch || true

elasticsearch-start:
	docker run -d -p 9200:9200 -p 9300:9300 \
		-e "discovery.type=single-node" \
		--network elasticsearch-network \
		--name elasticsearch elasticsearch:6.8.2

elasticsearch-data:
	curl -O https://download.elastic.co/demos/kibana/gettingstarted/7.x/accounts.zip
	unzip accounts.zip
	sleep 20
	curl -H 'Content-Type: application/x-ndjson' -XPOST 'localhost:9200/bank/account/_bulk?pretty' --data-binary @accounts.json

minio: minio-stop minio-start
	docker logs minio -f

minio-stop:
	docker rm -f minio || true

minio-start:
	docker run -d -p 9000:9000 --name minio \
		-e "MINIO_ACCESS_KEY=deadbeef-beefdead" \
		-e "MINIO_SECRET_KEY=beefbeefbeefbeef" \
		minio/minio server /data

mysql: mysql-network mysql-stop mysql-start
	docker logs mysql -f

mysql-network:
	docker network create mysql-network --driver bridge || true

mysql-stop:
	docker rm -f mysql || true

mysql-start:
	docker run -d -p 3306:3306 \
		-e MYSQL_ROOT_PASSWORD='my-secret-pw' \
		--network mysql-network \
		--name mysql mysql:5.7

mysql-client:
	docker run -it --rm \
		-e MYSQL_PWD='my-secret-pw' \
		--network mysql-network \
		--name mysql-client mysql mysql -hmysql -uroot

mysql-test: build
	scripts/mysql.sh

postgres: postgres-network postgres-stop postgres-start
	docker logs postgres -f

postgres-network:
	docker network create postgres-network --driver bridge || true

postgres-stop:
	docker rm -f postgres || true

postgres-start:
	docker run --name postgres \
	    --network postgres-network \
		-e POSTGRES_USER='dev-user' \
		-e POSTGRES_PASSWORD='dev-secret' \
		-e POSTGRES_DB='my_postgres_db' \
		-p 5432:5432 \
		-d postgres:9-alpine

postgres-client:
	docker exec -it \
		-e PGPASSWORD='dev-secret' \
		postgres psql -U 'dev-user' -d 'my_postgres_db'

postgres-test: build
	scripts/postgres.sh

mongodb: mongodb-network mongodb-stop mongodb-start
	docker logs mongodb -f

mongodb-network:
	docker network create mongodb-network --driver bridge || true

mongodb-stop:
	docker rm -f mongodb || true

mongodb-start:
	docker run --name mongodb \
		--network mongodb-network \
		-e MONGO_INITDB_ROOT_USERNAME='mongoadmin' \
		-e MONGO_INITDB_ROOT_PASSWORD='super-secret' \
		-p 27017:27017 \
		-d mongo:3.6

mongodb-client:
	docker run -it --rm \
		--network mongodb-network \
		--name mongodb-client mongo:3.6 mongo \
			--host mongodb \
        	-u 'mongoadmin' \
        	-p 'super-secret' \
        	--authenticationDatabase admin

cleanup:
	docker system prune --volumes -a
