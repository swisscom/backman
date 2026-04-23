package service

import (
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"github.com/swisscom/backman/config"
	"github.com/swisscom/backman/log"
	"github.com/swisscom/backman/service/elasticsearch"
	"github.com/swisscom/backman/service/mongodb"
	"github.com/swisscom/backman/service/mysql"
	"github.com/swisscom/backman/service/postgres"
	"github.com/swisscom/backman/service/redis"
	"github.com/swisscom/backman/state"
)

func Init() {
	mergeServiceBindings() // find and merge SERVICE_BINDING_ROOT/<service> into config.Services
	mergeVCAPServices()    // find and merge VCAP_SERVICES into config.Services

	validateServices() // final validation of all service instances in config.Services, enrich default values

	// setup service metrics
	for _, service := range config.Get().Services {
		// init prometheus state metrics to 0
		state.BackupInit(service)
		state.RestoreInit(service)

		// init backup files state & metrics in background
		go func(serviceType, name string) {
			_, _ = GetBackups(serviceType, name)
		}(service.Binding.Type, service.Name)
	}

	log.Debugf("services loaded: %+v", config.Get().Services)
}

func validateServices() {
	for serviceName, service := range config.Get().Services {
		// remove services without binding type, they are useless to us / invalid
		if len(service.Binding.Type) == 0 {
			log.Errorf("ignoring invalid service [%s], there is no service type defined", serviceName)
			delete(config.Get().Services, serviceName)
			continue
		}

		// enrich service bindings, try to figure out additional properties by parsing URI
		service.Binding = enrichBinding(service.Binding)

		// service name must be the same as map-key
		service.Name = serviceName

		// validate binding credentials for known service types
		// each of them knows best themselves what they need or require
		validBinding := true
		switch service.Type() {
		case config.Elasticsearch:
			validBinding = elasticsearch.VerifyBinding(service)
		case config.MongoDB:
			validBinding = mongodb.VerifyBinding(service)
		case config.MySQL:
			validBinding = mysql.VerifyBinding(service)
		case config.Postgres:
			validBinding = postgres.VerifyBinding(service)
		case config.Redis:
			validBinding = redis.VerifyBinding(service)
		}
		if !validBinding {
			log.Errorf("service binding for [%s] is not valid", serviceName)
			delete(config.Get().Services, serviceName)
			continue
		}

		// read timeout for service
		if service.Timeout.Seconds() <= 1 {
			service.Timeout.Duration = 1 * time.Hour // default
		}

		// read crontab schedule for service
		if len(service.Schedule) == 0 {
			// create a random schedule for daily backup as a fallback
			service.Schedule = fmt.Sprintf("%d %d %d * * *", rand.Intn(59), rand.Intn(59), rand.Intn(23))
		}

		// read retention days & files, with defaults as fallback
		if service.Retention.Days <= 0 {
			service.Retention.Days = 31 // default
		}
		if service.Retention.Files <= 0 {
			service.Retention.Files = 100 // default
		}

		// write values back
		config.Get().Services[serviceName] = service
	}
}

func enrichBinding(binding config.ServiceBinding) config.ServiceBinding {
	// figure out hostname & port from host if still missing
	if binding.Port == 0 {
		if len(binding.Host) > 0 && strings.Contains(binding.Host, ":") {
			if u, err := url.Parse(binding.Host); err == nil {
				binding.Host = u.Hostname()
				binding.Port, _ = strconv.Atoi(u.Port())
			}
		}
	}

	// figure out credentials from URI if missing
	if len(binding.URI) > 0 && strings.Contains(binding.URI, "://") {
		if u, err := url.Parse(binding.URI); err == nil {
			// set username and password if missing
			if len(binding.Username) == 0 {
				binding.Username = u.User.Username()
			}
			if len(binding.Password) == 0 {
				p, _ := u.User.Password()
				binding.Password = p
			}

			// set host and port too if still missing
			h, p, _ := net.SplitHostPort(canonicalHost(u))
			if len(binding.Host) == 0 {
				binding.Host = h
			}
			if binding.Port == 0 {
				binding.Port, _ = strconv.Atoi(p)
			}

			// set database if not defined yet but can be found in URI
			if len(binding.Database) == 0 {
				binding.Database = strings.TrimPrefix(u.Path, "/")
				rx := regexp.MustCompile(`([^\?]*)\?.*`) // trim connection options
				binding.Database = rx.ReplaceAllString(binding.Database, "${1}")
			}
		}
	}

	return binding
}

func GetServices(serviceType, serviceName string) []config.Service {
	services := make([]config.Service, 0)

	if len(serviceName) > 0 {
		// list only a specific service binding
		for _, service := range config.Get().Services {
			if service.Name == serviceName {
				services = append(services, service)
				break
			}
		}
	} else if len(serviceType) > 0 {
		// list services only for a specific service type
		for _, service := range config.Get().Services {
			if service.Binding.Type == serviceType {
				services = append(services, service)
			}
		}
	} else {
		// list all services
		for _, service := range config.Get().Services {
			services = append(services, service)
		}
	}

	return services
}

func GetService(serviceType, serviceName string) config.Service {
	// find and return the specific service instance
	for _, service := range config.Get().Services {
		if service.Name == serviceName && service.Binding.Type == serviceType {
			return service
		}
	}
	return config.Service{}
}

// canonicalHost returns url.Host but always with a ":port" suffix
// adapted from net/http/transport canonicalAddr
func canonicalHost(url *url.URL) string {
	portMap := map[string]string{
		"http":  "80",
		"https": "443",
	}
	addr := url.Hostname()
	port := url.Port()
	if port == "" {
		port = portMap[url.Scheme]
	}
	return net.JoinHostPort(addr, port)
}
