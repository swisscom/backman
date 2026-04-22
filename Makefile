.DEFAULT_GOAL := run
SHELL := /bin/bash
APP = backman
COMMIT_SHA = $(shell git rev-parse --short HEAD)
DOCKER_TAG = latest

.PHONY: help
## help: prints this help message
help:
	@echo "Usage:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

.PHONY: run
## run: runs main.go with the race detector
run:
	source _fixtures/env; source _fixtures/env_private; go run -race main.go

.PHONY: gin
## gin: runs main.go via gin (hot reloading)
gin:
	source _fixtures/env; source _fixtures/env_private; gin --all --immediate run main.go

.PHONY: build
## build: builds the application
build: clean
	@echo "Building binary ..."
	go build -o ${APP}

.PHONY: clean
## clean: cleans up binary files
clean:
	@echo "Cleaning up ..."
	@go clean

.PHONY: install
## install: installs the application
install:
	@echo "Installing binary ..."
	go install

.PHONY: test
## test: runs go test with the race detector
test: build
	@source _fixtures/env; go test -v -race ./...

.PHONY: init
## init: sets up go modules
init:
	@echo "Setting up modules ..."
	@go mod init 2>/dev/null; go mod tidy && go mod vendor

.PHONY: docker-build
## docker-build: builds docker image
docker-build: clean
	docker pull ubuntu:22.04
	docker build -t jamesclonk/${APP}:${COMMIT_SHA} $$PWD

.PHONY: docker-push
## docker-push: pushes docker image to dockerhub
docker-push: docker-build
	docker tag jamesclonk/${APP}:${COMMIT_SHA} jamesclonk/${APP}:${DOCKER_TAG}
	docker push jamesclonk/${APP}:${DOCKER_TAG}

.PHONY: docker-run
## docker-run: runs docker image locally
docker-run: docker-clean
	docker run -p 9990:8080 \
		--env-file _fixtures/dockerenv \
		--name ${APP} \
		jamesclonk/${APP}

.PHONY: docker-clean
## docker-clean: cleans up local docker image
docker-clean:
	docker rm -f ${APP} || true

.PHONY: docker-exec
## docker-exec: hijacks running docker container
docker-exec:
	@docker exec -it ${APP} /bin/bash

docker-check-version:
ifndef IMAGE_VERSION
	$(error IMAGE_VERSION not set)
endif

docker-set-tag:
DOCKER_TAG = $${IMAGE_VERSION}

.PHONY: docker-release
## docker-release: builds and releases docker image to dockerhub with given IMAGE_VERSION
docker-release: docker-check-version docker-set-tag docker-push

.PHONY: cleanup
cleanup: docker-cleanup
.PHONY: docker-cleanup
## docker-cleanup: cleans up local docker images and volumes
docker-cleanup:
	docker system prune --volumes -a

.PHONY: swagger
## swagger: generates swagger documentation
swagger:
	swagger generate spec -m -o ./swagger.yml
	swagger serve -F=swagger swagger.yml

.PHONY: elasticsearch
## elasticsearch: runs elasticsearch backend on docker
elasticsearch: elasticsearch-network elasticsearch-stop elasticsearch-start elasticsearch-data
	docker logs elasticsearch -f

.PHONY: elasticsearch-network
elasticsearch-network:
	docker network create elasticsearch-network --driver bridge || true

.PHONY: elasticsearch-cleanup
## elasticsearch-cleanup: cleans up elasticsearch backend
elasticsearch-cleanup: elasticsearch-stop
.PHONY: elasticsearch-stop
elasticsearch-stop:
	docker rm -f elasticsearch || true

.PHONY: elasticsearch-start
elasticsearch-start:
	docker run -d -p 9200:9200 -p 9300:9300 \
		-e "discovery.type=single-node" \
		--network elasticsearch-network \
		--name elasticsearch elasticsearch:6.8.2

.PHONY: elasticsearch-data
elasticsearch-data:
	curl -O https://download.elastic.co/demos/kibana/gettingstarted/7.x/accounts.zip
	unzip accounts.zip
	sleep 20
	curl -H 'Content-Type: application/x-ndjson' -XPOST 'localhost:9200/bank/account/_bulk?pretty' --data-binary @accounts.json

.PHONY: minio
## minio: runs minio backend on docker
minio: minio-stop minio-start
	docker logs minio -f

.PHONY: minio-cleanup
## minio-cleanup: cleans up minio backend
minio-cleanup: minio-stop
.PHONY: minio-stop
minio-stop:
	docker rm -f minio || true

.PHONY: minio-start
minio-start:
	docker run -d -p 9000:9000 --name minio \
		-e "MINIO_ACCESS_KEY=6d611e2d-330b-4e52-a27c-59064d6e8a62" \
		-e "MINIO_SECRET_KEY=eW9sbywgeW91IGhhdmUganVzdCBiZWVuIHRyb2xsZWQh" \
		minio/minio server /data

.PHONY: mysql
## mysql: runs mysql backend on docker
mysql: mysql-network mysql-stop mysql-start
	docker logs mysql -f

.PHONY: mysql-network
mysql-network:
	docker network create mysql-network --driver bridge || true

.PHONY: mysql-cleanup
## mysql-cleanup: cleans up mysql backend
mysql-cleanup: mysql-stop
.PHONY: mysql-stop
mysql-stop:
	docker rm -f mysql || true

.PHONY: mysql-start
mysql-start:
	docker run -d -p 3306:3306 \
		-e MYSQL_ROOT_PASSWORD='my-secret-pw' \
		--network mysql-network \
		--name mysql mysql:5.7

.PHONY: mysql-client
## mysql-client: connects to mysql backend with CLI
mysql-client:
	docker run -it --rm \
		-e MYSQL_PWD='my-secret-pw' \
		--network mysql-network \
		--name mysql-client mysql mysql -hmysql -uroot

.PHONY: mysql-test
## mysql-test: runs mysql integration tests
mysql-test: build
	scripts/mysql.sh

.PHONY: postgres
## postgres: runs postgres backend on docker
postgres: postgres-network postgres-stop postgres-start
	docker logs postgres -f

.PHONY: postgres-network
postgres-network:
	docker network create postgres-network --driver bridge || true

.PHONY: postgres-cleanup
## postgres-cleanup: cleans up postgres backend
postgres-cleanup: postgres-stop
.PHONY: postgres-stop
postgres-stop:
	docker rm -f postgres || true

.PHONY: postgres-start
postgres-start:
	docker run --name postgres \
		--network postgres-network \
		-e POSTGRES_USER='dev-user' \
		-e POSTGRES_PASSWORD='dev-secret' \
		-e POSTGRES_DB='my_postgres_db' \
		-p 5432:5432 \
		-d postgres:9-alpine

.PHONY: postgres-client
## postgres-client: connects to postgres backend with CLI
postgres-client:
	docker exec -it \
		-e PGPASSWORD='dev-secret' \
		postgres psql -U 'dev-user' -d 'my_postgres_db'

.PHONY: postgres-test
## postgres-test: runs postgres integration tests
postgres-test: build
	scripts/postgres.sh

.PHONY: mongodb
## mongodb: runs mongodb backend on docker
mongodb: mongodb-network mongodb-stop mongodb-start
	docker logs mongodb -f

.PHONY: mongodb-network
mongodb-network:
	docker network create mongodb-network --driver bridge || true

.PHONY: mongodb-cleanup
## mongodb-cleanup: cleans up mongodb backend
mongodb-cleanup: mongodb-stop
.PHONY: mongodb-stop
mongodb-stop:
	docker rm -f mongodb || true

.PHONY: mongodb-start
mongodb-start:
	docker run --name mongodb \
		--network mongodb-network \
		-e MONGO_INITDB_ROOT_USERNAME='mongoadmin' \
		-e MONGO_INITDB_ROOT_PASSWORD='super-secret' \
		-p 27017:27017 \
		-d mongo:3.6

.PHONY: mongodb-client
## mongodb-client: connects to mongodb backend with CLI
mongodb-client:
	docker run -it --rm \
		--network mongodb-network \
		--name mongodb-client mongo:3.6 mongo \
		--host mongodb \
		-u 'mongoadmin' \
		-p 'super-secret' \
		--authenticationDatabase admin

.PHONY: mongodb-test
## mongodb-test: runs mongodb integration tests
mongodb-test: build
	scripts/mongodb.sh

.PHONY: redis
## redis: runs redis backend on docker
redis: redis-network redis-stop redis-start
	docker logs redis -f

.PHONY: redis-network
redis-network:
	docker network create redis-network --driver bridge || true

.PHONY: redis-cleanup
## redis-cleanup: cleans up redis backend
redis-cleanup: redis-stop
.PHONY: redis-stop
redis-stop:
	docker rm -f redis || true

.PHONY: redis-start
redis-start:
	docker run --name redis \
		--network redis-network \
		-p 6379:6379 \
		-d redis \
		redis-server --requirepass 'very-secret'

.PHONY: redis-client
## redis-client: connects to redis backend with CLI
redis-client:
	docker run -it --rm \
		--network redis-network \
		--name redis-cli redis redis-cli \
		-h redis -a 'very-secret'

.PHONY: redis-test
## redis-test: runs redis integration tests
redis-test: build
	scripts/redis.sh
