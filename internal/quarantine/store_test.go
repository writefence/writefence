package quarantine_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/writefence/writefence/internal/quarantine"
	"github.com/writefence/writefence/internal/wal"
)

func TestStoreAppendAndList(t *testing.T) {
	store := quarantine.New(filepath.Join(t.TempDir(), "quarantine.jsonl"), "http://127.0.0.1:1")
	if err := store.Append(quarantine.Entry{
		TraceID: "adm_1",
		Path:    "documents/text",
		Method:  "POST",
		Doc:     wal.DocFields{Text: "[STATUS] pending"},
	}); err != nil {
		t.Fatal(err)
	}

	entries, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Status != quarantine.StatusPending {
		t.Fatalf("expected pending status, got %s", entries[0].Status)
	}
}

func TestStoreApproveForwardsAndUpdatesStatus(t *testing.T) {
	var forwarded atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	store := quarantine.New(filepath.Join(t.TempDir(), "quarantine.jsonl"), upstream.URL)
	if err := store.Append(quarantine.Entry{
		TraceID: "adm_approve",
		Path:    "documents/text",
		Method:  "POST",
		Doc:     wal.DocFields{Text: "[STATUS] quarantine candidate", Description: "desc"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.Approve("adm_approve"); err != nil {
		t.Fatal(err)
	}
	if forwarded.Load() != 1 {
		t.Fatalf("expected exactly one forwarded write, got %d", forwarded.Load())
	}

	entries, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 latest entry, got %d", len(entries))
	}
	if entries[0].Status != quarantine.StatusApproved {
		t.Fatalf("expected approved status, got %s", entries[0].Status)
	}
	if entries[0].ReviewedAt == "" {
		t.Fatal("expected reviewed_at to be set")
	}
}

func TestStoreRejectUpdatesStatusWithoutForward(t *testing.T) {
	var forwarded atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	store := quarantine.New(filepath.Join(t.TempDir(), "quarantine.jsonl"), upstream.URL)
	if err := store.Append(quarantine.Entry{
		TraceID: "adm_reject",
		Path:    "documents/text",
		Method:  "POST",
		Doc:     wal.DocFields{Text: "[STATUS] reject me"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.Reject("adm_reject"); err != nil {
		t.Fatal(err)
	}
	if forwarded.Load() != 0 {
		t.Fatalf("expected no forwarded writes, got %d", forwarded.Load())
	}

	entries, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Status != quarantine.StatusRejected {
		t.Fatalf("expected rejected status, got %s", entries[0].Status)
	}
}
