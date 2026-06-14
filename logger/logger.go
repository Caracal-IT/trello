// Package logger provides a structured, levelled logger modelled closely on
// Serilog's design principles:
//
//   - A global minimum level with per-source overrides (longest prefix wins)
//   - Message templates with named placeholders: "Hello {name}, age {age}"
//   - Correlation IDs (UUIDs) that flow through transactions via context.Context
//   - Immutable contextual loggers (Log) derived from a central Logger engine
//   - Pluggable, concurrent-safe Sinks (console, Elasticsearch, custom)
//
// # Quick start
//
//	log := logger.New(logger.Config{
//	    Level: "info",
//	    Overrides: map[string]string{
//	        "mqtt":      "warn",   // suppress chatty mqtt internals
//	        "publisher": "debug",  // verbose for the publisher source
//	    },
//	    Sinks: []logger.Sink{logger.NewConsoleSink("text", true)},
//	})
//	defer log.Close()
//
//	ctx, appLog := log.NewTransaction(context.Background(), "main")
//	appLog.Info("Starting {app} in {env}", logger.Fields{"app": "demo", "env": "dev"})
//
//	// Share the same correlationId across components:
//	mqttLog := log.ForContext(ctx, "mqtt")
//	mqttLog.Debug("Subscribed to {topic}", logger.Fields{"topic": "sensors/#"})
//
//	// Sticky fields for the lifetime of a batch:
//	batchLog := appLog.With(logger.Fields{"batch": 7, "sensorID": "node-01"})
//	batchLog.Info("Temperature {temp}°C", logger.Fields{"temp": 22.4})
package logger

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// context key

// ctxKey is an unexported type to prevent collisions with other packages that
// store values in context.
type ctxKey struct{}

// Config

// Config is the Logger's configuration.
// It is intentionally independent of the config package so that the logger can
// be instantiated in tests or other programs without pulling in YAML loading.
type Config struct {
	// Level is the global minimum severity: "debug"|"info"|"warn"|"error"|"fatal".
	// Entries below this level are silently discarded before any allocation occurs.
	Level string

	// Overrides maps source-name prefixes to level strings, mirroring Serilog's
	// MinimumLevel.Override behaviour.
	//
	// The longest prefix that is a prefix of the entry's source wins:
	//
	//   "mqtt"        -> catches "mqtt", "mqtt.client", "mqtt.broker"
	//   "mqtt.client" -> beats the "mqtt" rule for exactly "mqtt.client"
	//
	// Example:
	//   Overrides: map[string]string{
	//       "mqtt":        "warn",
	//       "mqtt.client": "debug",  // re-enable verbose for client specifically
	//   }
	Overrides map[string]string

	// Sinks are the write targets. All sinks receive every entry that passes
	// level filtering. Sinks are called synchronously in registration order.
	Sinks []Sink
}

// Logger

type sourceOverride struct {
	prefix string
	level  Level
}

// Logger is the central logging engine. It holds the sink list and the level
// configuration. All exported methods are safe for concurrent use.
//
// Obtain contextual Log values via For, ForContext, or NewTransaction.
// Never call log methods directly on Logger; always work through a Log.
type Logger struct {
	globalLevel Level
	overrides   []sourceOverride // read-only after New; no lock needed
	sinks       []Sink
	mu          sync.RWMutex // guards sinks only (AddSink is rarely called)
}

// New creates a Logger from cfg. It is inexpensive and does not start any
// goroutines - that is the responsibility of individual Sinks.
func New(cfg Config) *Logger {
	l := &Logger{
		globalLevel: ParseLevel(cfg.Level),
		sinks:       make([]Sink, len(cfg.Sinks)),
	}
	copy(l.sinks, cfg.Sinks)

	for prefix, lvlStr := range cfg.Overrides {
		l.overrides = append(l.overrides, sourceOverride{
			prefix: prefix,
			level:  ParseLevel(lvlStr),
		})
	}
	return l
}

// AddSink appends a Sink after construction. Thread-safe.
func (l *Logger) AddSink(s Sink) {
	l.mu.Lock()
	l.sinks = append(l.sinks, s)
	l.mu.Unlock()
}

// Close flushes and closes every registered Sink in order.
// Always call this on graceful shutdown to ensure buffered sinks (e.g.
// ElasticSink) deliver all queued entries before the process exits.
func (l *Logger) Close() {
	l.mu.RLock()
	sinks := l.sinks
	l.mu.RUnlock()
	for _, s := range sinks {
		err := s.Close()
		if err != nil {
			continue
		}
	}
}

// For returns a new Log scoped to source with a freshly minted UUID as its
// correlation ID. Use this when starting a top-level operation with no parent.
func (l *Logger) For(source string) *Log {
	return &Log{
		core:          l,
		correlationID: uuid.New().String(),
		source:        source,
	}
}

// NewTransaction mints a new UUID, stores it in a child context, and returns
// both the enriched context and a Log ready to use.
//
// Pass the returned context to sub-components that call ForContext so they
// automatically share the same correlation ID without receiving a *Log directly.
//
//	ctx, log := logger.NewTransaction(ctx, "main")
//	log.Info("Request received {path}", logger.Fields{"path": r.URL.Path})
//
//	// downstream - same correlationId, different source:
//	dbLog := logger.ForContext(ctx, "db")
func (l *Logger) NewTransaction(ctx context.Context, source string) (context.Context, *Log) {
	id := uuid.New().String()
	return context.WithValue(ctx, ctxKey{}, id), &Log{
		core:          l,
		correlationID: id,
		source:        source,
	}
}

// ForContext extracts the correlation ID stored by NewTransaction from ctx and
// returns a Log with the given source. If no ID is present a fresh UUID is
// generated, so callers never need to guard against a nil Log.
//
//	func handleMessage(ctx context.Context) {
//	    log := logger.ForContext(ctx, "handler")
//	    log.Info("Processing {event}", logger.Fields{"event": "temperature"})
//	}
func (l *Logger) ForContext(ctx context.Context, source string) *Log {
	id, _ := ctx.Value(ctxKey{}).(string)
	if id == "" {
		id = uuid.New().String()
	}
	return &Log{
		core:          l,
		correlationID: id,
		source:        source,
	}
}

// effectiveLevelFor scans the override list and returns the most specific
// (longest-prefix) level that applies to source, falling back to globalLevel.
func (l *Logger) effectiveLevelFor(source string) Level {
	// l.overrides is written once in New and never mutated - no lock needed.
	best := -1
	level := l.globalLevel
	for _, o := range l.overrides {
		if strings.HasPrefix(source, o.prefix) && len(o.prefix) > best {
			best = len(o.prefix)
			level = o.level
		}
	}
	return level
}

func (l *Logger) isEnabled(source string, lvl Level) bool {
	return lvl >= l.effectiveLevelFor(source)
}

func (l *Logger) emit(entry LogEntry) {
	l.mu.RLock()
	sinks := l.sinks
	l.mu.RUnlock()
	for _, s := range sinks {
		s.Write(entry)
	}
}

// Log (contextual logger)

// Log is an immutable, contextual logger that carries a correlation ID, a
// source name, and optional sticky fields. Every method that would alter state
// returns a new *Log value; the original is always left unchanged.
//
// Create Log values only through Logger methods - never construct directly.
type Log struct {
	core          *Logger
	correlationID string // UUID shared across one logical transaction
	source        string // component / package name, e.g. "mqtt.client"
	extraFields   Fields // sticky fields merged into every entry from this Log
}

// CorrelationID returns the UUID associated with this Log's transaction.
// Pass it through external system boundaries (HTTP headers, MQTT user
// properties) so traces can be reconstructed later.
func (l *Log) CorrelationID() string { return l.correlationID }

// Source returns the component/package name attached to this Log.
func (l *Log) Source() string { return l.source }

// ForSource returns a child Log with a different source name, inheriting the
// correlation ID and all sticky fields.
//
//	clientLog := txLog.ForSource("mqtt.client")
func (l *Log) ForSource(source string) *Log {
	c := *l
	c.source = source
	return &c
}

// WithCorrelationID returns a child Log with a specific correlation ID.
// Use this when the ID arrives from outside (e.g. an MQTT UserProperty or an
// HTTP X-Correlation-ID header) so inbound traces can be continued.
func (l *Log) WithCorrelationID(id string) *Log {
	c := *l
	c.correlationID = id
	return &c
}

// With returns a child Log with additional sticky fields merged in.
// Sticky fields are included in the Properties of every subsequent log entry
// produced by the returned Log, without having to be repeated at each call site.
//
//	// All entries from batchLog will carry sensorID and batch automatically.
//	batchLog := pubLog.With(logger.Fields{"sensorID": "node-01", "batch": 7})
//	batchLog.Info("Temperature {temp}°C", logger.Fields{"temp": 22.4})
//	// -> properties: {sensorID:"node-01", batch:7, temp:22.4}
func (l *Log) With(fields Fields) *Log {
	merged := make(Fields, len(l.extraFields)+len(fields))
	for k, v := range l.extraFields {
		merged[k] = v
	}
	for k, v := range fields {
		merged[k] = v
	}
	c := *l
	c.extraFields = merged
	return &c
}

// Levelled write methods

// Debug emits an entry at DEBUG level.
// Suppressed unless the effective level for this source is DEBUG.
func (l *Log) Debug(template string, fields ...Fields) { l.write(LevelDebug, template, fields) }

// Info emits an entry at INFO level.
func (l *Log) Info(template string, fields ...Fields) { l.write(LevelInfo, template, fields) }

// Warn emits an entry at WARN level.
func (l *Log) Warn(template string, fields ...Fields) { l.write(LevelWarn, template, fields) }

// Error emits an entry at ERROR level.
// Errors are always logged unless the effective level is FATAL or OFF.
func (l *Log) Error(template string, fields ...Fields) { l.write(LevelError, template, fields) }

// Fatal emits an entry at FATAL level and then calls os.Exit(1).
// All sinks are flushed via Logger.Close before the process exits, so
// buffered sinks (e.g. ElasticSink) are not silently truncated.
func (l *Log) Fatal(template string, fields ...Fields) {
	l.write(LevelFatal, template, fields)
	l.core.Close()
	panic("logger.Fatal: " + RenderTemplate(template, mergeFields(l.extraFields, fields)))
	// Callers that need os.Exit instead of panic can recover; the panic is used
	// so that stack traces are preserved and deferred functions still run.
}

// internal

// write is the single hot-path dispatch point for all levelled methods.
//
//  1. Cheap level check (no allocation on the miss path).
//  2. Single allocation that merges sticky + call-site fields.
//  3. Template render.
//  4. Fan-out to sinks.
func (l *Log) write(level Level, template string, fieldSets []Fields) {
	if !l.core.isEnabled(l.source, level) {
		return
	}

	props := mergeFields(l.extraFields, fieldSets)

	l.core.emit(LogEntry{
		Timestamp:       time.Now().UTC(),
		Level:           level,
		CorrelationID:   l.correlationID,
		Source:          l.source,
		MessageTemplate: template,
		Message:         RenderTemplate(template, props),
		Properties:      props,
	})
}

// mergeFields creates a single Fields map from a base and one or more overlay
// sets. Later sets win on key collisions. Returns nil if all inputs are empty.
func mergeFields(base Fields, overlays []Fields) Fields {
	total := len(base)
	for _, f := range overlays {
		total += len(f)
	}
	if total == 0 {
		return nil
	}
	out := make(Fields, total)
	for k, v := range base {
		out[k] = v
	}
	for _, f := range overlays {
		for k, v := range f {
			out[k] = v
		}
	}
	return out
}
