package rules_test

import (
	"encoding/json"
	"testing"

	"github.com/writefence/writefence/internal/rules"
)

func TestPrefixRuleViolation(t *testing.T) {
	rule := rules.NewPrefixRule(nil)
	v := rule.Evaluate(rules.Document{
		Path:   "documents/text",
		Method: "POST",
		Body:   []byte(`{"text":"Some document without prefix"}`),
		Text:   "Some document without prefix",
	})
	if v == nil {
		t.Fatal("expected violation for missing prefix")
	}
	if v.Rule != "prefix_required" {
		t.Errorf("expected rule=prefix_required, got %s", v.Rule)
	}
}

func TestPrefixRulePass(t *testing.T) {
	rule := rules.NewPrefixRule(nil)
	for _, prefix := range []string{"[STATUS]", "[DECISION]", "[SETUP]", "[CONFIG]", "[RUNBOOK]"} {
		body, _ := json.Marshal(map[string]string{"text": prefix + " some content"})
		v := rule.Evaluate(rules.Document{
			Path:   "documents/text",
			Method: "POST",
			Body:   body,
			Text:   prefix + " some content",
		})
		if v != nil {
			t.Errorf("prefix %q should pass, got violation: %+v", prefix, v)
		}
	}
}

func TestPrefixRuleNonWritePath(t *testing.T) {
	rule := rules.NewPrefixRule(nil)
	v := rule.Evaluate(rules.Document{
		Path:   "query",
		Method: "POST",
		Body:   []byte(`{"query":"something"}`),
		Text:   "",
	})
	if v != nil {
		t.Fatal("prefix rule must not fire on query path")
	}
}
