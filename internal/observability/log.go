package observability

import (
	"io"
	"log/slog"
	"strings"
)

// logLevel is process-wide. replaceObservability / reset swap it live.
var logLevel slog.LevelVar

func init() {
	logLevel.Set(slog.LevelInfo)
	// Quiet until serve installs JSON stdout. Mutations still call LogMutation.
	slog.SetDefault(NewJSONLogger(io.Discard))
}

// NewJSONLogger writes slog JSON to w. Level follows SetLevel.
func NewJSONLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: &logLevel}))
}

// SetDefaultJSON installs JSON slog as the process default logger.
func SetDefaultJSON(w io.Writer) *slog.Logger {
	l := NewJSONLogger(w)
	slog.SetDefault(l)
	return l
}

// SetLevel maps spec.observability.logLevel onto the process slog level.
func SetLevel(name string) {
	logLevel.Set(ParseLevel(name))
}

// LevelName is the current slog level token (debug|info|warn|error).
func LevelName() string {
	switch logLevel.Level() {
	case slog.LevelDebug:
		return "debug"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	default:
		return "info"
	}
}

// ParseLevel maps YAML tokens to slog. Unknown values are info.
func ParseLevel(name string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(name)) {
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

// LogMutation writes actor/operation/reason/revision at info. No raw bodies.
func LogMutation(actor, operation, reason, revision string) {
	slog.Info("mutation",
		"actor", actor,
		"operation", operation,
		"reason", reason,
		"revision", revision,
	)
}
