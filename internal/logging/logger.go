package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

const redactedValue = "[REDACTED]"

// Options configures the default service logger.
type Options struct {
	Level     string
	AddSource bool
	Service   string
	Version   string
	Env       string
	Output    io.Writer
}

// Setup builds and installs the process-wide default logger.
func Setup(opts Options) error {
	logger, err := New(opts)
	if err != nil {
		return err
	}
	slog.SetDefault(logger)
	return nil
}

// New builds a structured logger with global attribute redaction.
func New(opts Options) (*slog.Logger, error) {
	level := parseLevel(opts.Level)
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}

	handler := slog.NewJSONHandler(out, &slog.HandlerOptions{
		Level:       level,
		AddSource:   opts.AddSource,
		ReplaceAttr: RedactAttr,
	})
	logger := slog.New(handler)

	attrs := make([]any, 0, 6)
	if opts.Service != "" {
		attrs = append(attrs, "service", opts.Service)
	}
	if opts.Version != "" {
		attrs = append(attrs, "version", opts.Version)
	}
	if opts.Env != "" {
		attrs = append(attrs, "env", opts.Env)
	}
	if len(attrs) > 0 {
		logger = logger.With(attrs...)
	}

	return logger, nil
}

func parseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// RedactAttr masks sensitive attributes before serialization.
func RedactAttr(_ []string, attr slog.Attr) slog.Attr {
	if isSensitiveKey(attr.Key) {
		return slog.String(attr.Key, redactedValue)
	}
	return attr
}

func isSensitiveKey(key string) bool {
	normalized := normalizeKey(key)
	if normalized == "" {
		return false
	}

	switch normalized {
	case "authorization", "cookie", "setcookie", "token", "accesstoken", "refreshtoken", "clientsecret", "apikey", "xapikey", "password", "secret":
		return true
	}

	return strings.HasSuffix(normalized, "token") ||
		strings.HasSuffix(normalized, "secret") ||
		strings.HasSuffix(normalized, "password") ||
		strings.HasSuffix(normalized, "apikey")
}

func normalizeKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	replacer := strings.NewReplacer("-", "", "_", "", ".", "", " ", "")
	return replacer.Replace(key)
}
