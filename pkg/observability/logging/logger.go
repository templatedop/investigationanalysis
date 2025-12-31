// Package logging provides structured logging capabilities
package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

// Level represents log severity
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// String returns the string representation of the level
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a level string
func ParseLevel(s string) Level {
	switch s {
	case "debug", "DEBUG":
		return LevelDebug
	case "info", "INFO":
		return LevelInfo
	case "warn", "WARN", "warning", "WARNING":
		return LevelWarn
	case "error", "ERROR":
		return LevelError
	case "fatal", "FATAL":
		return LevelFatal
	default:
		return LevelInfo
	}
}

// Format represents log output format
type Format string

const (
	FormatJSON Format = "json"
	FormatText Format = "text"
)

// LogConfig configures the logger
type LogConfig struct {
	Level       Level  `json:"level"`
	Format      Format `json:"format"`
	Output      string `json:"output"` // "stdout", "stderr", or file path
	EnableColor bool   `json:"enable_color"`
	AddCaller   bool   `json:"add_caller"`
	AddTime     bool   `json:"add_time"`
}

// DefaultLogConfig returns default configuration
func DefaultLogConfig() *LogConfig {
	return &LogConfig{
		Level:       LevelInfo,
		Format:      FormatJSON,
		Output:      "stdout",
		EnableColor: true,
		AddCaller:   true,
		AddTime:     true,
	}
}

// Entry represents a log entry
type Entry struct {
	Level     Level                  `json:"level"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Caller    string                 `json:"caller,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// Logger provides structured logging
type Logger struct {
	config *LogConfig
	output io.Writer
	fields map[string]interface{}
	mu     sync.RWMutex
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// NewLogger creates a new logger
func NewLogger(config *LogConfig) (*Logger, error) {
	var output io.Writer
	switch config.Output {
	case "stdout", "":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		f, err := os.OpenFile(config.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		output = f
	}

	return &Logger{
		config: config,
		output: output,
		fields: make(map[string]interface{}),
	}, nil
}

// Default returns the default logger
func Default() *Logger {
	once.Do(func() {
		var err error
		defaultLogger, err = NewLogger(DefaultLogConfig())
		if err != nil {
			panic(err)
		}
	})
	return defaultLogger
}

// SetDefault sets the default logger
func SetDefault(l *Logger) {
	defaultLogger = l
}

// With returns a logger with additional fields
func (l *Logger) With(fields map[string]interface{}) *Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	return &Logger{
		config: l.config,
		output: l.output,
		fields: newFields,
	}
}

// WithField returns a logger with an additional field
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return l.With(map[string]interface{}{key: value})
}

// WithError returns a logger with an error field
func (l *Logger) WithError(err error) *Logger {
	return l.WithField("error", err.Error())
}

// WithContext returns a logger with context values
func (l *Logger) WithContext(ctx context.Context) *Logger {
	fields := make(map[string]interface{})

	// Extract common context values
	if traceID := ctx.Value(contextKeyTraceID); traceID != nil {
		fields["trace_id"] = traceID
	}
	if spanID := ctx.Value(contextKeySpanID); spanID != nil {
		fields["span_id"] = spanID
	}
	if requestID := ctx.Value(contextKeyRequestID); requestID != nil {
		fields["request_id"] = requestID
	}

	return l.With(fields)
}

// Log logs a message at the specified level
func (l *Logger) Log(level Level, msg string, fields ...map[string]interface{}) {
	if level < l.config.Level {
		return
	}

	entry := Entry{
		Level:   level,
		Message: msg,
		Fields:  make(map[string]interface{}),
	}

	if l.config.AddTime {
		entry.Timestamp = time.Now()
	}

	if l.config.AddCaller {
		_, file, line, ok := runtime.Caller(2)
		if ok {
			entry.Caller = fmt.Sprintf("%s:%d", file, line)
		}
	}

	// Add base fields
	l.mu.RLock()
	for k, v := range l.fields {
		entry.Fields[k] = v
	}
	l.mu.RUnlock()

	// Add additional fields
	for _, f := range fields {
		for k, v := range f {
			entry.Fields[k] = v
		}
	}

	l.write(entry)
}

// write outputs the log entry
func (l *Logger) write(entry Entry) {
	var output []byte
	var err error

	switch l.config.Format {
	case FormatJSON:
		output, err = json.Marshal(entry)
		if err != nil {
			return
		}
		output = append(output, '\n')
	case FormatText:
		output = l.formatText(entry)
	default:
		output, err = json.Marshal(entry)
		if err != nil {
			return
		}
		output = append(output, '\n')
	}

	l.mu.Lock()
	l.output.Write(output)
	l.mu.Unlock()
}

// formatText formats an entry as text
func (l *Logger) formatText(entry Entry) []byte {
	var buf []byte

	// Timestamp
	if l.config.AddTime {
		buf = append(buf, entry.Timestamp.Format("2006-01-02 15:04:05.000")...)
		buf = append(buf, ' ')
	}

	// Level with optional color
	if l.config.EnableColor {
		buf = append(buf, l.levelColor(entry.Level)...)
	}
	buf = append(buf, entry.Level.String()...)
	if l.config.EnableColor {
		buf = append(buf, "\033[0m"...)
	}
	buf = append(buf, ' ')

	// Caller
	if l.config.AddCaller && entry.Caller != "" {
		buf = append(buf, '[')
		buf = append(buf, entry.Caller...)
		buf = append(buf, "] "...)
	}

	// Message
	buf = append(buf, entry.Message...)

	// Fields
	if len(entry.Fields) > 0 {
		buf = append(buf, " {"...)
		first := true
		for k, v := range entry.Fields {
			if !first {
				buf = append(buf, ", "...)
			}
			buf = append(buf, fmt.Sprintf("%s=%v", k, v)...)
			first = false
		}
		buf = append(buf, '}')
	}

	buf = append(buf, '\n')
	return buf
}

// levelColor returns ANSI color code for level
func (l *Logger) levelColor(level Level) string {
	switch level {
	case LevelDebug:
		return "\033[36m" // Cyan
	case LevelInfo:
		return "\033[32m" // Green
	case LevelWarn:
		return "\033[33m" // Yellow
	case LevelError:
		return "\033[31m" // Red
	case LevelFatal:
		return "\033[35m" // Magenta
	default:
		return ""
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...map[string]interface{}) {
	l.Log(LevelDebug, msg, fields...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...map[string]interface{}) {
	l.Log(LevelInfo, msg, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...map[string]interface{}) {
	l.Log(LevelWarn, msg, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...map[string]interface{}) {
	l.Log(LevelError, msg, fields...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, fields ...map[string]interface{}) {
	l.Log(LevelFatal, msg, fields...)
	os.Exit(1)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Debug(fmt.Sprintf(format, args...))
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Warn(fmt.Sprintf(format, args...))
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...))
}

// Fatalf logs a formatted fatal message and exits
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.Fatal(fmt.Sprintf(format, args...))
}

// Context keys
type contextKey string

const (
	contextKeyTraceID   contextKey = "trace_id"
	contextKeySpanID    contextKey = "span_id"
	contextKeyRequestID contextKey = "request_id"
	contextKeyLogger    contextKey = "logger"
)

// WithTraceID adds trace ID to context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, contextKeyTraceID, traceID)
}

// WithSpanID adds span ID to context
func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, contextKeySpanID, spanID)
}

// WithRequestID adds request ID to context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, contextKeyRequestID, requestID)
}

// WithLogger adds logger to context
func WithLogger(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, contextKeyLogger, logger)
}

// FromContext retrieves logger from context
func FromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value(contextKeyLogger).(*Logger); ok {
		return logger
	}
	return Default()
}

// Package-level convenience functions

// Debug logs a debug message using default logger
func Debug(msg string, fields ...map[string]interface{}) {
	Default().Debug(msg, fields...)
}

// Info logs an info message using default logger
func Info(msg string, fields ...map[string]interface{}) {
	Default().Info(msg, fields...)
}

// Warn logs a warning message using default logger
func Warn(msg string, fields ...map[string]interface{}) {
	Default().Warn(msg, fields...)
}

// Error logs an error message using default logger
func Error(msg string, fields ...map[string]interface{}) {
	Default().Error(msg, fields...)
}

// Fatal logs a fatal message using default logger
func Fatal(msg string, fields ...map[string]interface{}) {
	Default().Fatal(msg, fields...)
}

// InvestigationLogger provides logging specifically for investigations
type InvestigationLogger struct {
	*Logger
	investigationID string
	claimID         string
}

// NewInvestigationLogger creates a logger for an investigation
func NewInvestigationLogger(investigationID, claimID string) *InvestigationLogger {
	return &InvestigationLogger{
		Logger: Default().With(map[string]interface{}{
			"investigation_id": investigationID,
			"claim_id":         claimID,
		}),
		investigationID: investigationID,
		claimID:         claimID,
	}
}

// AgentStart logs agent start
func (l *InvestigationLogger) AgentStart(agentName string) {
	l.Info("Agent started", map[string]interface{}{
		"agent": agentName,
		"event": "agent_start",
	})
}

// AgentEnd logs agent end
func (l *InvestigationLogger) AgentEnd(agentName string, duration time.Duration, err error) {
	fields := map[string]interface{}{
		"agent":    agentName,
		"event":    "agent_end",
		"duration": duration.String(),
	}
	if err != nil {
		fields["error"] = err.Error()
		l.Error("Agent failed", fields)
	} else {
		l.Info("Agent completed", fields)
	}
}

// RiskAssessed logs risk assessment
func (l *InvestigationLogger) RiskAssessed(score float64, level string) {
	l.Info("Risk assessed", map[string]interface{}{
		"event":      "risk_assessed",
		"risk_score": score,
		"risk_level": level,
	})
}

// DecisionMade logs decision
func (l *InvestigationLogger) DecisionMade(decision string, confidence float64) {
	l.Info("Decision made", map[string]interface{}{
		"event":      "decision_made",
		"decision":   decision,
		"confidence": confidence,
	})
}

// EscalatedToHuman logs human escalation
func (l *InvestigationLogger) EscalatedToHuman(reason string) {
	l.Warn("Escalated to human review", map[string]interface{}{
		"event":  "human_escalation",
		"reason": reason,
	})
}
