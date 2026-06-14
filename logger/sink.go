package logger

// Sink is the write target for resolved log entries.
// Implementations must be safe for concurrent use from multiple goroutines.
//
// Built-in sinks:
//   - NewConsoleSink  – human-readable text or structured JSON to stdout
//   - NewElasticSink  – async bulk-indexing into Elasticsearch
//
// To add a custom sink (file, Loki, Splunk, …) implement this two-method
// interface and pass it to logger.Config.Sinks.
type Sink interface {
	// Write receives a fully resolved LogEntry. It must not block the caller;
	// if the sink needs async behaviour it should buffer internally.
	Write(entry LogEntry)

	// Close flushes any internal buffer and releases resources.
	// Called once on graceful shutdown via Logger.Close().
	Close() error
}
