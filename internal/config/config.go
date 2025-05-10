package config

import (
	"flag"
	"os"
)

// Config contains settings for running the PubSub gRPC service.
type Config struct {
	GRPCAddr     string // Address and port for the gRPC service (e.g., "localhost:50051")
	LogType      string // Logger type (e.g., "slog")
	LogToConsole string // Enable logging to console (e.g., "true", "Y", "T", "1")
	LogToFile    string // Enable logging to file (e.g., "true", "Y", "T", "1")
	LogFile      string // Log file path (e.g., "logs/server.log")
}

// NewConfig initializes the configuration, prioritizing environment variables, then command-line flags, then defaults.
func NewConfig() *Config {
	return &Config{
		GRPCAddr:     configValue("GRPC_ADDRESS", "a", "localhost:50051", "address and port to run gRPC service"),
		LogType:      configValue("LOG_TYPE", "l", "slog", "logger type (e.g., slog)"),
		LogFile:      configValue("LOG_FILE", "logfile", "logs/pubsub.log", "log file path"),
		LogToConsole: configValue("LOG_TO_CONSOLE", "logconsole", "true", "enable logging to console (true, Y, T, 1)"),
		LogToFile:    configValue("LOG_TO_FILE", "logfileenable", "true", "enable logging to file (true, Y, T, 1)"),
	}
}

// configValue returns the value of a parameter based on the following priority:
// 1. Environment variable.
// 2. Command-line flag.
// 3. Default value.
func configValue(envVar, flagName, defaultValue, description string) string {

	envValue := os.Getenv(envVar)
	if envValue != "" {
		return envValue
	}

	// Create and parse a command-line flag.
	flagValue := flag.String(flagName, defaultValue, description)
	flag.Parse()
	return *flagValue
}
