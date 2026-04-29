package rules

import (
	"strings"

	"github.com/writefence/writefence/internal/state"
)

// categoryRequirements maps document prefix → what state flag must be true.
var categoryRequirements = map[string]struct {
	check   func(*state.State) bool
	message string
}{
	"[DECISION]": {
		check:   func(s *state.State) bool { return s.IsDecisionsChecked() },
		message: "Writing [DECISION] requires querying existing decisions first. Run: curl -s -X POST http://localhost:9622/documents/paginated -H 'Content-Type: application/json' -d '{\"limit\":100,\"offset\":0}'",
	},
}

// ShieldRule enforces per-category session state requirements.
type ShieldRule struct {
	s *state.State
}

func NewShieldRule(s *state.State) *ShieldRule {
	return &ShieldRule{s: s}
}

func (r *ShieldRule) Name() string {
	return "context_shield"
}

func (r *ShieldRule) Evaluate(doc Document) *Violation {
	if doc.Path != "documents/text" || doc.Method != "POST" {
		return nil
	}
	for prefix, req := range categoryRequirements {
		if strings.HasPrefix(doc.Text, prefix) {
			if !req.check(r.s) {
				return &Violation{
					Rule:       r.Name(),
					State:      StateBlocked,
					ReasonCode: "query_decisions_first",
					Reason:     req.message,
					Retryable:  true,
					RetryAfter: "query_decisions_first",
				}
			}
		}
	}
	return nil
}
