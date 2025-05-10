package logging

import (
	"github.com/YuarenArt/PubSubService/internal/config"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Logger is the minimal interface for structured logging.
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// NewLogger creates a new Logger instance based on the configuration.
func NewLogger(cfg *config.Config) Logger {
	switch cfg.LogType {
	case "slog":
		return newSlogLogger(cfg)
	default:
		panic("unsupported logger type")
	}
}

// SlogLogger wraps an slog.Logger to implement the Logger interface.
type SlogLogger struct {
	logger *slog.Logger
}

// newSlogLogger creates a new Logger instance using slog that writes to both a file and stdout.
func newSlogLogger(cfg *config.Config) Logger {
	var writers []io.Writer

	// Parse LogToConsole as boolean
	if isTrue(cfg.LogToConsole) {
		writers = append(writers, os.Stdout)
	}

	if isTrue(cfg.LogToFile) {
		logDir := filepath.Dir(cfg.LogFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			slog.Error("Failed to create logs directory", "error", err, "path", logDir)
			return &SlogLogger{
				logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{})),
			}
		}

		logFile, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			slog.Error("Failed to open log file", "error", err, "path", cfg.LogFile)
			return &SlogLogger{
				logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{})),
			}
		}
		writers = append(writers, logFile)
	}

	// If no writers are specified, default to console
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	// Combine writers for logging
	multiWriter := io.MultiWriter(writers...)

	handler := slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{})

	return &SlogLogger{
		logger: slog.New(handler),
	}
}

func (l *SlogLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.logger.Debug(msg, keysAndValues...)
}

func (l *SlogLogger) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Info(msg, keysAndValues...)
}

func (l *SlogLogger) Warn(msg string, keysAndValues ...interface{}) {
	l.logger.Warn(msg, keysAndValues...)
}

func (l *SlogLogger) Error(msg string, keysAndValues ...interface{}) {
	l.logger.Error(msg, keysAndValues...)
}

func isTrue(value string) bool {
	value = strings.ToLower(value)
	return value == "true" || value == "t" || value == "y" || value == "1"
}
