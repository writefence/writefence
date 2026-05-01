package wal_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/writefence/writefence/internal/wal"
)

// readEntries opens the JSONL file and decodes all lines into a slice of maps.
func readEntries(t *testing.T, path string) []map[string]interface{} {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("cannot open WAL file: %v", err)
	}
	defer f.Close()

	var entries []map[string]interface{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("invalid JSON line %q: %v", line, err)
		}
		entries = append(entries, m)
	}
	return entries
}

func TestWALWritesAllowedEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.jsonl")
	l := wal.NewLogger(path)

	l.Log(wal.Entry{
		Path:       "documents/text",
		Method:     "POST",
		Doc:        wal.DocFields{Text: "[STATUS] current work", Description: "status update"},
		Result:     "allowed",
		Rule:       "",
		RuleEvalMs: 2,
	})

	entries := readEntries(t, path)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]

	if e["ts"] == "" || e["ts"] == nil {
		t.Error("ts must be set")
	}
	if e["path"] != "documents/text" {
		t.Errorf("path mismatch: %v", e["path"])
	}
	if e["method"] != "POST" {
		t.Errorf("method mismatch: %v", e["method"])
	}
	if e["result"] != "allowed" {
		t.Errorf("result mismatch: %v", e["result"])
	}
	v, ok := e["rule"]
	if !ok {
		t.Error("expected 'rule' key to be present in allowed entry")
	} else if v != "" {
		t.Errorf("expected empty rule for allowed entry, got %q", v)
	}
	if e["rule_eval_ms"] == nil {
		t.Error("rule_eval_ms must be present")
	}

	doc, ok := e["doc"].(map[string]interface{})
	if !ok {
		t.Fatalf("doc must be an object, got %T", e["doc"])
	}
	if doc["text"] != "[STATUS] current work" {
		t.Errorf("doc.text mismatch: %v", doc["text"])
	}
	if doc["description"] != "status update" {
		t.Errorf("doc.description mismatch: %v", doc["description"])
	}
}

func TestWALUsesRestrictivePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "writefence-data")
	path := filepath.Join(dir, "wal.jsonl")
	l := wal.NewLogger(path)

	l.Log(wal.Entry{Doc: wal.DocFields{Text: "[STATUS] secure"}, Result: "allowed"})

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("expected data directory mode 0700, got %04o", got)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected WAL file mode 0600, got %04o", got)
	}
}

func TestWALTightensExistingLoosePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "writefence-data")
	path := filepath.Join(dir, "wal.jsonl")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	wal.NewLogger(path).Log(wal.Entry{Doc: wal.DocFields{Text: "[STATUS] secure"}, Result: "allowed"})

	assertMode(t, dir, 0o700)
	assertMode(t, path, 0o600)
}

func TestWALWritesBlockedEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.jsonl")
	l := wal.NewLogger(path)

	l.Log(wal.Entry{
		Path:       "documents/text",
		Method:     "POST",
		Doc:        wal.DocFields{Text: "Привет мир", Description: "test"},
		Result:     "blocked",
		Rule:       "english_only",
		RuleEvalMs: 1,
	})

	entries := readEntries(t, path)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]

	if e["result"] != "blocked" {
		t.Errorf("result must be 'blocked', got %v", e["result"])
	}
	if e["rule"] != "english_only" {
		t.Errorf("rule must be 'english_only', got %v", e["rule"])
	}
}

func TestWALAppendsMultiple(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.jsonl")
	l := wal.NewLogger(path)

	for i := 0; i < 3; i++ {
		l.Log(wal.Entry{
			Path:   "documents/text",
			Method: "POST",
			Doc:    wal.DocFields{Text: "[STATUS] entry", Description: "desc"},
			Result: "allowed",
			Rule:   "",
		})
	}

	entries := readEntries(t, path)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
}

func TestWALFailSilentOnBadPath(t *testing.T) {
	l := wal.NewLogger("/nonexistent-dir/wal.jsonl")

	// Must not panic
	l.Log(wal.Entry{
		Path:   "documents/text",
		Method: "POST",
		Doc:    wal.DocFields{Text: "[STATUS] test"},
		Result: "allowed",
	})
}

func TestWALTextTruncation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.jsonl")
	l := wal.NewLogger(path)

	// Build a string of 600 'a' characters
	longText := strings.Repeat("a", 600)

	l.Log(wal.Entry{
		Path:   "documents/text",
		Method: "POST",
		Doc:    wal.DocFields{Text: longText, Description: "trunc test"},
		Result: "allowed",
	})

	// 300 Cyrillic runes = 600 UTF-8 bytes — should NOT be truncated
	cyrShort := strings.Repeat("а", 300)
	l.Log(wal.Entry{Doc: wal.DocFields{Text: cyrShort}, Result: "allowed"})

	// 600 Cyrillic runes = 1200 UTF-8 bytes — must truncate to 500 runes
	cyrLong := strings.Repeat("а", 600)
	l.Log(wal.Entry{Doc: wal.DocFields{Text: cyrLong}, Result: "allowed"})

	entries := readEntries(t, path)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Verify ASCII truncation (entry 0)
	doc, ok := entries[0]["doc"].(map[string]interface{})
	if !ok {
		t.Fatalf("doc must be an object")
	}
	text, ok := doc["text"].(string)
	if !ok {
		t.Fatalf("doc.text must be a string")
	}
	if len(text) != 500 {
		t.Errorf("expected doc.text truncated to 500 chars, got %d", len(text))
	}

	// Verify Cyrillic short (entry 1) — not truncated
	doc1, ok := entries[1]["doc"].(map[string]interface{})
	if !ok {
		t.Fatalf("entry 1 doc must be an object")
	}
	text1, ok := doc1["text"].(string)
	if !ok {
		t.Fatalf("entry 1 doc.text must be a string")
	}
	if len([]rune(text1)) != 300 {
		t.Errorf("expected 300 runes for cyrShort, got %d", len([]rune(text1)))
	}

	// Verify Cyrillic long (entry 2) — truncated to 500 runes
	doc2, ok := entries[2]["doc"].(map[string]interface{})
	if !ok {
		t.Fatalf("entry 2 doc must be an object")
	}
	text2, ok := doc2["text"].(string)
	if !ok {
		t.Fatalf("entry 2 doc.text must be a string")
	}
	if len([]rune(text2)) != 500 {
		t.Errorf("expected 500 runes for cyrLong (truncated), got %d", len([]rune(text2)))
	}
}

func TestWALConcurrentWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.jsonl")
	l := wal.NewLogger(path)
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			l.Log(wal.Entry{
				Path:   "documents/text",
				Method: "POST",
				Doc:    wal.DocFields{Text: "[STATUS] concurrent"},
				Result: "allowed",
			})
		}()
	}
	wg.Wait()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != n {
		t.Fatalf("expected %d entries, got %d", n, len(lines))
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("expected %s mode %04o, got %04o", path, want, got)
	}
}
