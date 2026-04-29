package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/writefence/writefence/internal/config"
)

func FuzzLoadYAML(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("proxy:\n  addr: ':9999'\n"))
	f.Add([]byte("rules:\n  english:\n    threshold: 0.10\n"))
	f.Add([]byte("rules:\n  prefix:\n    allowed:\n      - '[STATUS]'\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		path := filepath.Join(t.TempDir(), "writefence.yaml")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		_, _ = config.Load(path)
	})
}
