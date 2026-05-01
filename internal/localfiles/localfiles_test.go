package localfiles_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/writefence/writefence/internal/localfiles"
)

func TestOpenAppendTightensExistingLooseParentAndFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "writefence-data")
	path := filepath.Join(dir, "wal.jsonl")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := localfiles.OpenAppend(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("next\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	assertMode(t, dir, 0o700)
	assertMode(t, path, 0o600)
}

func TestWriteFileTightensExistingLooseParentAndFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "writefence-data")
	path := filepath.Join(dir, "state.json")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := localfiles.WriteFile(path, []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}

	assertMode(t, dir, 0o700)
	assertMode(t, path, 0o600)
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
