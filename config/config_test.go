package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func init() {
	_ = os.Chdir("../")
}

func Test_Config_GetWithoutBindings(t *testing.T) {
	SetConfigFile("_fixtures/config_without_bindings.json")

	os.Unsetenv("BACKMAN_USERNAME")
	os.Setenv("BACKMAN_PASSWORD", "McClane")
	os.Unsetenv("BACKMAN_ENCRYPTION_KEY")

	c := new()
	assert.Equal(t, "debug", c.LogLevel)                                          // _fixtures/env specifies "debug", overriding "info" from config.json
	assert.Equal(t, true, c.LoggingTimestamp)                                     // _fixtures/env specifies "true", overriding "false" from config.json
	assert.Equal(t, "john", c.Username)                                           // from .env, overriding config.json
	assert.Equal(t, "McClane", c.Password)                                        // Setenv 'PASSWORD' overrides _fixtures/env and config.json
	assert.Equal(t, false, c.DisableAllColumnStatistics)                          // should be false by default
	assert.Equal(t, "dynstrg", c.S3.ServiceLabel)                                 // from env/config.json
	assert.Equal(t, "my-database-backups", c.S3.BucketName)                       // from env/config.json
	assert.Equal(t, "13 37 0/6 * * *", c.Services["my_postgres_db"].Schedule)     // from .env, overriding config.json
	assert.Equal(t, "2h0m0s", c.Services["my_postgres_db"].Timeout.String())      // from config.json
	assert.Equal(t, 15, c.Services["my_postgres_db"].Retention.Days)              // from .env, overriding config.json
	assert.Equal(t, 10, c.Services["my_postgres_db"].Retention.Files)             // from .env, overriding config.json
	assert.Equal(t, "0 45 0/4 * * *", c.Services["mongodb-for-backend"].Schedule) // from .env, overriding config.json
	assert.Equal(t, 500, c.Services["mongodb-for-backend"].Retention.Files)       // from .env, overriding config.json
	assert.Equal(t, "1 2 3 * * *", c.Services["other_postgres_db"].Schedule)      // from config.json
	assert.Equal(t, "secondary", c.Services["my_mongodb"].ReadPreference)         // from config.json
}

func Test_Config_GetWithBindings(t *testing.T) {
	SetConfigFile("_fixtures/config_with_bindings.json")

	os.Unsetenv("BACKMAN_CONFIG")

	os.Setenv("BACKMAN_USERNAME", "Hans")
	os.Unsetenv("BACKMAN_PASSWORD")
	os.Setenv("BACKMAN_ENCRYPTION_KEY", "x-secret-key")

	c := new()
	assert.Equal(t, "debug", c.LogLevel)                                    // from config.json
	assert.Equal(t, true, c.LoggingTimestamp)                               // from config.json
	assert.Equal(t, "Hans", c.Username)                                     // Setenv 'USERNAME' overrides config.json
	assert.Equal(t, "doe", c.Password)                                      // from config.json
	assert.Equal(t, false, c.DisableAllColumnStatistics)                    // should be false by default
	assert.Equal(t, "x-secret-key", c.S3.EncryptionKey)                     // from .env, overriding  config.json
	assert.Equal(t, "127.0.0.1:9000", c.S3.Host)                            // from config.json
	assert.Equal(t, "dynstrg", c.S3.ServiceLabel)                           // from config.json
	assert.Equal(t, "my-database-backups", c.S3.BucketName)                 // from config.json
	assert.Equal(t, "0 0 2 * * *", c.Services["my_postgres_db"].Schedule)   // from config.json
	assert.Equal(t, "15m0s", c.Services["my_postgres_db"].Timeout.String()) // from config.json
	assert.Equal(t, 5, c.Services["my_postgres_db"].Retention.Days)         // from config.json
	assert.Equal(t, 5, c.Services["my_postgres_db"].Retention.Files)        // from config.json
	assert.Equal(t, "0 0 2 * * *", c.Services["my-redis"].Schedule)         // from config.json
	assert.Equal(t, "redis-2", c.Services["my-redis"].Binding.Type)         // from config.json
	assert.Equal(t, "small", c.Services["my-redis"].Binding.Plan)           // from config.json
	assert.Equal(t, "very-secret", c.Services["my-redis"].Binding.Password) // from config.json
	assert.Equal(t, "127.0.0.1", c.Services["my-redis"].Binding.Host)       // from config.json
	assert.Equal(t, 6379, c.Services["my-redis"].Binding.Port)              // from config.json
	assert.Equal(t, "primary", c.Services["my_mongodb"].ReadPreference)     // from config.json
}
