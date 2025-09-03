package utils

import (
	"context"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
)

// Logger defines a unified logging interface that can be used across handlers and services
type Logger interface {
	// Basic logging methods
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)

	// Context-aware logging methods
	DebugContext(ctx context.Context, msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)

	// Structured logging with key-value pairs
	With(args ...any) Logger
	WithGroup(name string) Logger

	// Handler-specific methods for HTTP request logging
	LogRequest(method, path string, statusCode int, duration string, args ...any)
	LogError(err error, msg string, args ...any)
}

// SlogLogger implements Logger interface using slog
type SlogLogger struct {
	logger *slog.Logger
}

// NewSlogLogger creates a new logger wrapper around slog.Logger
func NewSlogLogger(logger *slog.Logger) Logger {
	return &SlogLogger{
		logger: logger,
	}
}

// NewDefaultLogger creates a default logger using slog with JSON output
func NewDefaultLogger() Logger {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	return NewSlogLogger(logger)
}

// NewDevelopmentLogger creates a logger optimized for development with text output
func NewDevelopmentLogger() Logger {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	return NewSlogLogger(logger)
}

// Basic logging methods
func (l *SlogLogger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

func (l *SlogLogger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

func (l *SlogLogger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

func (l *SlogLogger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

// Context-aware logging methods
func (l *SlogLogger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.logger.DebugContext(ctx, msg, args...)
}

func (l *SlogLogger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.logger.InfoContext(ctx, msg, args...)
}

func (l *SlogLogger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.logger.WarnContext(ctx, msg, args...)
}

func (l *SlogLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.logger.ErrorContext(ctx, msg, args...)
}

// Structured logging with key-value pairs
func (l *SlogLogger) With(args ...any) Logger {
	return &SlogLogger{
		logger: l.logger.With(args...),
	}
}

func (l *SlogLogger) WithGroup(name string) Logger {
	return &SlogLogger{
		logger: l.logger.WithGroup(name),
	}
}

// Handler-specific methods for HTTP request logging
func (l *SlogLogger) LogRequest(method, path string, statusCode int, duration string, args ...any) {
	level := slog.LevelInfo
	if statusCode >= 400 {
		level = slog.LevelWarn
	}
	if statusCode >= 500 {
		level = slog.LevelError
	}

	baseArgs := []any{
		"method", method,
		"path", path,
		"status_code", statusCode,
		"duration", duration,
	}

	allArgs := append(baseArgs, args...)
	l.logger.Log(context.Background(), level, "HTTP Request", allArgs...)
}

func (l *SlogLogger) LogError(err error, msg string, args ...any) {
	allArgs := append([]any{"error", err}, args...)
	l.logger.Error(msg, allArgs...)
}

// GetSlogLogger returns the underlying slog.Logger for direct access when needed
func (l *SlogLogger) GetSlogLogger() *slog.Logger {
	return l.logger
}

// LoggerMiddleware creates a Gin middleware for request logging
func LoggerMiddleware(logger Logger) func(*gin.Context) {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// Log using our logger instead of default Gin logger
		logger.LogRequest(
			param.Method,
			param.Path,
			param.StatusCode,
			param.Latency.String(),
			"client_ip", param.ClientIP,
			"user_agent", param.Request.UserAgent(),
		)
		return "" // Return empty string as we're handling logging ourselves
	})
}

// ContextLogger adds logger to Gin context
func ContextLogger(logger Logger) func(*gin.Context) {
	return func(c *gin.Context) {
		// Add logger with request context
		requestLogger := logger.With(
			"request_id", c.GetHeader("X-Request-ID"),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
		)

		// Store in context for handlers to use
		c.Set("logger", requestLogger)
		c.Next()
	}
}

// GetLoggerFromContext retrieves logger from Gin context
func GetLoggerFromContext(c *gin.Context) Logger {
	if logger, exists := c.Get("logger"); exists {
		if typedLogger, ok := logger.(Logger); ok {
			return typedLogger
		}
	}
	// Fallback to default logger
	return NewDefaultLogger()
}

// Compatibility functions to work with existing slog.Logger
func FromSlogLogger(slogger *slog.Logger) Logger {
	return NewSlogLogger(slogger)
}

func ToSlogLogger(logger Logger) *slog.Logger {
	if slogLogger, ok := logger.(*SlogLogger); ok {
		return slogLogger.GetSlogLogger()
	}
	// If it's not our SlogLogger, return default
	return slog.Default()
}
