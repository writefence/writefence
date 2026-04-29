package mcp_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/writefence/writefence/internal/mcp"
)

func rpc(method string, params interface{}) []byte {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}
	b, _ := json.Marshal(req)
	return b
}

func TestMCPListRules(t *testing.T) {
	srv := mcp.NewServer("/tmp/no-violations.jsonl")
	var buf bytes.Buffer
	srv.HandleRequest(bytes.NewReader(rpc("tools/call", map[string]interface{}{
		"name": "list_rules",
	})), &buf)

	var resp map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v\nraw: %s", err, buf.String())
	}
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
}

func TestMCPUnknownMethod(t *testing.T) {
	srv := mcp.NewServer("/tmp/no-violations.jsonl")
	var buf bytes.Buffer
	srv.HandleRequest(bytes.NewReader(rpc("unknown/method", nil)), &buf)

	var resp map[string]interface{}
	json.Unmarshal(buf.Bytes(), &resp)
	if resp["error"] == nil {
		t.Fatal("expected error for unknown method")
	}
}

func TestMCPGetViolationsLimit(t *testing.T) {
	// Create a temp violations file with 5 entries.
	dir := t.TempDir()
	logPath := filepath.Join(dir, "violations.jsonl")
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	for i := 0; i < 5; i++ {
		entry, _ := json.Marshal(map[string]interface{}{"rule": "test", "index": i})
		f.Write(append(entry, '\n'))
	}
	f.Close()

	srv := mcp.NewServer(logPath)

	// Request with limit=2 — should return only 2 entries.
	var buf bytes.Buffer
	srv.HandleRequest(bytes.NewReader(rpc("tools/call", map[string]interface{}{
		"name":  "get_violations",
		"limit": 2,
	})), &buf)

	var resp map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}

	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	textRaw := content[0].(map[string]interface{})["text"].(string)

	var entries []interface{}
	if err := json.Unmarshal([]byte(textRaw), &entries); err != nil {
		t.Fatalf("content is not JSON array: %v\nraw: %s", err, textRaw)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries with limit=2, got %d", len(entries))
	}

	// Request with default (no limit) — should return all 5 entries.
	buf.Reset()
	srv.HandleRequest(bytes.NewReader(rpc("tools/call", map[string]interface{}{
		"name": "get_violations",
	})), &buf)

	resp = map[string]interface{}{}
	json.Unmarshal(buf.Bytes(), &resp)
	result = resp["result"].(map[string]interface{})
	content = result["content"].([]interface{})
	textRaw = content[0].(map[string]interface{})["text"].(string)

	entries = nil
	json.Unmarshal([]byte(textRaw), &entries)
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries with default limit, got %d", len(entries))
	}
}

func TestMCPInvalidParams(t *testing.T) {
	srv := mcp.NewServer("/tmp/no-violations.jsonl")
	// Send malformed (non-object) params for tools/call.
	rawReq := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":"not-an-object"}`)
	var buf bytes.Buffer
	srv.HandleRequest(bytes.NewReader(rawReq), &buf)

	var resp map[string]interface{}
	json.Unmarshal(buf.Bytes(), &resp)
	if resp["error"] == nil {
		t.Fatal("expected -32602 error for malformed params")
	}
	errObj := resp["error"].(map[string]interface{})
	if code := errObj["code"].(float64); code != -32602 {
		t.Fatalf("expected code -32602, got %v", code)
	}
}
