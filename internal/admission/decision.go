package admission

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/writefence/writefence/internal/rules"
)

// Decision is the machine-readable admission contract returned by WriteFence.
type Decision struct {
	Decision       string `json:"decision"`
	Rule           string `json:"rule,omitempty"`
	RuleID         string `json:"rule_id,omitempty"`
	ReasonCode     string `json:"reason_code,omitempty"`
	Message        string `json:"message,omitempty"`
	SuggestedFix   string `json:"suggested_fix,omitempty"`
	Retryable      bool   `json:"retryable"`
	RetryAfter     string `json:"retry_after,omitempty"`
	ReviewRequired bool   `json:"review_required"`
	TraceID        string `json:"trace_id"`
}

// NewTraceID returns an admission decision trace id.
func NewTraceID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "adm_fallback"
	}
	return "adm_" + hex.EncodeToString(b[:])
}

// Allowed returns the minimal contract for a clean write.
func Allowed(traceID string) Decision {
	return Decision{
		Decision:  string(rules.StateAllowed),
		Retryable: false,
		TraceID:   traceID,
	}
}

// FromViolation converts a rule violation or warning into an admission decision.
func FromViolation(traceID string, v *rules.Violation) Decision {
	if v == nil {
		return Allowed(traceID)
	}
	return Decision{
		Decision:       string(v.StateOrDefault()),
		Rule:           v.Rule,
		RuleID:         v.Rule,
		ReasonCode:     v.ReasonCode,
		Message:        v.Reason,
		SuggestedFix:   v.Suggestion,
		Retryable:      v.Retryable,
		RetryAfter:     v.RetryAfter,
		ReviewRequired: v.ReviewRequired,
		TraceID:        traceID,
	}
}

// SetHeaders encodes the contract into response headers for pass-through responses.
func SetHeaders(h http.Header, d Decision) {
	h.Set("X-WriteFence-Decision", d.Decision)
	h.Set("X-WriteFence-Trace-Id", d.TraceID)
	h.Set("X-WriteFence-Retryable", strconv.FormatBool(d.Retryable))
	h.Set("X-WriteFence-Review-Required", strconv.FormatBool(d.ReviewRequired))
	if d.RuleID != "" {
		h.Set("X-WriteFence-Rule-Id", d.RuleID)
	}
	if d.ReasonCode != "" {
		h.Set("X-WriteFence-Reason-Code", d.ReasonCode)
	}
	if d.RetryAfter != "" {
		h.Set("X-WriteFence-Retry-After", d.RetryAfter)
	}
	if d.Decision == string(rules.StateWarned) && d.Message != "" {
		h.Set("X-WriteFence-Warning", d.Message)
	}
	if payload, err := json.Marshal(d); err == nil {
		h.Set("X-WriteFence-Admission-Decision", string(payload))
	}
}
