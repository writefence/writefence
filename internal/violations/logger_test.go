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
