# Configuration

backman can be configured via JSON configuration, either with a file `config.json` in its root directory (or any other location specified with the command parameter `-config <file>`), or by the environment variable `$BACKMAN_CONFIG`.
Values configured in `$BACKMAN_CONFIG` take precedence over `config.json`.
By default backman will assume useful values for all services/backups unless configured otherwise.

> **Note**: Configuration through `config.json` only makes sense when building your own docker image or deploying on Kubernetes, otherwise you cannot modify the file. For Cloud Foundry you will therefore most likely configure `$BACKMAN_CONFIG` in your `manifest.yml` or via `cf set-env`. If you are using the provided docker image `jamesclonk/backman` (as is default in the manifest) then there will be no configuration file present inside the image and all configuration options must to be set via environment variables.
For Kubernetes on the other hand it is very convenient to mount a `config.json` file from a **Secret** into the container.

On Cloud Foundry it is generally recommended to use the `$BACKMAN_CONFIG` environment variable for all your configuration needs. See [Cloud Foundry configuration](/docs/cloudfoundry/configuration.md) for further details. Service bindings are automatically detected via [`$VCAP_SERVICES`](https://docs.cloudfoundry.org/devguide/deploy-apps/environment-variable.html#VCAP-SERVICES) on Cloud Foundry.

On Kubernetes you should use a **Secret** containing the full `config.json` and then mount it as a volume into the container. See [Kubernetes configuration](/docs/kubernetes/configuration.md) for further details. Service bindings are automatically detected via [`$SERVICE_BINDING_ROOT`](https://servicebinding.io/application-developer/) on Kubernetes.

## Examples

These here are the default values backman will use if not configured otherwise:
```json
{
	"log_level": "info",
	"logging_timestamp": false,
	"disable_web": false,
	"disable_metrics": false,
	"disable_restore": false,
	"disable_metrics_logging": false,
	"disable_health_logging": false,
	"disable_all_column_statistics": false,
	"unprotected_metrics": false,
	"unprotected_health": false,
	"notifications": {},
	"s3": {
		"service_type": "s3"
	},
	"services": {}
}
```

A more comprehensive example of possible configuration options could look like this:
```json
{
	"log_level": "info",
	"logging_timestamp": false,
	"disable_web": false,
	"disable_metrics": false,
	"disable_restore": true,
	"disable_metrics_logging": false,
	"disable_health_logging": true,
	"disable_all_column_statistics": true,
	"unprotected_metrics": false,
	"unprotected_health": true,
	"notifications": {
		"teams": {
			"webhook": "https://example.webhook.office.com/webhookb2/deadbeef/IncomingWebhook/beefdead/deadbeef",
			"events": ["backup-success", "backup-failed"]
		}
	},
	"s3": {
		"service_type": "dynstrg",
		"bucket_name": "my-database-backups",
		"encryption_key":"a_super_strong_key",
		"skip_ssl_verification": false
	},
	"services": {
		...
		"<service-instance-name-ABC>": {
			"schedule": "<random-second> <random-minute> <random-hour> * * *",
			"timeout": "1h",
			"retention": {
				"days": 31,
				"files": 100
			}
		},
		"<service-instance-name-XYZ>": {
			"retention": {
				"days": 365,
				"files": 1000
			},
			"disable_column_statistics": true,
			"force_import": true,
			"ignore_tables": [ "confidential_table", "temporary_table" ]
		},
		"<custom-service>": {
			"log_stderr": true,
			"service_binding": {
				"type": "mysql",
				"provider": "mariadb",
				"host": "fancy-db.aws.amazon.com",
				"port": 3306,
				"uri": "mysql://my-db-user:db-pass@fancy-db.aws.amazon.com:3306/dbname-to-backup",
				"username": "my-db-user",
				"Password": "db-pass",
				"database": "dbname-to-backup",
				"ssl": {
					"ca_cert_path": "/path/to/ca-cert.pem",
					"client_cert_path": "/path/to/client-cert.pem",
					"client_key_path": "/path/to/client-key.pem",
					"verify_server_cert": true
				}
			}
		}
		...
	}
}
```

### *HTTP Basic Auth*

backman can be secured through *HTTP Basic Auth*, with a username and password provided either in the JSON configuration
```json
{
	"username": "http_basic_auth_user_abc",
	"password": "http_basic_auth_password_xyz"
}
```
or through the specific environment variables `$BACKMAN_USERNAME` and `$BACKMAN_PASSWORD` (see `manifest.yml`)

## JSON properties
- `log_level`: optional, specifies log output level, can be *info*, *warn*, *debug*, *error*
- `logging_timestamp`: optional, enable timestamping log output, not needed when deployed on Cloud Foundry
- `username`: optional, *HTTP Basic Auth* username
- `password`: optional, *HTTP Basic Auth* password
- `disable_web`: optional, disable web interface and api
- `disable_metrics`: optional, disable Prometheus metrics endpoint
- `disable_restore`: optional, disable restore function on API and web. It can be used to mitigate damage in case backman credentials are leaked. Enable it just before you might need to restore.
- `disable_metrics_logging`: optional, disables logging of HTTP requests towards /metrics endpoint
- `disable_health_logging`: optional, disables logging of HTTP requests towards /healthz endpoint
- `disable_all_column_statistics`: optional / MySQL-specific, forces disabling export of column statistics for all MySQL backups (overrides `services.<service-instance>.disable_column_statistics`)
- `unprotected_metrics`: optional, disable *HTTP Basic Auth* protection for Prometheus /metrics endpoint
- `unprotected_health`: optional, disable *HTTP Basic Auth* protection for /healthz endpoint
- `notifications.teams.webhook`: optional, setting a webhook URL will enable MS Teams notifications about backups
- `notifications.teams.events`: optional, list of events to send a Teams notification for. Can be *backup-started*, *backup-success*, *backup-failed*. Sends a notification for all events if empty.
- `s3.host`: optional, S3 hostname/endpoint to use, backman will try to find it within service instances if not specified
- `s3.access_key`: optional, S3 access-key credential, backman will try to find it within service instances if not specified
- `s3.secret_key`: optional, S3 secret-key credential, backman will try to find it within service instances if not specified
- `s3.disable_ssl`: optional, S3 client connections will use HTTP instead of HTTPS
- `s3.skip_ssl_verification`: optional, S3 client will still use HTTPS but skips certificate verification
- `s3.service_type`: optional, defines which service type or label backman will look for in service instances to find the S3-compatible object storage
- `s3.bucket_name`: optional, bucket to use on S3 storage, backman will use service-instance/binding-name if not configured
- `s3.encryption_key`: optional, defines the key which will be used to encrypt and decrypt backups as they are stored on the S3 can also be passed as an environment variable with the name `$BACKMAN_ENCRYPTION_KEY`
- `ssl.ca_cert_path`: optional, the path to the CA certificate (MySQL, PostgreSQL and MongoDB only)
- `ssl.client_cert_path`: optional, the path to the client certificate (MySQL and PostgreSQL only)
- `ssl.client_key_path`: optional, the path to the client key (MySQL and PostgreSQL only)
- `ssl.pem_key_password`: optional, the password for the PEM Key (MongoDB only)
- `ssl.pem_key_path`: optional, the path to the PEM Key (MongoDB only)
- `ssl.verify_server_cert`: optional, whether to verify the server certificate (MySQL and PostgreSQL only)

> **Note**: Usage of `s3.encryption_key` is not backward compatible! Backups generated without or with a different encryption key than before cannot be downloaded or restored anymore.

- `services.<service-instance>.schedule`: optional, defines cron schedule for running backups
- `services.<service-instance>.timeout`: optional, backman will abort a running backup/restore if timeout is exceeded
- `services.<service-instance>.retention.days`: optional, specifies how long backman will keep backups on S3 at maximum for this service instance
- `services.<service-instance>.retention.files`: optional, specifies how maximum number of files backman will keep on S3 for this service instance
- `services.<service-instance>.direct_s3`: optional / Elasticsearch-specific, bypasses backman internal backup stream and encryption entirely, streaming directly from/to S3 via elasticdump
- `services.<service-instance>.disable_column_statistics`: optional / MySQL-specific, allows for disabling export of column statistics. Set to `true` to avoid issues with pre-8.0 versions of MySQL (ignored if `disable_all_column_statistics` is `true`)
- `services.<service-instance>.force_import`: optional / MySQL-specific. Set to `true` to use the `--force` flag for mysql, ignoring any errors that might occur while importing backups
- `services.<service-instance>.read_preference`: optional / MongoDB-specific. Configures mongodump read preference mode. Defaults to "secondary" if not specified
- `services.<service-instance>.log_stderr`: optional. Outputs stderr of backup process to stdout in case of errors or timeouts
- `services.<service-instance>.local_backup_path`: optional / PostgreSQL-specific, path where to store backup files locally first before uploading them. Otherwise streams directly to S3 if not specified
- `services.<service-instance>.ignore_tables`: optional / MySQL-specific, array of table names to be ignored for the backup
- `services.<service-instance>.backup_options`: optional, allows specifying additional parameters and flags for service backup executable
- `services.<service-instance>.restore_options`: optional, allows specifying additional parameters and flags for service restore executable

### Explicit service bindings / credentials

It is also possible to configure service bindings and their credentials directly inside the configuration instead of having backman read and parse them from `$VCAP_SERVICES` or `$SERVICE_BINDING_ROOT/<service>` respectively. Read the [servicebinding spec](https://github.com/servicebinding/spec#well-known-secret-entries) for further information on these properties:
- `services.<service-instance>.service_binding.type`: specify service type, supported values are *elasticsearch*, *mysql*, *postgres*, *mongodb*, *redis*
- `services.<service-instance>.service_binding.provider`: optional, specify service provider
- `services.<service-instance>.service_binding.host`: specify service hostname
- `services.<service-instance>.service_binding.port`: specify service port
- `services.<service-instance>.service_binding.uri`: specify service URI
- `services.<service-instance>.service_binding.username`: specify service username credential
- `services.<service-instance>.service_binding.password`: specify service password credential
- `services.<service-instance>.service_binding.database`: optional, specify service database to backup

Here's an example with S3 (for backup storage) and 2 services with explicitely configured service bindings:
```json
{
	"logging_timestamp": true,
	"s3": {
		"bucket_name": "my-bucket-for-backups",
		"host": "s3.amazonaws.com",
		"access_key": "24a5ca67-6fd0-4f5a",
		"secret_key": "0eeb6338-be51-4838-9231-f421f4022ea0"
	},
	"services": {
		"production-db": {
			"schedule": "0 0 6 * * *",
			"timeout": "4h",
			"retention": {
				"days": 7,
				"files": 21
			},
			"service_binding": {
				"type": "postgres",
				"provider": "AWS RDS",
				"host": "postgres-c973ba.rds.amazonaws.com",
				"username": "admin",
				"password": "d44b5e36-7c3f-433a-b244-3e8d2a8e2e22",
				"port": 5432,
				"database": "productdb",
				"ssl": {
					"ca_cert_path": "/path/to/ca-cert.pem",
					"client_cert_path": "/path/to/client-cert.pem",
					"client_key_path": "/path/to/client-key.pem",
					"verify_server_cert": true
				}
			}
		},
		"cookie-cache": {
			"schedule": "0 0 0/4 * * *",
			"service_binding": {
				"type": "redis",
				"plan": "small",
				"host": "redis-instance-1.redis.amazonaws.com",
				"password": "fa786039-993b-4951-87a5-bf0adab8b848",
				"port": 6379
			}
		}
	}
}
```

> **Note**: Usually it should not be necessary to configure any credentials or bindings directly as shown above, since backman will autodetect your services and their bindings via `$VCAP_SERVICES` (Cloud Foundry) or `$SERVICE_BINDING_ROOT` (Kubernetes).
