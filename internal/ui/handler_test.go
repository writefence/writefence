package ui_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/writefence/writefence/internal/config"
	"github.com/writefence/writefence/internal/quarantine"
	"github.com/writefence/writefence/internal/ui"
	"github.com/writefence/writefence/internal/wal"
)

func testHandler(t *testing.T) (*ui.Handler, config.Config, *quarantine.Store) {
	t.Helper()
	tmp := t.TempDir()
	cfg := config.Defaults()
	cfg.Proxy.WALLog = filepath.Join(tmp, "wal.jsonl")
	cfg.Proxy.QuarantineLog = filepath.Join(tmp, "quarantine.jsonl")
	cfg.Proxy.StateFile = filepath.Join(tmp, "state.json")
	cfg.Proxy.ViolationsLog = filepath.Join(tmp, "violations.jsonl")
	cfg.Proxy.Upstream = "http://127.0.0.1:1"
	qs := quarantine.New(cfg.Proxy.QuarantineLog, cfg.Proxy.Upstream)
	return ui.NewHandler(cfg, qs), cfg, qs
}

func TestHandlerServesIndex(t *testing.T) {
	h, _, _ := testHandler(t)
	req := httptest.NewRequest(http.MethodGet, ui.MountPath, nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("unexpected content type %q", rec.Header().Get("Content-Type"))
	}
	if body := rec.Body.String(); !strings.Contains(body, "WriteFence") || !strings.Contains(body, "Quarantine") {
		t.Fatalf("index did not include expected UI text: %s", body)
	}
}

func TestOverviewReadsWALCounters(t *testing.T) {
	h, cfg, _ := testHandler(t)
	writeWAL(t, cfg.Proxy.WALLog,
		wal.Entry{Path: "documents/text", Method: "POST", Result: "allowed", TraceID: "adm_allowed"},
		wal.Entry{Path: "documents/text", Method: "POST", Result: "blocked", Rule: "prefix_required", TraceID: "adm_blocked"},
	)
	req := httptest.NewRequest(http.MethodGet, ui.MountPath+"/api/overview", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload struct {
		Counters map[string]int `json:"counters"`
		Feed     []wal.Entry    `json:"feed"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Counters["allowed"] != 1 || payload.Counters["blocked"] != 1 {
		t.Fatalf("unexpected counters: %+v", payload.Counters)
	}
	if len(payload.Feed) != 2 {
		t.Fatalf("expected 2 feed entries, got %d", len(payload.Feed))
	}
}

func TestQuarantineRejectEndpointUpdatesEntry(t *testing.T) {
	h, _, qs := testHandler(t)
	if err := qs.Append(quarantine.Entry{
		TraceID: "adm_reject_ui",
		Path:    "documents/text",
		Method:  "POST",
		Doc:     wal.DocFields{Text: "[STATUS] review me"},
	}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, ui.MountPath+"/api/quarantine/adm_reject_ui/reject", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	entries, err := qs.List()
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Status != quarantine.StatusRejected {
		t.Fatalf("expected rejected status, got %s", entries[0].Status)
	}
}

func TestReplayEndpointReturnsChangedCount(t *testing.T) {
	h, cfg, _ := testHandler(t)
	writeWAL(t, cfg.Proxy.WALLog,
		wal.Entry{Path: "documents/text", Method: "POST", Doc: wal.DocFields{Text: "missing prefix"}, Result: "allowed", TraceID: "adm_changed"},
	)
	req := httptest.NewRequest(http.MethodPost, ui.MountPath+"/api/replay", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Evaluated int `json:"evaluated_count"`
		Changed   int `json:"changed_decision_count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Evaluated != 1 || payload.Changed != 1 {
		t.Fatalf("unexpected replay payload: %+v", payload)
	}
}

func writeWAL(t *testing.T, path string, entries ...wal.Entry) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, entry := range entries {
		if entry.Ts == "" {
			entry.Ts = "2026-04-28T10:00:00Z"
		}
		if err := enc.Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
}
