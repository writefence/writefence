package state_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/writefence/writefence/internal/state"
)

func TestStateReadWriteRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := state.New(path)

	if s.IsQueried() {
		t.Fatal("expected IsQueried=false on fresh state")
	}

	s.MarkQueried()

	s2 := state.New(path)
	if !s2.IsQueried() {
		t.Fatal("expected IsQueried=true after MarkQueried + reload")
	}
}

func TestStateUsesRestrictivePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "writefence-data")
	path := filepath.Join(dir, "state.json")
	s := state.New(path)

	s.MarkQueried()

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
		t.Fatalf("expected state file mode 0600, got %04o", got)
	}
}

func TestStateTightensExistingLoosePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "writefence-data")
	path := filepath.Join(dir, "state.json")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	state.New(path).MarkQueried()

	assertMode(t, dir, 0o700)
	assertMode(t, path, 0o600)
}

func TestStateDecisionsChecked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := state.New(path)

	if s.IsDecisionsChecked() {
		t.Fatal("expected false on fresh state")
	}
	s.MarkDecisionsChecked()

	s2 := state.New(path)
	if !s2.IsDecisionsChecked() {
		t.Fatal("expected true after mark + reload")
	}
}

func TestStateMissingFile(t *testing.T) {
	s := state.New("/tmp/writefence-nonexistent-state-xyz.json")
	// Missing file must fail open (not block)
	if s.IsQueried() {
		t.Fatal("missing file should return false, not error")
	}
	// Cleanup just in case
	os.Remove("/tmp/writefence-nonexistent-state-xyz.json")
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
