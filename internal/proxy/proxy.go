package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/writefence/writefence/internal/admission"
	"github.com/writefence/writefence/internal/config"
	"github.com/writefence/writefence/internal/metrics"
	"github.com/writefence/writefence/internal/quarantine"
	"github.com/writefence/writefence/internal/rules"
	"github.com/writefence/writefence/internal/state"
	lightragstore "github.com/writefence/writefence/internal/store/lightrag"
	"github.com/writefence/writefence/internal/ui"
	"github.com/writefence/writefence/internal/violations"
	"github.com/writefence/writefence/internal/wal"
)

// Proxy is an HTTP reverse proxy with rule enforcement on LightRAG write paths.
type Proxy struct {
	rp      *httputil.ReverseProxy
	checks  []rules.RulePlugin
	state   *state.State
	logger  *violations.Logger
	wal     *wal.Logger
	qs      *quarantine.Store
	metrics *metrics.Registry
	ui      *ui.Handler
}

type decisionContextKey struct{}

// New creates a Proxy from a Config that forwards to cfg.Proxy.Upstream and enforces all rules.
// ec and qc may be nil — semantic dedup is disabled when either is nil (fail-open).
// wl may be nil — WAL logging is skipped when nil (fail-open).
// m may be nil — metrics are skipped when nil (fail-open).
func New(cfg *config.Config, s *state.State, vl *violations.Logger, ec rules.EmbedClient, qc *rules.QdrantClient, wl *wal.Logger, m *metrics.Registry) *Proxy {
	target, _ := url.Parse(cfg.Proxy.Upstream)
	rp := httputil.NewSingleHostReverseProxy(target)
	rp.ModifyResponse = func(resp *http.Response) error {
		if decision, ok := resp.Request.Context().Value(decisionContextKey{}).(admission.Decision); ok {
			admission.SetHeaders(resp.Header, decision)
		}
		return nil
	}

	qs := quarantine.New(cfg.Proxy.QuarantineLog, cfg.Proxy.Upstream)
	p := &Proxy{
		rp:      rp,
		state:   s,
		logger:  vl,
		wal:     wl,
		qs:      qs,
		metrics: m,
	}
	p.ui = ui.NewHandler(*cfg, qs)

	var onMerge func()
	if m != nil {
		onMerge = func() { m.IncDedupMerge() }
	}

	p.checks = []rules.RulePlugin{
		rules.NewEnglishRule(cfg.Rules.English.Threshold),
		rules.NewPrefixRule(cfg.Rules.Prefix.Allowed),
		rules.NewShieldRule(s),
		rules.NewDedupRule(lightragstore.New(cfg.Proxy.Upstream), onMerge),
		rules.NewSemanticDedupRule(ec, qc, cfg.Rules.SemanticDedup.Threshold),
	}

	return p
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, ui.MountPath) {
		p.ui.ServeHTTP(w, r)
		return
	}

	// Serve Prometheus metrics before any other logic.
	if r.URL.Path == "/metrics" {
		if p.metrics == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		p.metrics.WritePrometheus(w)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")

	// Track query calls for session state
	if (path == "query" || path == "query/stream") && r.Method == "POST" {
		p.state.MarkQueried()
	}
	if path == "documents/paginated" && r.Method == "POST" {
		p.state.MarkDecisionsChecked()
	}

	// Only enforce rules on document writes
	if path == "documents/text" && r.Method == "POST" {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", 500)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		// Pre-parse the document fields once for all rules and WAL.
		var payload struct {
			Text        string `json:"text"`
			Description string `json:"description"`
		}
		json.Unmarshal(body, &payload) // ignore error — rules handle empty Text

		doc := rules.Document{
			Path:        path,
			Method:      r.Method,
			Body:        body,
			Text:        payload.Text,
			Description: payload.Description,
		}

		traceID := admission.NewTraceID()
		start := time.Now()
		var warned *rules.Violation
		for _, rule := range p.checks {
			if v := rule.Evaluate(doc); v != nil {
				if v.StateOrDefault() == rules.StateWarned {
					if warned == nil {
						warned = v
					}
					continue
				}

				decision := admission.FromViolation(traceID, v)
				latencyMs := time.Since(start).Milliseconds()
				p.logger.Log(violations.Entry{
					Rule:    v.Rule,
					Path:    path,
					Reason:  v.Reason,
					Preview: v.Suggestion,
				})
				if p.wal != nil {
					p.wal.Log(wal.Entry{
						Path:           path,
						Method:         r.Method,
						Doc:            wal.DocFields{Text: payload.Text, Description: payload.Description},
						Result:         decision.Decision,
						Rule:           v.Rule,
						TraceID:        decision.TraceID,
						ReasonCode:     decision.ReasonCode,
						Message:        decision.Message,
						SuggestedFix:   decision.SuggestedFix,
						Retryable:      decision.Retryable,
						ReviewRequired: decision.ReviewRequired,
						RuleEvalMs:     latencyMs,
					})
				}
				if p.metrics != nil {
					p.metrics.IncViolation(v.Rule)
					p.metrics.IncRequest(path, decision.Decision)
					p.metrics.ObserveDuration(float64(latencyMs) / 1000.0)
				}
				if decision.Decision == string(rules.StateQuarantined) {
					if p.qs != nil {
						_ = p.qs.Append(quarantine.Entry{
							TraceID:        decision.TraceID,
							Path:           path,
							Method:         r.Method,
							Doc:            wal.DocFields{Text: payload.Text, Description: payload.Description},
							Decision:       decision.Decision,
							Status:         quarantine.StatusPending,
							Rule:           decision.RuleID,
							ReasonCode:     decision.ReasonCode,
							Message:        decision.Message,
							SuggestedFix:   decision.SuggestedFix,
							ReviewRequired: decision.ReviewRequired,
						})
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusAccepted)
					_ = json.NewEncoder(w).Encode(decision)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(422)
				_ = json.NewEncoder(w).Encode(decision)
				return
			}
		}
		latencyMs := time.Since(start).Milliseconds()
		decision := admission.Allowed(traceID)
		if warned != nil {
			decision = admission.FromViolation(traceID, warned)
			p.logger.Log(violations.Entry{
				Rule:    warned.Rule,
				Path:    path,
				Reason:  warned.Reason,
				Preview: warned.Suggestion,
			})
			if p.metrics != nil {
				p.metrics.IncViolation(warned.Rule)
			}
		}
		if p.wal != nil {
			p.wal.Log(wal.Entry{
				Path:           path,
				Method:         r.Method,
				Doc:            wal.DocFields{Text: payload.Text, Description: payload.Description},
				Result:         decision.Decision,
				Rule:           decision.RuleID,
				TraceID:        decision.TraceID,
				ReasonCode:     decision.ReasonCode,
				Message:        decision.Message,
				SuggestedFix:   decision.SuggestedFix,
				Retryable:      decision.Retryable,
				ReviewRequired: decision.ReviewRequired,
				RuleEvalMs:     latencyMs,
			})
		}
		if p.metrics != nil {
			p.metrics.IncRequest(path, decision.Decision)
			p.metrics.ObserveDuration(float64(latencyMs) / 1000.0)
		}
		r = r.WithContext(context.WithValue(r.Context(), decisionContextKey{}, decision))
	}

	p.rp.ServeHTTP(w, r)
}
