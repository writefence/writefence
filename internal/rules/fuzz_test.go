package rules_test

import (
	"testing"

	"github.com/writefence/writefence/internal/rules"
)

func FuzzCyrillicRatio(f *testing.F) {
	f.Add("")
	f.Add("[STATUS] current work")
	f.Add("[STATUS] это русский текст")
	f.Add("\xff\xfe\xfd")

	f.Fuzz(func(t *testing.T, text string) {
		ratio := rules.CyrillicRatio(text)
		if ratio < 0 || ratio > 1 {
			t.Fatalf("ratio out of range: %f", ratio)
		}
	})
}

func FuzzRuleEvaluate(f *testing.F) {
	f.Add("[STATUS] current work")
	f.Add("missing prefix")
	f.Add("[STATUS] это русский текст")
	f.Add("\xff\xfe\xfd")

	english := rules.NewEnglishRule(0.05)
	prefix := rules.NewPrefixRule(nil)

	f.Fuzz(func(t *testing.T, text string) {
		doc := rules.Document{
			Path:   "documents/text",
			Method: "POST",
			Text:   text,
		}
		_ = english.Evaluate(doc)
		_ = prefix.Evaluate(doc)
	})
}
