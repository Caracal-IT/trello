package logger

import "time"

// Fields is an arbitrary bag of structured key-value data attached to a log
// message or baked into a Log for the lifetime of a transaction.
//
//	logger.Fields{"broker": "tcp://…", "clientID": "go-demo", "qos": 1}
type Fields map[string]any

// LogEntry is the fully-resolved record written to each Sink.
// All fields are set before the entry leaves the Log; sinks are read-only consumers.
type LogEntry struct {
	Timestamp       time.Time // UTC
	Level           Level
	CorrelationID   string // UUID that links every log line in a single transaction
	Source          string // component / package name, e.g. "mqtt.client"
	MessageTemplate string // raw template,   e.g. "Hello {name}, I am {age}"
	Message         string // rendered text,  e.g. "Hello ettiene, I am 42"
	Properties      Fields // merged call-site + sticky fields; used by sinks for indexing
}
