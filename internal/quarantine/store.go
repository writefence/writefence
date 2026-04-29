package quarantine

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/writefence/writefence/internal/wal"
)

const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
)

// Entry is one quarantined write plus its review status.
type Entry struct {
	Ts             string        `json:"ts"`
	ReviewedAt     string        `json:"reviewed_at,omitempty"`
	TraceID        string        `json:"trace_id"`
	Path           string        `json:"path"`
	Method         string        `json:"method"`
	Doc            wal.DocFields `json:"doc"`
	Decision       string        `json:"decision"`
	Status         string        `json:"status"`
	Rule           string        `json:"rule,omitempty"`
	ReasonCode     string        `json:"reason_code,omitempty"`
	Message        string        `json:"message,omitempty"`
	SuggestedFix   string        `json:"suggested_fix,omitempty"`
	ReviewRequired bool          `json:"review_required"`
}

// Store persists quarantined writes in a local append-only JSONL file.
type Store struct {
	path     string
	upstream string
	client   *http.Client
	mu       sync.Mutex
}

func New(path, upstream string) *Store {
	return &Store{
		path:     path,
		upstream: strings.TrimRight(upstream, "/"),
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *Store) Append(entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.appendLocked(entry)
}

func (s *Store) List() ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listLocked()
}

func (s *Store) Approve(traceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.listLocked()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.TraceID != traceID {
			continue
		}
		if entry.Status != StatusPending {
			return fmt.Errorf("trace_id %s is already %s", traceID, entry.Status)
		}
		if err := s.forward(entry); err != nil {
			return err
		}
		entry.Status = StatusApproved
		entry.ReviewedAt = time.Now().UTC().Format(time.RFC3339)
		return s.appendLocked(entry)
	}
	return fmt.Errorf("trace_id %s not found", traceID)
}

func (s *Store) Reject(traceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.listLocked()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.TraceID != traceID {
			continue
		}
		if entry.Status != StatusPending {
			return fmt.Errorf("trace_id %s is already %s", traceID, entry.Status)
		}
		entry.Status = StatusRejected
		entry.ReviewedAt = time.Now().UTC().Format(time.RFC3339)
		return s.appendLocked(entry)
	}
	return fmt.Errorf("trace_id %s not found", traceID)
}

func (s *Store) forward(entry Entry) error {
	body, err := json.Marshal(map[string]string{
		"text":        entry.Doc.Text,
		"description": entry.Doc.Description,
	})
	if err != nil {
		return err
	}
	resp, err := s.client.Post(s.upstream+"/documents/text", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("forward failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

func (s *Store) listLocked() ([]Entry, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	latest := map[string]Entry{}
	var order []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, err
		}
		if _, seen := latest[entry.TraceID]; !seen {
			order = append(order, entry.TraceID)
		}
		latest[entry.TraceID] = entry
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(order))
	for _, traceID := range order {
		entries = append(entries, latest[traceID])
	}
	return entries, nil
}

func (s *Store) appendLocked(entry Entry) error {
	if entry.Ts == "" {
		entry.Ts = time.Now().UTC().Format(time.RFC3339)
	}
	if entry.Decision == "" {
		entry.Decision = "quarantined"
	}
	if entry.Status == "" {
		entry.Status = StatusPending
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	return enc.Encode(entry)
}
