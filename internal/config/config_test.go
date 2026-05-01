package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/writefence/writefence/internal/config"
)

func TestDefaultsAreValid(t *testing.T) {
	cfg := config.Defaults()
	if cfg.Proxy.Addr != "127.0.0.1:9622" {
		t.Errorf("expected addr 127.0.0.1:9622, got %s", cfg.Proxy.Addr)
	}
	if cfg.Rules.English.Threshold != 0.05 {
		t.Errorf("expected english threshold 0.05, got %f", cfg.Rules.English.Threshold)
	}
	if cfg.Proxy.QuarantineLog == "" {
		t.Error("expected non-empty quarantine log default")
	}
	if len(cfg.Rules.Prefix.Allowed) == 0 {
		t.Error("expected non-empty prefix allowed list")
	}
}

func TestDefaultsHonorDataDirOverride(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "writefence-data")
	t.Setenv("WRITEFENCE_DATA_DIR", dataDir)

	cfg := config.Defaults()
	for name, path := range map[string]string{
		"state_file":     cfg.Proxy.StateFile,
		"violations_log": cfg.Proxy.ViolationsLog,
		"wal_log":        cfg.Proxy.WALLog,
		"quarantine_log": cfg.Proxy.QuarantineLog,
	} {
		if filepath.Dir(path) != dataDir {
			t.Errorf("%s should be under WRITEFENCE_DATA_DIR: %s", name, path)
		}
	}
}

func TestLoadEmptyPath(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Proxy.Addr != "127.0.0.1:9622" {
		t.Error("empty path should return defaults")
	}
}

func TestLoadYAMLFile(t *testing.T) {
	yaml := `
proxy:
  addr: ":9999"
  upstream: "http://localhost:8080"
rules:
  english:
    threshold: 0.10
  prefix:
    allowed:
      - "[STATUS]"
      - "[DECISION]"
`
	path := filepath.Join(t.TempDir(), "writefence.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Proxy.Addr != ":9999" {
		t.Errorf("expected :9999, got %s", cfg.Proxy.Addr)
	}
	if cfg.Rules.English.Threshold != 0.10 {
		t.Errorf("expected threshold 0.10, got %f", cfg.Rules.English.Threshold)
	}
	if len(cfg.Rules.Prefix.Allowed) != 2 {
		t.Errorf("expected 2 prefixes, got %d", len(cfg.Rules.Prefix.Allowed))
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/path/writefence.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
