package rules_test

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/writefence/writefence/internal/rules"
	"github.com/writefence/writefence/internal/store"
)

type mockStore struct {
	mu          sync.Mutex
	docs        []store.Doc
	deleteCalls [][]string
	deleted     chan struct{}
}

func newMockStore(docs []store.Doc) *mockStore {
	return &mockStore{docs: docs, deleted: make(chan struct{}, 1)}
}

func (m *mockStore) ListDocs(limit int) ([]store.Doc, error) {
	return m.docs, nil
}

func (m *mockStore) DeleteDocs(ids []string) error {
	m.mu.Lock()
	m.deleteCalls = append(m.deleteCalls, ids)
	m.mu.Unlock()
	select {
	case m.deleted <- struct{}{}:
	default:
	}
	return nil
}

func TestDedupRuleIgnoresNonStatus(t *testing.T) {
	rule := rules.NewDedupRule(newMockStore(nil), nil)
	body, _ := json.Marshal(map[string]string{"text": "[DECISION] some decision"})
	v := rule.Evaluate(rules.Document{
		Path:   "documents/text",
		Method: "POST",
		Body:   body,
		Text:   "[DECISION] some decision",
	})
	if v != nil {
		t.Fatalf("dedup rule must not fire on [DECISION]: %+v", v)
	}
}

func TestDedupRuleIgnoresReadPaths(t *testing.T) {
	rule := rules.NewDedupRule(newMockStore(nil), nil)
	v := rule.Evaluate(rules.Document{
		Path:   "query",
		Method: "POST",
		Body:   []byte(`{"query":"test"}`),
		Text:   "",
	})
	if v != nil {
		t.Fatalf("dedup rule must not fire on query: %+v", v)
	}
}

func TestDedupRuleCallsDeleteOnStatus(t *testing.T) {
	ms := newMockStore([]store.Doc{
		{ID: "doc-1", Summary: "[STATUS] old work"},
	})
	rule := rules.NewDedupRule(ms, nil)
	body, _ := json.Marshal(map[string]string{"text": "[STATUS] new current work"})
	rule.Evaluate(rules.Document{
		Path:   "documents/text",
		Method: "POST",
		Body:   body,
		Text:   "[STATUS] new current work",
	})

	select {
	case <-ms.deleted:
		// success
	case <-timeoutChan(500):
		t.Fatal("timed out waiting for DeleteDocs to be called")
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()
	if len(ms.deleteCalls) == 0 {
		t.Error("expected dedup rule to call DeleteDocs for old STATUS doc")
	}
}

func timeoutChan(ms int) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		close(ch)
	}()
	return ch
}
