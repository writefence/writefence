package rules_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/writefence/writefence/internal/embed"
	"github.com/writefence/writefence/internal/rules"
)

func makeVec(val float32, dims int) []float32 {
	v := make([]float32, dims)
	for i := range v {
		v[i] = val
	}
	return v
}

func TestSemanticDedupSkipsNonWrite(t *testing.T) {
	rule := rules.NewSemanticDedupRule(nil, nil, 0.98)
	v := rule.Evaluate(rules.Document{
		Path:   "query",
		Method: "POST",
		Body:   []byte(`{"query":"test"}`),
		Text:   "",
	})
	if v != nil {
		t.Fatal("semantic dedup must not fire on query path")
	}
}

func TestSemanticDedupDisabledWhenNilClients(t *testing.T) {
	rule := rules.NewSemanticDedupRule(nil, nil, 0.98)
	body, _ := json.Marshal(map[string]string{"text": "[STATUS] something"})
	v := rule.Evaluate(rules.Document{
		Path:   "documents/text",
		Method: "POST",
		Body:   body,
		Text:   "[STATUS] something",
	})
	if v != nil {
		t.Fatal("semantic dedup must be no-op when clients are nil")
	}
}

func TestSemanticDedupPassesWhenNoSimilar(t *testing.T) {
	embedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"embedding": makeVec(0.1, 4096)})
	}))
	defer embedSrv.Close()

	qdrantSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" { // upsert
			w.WriteHeader(200)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"result": []interface{}{}})
	}))
	defer qdrantSrv.Close()

	ec := embed.NewClient(embedSrv.URL, "qwen3-embedding:8b")
	qc := &rules.QdrantClient{BaseURL: qdrantSrv.URL, Collection: "writefence_embeddings"}
	rule := rules.NewSemanticDedupRule(ec, qc, 0.98)

	body, _ := json.Marshal(map[string]string{"text": "[STATUS] unique document"})
	v := rule.Evaluate(rules.Document{
		Path:   "documents/text",
		Method: "POST",
		Body:   body,
		Text:   "[STATUS] unique document",
	})
	if v != nil {
		t.Fatalf("expected pass for no similar docs, got: %+v", v)
	}
}

func TestSemanticDedupBlocksNearDuplicate(t *testing.T) {
	embedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"embedding": makeVec(0.5, 4096)})
	}))
	defer embedSrv.Close()

	qdrantSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": []map[string]interface{}{
				{"id": "existing-doc", "score": 0.99},
			},
		})
	}))
	defer qdrantSrv.Close()

	ec := embed.NewClient(embedSrv.URL, "qwen3-embedding:8b")
	qc := &rules.QdrantClient{BaseURL: qdrantSrv.URL, Collection: "writefence_embeddings"}
	rule := rules.NewSemanticDedupRule(ec, qc, 0.98)

	body, _ := json.Marshal(map[string]string{"text": "[STATUS] nearly identical document"})
	v := rule.Evaluate(rules.Document{
		Path:   "documents/text",
		Method: "POST",
		Body:   body,
		Text:   "[STATUS] nearly identical document",
	})
	if v == nil {
		t.Fatal("expected violation for near-duplicate document")
	}
	if v.Rule != "semantic_dedup" {
		t.Errorf("expected rule=semantic_dedup, got %s", v.Rule)
	}
}

func TestSemanticDedupQuarantinesPossibleDuplicate(t *testing.T) {
	embedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"embedding": makeVec(0.5, 4096)})
	}))
	defer embedSrv.Close()

	qdrantSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": []map[string]interface{}{
				{"id": "existing-doc", "score": 0.90},
			},
		})
	}))
	defer qdrantSrv.Close()

	ec := embed.NewClient(embedSrv.URL, "qwen3-embedding:8b")
	qc := &rules.QdrantClient{BaseURL: qdrantSrv.URL, Collection: "writefence_embeddings"}
	rule := rules.NewSemanticDedupRule(ec, qc, 0.98)

	body, _ := json.Marshal(map[string]string{"text": "[STATUS] similar document"})
	v := rule.Evaluate(rules.Document{
		Path:   "documents/text",
		Method: "POST",
		Body:   body,
		Text:   "[STATUS] similar document",
	})
	if v == nil {
		t.Fatal("expected quarantine violation for similar document")
	}
	if v.State != rules.StateQuarantined {
		t.Fatalf("expected quarantined state, got %s", v.State)
	}
	if v.ReasonCode != "needs_manual_review" {
		t.Fatalf("expected needs_manual_review reason, got %s", v.ReasonCode)
	}
}
