package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// ElasticConfig configures the Elasticsearch bulk-indexing sink.
type ElasticConfig struct {
	URL           string // e.g. "http://localhost:9200"
	Index         string // index name, e.g. "app-logs"
	Username      string // HTTP basic auth (empty = no auth)
	Password      string
	BulkSize      int           // flush when buffer reaches this many entries (default 100)
	FlushInterval time.Duration // periodic flush interval (default 5 s)
}

// ElasticSink asynchronously bulk-indexes log entries into Elasticsearch.
//
// Write is non-blocking: entries are queued on an internal channel. A single
// background goroutine owns the buffer and HTTP client, so there is no lock
// contention on the hot path.
//
// On Close the queue is drained and a final flush is sent before the goroutine
// exits, so no entries are silently dropped on graceful shutdown.
type ElasticSink struct {
	cfg    ElasticConfig
	client *http.Client

	// The background goroutine owns buf exclusively after Start; no mutex needed
	// because only the goroutine reads/writes buf.
	buf []LogEntry

	ch   chan LogEntry
	done chan struct{}
	wg   sync.WaitGroup
}

// NewElasticSink creates and starts an ElasticSink.
// Safe to use immediately after construction.
func NewElasticSink(cfg ElasticConfig) *ElasticSink {
	if cfg.BulkSize <= 0 {
		cfg.BulkSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}

	s := &ElasticSink{
		cfg:    cfg,
		client: &http.Client{Timeout: 15 * time.Second},
		// Channel sized to ~10× bulk so bursts don't block callers.
		ch:   make(chan LogEntry, cfg.BulkSize*10),
		done: make(chan struct{}),
	}

	s.wg.Add(1)
	go s.run()
	return s
}

// Write enqueues entry. If the internal channel is full the entry is dropped
// rather than blocking the caller — losing a log line is always preferable to
// slowing down the application.
func (s *ElasticSink) Write(entry LogEntry) {
	select {
	case s.ch <- entry:
	default:
		fmt.Fprintf(os.Stderr, "[elastic-sink] channel full – entry dropped (level=%s source=%s)\n",
			entry.Level.Short(), entry.Source)
	}
}

// Close drains the queue, sends a final bulk request, and shuts the goroutine down.
// Blocks until the goroutine has exited.
func (s *ElasticSink) Close() error {
	close(s.done)
	s.wg.Wait()
	return nil
}

// ── background goroutine ──────────────────────────────────────────────────────

func (s *ElasticSink) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.cfg.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		// New entry arrived.
		case entry := <-s.ch:
			s.buf = append(s.buf, entry)
			if len(s.buf) >= s.cfg.BulkSize {
				s.flush()
			}

		// Periodic flush so low-volume services still ship logs promptly.
		case <-ticker.C:
			s.flush()

		// Shutdown: drain remaining channel entries, then final flush.
		case <-s.done:
		drain:
			for {
				select {
				case entry := <-s.ch:
					s.buf = append(s.buf, entry)
				default:
					break drain
				}
			}
			s.flush()
			return
		}
	}
}

func (s *ElasticSink) flush() {
	if len(s.buf) == 0 {
		return
	}

	batch := make([]LogEntry, len(s.buf))
	copy(batch, s.buf)
	s.buf = s.buf[:0] // reset without releasing the underlying array

	if err := s.sendBulk(batch); err != nil {
		fmt.Fprintf(os.Stderr,
			"[elastic-sink] bulk send failed (dropped %d entries): %v\n",
			len(batch), err)
	}
}

// ── HTTP / Elasticsearch ──────────────────────────────────────────────────────

// sendBulk serialises the batch using Elasticsearch's Bulk API (NDJSON).
//
//	POST /_bulk
//	{"index":{"_index":"my-index"}}   ← action meta line
//	{"@timestamp":"…","level":"…",…}  ← document line
//	{"index":{"_index":"my-index"}}
//	…
func (s *ElasticSink) sendBulk(entries []LogEntry) error {
	var body bytes.Buffer

	for _, e := range entries {
		// Action line
		meta, _ := json.Marshal(map[string]any{
			"index": map[string]string{"_index": s.cfg.Index},
		})
		body.Write(meta)
		body.WriteByte('\n')

		// Document line
		doc, _ := json.Marshal(s.toDoc(e))
		body.Write(doc)
		body.WriteByte('\n')
	}

	req, err := http.NewRequest(http.MethodPost, s.cfg.URL+"/_bulk", &body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	if s.cfg.Username != "" {
		req.SetBasicAuth(s.cfg.Username, s.cfg.Password)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body) // drain so the connection can be reused

	if resp.StatusCode >= 400 {
		return fmt.Errorf("elasticsearch returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// toDoc converts a LogEntry to the Elasticsearch document shape.
//
//	{
//	  "@timestamp":      "2026-06-14T10:30:15.234Z",
//	  "level":           "INFO",
//	  "correlationId":   "a1b2c3d4-…",
//	  "source":          "publisher",
//	  "messageTemplate": "Temperature reading {temp}°C published",
//	  "message":         "Temperature reading 20.3°C published",
//	  "properties":      {"temp": 20.3, "sensorID": "node-01", "batch": 1}
//	}
func (s *ElasticSink) toDoc(e LogEntry) map[string]any {
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
	return doc
}
