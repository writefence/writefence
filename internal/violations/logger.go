package violations

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/writefence/writefence/internal/localfiles"
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
	f, err := localfiles.OpenAppend(l.path)
	if err != nil {
		log.Printf("[writefence/violations] cannot open log: %v", err)
		return
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(e)
}
