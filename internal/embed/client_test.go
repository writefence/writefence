package embed_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/writefence/writefence/internal/embed"
)

func makeEmbedServer(dims int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vec := make([]float32, dims)
		for i := range vec {
			vec[i] = float32(i) * 0.001
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"embedding": vec})
	}))
}

func TestClientEmbed(t *testing.T) {
	srv := makeEmbedServer(4096)
	defer srv.Close()
	c := embed.NewClient(srv.URL, "qwen3-embedding:8b")
	vec, err := c.Embed("hello world")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 4096 {
		t.Errorf("expected 4096 dims, got %d", len(vec))
	}
}

func TestClientEmbedEmpty(t *testing.T) {
	srv := makeEmbedServer(4096)
	defer srv.Close()
	c := embed.NewClient(srv.URL, "qwen3-embedding:8b")
	_, err := c.Embed("")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}
