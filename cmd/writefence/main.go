package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/writefence/writefence/internal/config"
	"github.com/writefence/writefence/internal/embed"
	"github.com/writefence/writefence/internal/metrics"
	"github.com/writefence/writefence/internal/proxy"
	"github.com/writefence/writefence/internal/rules"
	"github.com/writefence/writefence/internal/state"
	"github.com/writefence/writefence/internal/violations"
	"github.com/writefence/writefence/internal/wal"
)

func main() {
	defaults := config.Defaults()

	configFile := flag.String("config", "", "path to YAML config file (optional)")
	addr := flag.String("addr", defaults.Proxy.Addr, "address to listen on")
	upstream := flag.String("upstream", defaults.Proxy.Upstream, "upstream LightRAG URL")
	stateFile := flag.String("state", defaults.Proxy.StateFile, "session state file path")
	violLog := flag.String("violations-log", defaults.Proxy.ViolationsLog, "violations JSONL log path")
	walLog := flag.String("wal-log", defaults.Proxy.WALLog, "write-ahead log JSONL path")
	metricsEnabled := flag.Bool("metrics", defaults.Proxy.MetricsEnabled, "serve Prometheus metrics at /metrics")
	embedURL := flag.String("embed-url", defaults.Rules.SemanticDedup.EmbedURL, "Ollama URL for embeddings (empty = semantic dedup disabled)")
	embedModel := flag.String("embed-model", defaults.Rules.SemanticDedup.EmbedModel, "Ollama embedding model name")
	qdrantURL := flag.String("qdrant-url", defaults.Rules.SemanticDedup.QdrantURL, "Qdrant URL for vector search (empty = semantic dedup disabled)")
	flag.Parse()

	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// CLI flags override YAML values when explicitly provided.
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "addr":
			cfg.Proxy.Addr = *addr
		case "upstream":
			cfg.Proxy.Upstream = *upstream
		case "state":
			cfg.Proxy.StateFile = *stateFile
		case "violations-log":
			cfg.Proxy.ViolationsLog = *violLog
		case "wal-log":
			cfg.Proxy.WALLog = *walLog
		case "metrics":
			cfg.Proxy.MetricsEnabled = *metricsEnabled
		case "embed-url":
			cfg.Rules.SemanticDedup.EmbedURL = *embedURL
		case "embed-model":
			cfg.Rules.SemanticDedup.EmbedModel = *embedModel
		case "qdrant-url":
			cfg.Rules.SemanticDedup.QdrantURL = *qdrantURL
		}
	})

	s := state.New(cfg.Proxy.StateFile)
	vl := violations.NewLogger(cfg.Proxy.ViolationsLog)
	wl := wal.NewLogger(cfg.Proxy.WALLog)

	var ec rules.EmbedClient
	var qc *rules.QdrantClient
	if cfg.Rules.SemanticDedup.EmbedURL != "" && cfg.Rules.SemanticDedup.QdrantURL != "" {
		ec = embed.NewClient(cfg.Rules.SemanticDedup.EmbedURL, cfg.Rules.SemanticDedup.EmbedModel)
		qc = &rules.QdrantClient{BaseURL: cfg.Rules.SemanticDedup.QdrantURL, Collection: "writefence_embeddings"}
		ensureQdrantCollection(cfg.Rules.SemanticDedup.QdrantURL, 4096)
		fmt.Printf("writefence: semantic dedup enabled (model=%s)\n", cfg.Rules.SemanticDedup.EmbedModel)
	}

	var m *metrics.Registry
	if cfg.Proxy.MetricsEnabled {
		m = metrics.New()
	}
	p := proxy.New(&cfg, s, vl, ec, qc, wl, m)

	srv := &http.Server{Addr: cfg.Proxy.Addr, Handler: p}

	go func() {
		fmt.Printf("writefence proxy listening on %s → %s\n", cfg.Proxy.Addr, cfg.Proxy.Upstream)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("writefence shutting down")
}

func ensureQdrantCollection(qdrantURL string, dims int) {
	body := fmt.Sprintf(`{"vectors":{"size":%d,"distance":"Cosine"}}`, dims)
	req, _ := http.NewRequest("PUT",
		qdrantURL+"/collections/writefence_embeddings",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[writefence] qdrant collection setup: %v", err)
		return
	}
	resp.Body.Close()
}
