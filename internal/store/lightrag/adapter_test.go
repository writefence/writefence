package lightrag_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	lightragstore "github.com/writefence/writefence/internal/store/lightrag"
)

func TestAdapterListDocs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/documents/paginated" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"documents": []map[string]interface{}{
					{"id": "abc", "content_summary": "[STATUS] current work"},
					{"id": "def", "content_summary": "[DECISION] chose Go"},
				},
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	a := lightragstore.New(srv.URL)
	docs, err := a.ListDocs(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d", len(docs))
	}
	if docs[0].ID != "abc" {
		t.Errorf("expected first doc id=abc, got %s", docs[0].ID)
	}
}

func TestAdapterDeleteDoc(t *testing.T) {
	deleted := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" && r.URL.Path == "/documents/delete_document" {
			deleted = true
			w.WriteHeader(200)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	a := lightragstore.New(srv.URL)
	err := a.DeleteDocs([]string{"doc-1", "doc-2"})
	if err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Error("expected DELETE to be called")
	}
}

func TestAdapterDeleteDocError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	a := lightragstore.New(srv.URL)
	err := a.DeleteDocs([]string{"doc-1"})
	if err == nil {
		t.Fatal("expected error for HTTP 500 response")
	}
}
