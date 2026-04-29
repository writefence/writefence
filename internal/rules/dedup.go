package rules

import (
	"log"
	"strings"

	"github.com/writefence/writefence/internal/store"
)

// DedupRule deletes all existing [STATUS] docs before a new STATUS write.
// It always returns nil (never blocks — it's a side-effect rule).
type DedupRule struct {
	s       store.Store
	onMerge func() // called after each successful STATUS dedup; may be nil
}

// NewDedupRule creates a DedupRule with an optional onMerge callback.
// onMerge is called (once per successful delete batch) to record metrics;
// pass nil to disable.
func NewDedupRule(s store.Store, onMerge func()) *DedupRule {
	return &DedupRule{s: s, onMerge: onMerge}
}

func (r *DedupRule) Name() string {
	return "status_dedup"
}

func (r *DedupRule) Evaluate(doc Document) *Violation {
	if doc.Path != "documents/text" || doc.Method != "POST" {
		return nil
	}
	if !strings.HasPrefix(doc.Text, "[STATUS]") {
		return nil
	}
	go r.deleteExistingStatus()
	return nil
}

func (r *DedupRule) deleteExistingStatus() {
	docs, err := r.s.ListDocs(100)
	if err != nil {
		log.Printf("[writefence/dedup] list error: %v", err)
		return
	}
	var ids []string
	for _, doc := range docs {
		if strings.Contains(doc.Summary, "[STATUS]") {
			ids = append(ids, doc.ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	if err := r.s.DeleteDocs(ids); err != nil {
		log.Printf("[writefence/dedup] delete error: %v", err)
		return
	}
	log.Printf("[writefence/dedup] deleted %d old STATUS doc(s)", len(ids))
	if r.onMerge != nil {
		r.onMerge()
	}
}
