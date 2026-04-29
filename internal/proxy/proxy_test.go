package proxy_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/writefence/writefence/internal/config"
	"github.com/writefence/writefence/internal/embed"
	"github.com/writefence/writefence/internal/metrics"
	"github.com/writefence/writefence/internal/proxy"
	"github.com/writefence/writefence/internal/rules"
	"github.com/writefence/writefence/internal/state"
	"github.com/writefence/writefence/internal/violations"
	"github.com/writefence/writefence/internal/wal"
)

func newTestProxy(t *testing.T, upstream *httptest.Server) *proxy.Proxy {
	statePath := filepath.Join(t.TempDir(), "state.json")
	s := state.New(statePath)
	s.MarkQueried()
	s.MarkDecisionsChecked()
	vl := violations.NewLogger(filepath.Join(t.TempDir(), "violations.jsonl"))
	wl := wal.NewLogger(filepath.Join(t.TempDir(), "wal.jsonl"))
	cfg := config.Defaults()
	cfg.Proxy.Upstream = upstream.URL
	cfg.Proxy.QuarantineLog = filepath.Join(t.TempDir(), "quarantine.jsonl")
	return proxy.New(&cfg, s, vl, nil, nil, wl, metrics.New())
}

func newSemanticProxy(t *testing.T, upstream *httptest.Server, score float64) *proxy.Proxy {
	statePath := filepath.Join(t.TempDir(), "state.json")
	s := state.New(statePath)
	s.MarkQueried()
	s.MarkDecisionsChecked()
	vl := violations.NewLogger(filepath.Join(t.TempDir(), "violations.jsonl"))
	wl := wal.NewLogger(filepath.Join(t.TempDir(), "wal.jsonl"))
	cfg := config.Defaults()
	cfg.Proxy.Upstream = upstream.URL
	cfg.Proxy.QuarantineLog = filepath.Join(t.TempDir(), "quarantine.jsonl")

	embedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"embedding": make([]float32, 4096)})
	}))
	t.Cleanup(embedSrv.Close)

	qdrantSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusOK)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": []map[string]interface{}{
				{"id": "existing-doc", "score": score},
			},
		})
	}))
	t.Cleanup(qdrantSrv.Close)

	ec := embed.NewClient(embedSrv.URL, "qwen3-embedding:8b")
	qc := &rules.QdrantClient{BaseURL: qdrantSrv.URL, Collection: "writefence_embeddings"}
	return proxy.New(&cfg, s, vl, ec, qc, wl, metrics.New())
}

func TestProxyPassesReadThrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	p := newTestProxy(t, upstream)
	srv := httptest.NewServer(p)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/query", "application/json", bytes.NewBufferString(`{"query":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestProxyBlocksRussian(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer upstream.Close()

	p := newTestProxy(t, upstream)
	srv := httptest.NewServer(p)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"text": "[STATUS] это русский текст"})
	resp, err := http.Post(srv.URL+"/documents/text", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		t.Errorf("expected 422 for Russian text, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["rule"] != "english_only" {
		t.Errorf("expected rule=english_only in response, got %v", result["rule"])
	}
	if result["decision"] != "blocked" {
		t.Errorf("expected decision=blocked in response, got %v", result["decision"])
	}
	if result["trace_id"] == "" {
		t.Errorf("expected non-empty trace_id, got %v", result["trace_id"])
	}
}

func TestProxyBlocksMissingPrefix(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer upstream.Close()

	p := newTestProxy(t, upstream)
	srv := httptest.NewServer(p)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"text": "document without prefix"})
	resp, err := http.Post(srv.URL+"/documents/text", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		t.Errorf("expected 422 for missing prefix, got %d", resp.StatusCode)
	}
}

func TestProxyAllowsValidWrite(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"abc123"}`))
	}))
	defer upstream.Close()

	p := newTestProxy(t, upstream)
	srv := httptest.NewServer(p)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"text": "[DECISION] chose Go over Python"})
	resp, err := http.Post(srv.URL+"/documents/text", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for valid write, got %d", resp.StatusCode)
	}
	if resp.Header.Get("X-WriteFence-Decision") != "allowed" {
		t.Errorf("expected X-WriteFence-Decision=allowed, got %q", resp.Header.Get("X-WriteFence-Decision"))
	}
	if resp.Header.Get("X-WriteFence-Trace-Id") == "" {
		t.Error("expected X-WriteFence-Trace-Id header")
	}
}

func TestProxyWarnsOnMixedLanguageWrite(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"warn-123"}`))
	}))
	defer upstream.Close()

	p := newTestProxy(t, upstream)
	srv := httptest.NewServer(p)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"text": "[STATUS] current work detail yaя"})
	resp, err := http.Post(srv.URL+"/documents/text", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 for warned write, got %d", resp.StatusCode)
	}
	if resp.Header.Get("X-WriteFence-Decision") != "warned" {
		t.Fatalf("expected X-WriteFence-Decision=warned, got %q", resp.Header.Get("X-WriteFence-Decision"))
	}
	if resp.Header.Get("X-WriteFence-Warning") == "" {
		t.Fatal("expected X-WriteFence-Warning header")
	}
}

func TestProxyQuarantinesAndDoesNotForward(t *testing.T) {
	var writeCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/documents/paginated" {
			w.WriteHeader(200)
			w.Write([]byte(`{"documents":[]}`))
			return
		}
		if r.URL.Path == "/documents/text" {
			writeCalls++
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"upstream"}`))
	}))
	defer upstream.Close()

	p := newSemanticProxy(t, upstream, 0.90)
	srv := httptest.NewServer(p)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"text": "[STATUS] candidate for quarantine"})
	resp, err := http.Post(srv.URL+"/documents/text", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 for quarantined write, got %d", resp.StatusCode)
	}
	if writeCalls != 0 {
		t.Fatalf("expected zero upstream write calls for quarantined write, got %d", writeCalls)
	}
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["decision"] != "quarantined" {
		t.Fatalf("expected decision=quarantined, got %v", result["decision"])
	}
}

func TestProxyMetricsEndpoint(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer upstream.Close()

	p := newTestProxy(t, upstream)
	srv := httptest.NewServer(p)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200 from /metrics, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("expected Content-Type text/plain, got %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "writefence_requests_total") {
		t.Errorf("expected writefence_requests_total in /metrics output, got:\n%s", body)
	}
}

func TestProxyMetricsEndpointDisabled(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer upstream.Close()

	dir := t.TempDir()
	cfg := config.Defaults()
	cfg.Proxy.Upstream = upstream.URL
	cfg.Proxy.StateFile = filepath.Join(dir, "state.json")
	cfg.Proxy.ViolationsLog = filepath.Join(dir, "violations.jsonl")
	cfg.Proxy.WALLog = filepath.Join(dir, "wal.jsonl")
	cfg.Proxy.QuarantineLog = filepath.Join(dir, "quarantine.jsonl")
	cfg.Proxy.MetricsEnabled = false
	p := proxy.New(&cfg, state.New(cfg.Proxy.StateFile), violations.NewLogger(cfg.Proxy.ViolationsLog), nil, nil, wal.NewLogger(cfg.Proxy.WALLog), nil)
	srv := httptest.NewServer(p)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 from disabled /metrics, got %d", resp.StatusCode)
	}
}

func TestProxyQuarantineWritesLog(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/documents/paginated" {
			w.WriteHeader(200)
			w.Write([]byte(`{"documents":[]}`))
			return
		}
		w.WriteHeader(200)
	}))
	defer upstream.Close()

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	quarantinePath := filepath.Join(tmpDir, "quarantine.jsonl")
	s := state.New(statePath)
	s.MarkQueried()
	s.MarkDecisionsChecked()
	vl := violations.NewLogger(filepath.Join(tmpDir, "violations.jsonl"))
	wl := wal.NewLogger(filepath.Join(tmpDir, "wal.jsonl"))
	cfg := config.Defaults()
	cfg.Proxy.Upstream = upstream.URL
	cfg.Proxy.QuarantineLog = quarantinePath
	embedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"embedding": make([]float32, 4096)})
	}))
	defer embedSrv.Close()
	qdrantSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusOK)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": []map[string]interface{}{
				{"id": "existing-doc", "score": 0.90},
			},
		})
	}))
	defer qdrantSrv.Close()
	ec := embed.NewClient(embedSrv.URL, "qwen3-embedding:8b")
	qc := &rules.QdrantClient{BaseURL: qdrantSrv.URL, Collection: "writefence_embeddings"}
	p := proxy.New(&cfg, s, vl, ec, qc, wl, metrics.New())

	srv := httptest.NewServer(p)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"text": "[STATUS] candidate for quarantine"})
	resp, err := http.Post(srv.URL+"/documents/text", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	data, err := os.ReadFile(quarantinePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\"status\":\"pending\"") {
		t.Fatalf("expected pending quarantine entry, got %s", string(data))
	}
}
