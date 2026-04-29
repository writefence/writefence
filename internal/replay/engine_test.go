package replay_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/writefence/writefence/internal/config"
	"github.com/writefence/writefence/internal/replay"
)

func TestReplayDetectsDecisionChanges(t *testing.T) {
	walPath := filepath.Join(t.TempDir(), "wal.jsonl")
	data := "" +
		`{"ts":"2026-04-28T09:00:00Z","path":"documents/text","method":"POST","doc":{"text":"[STATUS] clean english"},"result":"allowed","trace_id":"adm_allowed"}` + "\n" +
		`{"ts":"2026-04-28T09:01:00Z","path":"documents/text","method":"POST","doc":{"text":"[STATUS] current work detail yaя"},"result":"allowed","trace_id":"adm_warn"}` + "\n" +
		`{"ts":"2026-04-28T09:02:00Z","path":"documents/text","method":"POST","doc":{"text":"document without prefix"},"result":"allowed","trace_id":"adm_block"}` + "\n"
	if err := os.WriteFile(walPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := replay.New(config.Defaults())
	results, err := engine.Run(walPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].Changed {
		t.Fatal("expected first result to remain allowed")
	}
	if results[1].NewDecision != "warned" || !results[1].Changed {
		t.Fatalf("expected second result to change to warned, got %+v", results[1])
	}
	if results[1].ReasonCode != "mixed_language_warning" {
		t.Fatalf("expected mixed_language_warning, got %s", results[1].ReasonCode)
	}
	if results[2].NewDecision != "blocked" || !results[2].Changed {
		t.Fatalf("expected third result to change to blocked, got %+v", results[2])
	}
	if results[2].Rule != "prefix_required" {
		t.Fatalf("expected prefix_required rule, got %s", results[2].Rule)
	}
}

func TestReplaySkipsNonWriteEntries(t *testing.T) {
	walPath := filepath.Join(t.TempDir(), "wal.jsonl")
	data := "" +
		`{"ts":"2026-04-28T09:00:00Z","path":"query","method":"POST","doc":{"text":"ignored"},"result":"allowed","trace_id":"adm_query"}` + "\n" +
		`{"ts":"2026-04-28T09:01:00Z","path":"documents/text","method":"POST","doc":{"text":"[STATUS] clean english"},"result":"allowed","trace_id":"adm_write"}` + "\n"
	if err := os.WriteFile(walPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := replay.New(config.Defaults())
	results, err := engine.Run(walPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].TraceID != "adm_write" {
		t.Fatalf("expected adm_write, got %s", results[0].TraceID)
	}
}

func TestReplayReturnsParseErrorOnMalformedLine(t *testing.T) {
	walPath := filepath.Join(t.TempDir(), "wal.jsonl")
	if err := os.WriteFile(walPath, []byte("{not-json}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := replay.New(config.Defaults())
	if _, err := engine.Run(walPath); err == nil {
		t.Fatal("expected parse error for malformed WAL line")
	}
}
