package ui

import (
	"bufio"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/writefence/writefence/internal/config"
	"github.com/writefence/writefence/internal/quarantine"
	"github.com/writefence/writefence/internal/replay"
	"github.com/writefence/writefence/internal/wal"
)

const MountPath = "/_writefence"

type Handler struct {
	cfg config.Config
	qs  *quarantine.Store
}

type statusItem struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Detail string `json:"detail"`
}

type ruleItem struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	Detail  string `json:"detail"`
}

type replayResponse struct {
	LastRun   string          `json:"last_run"`
	Evaluated int             `json:"evaluated_count"`
	Changed   int             `json:"changed_decision_count"`
	Results   []replay.Result `json:"results"`
	Error     string          `json:"error,omitempty"`
}

func NewHandler(cfg config.Config, qs *quarantine.Store) *Handler {
	return &Handler{cfg: cfg, qs: qs}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, MountPath)
	if path == "" || path == "/" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(indexHTML))
		return
	}
	if !strings.HasPrefix(path, "/api/") {
		http.NotFound(w, r)
		return
	}

	switch {
	case r.Method == http.MethodGet && path == "/api/overview":
		h.writeJSON(w, h.overview())
	case r.Method == http.MethodGet && path == "/api/violations":
		h.writeJSON(w, h.violations())
	case r.Method == http.MethodGet && path == "/api/quarantine":
		h.writeJSON(w, h.quarantineEntries())
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/api/quarantine/"):
		h.quarantineAction(w, path)
	case r.Method == http.MethodPost && path == "/api/replay":
		h.runReplay(w)
	case r.Method == http.MethodGet && path == "/api/config":
		h.writeJSON(w, h.configView())
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) overview() map[string]interface{} {
	entries, err := readWALEntries(h.cfg.Proxy.WALLog)
	counters := map[string]int{"allowed": 0, "warned": 0, "quarantined": 0, "blocked": 0}
	for _, entry := range entries {
		if _, ok := counters[entry.Result]; ok {
			counters[entry.Result]++
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Ts > entries[j].Ts })
	if len(entries) > 50 {
		entries = entries[:50]
	}
	return map[string]interface{}{
		"status":   h.status(),
		"counters": counters,
		"feed":     entries,
		"rules":    h.rules(),
		"error":    errString(err),
	}
}

func (h *Handler) violations() map[string]interface{} {
	entries, err := readWALEntries(h.cfg.Proxy.WALLog)
	filtered := make([]wal.Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.Result == "blocked" || entry.Result == "warned" || entry.Result == "quarantined" {
			filtered = append(filtered, entry)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Ts > filtered[j].Ts })
	return map[string]interface{}{"entries": filtered, "error": errString(err)}
}

func (h *Handler) quarantineEntries() map[string]interface{} {
	entries, err := h.qs.List()
	sort.Slice(entries, func(i, j int) bool { return entries[i].Ts > entries[j].Ts })
	return map[string]interface{}{"entries": entries, "error": errString(err)}
}

func (h *Handler) quarantineAction(w http.ResponseWriter, path string) {
	parts := strings.Split(strings.TrimPrefix(path, "/api/quarantine/"), "/")
	if len(parts) != 2 || parts[0] == "" {
		http.Error(w, "expected /api/quarantine/{trace_id}/{approve|reject}", http.StatusBadRequest)
		return
	}
	var err error
	switch parts[1] {
	case "approve":
		err = h.qs.Approve(parts[0])
	case "reject":
		err = h.qs.Reject(parts[0])
	default:
		http.Error(w, "unknown quarantine action", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.writeJSON(w, map[string]string{"status": parts[1], "trace_id": parts[0]})
}

func (h *Handler) runReplay(w http.ResponseWriter) {
	engine := replay.New(h.cfg)
	results, err := engine.Run(h.cfg.Proxy.WALLog)
	resp := replayResponse{LastRun: time.Now().UTC().Format(time.RFC3339)}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			h.writeJSON(w, resp)
			return
		}
		resp.Error = err.Error()
		w.WriteHeader(http.StatusBadRequest)
		h.writeJSON(w, resp)
		return
	}
	resp.Results = results
	resp.Evaluated = len(results)
	for _, result := range results {
		if result.Changed {
			resp.Changed++
		}
	}
	h.writeJSON(w, resp)
}

func (h *Handler) configView() map[string]interface{} {
	return map[string]interface{}{
		"proxy": map[string]interface{}{
			"addr":            h.cfg.Proxy.Addr,
			"upstream":        h.cfg.Proxy.Upstream,
			"state_file":      h.cfg.Proxy.StateFile,
			"violations_log":  h.cfg.Proxy.ViolationsLog,
			"wal_log":         h.cfg.Proxy.WALLog,
			"quarantine_log":  h.cfg.Proxy.QuarantineLog,
			"metrics_enabled": h.cfg.Proxy.MetricsEnabled,
		},
		"rules": h.rules(),
	}
}

func (h *Handler) status() []statusItem {
	return []statusItem{
		{Name: "proxy", State: "online", Detail: h.cfg.Proxy.Addr},
		{Name: "upstream", State: "configured", Detail: h.cfg.Proxy.Upstream},
		fileStatus("WAL", h.cfg.Proxy.WALLog),
		fileStatus("quarantine", h.cfg.Proxy.QuarantineLog),
		{Name: "rule engine", State: "enabled", Detail: "built-in local rules"},
	}
}

func (h *Handler) rules() []ruleItem {
	return []ruleItem{
		{ID: "english_only", Enabled: true, Detail: "threshold " + floatString(h.cfg.Rules.English.Threshold)},
		{ID: "prefix_required", Enabled: len(h.cfg.Rules.Prefix.Allowed) > 0, Detail: strings.Join(h.cfg.Rules.Prefix.Allowed, ", ")},
		{ID: "context_shield", Enabled: true, Detail: "requires decision review before [DECISION] writes"},
		{ID: "status_dedup", Enabled: true, Detail: "deduplicates [STATUS] writes"},
		{ID: "semantic_dedup", Enabled: h.cfg.Rules.SemanticDedup.EmbedURL != "" && h.cfg.Rules.SemanticDedup.QdrantURL != "", Detail: "threshold " + floatString(h.cfg.Rules.SemanticDedup.Threshold)},
	}
}

func readWALEntries(path string) ([]wal.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var entries []wal.Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry wal.Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return entries, err
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

func fileStatus(name, path string) statusItem {
	if _, err := os.Stat(path); err == nil {
		return statusItem{Name: name, State: "ready", Detail: path}
	}
	return statusItem{Name: name, State: "empty", Detail: path}
}

func (h *Handler) writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func floatString(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
