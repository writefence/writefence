// Package wal provides a Write-Ahead Log for WriteFence.
// Every document write attempt (allowed or blocked) is appended as a JSON
// line to a JSONL file, enabling point-in-time recovery and policy simulation.
package wal

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DocFields holds the document payload fields captured in a WAL entry.
// Text is truncated to 500 characters to bound file growth.
type DocFields struct {
	Text        string `json:"text"`
	Description string `json:"description,omitempty"`
}

// Entry is one WAL record written to the JSONL log.
type Entry struct {
	Ts             string    `json:"ts"`
	Path           string    `json:"path"`
	Method         string    `json:"method"`
	Doc            DocFields `json:"doc"`
	Result         string    `json:"result"` // "allowed", "warned", "quarantined", "blocked"
	Rule           string    `json:"rule"`
	TraceID        string    `json:"trace_id,omitempty"`
	ReasonCode     string    `json:"reason_code,omitempty"`
	Message        string    `json:"message,omitempty"`
	SuggestedFix   string    `json:"suggested_fix,omitempty"`
	Retryable      bool      `json:"retryable"`
	ReviewRequired bool      `json:"review_required"`
	RuleEvalMs     int64     `json:"rule_eval_ms"`
}

// Logger appends structured JSON lines to a WAL JSONL file.
// It is safe for concurrent use. All errors are fail-silent — a WAL
// failure must never block a proxy request.
type Logger struct {
	mu   sync.Mutex
	path string
}

// NewLogger returns a Logger that writes to path.
// The file is created (or appended to) on the first Log call.
func NewLogger(path string) *Logger {
	return &Logger{path: path}
}

const maxTextLen = 500

// Log appends e to the JSONL file. The Ts field is always overwritten with the
// current UTC time. If doc.text exceeds 500 characters it is truncated.
// Any I/O error is logged to stderr and silently dropped.
func (l *Logger) Log(e Entry) {
	// Always stamp the entry just before writing.
	e.Ts = time.Now().UTC().Format(time.RFC3339)

	// Truncate doc.text to avoid huge WAL files.
	if len([]rune(e.Doc.Text)) > maxTextLen {
		runes := []rune(e.Doc.Text)
		e.Doc.Text = string(runes[:maxTextLen])
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(l.path), 0755); err != nil {
		log.Printf("[writefence/wal] cannot create log directory: %v", err)
		return
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[writefence/wal] cannot open log: %v", err)
		return
	}
	defer f.Close()

	_ = json.NewEncoder(f).Encode(e)
}
