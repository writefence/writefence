package rules_test

import (
	"testing"

	"github.com/writefence/writefence/internal/rules"
)

func TestCyrillicRatio(t *testing.T) {
	cases := []struct {
		input    string
		wantHigh bool // > 0.05
	}{
		{"Hello world", false},
		{"Привет мир", true},
		{"[STATUS] current work", false},
		{"[STATUS] текущая работа описание", true},
		{"", false},
		{"abc абв", true}, // ~50% cyrillic
	}
	for _, c := range cases {
		r := rules.CyrillicRatio(c.input)
		high := r > 0.05
		if high != c.wantHigh {
			t.Errorf("CyrillicRatio(%q) = %.3f, wantHigh=%v", c.input, r, c.wantHigh)
		}
	}
}

func TestEnglishRuleViolation(t *testing.T) {
	rule := rules.NewEnglishRule(0.05)
	v := rule.Evaluate(rules.Document{
		Path:   "documents/text",
		Method: "POST",
		Body:   []byte(`{"text":"Привет мир это тест"}`),
		Text:   "Привет мир это тест",
	})
	if v == nil {
		t.Fatal("expected violation for Russian text")
	}
	if v.Rule != "english_only" {
		t.Errorf("expected rule=english_only, got %s", v.Rule)
	}
	if v.State != rules.StateBlocked {
		t.Errorf("expected blocked state, got %s", v.State)
	}
	if v.ReasonCode != "non_english_content" {
		t.Errorf("expected reason_code non_english_content, got %s", v.ReasonCode)
	}
	// Suggestion must be present
	if v.Suggestion == "" {
		t.Error("expected non-empty suggestion in violation")
	}
}

func TestEnglishRuleWarnsOnMixedLanguage(t *testing.T) {
	rule := rules.NewEnglishRule(0.05)
	v := rule.Evaluate(rules.Document{
		Path:   "documents/text",
		Method: "POST",
		Text:   "[STATUS] current work detail yaя",
	})
	if v == nil {
		t.Fatal("expected warning for mixed-language text")
	}
	if v.State != rules.StateWarned {
		t.Fatalf("expected warned state, got %s", v.State)
	}
	if v.ReasonCode != "mixed_language_warning" {
		t.Fatalf("expected mixed_language_warning, got %s", v.ReasonCode)
	}
	if v.Retryable {
		t.Fatal("warning should not be retryable")
	}
}

func TestEnglishRulePass(t *testing.T) {
	rule := rules.NewEnglishRule(0.05)
	v := rule.Evaluate(rules.Document{
		Path:   "documents/text",
		Method: "POST",
		Body:   []byte(`{"text":"[STATUS] current work done"}`),
		Text:   "[STATUS] current work done",
	})
	if v != nil {
		t.Fatalf("expected no violation for English text, got: %+v", v)
	}
}

func TestEnglishRuleNonWritePath(t *testing.T) {
	rule := rules.NewEnglishRule(0.05)
	// Rule only fires on documents/text POST — query path must pass through
	v := rule.Evaluate(rules.Document{
		Path:   "query",
		Method: "POST",
		Body:   []byte(`{"query":"какой статус"}`),
		Text:   "",
	})
	if v != nil {
		t.Fatal("english rule must not fire on query path")
	}
}
