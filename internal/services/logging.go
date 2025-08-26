package services

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
)

// LogLevel represents different log levels for service operations
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// ServiceLogger provides structured logging for service layer operations
type ServiceLogger struct {
	logger *slog.Logger
	config LogConfig
}

type LogConfig struct {
	Service       string
	Component     string
	EnableMetrics bool
	EnableDebug   bool
}

func NewServiceLogger(logger *slog.Logger, config LogConfig) *ServiceLogger {
	return &ServiceLogger{
		logger: logger.With("service", config.Service, "component", config.Component),
		config: config,
	}
}

// ===== OPERATION LOGGING =====

func (l *ServiceLogger) LogOperation(ctx context.Context, operation string, userID uint, resourceID uint, resourceType string, duration time.Duration, err error) {
	logLevel := LogLevelInfo
	status := "success"

	if err != nil {
		logLevel = LogLevelError
		status = "error"

		// Adjust log level based on error type
		if IsValidation(err) || IsBusinessRule(err) {
			logLevel = LogLevelWarn
			status = "validation_error"
		} else if IsUnauthorized(err) {
			logLevel = LogLevelWarn
			status = "unauthorized"
		} else if IsNotFound(err) {
			logLevel = LogLevelInfo
			status = "not_found"
		}
	}

	attrs := []slog.Attr{
		slog.String("operation", operation),
		slog.Uint64("user_id", uint64(userID)),
		slog.Uint64("resource_id", uint64(resourceID)),
		slog.String("resource_type", resourceType),
		slog.String("status", status),
		slog.Duration("duration", duration),
	}

	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))

		// Add error details for different error types
		if validationErr, ok := err.(ValidationErrors); ok {
			attrs = append(attrs, slog.Int("validation_errors_count", len(validationErr)))
		} else if businessErr, ok := err.(*BusinessRuleError); ok {
			attrs = append(attrs, slog.String("business_rule", businessErr.Rule))
		} else if permErr, ok := err.(*PermissionError); ok {
			attrs = append(attrs, slog.String("permission_action", permErr.Action))
		}
	}

	// Add request context if available
	if requestID := ctx.Value("request_id"); requestID != nil {
		attrs = append(attrs, slog.String("request_id", requestID.(string)))
	}

	// Add caller information for errors
	if err != nil {
		if pc, file, line, ok := runtime.Caller(2); ok {
			if fn := runtime.FuncForPC(pc); fn != nil {
				attrs = append(attrs,
					slog.String("caller_func", fn.Name()),
					slog.String("caller_file", file),
					slog.Int("caller_line", line),
				)
			}
		}
	}

	message := fmt.Sprintf("%s operation %s", operation, status)

	switch logLevel {
	case LogLevelDebug:
		if l.config.EnableDebug {
			l.logger.LogAttrs(ctx, slog.LevelDebug, message, attrs...)
		}
	case LogLevelInfo:
		l.logger.LogAttrs(ctx, slog.LevelInfo, message, attrs...)
	case LogLevelWarn:
		l.logger.LogAttrs(ctx, slog.LevelWarn, message, attrs...)
	case LogLevelError:
		l.logger.LogAttrs(ctx, slog.LevelError, message, attrs...)
	}
}

func (l *ServiceLogger) LogValidationError(ctx context.Context, operation string, userID uint, validationErrors ValidationErrors) {
	attrs := []slog.Attr{
		slog.String("operation", operation),
		slog.Uint64("user_id", uint64(userID)),
		slog.Int("error_count", len(validationErrors)),
	}

	// Add individual validation errors
	for i, err := range validationErrors {
		if i < 5 { // Limit to first 5 errors to avoid log spam
			attrs = append(attrs, slog.Group(fmt.Sprintf("error_%d", i+1),
				slog.String("field", err.Field),
				slog.String("message", err.Message),
				slog.Any("value", err.Value),
			))
		}
	}

	l.logger.LogAttrs(ctx, slog.LevelWarn, "Validation failed", attrs...)
}

func (l *ServiceLogger) LogBusinessRuleViolation(ctx context.Context, operation string, userID uint, rule *BusinessRuleError) {
	attrs := []slog.Attr{
		slog.String("operation", operation),
		slog.Uint64("user_id", uint64(userID)),
		slog.String("rule", rule.Rule),
		slog.String("message", rule.Message),
	}

	// Add context information
	if rule.Context != nil {
		for key, value := range rule.Context {
			attrs = append(attrs, slog.Any(fmt.Sprintf("context_%s", key), value))
		}
	}

	l.logger.LogAttrs(ctx, slog.LevelWarn, "Business rule violation", attrs...)
}

func (l *ServiceLogger) LogPermissionDenied(ctx context.Context, operation string, permError *PermissionError) {
	attrs := []slog.Attr{
		slog.String("operation", operation),
		slog.Uint64("user_id", uint64(permError.UserID)),
		slog.Uint64("resource_id", uint64(permError.ResourceID)),
		slog.String("resource_type", permError.Resource),
		slog.String("action", permError.Action),
		slog.String("reason", permError.Reason),
	}

	l.logger.LogAttrs(ctx, slog.LevelWarn, "Permission denied", attrs...)
}

// ===== AUDIT LOGGING =====

func (l *ServiceLogger) LogAuditEvent(ctx context.Context, event AuditEvent) {
	attrs := []slog.Attr{
		slog.String("event_type", string(event.Type)),
		slog.Uint64("user_id", uint64(event.UserID)),
		slog.Uint64("resource_id", uint64(event.ResourceID)),
		slog.String("resource_type", event.ResourceType),
		slog.String("action", event.Action),
		slog.Time("timestamp", event.Timestamp),
	}

	if event.OldValue != nil {
		attrs = append(attrs, slog.Any("old_value", event.OldValue))
	}

	if event.NewValue != nil {
		attrs = append(attrs, slog.Any("new_value", event.NewValue))
	}

	if event.Metadata != nil {
		for key, value := range event.Metadata {
			attrs = append(attrs, slog.Any(fmt.Sprintf("meta_%s", key), value))
		}
	}

	if event.IPAddress != "" {
		attrs = append(attrs, slog.String("ip_address", event.IPAddress))
	}

	if event.UserAgent != "" {
		attrs = append(attrs, slog.String("user_agent", event.UserAgent))
	}

	l.logger.LogAttrs(ctx, slog.LevelInfo, fmt.Sprintf("Audit: %s %s", event.Action, event.ResourceType), attrs...)
}

// ===== PERFORMANCE LOGGING =====

func (l *ServiceLogger) LogPerformanceMetrics(ctx context.Context, operation string, metrics PerformanceMetrics) {
	if !l.config.EnableMetrics {
		return
	}

	attrs := []slog.Attr{
		slog.String("operation", operation),
		slog.Duration("total_duration", metrics.TotalDuration),
		slog.Duration("db_duration", metrics.DatabaseDuration),
		slog.Duration("validation_duration", metrics.ValidationDuration),
		slog.Int("queries_count", metrics.QueriesExecuted),
		slog.Int("cache_hits", metrics.CacheHits),
		slog.Int("cache_misses", metrics.CacheMisses),
	}

	if metrics.MemoryUsed > 0 {
		attrs = append(attrs, slog.Int64("memory_used_bytes", metrics.MemoryUsed))
	}

	if len(metrics.SlowQueries) > 0 {
		attrs = append(attrs, slog.Int("slow_queries_count", len(metrics.SlowQueries)))
	}

	l.logger.LogAttrs(ctx, slog.LevelDebug, "Performance metrics", attrs...)
}

// ===== SECURITY LOGGING =====

func (l *ServiceLogger) LogSecurityEvent(ctx context.Context, event SecurityEvent) {
	logLevel := slog.LevelWarn
	if event.Severity == SecuritySeverityHigh {
		logLevel = slog.LevelError
	}

	attrs := []slog.Attr{
		slog.String("security_event", string(event.Type)),
		slog.String("severity", string(event.Severity)),
		slog.Uint64("user_id", uint64(event.UserID)),
		slog.String("description", event.Description),
		slog.Time("timestamp", event.Timestamp),
	}

	if event.IPAddress != "" {
		attrs = append(attrs, slog.String("ip_address", event.IPAddress))
	}

	if event.UserAgent != "" {
		attrs = append(attrs, slog.String("user_agent", event.UserAgent))
	}

	if event.Metadata != nil {
		for key, value := range event.Metadata {
			attrs = append(attrs, slog.Any(fmt.Sprintf("meta_%s", key), value))
		}
	}

	l.logger.LogAttrs(ctx, logLevel, fmt.Sprintf("Security: %s", event.Description), attrs...)
}

// ===== ERROR RECOVERY LOGGING =====

func (l *ServiceLogger) LogRecovery(ctx context.Context, operation string, userID uint, recovered interface{}, stack []byte) {
	attrs := []slog.Attr{
		slog.String("operation", operation),
		slog.Uint64("user_id", uint64(userID)),
		slog.Any("panic_value", recovered),
		slog.String("stack_trace", string(stack)),
	}

	l.logger.LogAttrs(ctx, slog.LevelError, "Panic recovered", attrs...)
}

// ===== STRUCTURED LOGGING TYPES =====

type AuditEventType string

const (
	AuditEventCreate AuditEventType = "create"
	AuditEventRead   AuditEventType = "read"
	AuditEventUpdate AuditEventType = "update"
	AuditEventDelete AuditEventType = "delete"
	AuditEventAccess AuditEventType = "access"
)

type AuditEvent struct {
	Type         AuditEventType         `json:"type"`
	UserID       uint                   `json:"user_id"`
	ResourceID   uint                   `json:"resource_id"`
	ResourceType string                 `json:"resource_type"`
	Action       string                 `json:"action"`
	OldValue     interface{}            `json:"old_value,omitempty"`
	NewValue     interface{}            `json:"new_value,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
	IPAddress    string                 `json:"ip_address,omitempty"`
	UserAgent    string                 `json:"user_agent,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type PerformanceMetrics struct {
	TotalDuration      time.Duration   `json:"total_duration"`
	DatabaseDuration   time.Duration   `json:"database_duration"`
	ValidationDuration time.Duration   `json:"validation_duration"`
	QueriesExecuted    int             `json:"queries_executed"`
	CacheHits          int             `json:"cache_hits"`
	CacheMisses        int             `json:"cache_misses"`
	MemoryUsed         int64           `json:"memory_used"`
	SlowQueries        []SlowQueryInfo `json:"slow_queries,omitempty"`
}

type SlowQueryInfo struct {
	Query    string        `json:"query"`
	Duration time.Duration `json:"duration"`
	Args     []interface{} `json:"args,omitempty"`
}

type SecurityEventType string
type SecuritySeverity string

const (
	SecurityEventUnauthorizedAccess  SecurityEventType = "unauthorized_access"
	SecurityEventSuspiciousActivity  SecurityEventType = "suspicious_activity"
	SecurityEventRateLimitExceeded   SecurityEventType = "rate_limit_exceeded"
	SecurityEventInvalidToken        SecurityEventType = "invalid_token"
	SecurityEventPrivilegeEscalation SecurityEventType = "privilege_escalation"

	SecuritySeverityLow    SecuritySeverity = "low"
	SecuritySeverityMedium SecuritySeverity = "medium"
	SecuritySeverityHigh   SecuritySeverity = "high"
)

type SecurityEvent struct {
	Type        SecurityEventType      `json:"type"`
	Severity    SecuritySeverity       `json:"severity"`
	UserID      uint                   `json:"user_id"`
	Description string                 `json:"description"`
	Timestamp   time.Time              `json:"timestamp"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ===== MIDDLEWARE AND HELPERS =====

// ContextualLogger wraps operations with automatic logging
type ContextualLogger struct {
	logger    *ServiceLogger
	operation string
	userID    uint
	startTime time.Time
	ctx       context.Context
}

func (l *ServiceLogger) WithOperation(ctx context.Context, operation string, userID uint) *ContextualLogger {
	return &ContextualLogger{
		logger:    l,
		operation: operation,
		userID:    userID,
		startTime: time.Now(),
		ctx:       ctx,
	}
}

func (cl *ContextualLogger) LogResult(resourceID uint, resourceType string, err error) {
	duration := time.Since(cl.startTime)
	cl.logger.LogOperation(cl.ctx, cl.operation, cl.userID, resourceID, resourceType, duration, err)

	// Log specific error types with additional context
	if err != nil {
		if validationErrors, ok := err.(ValidationErrors); ok {
			cl.logger.LogValidationError(cl.ctx, cl.operation, cl.userID, validationErrors)
		} else if businessErr, ok := err.(*BusinessRuleError); ok {
			cl.logger.LogBusinessRuleViolation(cl.ctx, cl.operation, cl.userID, businessErr)
		} else if permErr, ok := err.(*PermissionError); ok {
			cl.logger.LogPermissionDenied(cl.ctx, cl.operation, permErr)
		}
	}
}

func (cl *ContextualLogger) LogAudit(eventType AuditEventType, resourceID uint, resourceType string, oldValue, newValue interface{}) {
	event := AuditEvent{
		Type:         eventType,
		UserID:       cl.userID,
		ResourceID:   resourceID,
		ResourceType: resourceType,
		Action:       cl.operation,
		OldValue:     oldValue,
		NewValue:     newValue,
		Timestamp:    time.Now(),
	}

	// Extract request context
	if cl.ctx != nil {
		if ip := cl.ctx.Value("client_ip"); ip != nil {
			event.IPAddress = ip.(string)
		}
		if ua := cl.ctx.Value("user_agent"); ua != nil {
			event.UserAgent = ua.(string)
		}
	}

	cl.logger.LogAuditEvent(cl.ctx, event)
}

func (cl *ContextualLogger) LogSecurity(eventType SecurityEventType, severity SecuritySeverity, description string, metadata map[string]interface{}) {
	event := SecurityEvent{
		Type:        eventType,
		Severity:    severity,
		UserID:      cl.userID,
		Description: description,
		Timestamp:   time.Now(),
		Metadata:    metadata,
	}

	// Extract request context
	if cl.ctx != nil {
		if ip := cl.ctx.Value("client_ip"); ip != nil {
			event.IPAddress = ip.(string)
		}
		if ua := cl.ctx.Value("user_agent"); ua != nil {
			event.UserAgent = ua.(string)
		}
	}

	cl.logger.LogSecurityEvent(cl.ctx, event)
}

// ===== ERROR FORMATTING HELPERS =====

func FormatError(err error) map[string]interface{} {
	if err == nil {
		return nil
	}

	result := map[string]interface{}{
		"message": err.Error(),
		"type":    "unknown",
	}

	switch e := err.(type) {
	case ValidationErrors:
		result["type"] = "validation"
		result["count"] = len(e)

		fields := make([]map[string]interface{}, len(e))
		for i, validationErr := range e {
			fields[i] = map[string]interface{}{
				"field":   validationErr.Field,
				"message": validationErr.Message,
				"value":   validationErr.Value,
			}
		}
		result["errors"] = fields

	case *BusinessRuleError:
		result["type"] = "business_rule"
		result["rule"] = e.Rule
		result["context"] = e.Context

	case *PermissionError:
		result["type"] = "permission"
		result["user_id"] = e.UserID
		result["resource_id"] = e.ResourceID
		result["resource"] = e.Resource
		result["action"] = e.Action
		result["reason"] = e.Reason

	default:
		// Check known error types
		if IsNotFound(err) {
			result["type"] = "not_found"
		} else if IsUnauthorized(err) {
			result["type"] = "unauthorized"
		} else if IsConflict(err) {
			result["type"] = "conflict"
		}
	}

	return result
}

// SanitizeForLogging removes sensitive information from data before logging
func SanitizeForLogging(data interface{}) interface{} {
	if data == nil {
		return nil
	}

	switch v := data.(type) {
	case string:
		return sanitizeString(v)
	case map[string]interface{}:
		return sanitizeMap(v)
	case []interface{}:
		return sanitizeSlice(v)
	default:
		return data
	}
}

func sanitizeString(s string) string {
	sensitiveFields := []string{"password", "token", "key", "secret", "auth"}
	lowerS := strings.ToLower(s)

	for _, field := range sensitiveFields {
		if strings.Contains(lowerS, field) {
			return "[REDACTED]"
		}
	}

	return s
}

func sanitizeMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	sensitiveKeys := []string{"password", "token", "key", "secret", "auth", "credential"}

	for k, v := range m {
		lowerK := strings.ToLower(k)
		sensitive := false

		for _, sensitiveKey := range sensitiveKeys {
			if strings.Contains(lowerK, sensitiveKey) {
				sensitive = true
				break
			}
		}

		if sensitive {
			result[k] = "[REDACTED]"
		} else {
			result[k] = SanitizeForLogging(v)
		}
	}

	return result
}

func sanitizeSlice(s []interface{}) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		result[i] = SanitizeForLogging(v)
	}
	return result
}
