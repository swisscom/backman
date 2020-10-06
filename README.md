# :minidisc: backman

[![CircleCI](https://circleci.com/gh/swisscom/backman.svg?style=svg)](https://circleci.com/gh/swisscom/backman)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](https://github.com/swisscom/backman/blob/master/LICENSE)
[![Platform](https://img.shields.io/badge/platform-Cloud%20Foundry-lightgrey)](https://developer.swisscom.com/)

a backup-manager app for [Cloud Foundry](https://www.cloudfoundry.org/)

## Supported databases

- MariaDB / MySQL
- PostgreSQL
- MongoDB
- Elasticsearch
- Redis

## Usage

1. pick a Cloud Foundry provider.
   I'd suggest the [Swisscom AppCloud](https://developer.swisscom.com/)
2. create a service instance of an S3-compatible object storage
3. modify the provided `manifest.yml`, specify your service instance(s)
4. configure backman, either through the provided `config.json` or by the environment variable `BACKMAN_CONFIG` (see `manifest.yml`)
5. deploy the app
6. enjoy!

#### Using Cloud Foundry tasks

backman also supports running as a one-off task inside Cloud Foundry. Simply push the app as normal, stop it, and then run it via `cf run-task` with `/app/backman -backup <service_name>` as task command to run a backup. For restoring an existing backup you can use `/app/backman -restore <service_name> -filename <backup_filename>`. (or just `backman ...` if the app was pushed with native buildpacks and not as a docker image)

## Configuration

backman can be configured via JSON configuration, either with a file `config.json` in it's root directory, or by the environment variable `BACKMAN_CONFIG`.
Values configured in `BACKMAN_CONFIG` take precedence over `config.json`.
By default backman will assume useful values for all services/backups unless configured otherwise.

**Note:** Configuration via the `config.json` only makes sense when either pushing with buildpacks to CF, or by building your own docker image.
If you are using the provided docker image `jamesclonk/backman` (as is default in the manifest) then there will be no configuration file and all configuration options need to be set via environment variables.

It is generally recommended to use the `BACKMAN_CONFIG` environment variable for all your configuration needs.

These here are the default values backman will use if not configured via JSON:
```json
{
	"log_level": "info",
	"logging_timestamp": false,
	"disable_web": false,
  "disable_metrics": false,
  "enable_apm": false,
	"s3": {
		"service_label": "dynstrg",
	},
	"services": {
		...
		"<service-instance-name>": {
			"schedule": "<random-second> <random-minute> <random-hour> * * *",
			"timeout": "1h",
			"retention": {
				"days": 31,
				"files": 100
			}
		}
		...
	}
}
```

backman can be secured through HTTP basic auth, with username and password provided either in the JSON configuration
```json
{
	"username": "http_basic_auth_user_abc",
	"password": "http_basic_auth_password_xyz"
}
```
or through the specific environment variables `BACKMAN_USERNAME` and `BACKMAN_PASSWORD` (see `manifest.yml`)

Possible JSON properties:
- `log_level`: optional, specifies log output level, can be *info*, *warn*, *debug*, *error*
- `logging_timestamp`: optional, enable timestamping log output, not needed when deployed on Cloud Foundry
- `username`: optional, HTTP basic auth username
- `password`: optional, HTTP basic auth password
- `disable_web`: optional, disable web interface and api
- `disable_metrics`: optional, disable Prometheus metrics endpoint
- `enable_apm`: optional, enable apm logging, needs to be [configured accordingly](https://www.elastic.co/guide/en/apm/agent/go/current/getting-started.html)
- `s3.service_label`: optional, defines which service label backman will look for to find the S3-compatible object storage
- `s3.bucket_name`: optional, bucket to use on S3 storage, backman will use service-instance/binding-name if not configured
- `services.<service-instance>.schedule`: optional, defines cron schedule for running backups
- `services.<service-instance>.timeout`: optional, backman will abort a running backup/restore if timeout is exceeded
- `services.<service-instance>.retention.days`: optional, specifies how long backman will keep backups on S3 at maximum for this service instance
- `services.<service-instance>.retention.files`: optional, specifies how maximum number of files backman will keep on S3 for this service instance

## Metrics

backman exposes a couple of metrics via [Prometheus](https://prometheus.io/docs/introduction/overview/) endpoint `/metrics`.

Example:
```
$ curl localhost:9990/metrics

# HELP backman_backup_files_total Number of backup files in total per service.
# TYPE backman_backup_files_total gauge
backman_backup_files_total{name="my-elasticsearch",type="elasticsearch"} 7
backman_backup_files_total{name="my_mongodb",type="mongodb"} 1
backman_backup_files_total{name="my_postgres_db",type="postgres"} 25
# HELP backman_backup_filesize_last Filesize of last / most recent backup file per service.
# TYPE backman_backup_filesize_last gauge
backman_backup_filesize_last{name="my-elasticsearch",type="elasticsearch"} 58404
backman_backup_filesize_last{name="my_mongodb",type="mongodb"} 1067
backman_backup_filesize_last{name="my_postgres_db",type="postgres"} 684
# HELP backman_backup_filesize_total Total filesize sum of all backup files per service.
# TYPE backman_backup_filesize_total gauge
backman_backup_filesize_total{name="my-elasticsearch",type="elasticsearch"} 408740
backman_backup_filesize_total{name="my_mongodb",type="mongodb"} 1067
backman_backup_filesize_total{name="my_postgres_db",type="postgres"} 7404
# HELP backman_backup_failures_total Total number of backup failures per service.
# TYPE backman_backup_failures_total counter
backman_backup_failures_total{name="my-elasticsearch",type="Elasticsearch"} 3
backman_backup_failures_total{name="my_mongodb",type="MongoDB"} 1
backman_backup_failures_total{name="my_postgres_db",type="PostgreSQL"} 3
# HELP backman_backup_success_total Total number of backup failures per service.
# TYPE backman_backup_success_total counter
backman_backup_success_total{name="my-elasticsearch",type="Elasticsearch"} 18
backman_backup_success_total{name="my_mongodb",type="MongoDB"} 4
backman_backup_success_total{name="my_postgres_db",type="PostgreSQL"} 4
# HELP backman_backup_queued Backups currently in queue per service.
# TYPE backman_backup_queued gauge
backman_backup_queued{name="my-elasticsearch",type="elasticsearch"} 0
backman_backup_queued{name="my_mongodb",type="mongodb"} 0
backman_backup_queued{name="my_postgres_db",type="postgres"} 0
# HELP backman_backup_running Current running state of backups triggered per service.
# TYPE backman_backup_running gauge
backman_backup_running{name="my-elasticsearch",type="elasticsearch"} 0
backman_backup_running{name="my_mongodb",type="mongodb"} 0
backman_backup_running{name="my_postgres_db",type="postgres"} 0
# HELP backman_backup_total Total number of backups triggered per service.
# TYPE backman_backup_total counter
backman_backup_total{name="my-elasticsearch",type="Elasticsearch"} 21
backman_backup_total{name="my_mongodb",type="MongoDB"} 5
backman_backup_total{name="my_postgres_db",type="PostgreSQL"} 7
# HELP backman_restore_failures_total Total number of restore failures per service.
# TYPE backman_restore_failures_total counter
backman_restore_failures_total{name="my-elasticsearch",type="Elasticsearch"} 2
# HELP backman_restore_success_total Total number of successful restores per service.
# TYPE backman_restore_success_total counter
backman_restore_success_total{name="my-elasticsearch",type="Elasticsearch"} 1
backman_restore_success_total{name="my_mongodb",type="MongoDB"} 2
# HELP backman_restore_queued Restores currently in queue per service.
# TYPE backman_restore_queued gauge
backman_restore_queued{name="my-elasticsearch",type="elasticsearch"} 0
backman_restore_queued{name="my_mongodb",type="mongodb"} 0
backman_restore_queued{name="my_postgres_db",type="postgres"} 0
# HELP backman_restore_running Current running state of restores triggered per service.
# TYPE backman_restore_running gauge
backman_restore_running{name="my-elasticsearch",type="elasticsearch"} 1
backman_restore_running{name="my_mongodb",type="mongodb"} 0
backman_restore_running{name="my_postgres_db",type="postgres"} 0
# HELP backman_restore_total Total number of restores triggered per service.
# TYPE backman_restore_total counter
backman_restore_total{name="my-elasticsearch",type="Elasticsearch"} 3
backman_restore_total{name="my_mongodb",type="MongoDB"} 2
# HELP backman_scheduler_backup_failures_total Total number of backup failures over crontab-schedule.
# TYPE backman_scheduler_backup_failures_total counter
backman_scheduler_backup_failures_total 0
# HELP backman_scheduler_backup_success_total Total number of successful backups over crontab-schedule.
# TYPE backman_scheduler_backup_success_total counter
backman_scheduler_backup_success_total 4
# HELP backman_scheduler_runs_total Total number of backup runs triggered over crontab-schedule.
# TYPE backman_scheduler_runs_total counter
backman_scheduler_runs_total 4
```

## API

backman has an API which can be used to trigger backups & restores.
Have a look at the [Swagger documentation](https://petstore.swagger.io/?url=https://raw.githubusercontent.com/swisscom/backman/master/swagger.yml)

## Screenshots

* shows all bound service instances

![backman services](https://raw.githubusercontent.com/swisscom/backman/master/static/images/backman_services_listing.png "backman services")

* display service, trigger backups/restores

![backman service](https://raw.githubusercontent.com/swisscom/backman/master/static/images/backman_service_view.png "backman service")
