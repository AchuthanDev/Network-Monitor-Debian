package config

import (
	"os"
	"strconv"
	"time"

	"github.com/AchuthanDev/Network-Monitor-Debian/features/network-usage/classifier"
)

type Config struct {
	BindAddress      string
	Port             string
	DatabaseURL      string
	HostProcRoot     string
	PollInterval     time.Duration
	RetentionRawDays int
	Classifier       classifier.Config
}

func Load() (Config, error) {
	cfg := Config{
		BindAddress:      getenv("NETWORK_MONITOR_BIND_ADDRESS", "0.0.0.0"),
		Port:             getenv("NETWORK_MONITOR_COLLECTOR_PORT", "9091"),
		DatabaseURL:      getenv("NETWORK_MONITOR_DATABASE_URL", ""),
		HostProcRoot:     getenv("NETWORK_MONITOR_HOST_PROC_ROOT", "/host/proc"),
		PollInterval:     durationFromEnv("NETWORK_MONITOR_COLLECTOR_INTERVAL", 15*time.Second),
		RetentionRawDays: intFromEnv("NETWORK_MONITOR_RAW_RETENTION_DAYS", 7),
		Classifier:       classifier.DefaultConfig(),
	}

	if lanRaw := os.Getenv("NETWORK_MONITOR_LAN_CIDRS"); lanRaw != "" {
		prefixes, err := classifier.ParseCIDRList(lanRaw)
		if err != nil {
			return Config{}, err
		}
		cfg.Classifier.LANCIDRs = prefixes
	}
	if dockerRaw := os.Getenv("NETWORK_MONITOR_DOCKER_CIDRS"); dockerRaw != "" {
		prefixes, err := classifier.ParseCIDRList(dockerRaw)
		if err != nil {
			return Config{}, err
		}
		cfg.Classifier.DockerCIDRs = prefixes
	}

	return cfg, nil
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func durationFromEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func intFromEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
