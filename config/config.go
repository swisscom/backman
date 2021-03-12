package config

import (
	"encoding/json"
	"errors"
	"github.com/minio/minio-go/v6/pkg/encrypt"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"
)

var (
	config Config
	once   sync.Once
)

type Config struct {
	LogLevel           string `json:"log_level"`
	LoggingTimestamp   bool   `json:"logging_timestamp"`
	Username           string
	Password           string
	DisableWeb         bool `json:"disable_web"`
	DisableMetrics     bool `json:"disable_metrics"`
	UnprotectedMetrics bool `json:"unprotected_metrics"`
	S3                 S3Config
	Services           map[string]ServiceConfig
	Foreground         bool
}

type S3Config struct {
	DisableSSL    bool   `json:"disable_ssl"`
	ServiceLabel  string `json:"service_label"`
	ServiceName   string `json:"service_name"`
	BucketName    string `json:"bucket_name"`
	EncryptionKey string `json:"encryption_key"`
	ServerSideEncryption string `json:"server_side_encryption"`
}

type ServiceConfig struct {
	Schedule  string
	Timeout   TimeoutDuration
	Retention struct {
		Days  int
		Files int
	}
	DisableColumnStatistics bool   `json:"disable_column_statistics"`
	ForceImport             bool   `json:"force_import"`
	LocalBackupPath         string `json:"local_backup_path"`
}

type TimeoutDuration struct {
	time.Duration
}

func (td TimeoutDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(td.String())
}

func (td *TimeoutDuration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		td.Duration = time.Duration(value)
		return nil
	case string:
		var err error
		td.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
		return nil
	default:
		return errors.New("invalid duration")
	}
}

func Get() *Config {
	once.Do(func() {
		// initialize
		config = Config{
			Services: make(map[string]ServiceConfig),
		}

		// first load config file, if it exists
		if _, err := os.Stat("config.json"); err == nil {
			data, err := ioutil.ReadFile("config.json")
			if err != nil {
				log.Println("could not load 'config.json'")
				log.Fatalln(err.Error())
			}
			if err := json.Unmarshal(data, &config); err != nil {
				log.Println("could not parse 'config.json'")
				log.Fatalln(err.Error())
			}
		}

		// now load & overwrite with env provided config, if it exists
		env := os.Getenv("BACKMAN_CONFIG")
		if len(env) > 0 {
			envConfig := Config{}
			if err := json.Unmarshal([]byte(env), &envConfig); err != nil {
				log.Println("could not parse environment variable 'BACKMAN_CONFIG'")
				log.Fatalln(err.Error())
			}

			// merge config values
			if len(envConfig.LogLevel) > 0 {
				config.LogLevel = envConfig.LogLevel
			}
			if envConfig.LoggingTimestamp {
				config.LoggingTimestamp = envConfig.LoggingTimestamp
			}
			if len(envConfig.Username) > 0 {
				config.Username = envConfig.Username
			}
			if len(envConfig.Password) > 0 {
				config.Password = envConfig.Password
			}
			if envConfig.DisableWeb {
				config.DisableWeb = envConfig.DisableWeb
			}
			if envConfig.DisableMetrics {
				config.DisableMetrics = envConfig.DisableMetrics
			}
			if envConfig.UnprotectedMetrics {
				config.UnprotectedMetrics = envConfig.UnprotectedMetrics
			}
			if envConfig.S3.DisableSSL {
				config.S3.DisableSSL = envConfig.S3.DisableSSL
			}
			if len(envConfig.S3.ServiceLabel) > 0 {
				config.S3.ServiceLabel = envConfig.S3.ServiceLabel
			}
			if len(envConfig.S3.ServiceName) > 0 {
				config.S3.ServiceName = envConfig.S3.ServiceName
			}
			if len(envConfig.S3.BucketName) > 0 {
				config.S3.BucketName = envConfig.S3.BucketName
			}
			if len(envConfig.S3.EncryptionKey) > 0 {
				config.S3.EncryptionKey = envConfig.S3.EncryptionKey
			}
			if len(envConfig.S3.ServerSideEncryption) > 0 {
				if envConfig.S3.ServerSideEncryption != string(encrypt.S3) {
					log.Fatalln("only S3 mananged encryption(SSE-S3) is supported for now")
				}
				config.S3.ServerSideEncryption = envConfig.S3.ServerSideEncryption
			}
			for serviceName, serviceConfig := range envConfig.Services {
				mergedServiceConfig := config.Services[serviceName]
				if len(serviceConfig.Schedule) > 0 {
					mergedServiceConfig.Schedule = serviceConfig.Schedule
				}
				if serviceConfig.Timeout.Seconds() > 1 {
					mergedServiceConfig.Timeout = serviceConfig.Timeout
				}
				if serviceConfig.Retention.Days > 0 {
					mergedServiceConfig.Retention.Days = serviceConfig.Retention.Days
				}
				if serviceConfig.Retention.Files > 0 {
					mergedServiceConfig.Retention.Files = serviceConfig.Retention.Files
				}
				if serviceConfig.DisableColumnStatistics {
					mergedServiceConfig.DisableColumnStatistics = serviceConfig.DisableColumnStatistics
				}
				if serviceConfig.ForceImport {
					mergedServiceConfig.ForceImport = serviceConfig.ForceImport
				}
				if len(serviceConfig.LocalBackupPath) > 0 {
					mergedServiceConfig.LocalBackupPath = serviceConfig.LocalBackupPath
				}
				config.Services[serviceName] = mergedServiceConfig
			}
		}

		// ensure we have default values
		if len(config.LogLevel) == 0 {
			config.LogLevel = "info"
		}
		if len(config.S3.ServiceLabel) == 0 {
			config.S3.ServiceLabel = "dynstrg"
		}

		// use username & password from env if defined
		if len(os.Getenv("BACKMAN_USERNAME")) > 0 {
			config.Username = os.Getenv("BACKMAN_USERNAME")
		}
		if len(os.Getenv("BACKMAN_PASSWORD")) > 0 {
			config.Password = os.Getenv("BACKMAN_PASSWORD")
		}

		// use s3 encryption key from env if defined
		if len(os.Getenv("BACKMAN_ENCRYPTION_KEY")) > 0 {
			config.S3.EncryptionKey = os.Getenv("BACKMAN_ENCRYPTION_KEY")
		}
	})
	return &config
}