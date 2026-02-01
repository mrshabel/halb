package halb

import (
	"fmt"
	"log/slog"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// defaults
const (
	defaultServerPort    = 80
	defaultStrategy      = RoundRobin
	defaultServerTimeout = 30 * time.Second
)

// lock-free configuration provider
type ConfigProvider struct {
	// store holds the config struct
	store atomic.Value
}

// app config
type Config struct {
	Server ServerConfig `mapstructure:"server"`
	// service name mapping to config
	Services map[string]ServiceConfig `mapstructure:"services"`
}

// server config
type ServerConfig struct {
	Port    int           `mapstructure:"port"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// upstream service config
type ServiceConfig struct {
	Host string `mapstructure:"host"`
	// load balancing strategy
	Strategy LoadBalancingStrategy `mapstructure:"strategy"`
	// server urls
	Servers []string     `mapstructure:"servers"`
	Health  HealthConfig `mapstructure:"health"`
}

// health check config
type HealthConfig struct {
	Path     string        `mapstructure:"path"`
	Interval time.Duration `mapstructure:"interval"`
}

// Enabled determines if health check is present
func (h HealthConfig) Enabled() bool {
	return h.Path != ""
}

func NewConfig(configPath string) (*ConfigProvider, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// set defaults
	viper.SetDefault("server.port", defaultServerPort)
	viper.SetDefault("server.timeout", defaultServerTimeout)

	return &ConfigProvider{}, nil
}

// Load reads the configuration file
func (c *ConfigProvider) Load() (*Config, error) {
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	// unmarshal and validate config
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Watch monitors the config file for changes
func (c *ConfigProvider) Watch(onChange func(cfg *Config)) {

	viper.OnConfigChange(func(in fsnotify.Event) {
		slog.Info("Config changes detected. Reloading now...")
		// load new config and replace content
		newCfg, err := c.Load()
		if err != nil {
			slog.Error("Failed to reload config after detecting changes", slog.String("error", err.Error()))
			return
		}

		// trigger callback
		onChange(newCfg)

		slog.Info("Config reloaded successfully", slog.String("file", in.Name))
	})

	viper.WatchConfig()
}

// config validation
func (c *Config) validate() error {
	// server config
	if len(c.Services) == 0 {
		return fmt.Errorf("no services defined")
	}

	for name, service := range c.Services {
		if len(service.Servers) == 0 {
			return fmt.Errorf("service %q must have at least one server", name)
		}

		if service.Host == "" {
			return fmt.Errorf("service %s: host is required", name)
		}

		for _, s := range service.Servers {
			// parse request url with http scheme
			u, err := url.ParseRequestURI(s)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				return fmt.Errorf("service %q: invalid server url %q", name, s)
			}

			if u.Host == "" {
				return fmt.Errorf("service %q: server url %q missing host", name, s)
			}
		}

		if service.Health.Enabled() {
			if service.Health.Interval < 1*time.Second {
				return fmt.Errorf("service %q: health check interval must be >= 1s", name)
			}

			if service.Health.Path == "" {
				return fmt.Errorf("service %s: health check path is required", name)
			}
		}
	}
	return nil
}
