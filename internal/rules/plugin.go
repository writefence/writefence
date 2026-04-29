package rules

// ViolationState is the high-level outcome of rule evaluation.
type ViolationState string

const (
	StateAllowed     ViolationState = "allowed"
	StateWarned      ViolationState = "warned"
	StateQuarantined ViolationState = "quarantined"
	StateBlocked     ViolationState = "blocked"
)

// Document represents an intercepted HTTP request to the knowledge store.
// Text/Description are pre-parsed from the JSON body when available.
type Document struct {
	Path        string // e.g. "documents/text"
	Method      string // e.g. "POST"
	Body        []byte // raw JSON body
	Text        string // pre-parsed .text field; empty if not a doc write or parse failed
	Description string // pre-parsed .description field when present
}

// Violation is returned by a rule when the check fails.
type Violation struct {
	Rule           string         `json:"rule"`
	State          ViolationState `json:"state,omitempty"`
	ReasonCode     string         `json:"reason_code,omitempty"`
	Reason         string         `json:"reason"`
	Suggestion     string         `json:"suggestion,omitempty"`
	Retryable      bool           `json:"retryable"`
	RetryAfter     string         `json:"retry_after,omitempty"`
	ReviewRequired bool           `json:"review_required,omitempty"`
}

// StateOrDefault preserves backward compatibility for rules that still return a
// bare violation without explicitly setting a state.
func (v *Violation) StateOrDefault() ViolationState {
	if v == nil || v.State == "" {
		return StateBlocked
	}
	return v.State
}

// RulePlugin is the stable rule contract used by built-in rules today and
// external rule loaders in later phases.
type RulePlugin interface {
	Name() string
	Evaluate(doc Document) *Violation
}
