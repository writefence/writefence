package rules

import (
	"fmt"
	"strings"
)

var defaultPrefixes = []string{
	"[STATUS]", "[DECISION]", "[SETUP]", "[CONFIG]", "[RUNBOOK]",
}

// PrefixRule enforces that document text starts with a known prefix.
type PrefixRule struct {
	prefixes []string
}

// NewPrefixRule creates a PrefixRule with the given allowed prefixes.
// If prefixes is empty, the default set is used.
func NewPrefixRule(prefixes []string) *PrefixRule {
	if len(prefixes) == 0 {
		prefixes = defaultPrefixes
	}
	return &PrefixRule{prefixes: prefixes}
}

func (r *PrefixRule) Name() string {
	return "prefix_required"
}

func (r *PrefixRule) Evaluate(doc Document) *Violation {
	if doc.Path != "documents/text" || doc.Method != "POST" {
		return nil
	}
	for _, p := range r.prefixes {
		if strings.HasPrefix(doc.Text, p) {
			return nil
		}
	}
	runes := []rune(doc.Text)
	preview := doc.Text
	if len(runes) > 60 {
		preview = string(runes[:60]) + "…"
	}
	return &Violation{
		Rule:       r.Name(),
		State:      StateBlocked,
		ReasonCode: "missing_prefix",
		Reason: fmt.Sprintf(
			"Document text must start with one of: %s. Got: %q",
			strings.Join(r.prefixes, ", "),
			preview,
		),
		Retryable: true,
	}
}
