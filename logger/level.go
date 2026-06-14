package logger

import "strings"

// Level represents a log severity level.
type Level int8

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
	LevelOff // disables all output
)

// String returns the full level name (e.g. "INFO").
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
		return "OFF"
	}
}

// Short returns a fixed 3-char abbreviation for compact console output.
func (l Level) Short() string {
	switch l {
	case LevelDebug:
		return "DBG"
	case LevelInfo:
		return "INF"
	case LevelWarn:
		return "WRN"
	case LevelError:
		return "ERR"
	case LevelFatal:
		return "FTL"
	default:
		return "OFF"
	}
}

// ParseLevel converts a string to a Level (case-insensitive).
// Unrecognised strings return LevelOff.
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug", "verbose", "trace":
		return LevelDebug
	case "info", "information":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal", "critical":
		return LevelFatal
	default:
		return LevelOff
	}
}
