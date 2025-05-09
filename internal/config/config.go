package config

import (
	"flag"
	"os"
)

// Config contains settings for running the PubSub gRPC service.
type Config struct {
	GRPCAddr string // Address and port for the gRPC service (e.g., "localhost:50051")
	LogLevel string // Logging level (e.g., "debug", "info", "warn", "error")
}

// NewConfig initializes the configuration, prioritizing environment variables, then command-line flags, then defaults.
func NewConfig() *Config {
	return &Config{
		GRPCAddr: configValue("GRPC_ADDRESS", "a", "localhost:50051", "address and port to run gRPC service"),
		LogLevel: configValue("LOG_LEVEL", "l", "info", "logging level (debug, info, warn, error)"),
	}
}

// configValue returns the value of a parameter based on the following priority:
// 1. Environment variable.
// 2. Command-line flag.
// 3. Default value.
func configValue(envVar, flagName, defaultValue, description string) string {
	// Read environment variable.
	envValue := os.Getenv(envVar)
	if envValue != "" {
		return envValue
	}

	// Create and parse a command-line flag.
	flagValue := flag.String(flagName, defaultValue, description)
	flag.Parse()
	return *flagValue
}
