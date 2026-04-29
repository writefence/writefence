package rules_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/writefence/writefence/internal/rules"
	"github.com/writefence/writefence/internal/state"
)

func TestShieldBlocksDecisionWithoutCheck(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	s := state.New(statePath) // fresh state: decisions_checked = false
	rule := rules.NewShieldRule(s)

	body, _ := json.Marshal(map[string]string{"text": "[DECISION] some decision"})
	v := rule.Evaluate(rules.Document{
		Path:   "documents/text",
		Method: "POST",
		Body:   body,
		Text:   "[DECISION] some decision",
	})
	if v == nil {
		t.Fatal("expected violation: writing [DECISION] without prior decisions check")
	}
	if v.Rule != "context_shield" {
		t.Errorf("expected rule=context_shield, got %s", v.Rule)
	}
}

func TestShieldAllowsDecisionAfterCheck(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	s := state.New(statePath)
	s.MarkDecisionsChecked()

	rule := rules.NewShieldRule(s)
	body, _ := json.Marshal(map[string]string{"text": "[DECISION] some decision"})
	v := rule.Evaluate(rules.Document{
		Path:   "documents/text",
		Method: "POST",
		Body:   body,
		Text:   "[DECISION] some decision",
	})
	if v != nil {
		t.Fatalf("expected no violation after MarkDecisionsChecked, got: %+v", v)
	}
}

func TestShieldAllowsStatusWithoutCheck(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	s := state.New(statePath) // fresh state
	rule := rules.NewShieldRule(s)

	body, _ := json.Marshal(map[string]string{"text": "[STATUS] current work"})
	v := rule.Evaluate(rules.Document{
		Path:   "documents/text",
		Method: "POST",
		Body:   body,
		Text:   "[STATUS] current work",
	})
	if v != nil {
		t.Fatalf("[STATUS] should not be blocked by context shield, got: %+v", v)
	}
}
