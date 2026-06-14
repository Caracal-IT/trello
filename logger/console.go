package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// ANSI escape sequences used by the text formatter.
const (
	ansiReset   = "\033[0m"
	ansiGray    = "\033[90m"
	ansiCyan    = "\033[36m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiRed     = "\033[31m"
	ansiBoldRed = "\033[1;31m"
	ansiMagenta = "\033[35m"
)

// ConsoleSink writes log entries to stdout.
//
//   - format "text"  -> human-readable, optionally with ANSI colour
//   - format "json"  -> one JSON object per line (ready for Filebeat/Fluentd)
type ConsoleSink struct {
	out       io.Writer
	useJSON   bool
	colorized bool
}

// NewConsoleSink returns a ready-to-use ConsoleSink.
//
//	NewConsoleSink("text", true)   // dev: coloured human output
//	NewConsoleSink("json", false)  // prod: one-line JSON per entry
func NewConsoleSink(format string, colorized bool) *ConsoleSink {
	return &ConsoleSink{
		out:       os.Stdout,
		useJSON:   format == "json",
		colorized: colorized,
	}
}

func (s *ConsoleSink) Write(e LogEntry) {
	if s.useJSON {
		s.writeJSON(e)
	} else {
		s.writeText(e)
	}
}

func (s *ConsoleSink) Close() error { return nil }

// text format
//
// 2026-06-14 10:30:15.234 UTC [INF] [a1b2c3d4] [publisher       ] Temperature reading 20.3C published
// gray timestamp              col   gray corr  gray src (16 col)  default message

func (s *ConsoleSink) writeText(e LogEntry) {
	ts := s.paint(ansiGray, e.Timestamp.UTC().Format("2006-01-02 15:04:05.000")+" UTC")
	lvl := s.levelBadge(e.Level)
	corr := s.paint(ansiGray, "["+shortID(e.CorrelationID)+"]")
	src := s.paint(ansiGray, fmt.Sprintf("[%-16s]", e.Source))

	fmt.Fprintf(s.out, "%s %s %s %s %s\n", ts, lvl, corr, src, e.Message)
}

func (s *ConsoleSink) levelBadge(l Level) string {
	badge := fmt.Sprintf("[%s]", l.Short())
	if !s.colorized {
		return badge
	}
	var code string
	switch l {
	case LevelDebug:
		code = ansiCyan
	case LevelInfo:
		code = ansiGreen
	case LevelWarn:
		code = ansiYellow
	case LevelError:
		code = ansiRed
	case LevelFatal:
		code = ansiBoldRed
	default:
		return badge
	}
	return code + badge + ansiReset
}

func (s *ConsoleSink) paint(code, text string) string {
	if !s.colorized {
		return text
	}
	return code + text + ansiReset
}

// JSON format
//
// {"@timestamp":"...","level":"INFO","correlationId":"...","source":"...",
//  "messageTemplate":"...","message":"...","properties":{...}}

func (s *ConsoleSink) writeJSON(e LogEntry) {
	doc := map[string]any{
		"@timestamp":      e.Timestamp.UTC().Format(time.RFC3339Nano),
		"level":           e.Level.String(),
		"correlationId":   e.CorrelationID,
		"source":          e.Source,
		"messageTemplate": e.MessageTemplate,
		"message":         e.Message,
	}
	if len(e.Properties) > 0 {
		doc["properties"] = e.Properties
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return
	}
	s.out.Write(b)            //nolint:errcheck
	s.out.Write([]byte{'\n'}) //nolint:errcheck
}

// helpers

// shortID returns the first 8 hex digits of id, suitable for compact display.
// UUID format is xxxxxxxx-xxxx-... so id[:8] is always hex, no hyphens.
func shortID(id string) string {
	const width = 8
	if id == "" {
		return "--------"
	}
	if len(id) >= width {
		return id[:width]
	}
	return fmt.Sprintf("%-*s", width, id)
}
