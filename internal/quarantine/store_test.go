package quarantine_test

import (
	"net/http"
	"net/http/httptest"
	"os"
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

func TestStoreUsesRestrictivePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "writefence-data")
	path := filepath.Join(dir, "quarantine.jsonl")
	store := quarantine.New(path, "http://127.0.0.1:1")

	if err := store.Append(quarantine.Entry{TraceID: "adm_secure", Doc: wal.DocFields{Text: "[STATUS] pending"}}); err != nil {
		t.Fatal(err)
	}

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
		t.Fatalf("expected quarantine file mode 0600, got %04o", got)
	}
}

func TestStoreTightensExistingLoosePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "writefence-data")
	path := filepath.Join(dir, "quarantine.jsonl")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	store := quarantine.New(path, "http://127.0.0.1:1")
	if err := store.Append(quarantine.Entry{TraceID: "adm_secure_existing", Doc: wal.DocFields{Text: "[STATUS] pending"}}); err != nil {
		t.Fatal(err)
	}

	assertMode(t, dir, 0o700)
	assertMode(t, path, 0o600)
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
