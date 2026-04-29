package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the top-level WriteFence configuration.
type Config struct {
	Proxy ProxyConfig `yaml:"proxy"`
	Rules RulesConfig `yaml:"rules"`
}

// ProxyConfig holds listener and upstream settings.
type ProxyConfig struct {
	Addr           string `yaml:"addr"`
	Upstream       string `yaml:"upstream"`
	StateFile      string `yaml:"state_file"`
	ViolationsLog  string `yaml:"violations_log"`
	WALLog         string `yaml:"wal_log"`
	QuarantineLog  string `yaml:"quarantine_log"`
	MetricsEnabled bool   `yaml:"metrics_enabled"`
}

// RulesConfig groups all rule configurations.
type RulesConfig struct {
	English       EnglishConfig       `yaml:"english"`
	Prefix        PrefixConfig        `yaml:"prefix"`
	SemanticDedup SemanticDedupConfig `yaml:"semantic_dedup"`
}

// EnglishConfig configures the English-only rule.
type EnglishConfig struct {
	Threshold float64 `yaml:"threshold"`
}

// PrefixConfig configures the required-prefix rule.
type PrefixConfig struct {
	Allowed []string `yaml:"allowed"`
}

// SemanticDedupConfig configures semantic deduplication.
type SemanticDedupConfig struct {
	Threshold  float64 `yaml:"threshold"`
	EmbedURL   string  `yaml:"embed_url"`
	EmbedModel string  `yaml:"embed_model"`
	QdrantURL  string  `yaml:"qdrant_url"`
}

// Defaults returns a Config with all production defaults pre-filled.
func Defaults() Config {
	dataDir := defaultDataDir()
	return Config{
		Proxy: ProxyConfig{
			Addr:           ":9622",
			Upstream:       "http://127.0.0.2:9621",
			StateFile:      filepath.Join(dataDir, "session-state.json"),
			ViolationsLog:  filepath.Join(dataDir, "writefence-violations.jsonl"),
			WALLog:         filepath.Join(dataDir, "writefence-wal.jsonl"),
			QuarantineLog:  filepath.Join(dataDir, "writefence-quarantine.jsonl"),
			MetricsEnabled: true,
		},
		Rules: RulesConfig{
			English: EnglishConfig{Threshold: 0.05},
			Prefix: PrefixConfig{
				Allowed: []string{"[STATUS]", "[DECISION]", "[SETUP]", "[CONFIG]", "[RUNBOOK]"},
			},
			SemanticDedup: SemanticDedupConfig{
				Threshold:  0.98,
				EmbedModel: "qwen3-embedding:8b",
			},
		},
	}
}

func defaultDataDir() string {
	if dir := os.Getenv("WRITEFENCE_DATA_DIR"); dir != "" {
		return dir
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".writefence")
	}
	return ".writefence"
}

// Load reads a YAML config file and merges it into defaults.
// If path is empty, returns defaults unchanged.
func Load(path string) (Config, error) {
	cfg := Defaults()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
