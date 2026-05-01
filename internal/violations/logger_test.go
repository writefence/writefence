package violations_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/writefence/writefence/internal/violations"
)

func TestLoggerWritesJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "violations.jsonl")
	l := violations.NewLogger(path)

	l.Log(violations.Entry{
		Rule:    "english_only",
		Path:    "documents/text",
		Reason:  "42% Cyrillic",
		Preview: "[STATUS] текущая работа",
	})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("log file not created:", err)
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("not valid JSON: %v\ndata: %s", err, data)
	}
	if entry["rule"] != "english_only" {
		t.Errorf("expected rule=english_only, got %v", entry["rule"])
	}
	if _, ok := entry["ts"]; !ok {
		t.Error("expected ts field in log entry")
	}
}

func TestLoggerUsesRestrictivePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "writefence-data")
	path := filepath.Join(dir, "violations.jsonl")
	l := violations.NewLogger(path)

	l.Log(violations.Entry{Rule: "prefix_required", Path: "documents/text"})

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
		t.Fatalf("expected violations file mode 0600, got %04o", got)
	}
}

func TestLoggerTightensExistingLoosePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "writefence-data")
	path := filepath.Join(dir, "violations.jsonl")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	violations.NewLogger(path).Log(violations.Entry{Rule: "prefix_required", Path: "documents/text"})

	assertMode(t, dir, 0o700)
	assertMode(t, path, 0o600)
}

func TestLoggerAppendsMultiple(t *testing.T) {
	path := filepath.Join(t.TempDir(), "violations.jsonl")
	l := violations.NewLogger(path)

	for i := 0; i < 3; i++ {
		l.Log(violations.Entry{Rule: "prefix_required", Path: "documents/text"})
		time.Sleep(time.Millisecond)
	}

	data, _ := os.ReadFile(path)
	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if lines != 3 {
		t.Errorf("expected 3 log lines, got %d", lines)
	}
}

func TestLoggerNoFileNoError(t *testing.T) {
	l := violations.NewLogger("/tmp/writefence-test-violations-noop.jsonl")
	l.Log(violations.Entry{Rule: "test"})
	os.Remove("/tmp/writefence-test-violations-noop.jsonl")
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
