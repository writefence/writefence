// Package localfiles centralizes restrictive permissions for local WriteFence
// data that may contain memory payload previews or operator decisions.
package localfiles

import (
	"os"
	"path/filepath"
)

const (
	DirMode  os.FileMode = 0o700
	FileMode os.FileMode = 0o600
)

// EnsureParent creates the parent directory for path with owner-only access.
func EnsureParent(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, DirMode); err != nil {
		return err
	}
	if !shouldTightenDir(dir) {
		return nil
	}
	return os.Chmod(dir, DirMode)
}

// OpenAppend opens path for append, creating it with owner-only access.
func OpenAppend(path string) (*os.File, error) {
	if err := EnsureParent(path); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, FileMode)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(FileMode); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// WriteFile writes path with owner-only file permissions.
func WriteFile(path string, data []byte) error {
	if err := EnsureParent(path); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, FileMode); err != nil {
		return err
	}
	return os.Chmod(path, FileMode)
}

func shouldTightenDir(dir string) bool {
	clean := filepath.Clean(dir)
	if clean == "." || clean == string(os.PathSeparator) {
		return false
	}
	if temp := os.TempDir(); temp != "" && clean == filepath.Clean(temp) {
		return false
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" && clean == filepath.Clean(home) {
		return false
	}
	info, err := os.Stat(clean)
	if err != nil {
		return true
	}
	return info.Mode()&os.ModeSticky == 0
}
