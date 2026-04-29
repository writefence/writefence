package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type document struct {
	ID             string `json:"id"`
	ContentSummary string `json:"content_summary"`
	Text           string `json:"text"`
	Description    string `json:"description,omitempty"`
}

type store struct {
	mu   sync.Mutex
	path string
	docs []document
}

func main() {
	addr := flag.String("addr", "127.0.0.1:9621", "listen address")
	dataDir := flag.String("data-dir", defaultDataDir(), "directory for mock-store JSONL data")
	flag.Parse()

	s := &store{path: filepath.Join(*dataDir, "mock-memory-store.jsonl")}
	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		log.Fatal(err)
	}
	if err := s.load(); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/documents/text", s.handleText)
	mux.HandleFunc("/documents/paginated", s.handlePaginated)
	mux.HandleFunc("/documents/delete_document", s.handleDelete)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	log.Printf("writefence mock memory store listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func defaultDataDir() string {
	if dir := os.Getenv("WRITEFENCE_DEMO_STORE_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(os.TempDir(), "writefence-mock-store")
}

func (s *store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var d document
		if err := json.Unmarshal([]byte(line), &d); err != nil {
			return err
		}
		s.docs = append(s.docs, d)
	}
	return nil
}

func (s *store) handleText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		Text        string `json:"text"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()
	d := document{
		ID:             fmt.Sprintf("mock_%d", now.UnixNano()),
		ContentSummary: summarize(payload.Text),
		Text:           payload.Text,
		Description:    payload.Description,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.docs = append(s.docs, d)
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(d); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"id": d.ID, "status": "stored"})
}

func (s *store) handlePaginated(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	end := req.Offset + req.Limit
	if end > len(s.docs) {
		end = len(s.docs)
	}
	docs := []document{}
	if req.Offset < len(s.docs) {
		docs = append(docs, s.docs[req.Offset:end]...)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"documents": docs,
		"total":     len(s.docs),
	})
}

func (s *store) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		DocIDs []string `json:"doc_ids"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	remove := map[string]bool{}
	for _, id := range req.DocIDs {
		remove[id] = true
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.docs[:0]
	for _, d := range s.docs {
		if !remove[d.ID] {
			kept = append(kept, d)
		}
	}
	s.docs = kept
	if err := s.rewrite(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (s *store) rewrite() error {
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, d := range s.docs {
		if err := enc.Encode(d); err != nil {
			return err
		}
	}
	return nil
}

func summarize(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) <= 160 {
		return text
	}
	return text[:157] + "..."
}
