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
