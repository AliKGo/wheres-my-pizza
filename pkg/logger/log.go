package logger

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"
	"wheres-my-pizza/internal/core/domain/models"
)

const (
	LevelDebug string = "DEBUG"
	LevelInfo  string = "INFO"
	LevelWarn  string = "WARN"
	LevelError string = "ERROR"
)

type Logger interface {
	Debug(ctx context.Context, action, msg string, args ...any)
	Info(ctx context.Context, action, msg string, args ...any)
	Warn(ctx context.Context, action, msg string, args ...any)
	Error(ctx context.Context, action, msg string, err error, args ...any)
	GetSlogLogger() *slog.Logger
}

type logger struct {
	slog     *slog.Logger
	service  string
	hostname string
}

// Initialize logger with service name and log level
func InitLogger(serviceName, logLevel string) Logger {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	level := new(slog.LevelVar)
	switch logLevel {
	case LevelDebug:
		level.Set(slog.LevelDebug)
	case LevelInfo:
		level.Set(slog.LevelInfo)
	case LevelWarn:
		level.Set(slog.LevelWarn)
	case LevelError:
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelInfo)
	}

	// Custom handler to add request_id and rename message field
	handler := &contextHandler{
		handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				// Rename 'msg' to 'message'
				if a.Key == slog.MessageKey {
					return slog.Attr{Key: "message", Value: a.Value}
				}
				// Format time as ISO 8601
				if a.Key == slog.TimeKey {
					if t, ok := a.Value.Any().(time.Time); ok {
						return slog.Attr{Key: "timestamp", Value: slog.StringValue(t.Format(time.RFC3339))}
					}
				}
				return a
			},
		}),
	}

	// Create base logger with service and hostname
	base := slog.New(handler).With(
		slog.String("service", serviceName),
		slog.String("hostname", hostname),
	)

	return &logger{
		slog:     base,
		service:  serviceName,
		hostname: hostname,
	}
}

// Context handler to inject request_id
type contextHandler struct {
	handler slog.Handler
}

func (h *contextHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return h.handler.Enabled(ctx, lvl)
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if reqID, ok := ctx.Value(models.GetRequestIDKey()).(string); ok {
		r.AddAttrs(slog.String("request_id", reqID))
	}
	return h.handler.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{handler: h.handler.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{handler: h.handler.WithGroup(name)}
}

// Logger methods
func (l *logger) Debug(ctx context.Context, action, msg string, args ...any) {
	l.slog.DebugContext(ctx, msg, append(args, "action", action)...)
}

func (l *logger) Info(ctx context.Context, action, msg string, args ...any) {
	l.slog.InfoContext(ctx, msg, append(args, "action", action)...)
}

func (l *logger) Warn(ctx context.Context, action, msg string, args ...any) {
	l.slog.WarnContext(ctx, msg, append(args, "action", action)...)
}

func (l *logger) Error(ctx context.Context, action, msg string, err error, args ...any) {
	attrs := []any{
		"action", action,
		"error", slog.GroupValue(
			slog.String("msg", err.Error()),
			slog.String("stack", getStack()),
		),
	}
	attrs = append(attrs, args...)
	l.slog.ErrorContext(ctx, msg, attrs...)
}

func (l *logger) GetSlogLogger() *slog.Logger {
	return l.slog
}

// Helper to set request_id in context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, models.GetRequestIDKey(), requestID)
}

// getStack return stack info
func getStack() string {
	var stackBuf [4096]byte
	n := runtime.Stack(stackBuf[:], false)
	stack := string(stackBuf[:n])

	lines := strings.SplitSeq(stack, "\n")
	for line := range lines {
		if strings.Contains(line, "\t") &&
			!strings.Contains(line, "runtime/") &&
			!strings.Contains(line, "pkg/logger") {
			// Return just the first relevant file:line
			if idx := strings.LastIndex(line, "/"); idx > 0 {
				line = line[idx+1:]
			}
			line = strings.TrimSpace(strings.TrimPrefix(line, "\t"))
			if colon := strings.Index(line, ":"); colon > 0 {
				return line[:colon] + ":" + strings.Split(line[colon+1:], " ")[0]
			}
			return line
		}
	}
	return ""
}
