package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LogLevel represents the logging level
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger provides structured logging with multiple output targets
type Logger struct {
	slogger   *slog.Logger
	level     LogLevel
	component string
	logFile   *os.File
}

// Config holds logger configuration
type Config struct {
	Level     LogLevel
	Component string
	LogFile   string
	UseJSON   bool
	AddSource bool
}

// Global logger instance
var defaultLogger *Logger

// Initialize sets up the global logger
func Initialize(config Config) error {
	var err error
	defaultLogger, err = New(config)
	return err
}

// New creates a new logger instance
func New(config Config) (*Logger, error) {
	logger := &Logger{
		level:     config.Level,
		component: config.Component,
	}

	// Prepare output writers
	var writers []io.Writer
	writers = append(writers, os.Stdout)

	// Add file output if specified
	if config.LogFile != "" {
		// Create log directory if needed
		logDir := filepath.Dir(config.LogFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		file, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		logger.logFile = file
		writers = append(writers, file)
	}

	// Create multi-writer
	multiWriter := io.MultiWriter(writers...)

	// Configure slog options
	opts := &slog.HandlerOptions{
		Level:     slogLevel(config.Level),
		AddSource: config.AddSource,
	}

	// Choose handler based on format preference
	var handler slog.Handler
	if config.UseJSON {
		handler = slog.NewJSONHandler(multiWriter, opts)
	} else {
		handler = slog.NewTextHandler(multiWriter, opts)
	}

	logger.slogger = slog.New(handler)

	return logger, nil
}

// slogLevel converts LogLevel to slog.Level
func slogLevel(level LogLevel) slog.Level {
	switch level {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Close closes the logger and any open files
func (l *Logger) Close() error {
	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

// WithComponent creates a new logger with the specified component
func (l *Logger) WithComponent(component string) *Logger {
	newLogger := *l
	newLogger.component = component
	return &newLogger
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...any) {
	l.log(LevelDebug, msg, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...any) {
	l.log(LevelInfo, msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...any) {
	l.log(LevelWarn, msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...any) {
	l.log(LevelError, msg, args...)
}

// log is the internal logging method
func (l *Logger) log(level LogLevel, msg string, args ...any) {
	if l == nil || l.slogger == nil {
		return
	}

	// Add component to attributes
	attrs := []slog.Attr{
		slog.String("component", l.component),
		slog.Time("timestamp", time.Now()),
	}

	// Add caller information
	if _, file, line, ok := runtime.Caller(2); ok {
		attrs = append(attrs, slog.String("caller", fmt.Sprintf("%s:%d", filepath.Base(file), line)))
	}

	// Process additional arguments
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key := fmt.Sprintf("%v", args[i])
			value := args[i+1]
			attrs = append(attrs, slog.Any(key, value))
		}
	}

	// Log with appropriate level and attributes
	switch level {
	case LevelDebug:
		l.slogger.Debug(msg, slog.Any("attrs", attrs))
	case LevelInfo:
		l.slogger.Info(msg, slog.Any("attrs", attrs))
	case LevelWarn:
		l.slogger.Warn(msg, slog.Any("attrs", attrs))
	case LevelError:
		l.slogger.Error(msg, slog.Any("attrs", attrs))
	}
}

// Global logging functions that use the default logger

// Debug logs a debug message using the default logger
func Debug(msg string, args ...any) {
	if defaultLogger != nil {
		defaultLogger.Debug(msg, args...)
	}
}

// Info logs an info message using the default logger
func Info(msg string, args ...any) {
	if defaultLogger != nil {
		defaultLogger.Info(msg, args...)
	}
}

// Warn logs a warning message using the default logger
func Warn(msg string, args ...any) {
	if defaultLogger != nil {
		defaultLogger.Warn(msg, args...)
	}
}

// Error logs an error message using the default logger
func Error(msg string, args ...any) {
	if defaultLogger != nil {
		defaultLogger.Error(msg, args...)
	}
}

// LogLevelFromString parses a log level from string
func LogLevelFromString(s string) LogLevel {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// String returns the string representation of a log level
func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "info"
	}
}