package replay

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/writefence/writefence/internal/config"
	"github.com/writefence/writefence/internal/rules"
	"github.com/writefence/writefence/internal/wal"
)

// Result is one replay comparison between original and current policy outcome.
type Result struct {
	TraceID      string `json:"trace_id"`
	OrigDecision string `json:"orig_decision"`
	NewDecision  string `json:"new_decision"`
	Changed      bool   `json:"changed"`
	Rule         string `json:"rule"`
	ReasonCode   string `json:"reason_code"`
	TextPreview  string `json:"text_preview"`
}

// Engine replays WAL entries against the current lightweight policy set.
type Engine struct {
	checks []rules.RulePlugin
}

// New returns a replay engine that evaluates only deterministic local rules.
// Semantic dedup and context/session-dependent rules are intentionally skipped.
func New(cfg config.Config) *Engine {
	return &Engine{
		checks: []rules.RulePlugin{
			rules.NewEnglishRule(cfg.Rules.English.Threshold),
			rules.NewPrefixRule(cfg.Rules.Prefix.Allowed),
		},
	}
}

// Run re-evaluates WAL entries from path against the current policy.
func (e *Engine) Run(path string) ([]Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var results []Result
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry wal.Entry
		if err := jsonUnmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parse WAL entry: %w", err)
		}
		if entry.Path != "documents/text" || entry.Method != "POST" {
			continue
		}

		doc := rules.Document{
			Path:        entry.Path,
			Method:      entry.Method,
			Text:        entry.Doc.Text,
			Description: entry.Doc.Description,
		}
		newDecision, ruleID, reasonCode := e.evaluate(doc)
		results = append(results, Result{
			TraceID:      entry.TraceID,
			OrigDecision: entry.Result,
			NewDecision:  newDecision,
			Changed:      entry.Result != newDecision,
			Rule:         ruleID,
			ReasonCode:   reasonCode,
			TextPreview:  previewText(entry.Doc.Text, 60),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (e *Engine) evaluate(doc rules.Document) (decision, ruleID, reasonCode string) {
	var warned *rules.Violation
	for _, rule := range e.checks {
		if v := rule.Evaluate(doc); v != nil {
			switch v.StateOrDefault() {
			case rules.StateWarned:
				if warned == nil {
					warned = v
				}
			default:
				return string(v.StateOrDefault()), v.Rule, v.ReasonCode
			}
		}
	}
	if warned != nil {
		return string(rules.StateWarned), warned.Rule, warned.ReasonCode
	}
	return string(rules.StateAllowed), "", ""
}

func previewText(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "…"
}

// jsonUnmarshal is isolated for tests around malformed input handling.
var jsonUnmarshal = func(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
