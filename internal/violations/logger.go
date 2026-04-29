package violations

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry is one violation record written to the JSONL log.
type Entry struct {
	Ts      string `json:"ts"`
	Rule    string `json:"rule"`
	Path    string `json:"path"`
	Reason  string `json:"reason,omitempty"`
	Preview string `json:"preview,omitempty"`
}

// Logger appends structured JSON lines to a violations log file.
type Logger struct {
	mu   sync.Mutex
	path string
}

func NewLogger(path string) *Logger {
	return &Logger{path: path}
}

func (l *Logger) Log(e Entry) {
	e.Ts = time.Now().UTC().Format(time.RFC3339)
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.path), 0755); err != nil {
		log.Printf("[writefence/violations] cannot create log directory: %v", err)
		return
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[writefence/violations] cannot open log: %v", err)
		return
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(e)
}
