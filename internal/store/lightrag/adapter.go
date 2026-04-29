package lightrag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/writefence/writefence/internal/store"
)

type Adapter struct {
	baseURL string
	client  *http.Client
}

func New(baseURL string) *Adapter {
	return &Adapter{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (a *Adapter) ListDocs(limit int) ([]store.Doc, error) {
	body, _ := json.Marshal(map[string]int{"limit": limit, "offset": 0})
	resp, err := a.client.Post(a.baseURL+"/documents/paginated", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)

	var result struct {
		Documents []struct {
			ID             string `json:"id"`
			ContentSummary string `json:"content_summary"`
		} `json:"documents"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	docs := make([]store.Doc, len(result.Documents))
	for i, d := range result.Documents {
		docs[i] = store.Doc{ID: d.ID, Summary: d.ContentSummary}
	}
	return docs, nil
}

func (a *Adapter) DeleteDocs(ids []string) error {
	body, _ := json.Marshal(map[string]interface{}{
		"doc_ids":          ids,
		"delete_llm_cache": true,
	})
	req, _ := http.NewRequest("DELETE", a.baseURL+"/documents/delete_document", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed: HTTP %d: %s", resp.StatusCode, bytes.TrimSpace(b))
	}
	return nil
}
