package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// EmbedClient produces embedding vectors from text.
type EmbedClient interface {
	Embed(text string) ([]float32, error)
}

// QdrantClient wraps Qdrant REST API for vector search and upsert.
type QdrantClient struct {
	BaseURL    string
	Collection string
	http       *http.Client
}

func (q *QdrantClient) client() *http.Client {
	if q.http == nil {
		q.http = &http.Client{Timeout: 10 * time.Second}
	}
	return q.http
}

func (q *QdrantClient) SearchSimilar(vec []float32) (score float64, id string, err error) {
	body, _ := json.Marshal(map[string]interface{}{
		"vector":       vec,
		"limit":        1,
		"with_payload": false,
	})
	url := fmt.Sprintf("%s/collections/%s/points/search", q.BaseURL, q.Collection)
	resp, err := q.client().Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var result struct {
		Result []struct {
			ID    interface{} `json:"id"`
			Score float64     `json:"score"`
		} `json:"result"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return 0, "", fmt.Errorf("qdrant search parse error: %w", err)
	}
	if len(result.Result) == 0 {
		return 0, "", nil
	}
	return result.Result[0].Score, fmt.Sprintf("%v", result.Result[0].ID), nil
}

func (q *QdrantClient) UpsertVec(id string, vec []float32) {
	body, _ := json.Marshal(map[string]interface{}{
		"points": []map[string]interface{}{
			{"id": id, "vector": vec},
		},
	})
	url := fmt.Sprintf("%s/collections/%s/points", q.BaseURL, q.Collection)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	if err != nil {
		log.Printf("[writefence/semantic_dedup] upsert request build error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := q.client().Do(req)
	if err != nil {
		log.Printf("[writefence/semantic_dedup] upsert error: %v", err)
		return
	}
	resp.Body.Close()
}

// SemanticDedupRule blocks near-duplicate document writes using embedding similarity.
type SemanticDedupRule struct {
	embed     EmbedClient
	qdrant    *QdrantClient
	threshold float64
}

const quarantineThreshold = 0.85

func NewSemanticDedupRule(ec EmbedClient, qc *QdrantClient, threshold float64) *SemanticDedupRule {
	return &SemanticDedupRule{embed: ec, qdrant: qc, threshold: threshold}
}

func (r *SemanticDedupRule) Name() string {
	return "semantic_dedup"
}

func (r *SemanticDedupRule) Evaluate(doc Document) *Violation {
	if doc.Path != "documents/text" || doc.Method != "POST" {
		return nil
	}
	if r.embed == nil || r.qdrant == nil {
		return nil // disabled when not configured
	}
	if doc.Text == "" {
		return nil
	}

	vec, err := r.embed.Embed(doc.Text)
	if err != nil {
		log.Printf("[writefence/semantic_dedup] embed error: %v", err)
		return nil // fail open
	}

	score, existingID, err := r.qdrant.SearchSimilar(vec)
	if err != nil {
		log.Printf("[writefence/semantic_dedup] search error: %v", err)
		return nil // fail open
	}

	if score >= r.threshold {
		return &Violation{
			Rule:       r.Name(),
			State:      StateBlocked,
			ReasonCode: "duplicate_write",
			Reason: fmt.Sprintf(
				"Near-duplicate detected (similarity=%.3f >= threshold=%.2f). Existing doc: %s. Update the existing document instead.",
				score, r.threshold, existingID,
			),
			Retryable: true,
		}
	}
	if score >= quarantineThreshold {
		return &Violation{
			Rule:           r.Name(),
			State:          StateQuarantined,
			ReasonCode:     "needs_manual_review",
			Reason:         fmt.Sprintf("Possible near-duplicate detected (similarity=%.3f). Existing doc: %s. Write was quarantined for human review.", score, existingID),
			Suggestion:     "Review the pending write and either approve it or merge the content into the existing document.",
			Retryable:      false,
			RetryAfter:     "manual_review",
			ReviewRequired: true,
		}
	}

	// Store embedding async for future checks
	preview := doc.Text
	if len(preview) > 40 {
		preview = preview[:40]
	}
	docID := fmt.Sprintf("%x", []byte(strings.NewReplacer(" ", "", "[", "", "]", "").Replace(preview))[:8])
	go r.qdrant.UpsertVec(docID, vec)

	return nil
}
