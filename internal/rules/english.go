package rules

import (
	"fmt"
	"regexp"
	"unicode/utf8"
)

var cyrillicRE = regexp.MustCompile(`[\x{0400}-\x{04FF}]`)

// CyrillicRatio returns the fraction of Cyrillic runes in text.
func CyrillicRatio(text string) float64 {
	if text == "" {
		return 0
	}
	matches := cyrillicRE.FindAllString(text, -1)
	total := utf8.RuneCountInString(text)
	if total == 0 {
		return 0
	}
	return float64(len(matches)) / float64(total)
}

// EnglishRule enforces that LightRAG document writes are in English.
type EnglishRule struct {
	threshold float64
}

const warnThreshold = 0.02

func NewEnglishRule(threshold float64) *EnglishRule {
	return &EnglishRule{threshold: threshold}
}

func (r *EnglishRule) Name() string {
	return "english_only"
}

func (r *EnglishRule) Evaluate(doc Document) *Violation {
	if doc.Path != "documents/text" || doc.Method != "POST" {
		return nil
	}
	ratio := CyrillicRatio(doc.Text)
	if ratio <= warnThreshold {
		return nil
	}
	runes := []rune(doc.Text)
	preview := doc.Text
	if len(runes) > 80 {
		preview = string(runes[:80]) + "…"
	}
	if ratio <= r.threshold {
		return &Violation{
			Rule:       r.Name(),
			State:      StateWarned,
			ReasonCode: "mixed_language_warning",
			Reason: fmt.Sprintf(
				"Document contains %.1f%% Cyrillic characters. Write was allowed, but the memory entry mixes languages.",
				ratio*100,
			),
			Suggestion: "Prefer a fully English document to keep retrieval and dedup behavior consistent.",
			Retryable:  false,
		}
	}
	return &Violation{
		Rule:       r.Name(),
		State:      StateBlocked,
		ReasonCode: "non_english_content",
		Reason: fmt.Sprintf(
			"Document contains %.1f%% Cyrillic characters. LightRAG documents must be in English. Translate and retry.",
			ratio*100,
		),
		Suggestion: fmt.Sprintf("Detected text (first 80 chars): %q — please provide an English translation.", preview),
		Retryable:  true,
	}
}
