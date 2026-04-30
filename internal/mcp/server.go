package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// Server is the WriteFence MCP server exposing operator tools over JSON-RPC 2.0.
type Server struct {
	violationsLog string
}

func NewServer(violationsLog string) *Server {
	return &Server{violationsLog: violationsLog}
}

// Run reads JSON-RPC requests from stdin line by line and writes responses to stdout.
func (s *Server) Run() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		s.HandleRequest(bytes.NewReader(scanner.Bytes()), os.Stdout)
	}
}

// HandleRequest processes one JSON-RPC request from r and writes the response to w.
func (s *Server) HandleRequest(r io.Reader, w io.Writer) {
	var req jsonRPCRequest
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			Error:   map[string]interface{}{"code": -32700, "message": "parse error"},
		})
		return
	}

	var result interface{}
	var rpcErr interface{}

	switch req.Method {
	case "tools/list":
		result = s.toolsList()
	case "tools/call":
		var p struct {
			Name string `json:"name"`
		}
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &p); err != nil {
				rpcErr = map[string]interface{}{"code": -32602, "message": "invalid params"}
				break
			}
		}
		result, rpcErr = s.toolsCall(p.Name, req.Params)
	default:
		rpcErr = map[string]interface{}{"code": -32601, "message": fmt.Sprintf("method not found: %s", req.Method)}
	}

	json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
		Error:   rpcErr,
	})
}

func (s *Server) toolsList() interface{} {
	return map[string]interface{}{
		"tools": []map[string]interface{}{
			{
				"name":        "list_rules",
				"description": "List all active writefence enforcement rules",
				"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
			},
			{
				"name":        "get_violations",
				"description": "Get recent violations from the writefence log",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"limit": map[string]interface{}{"type": "integer", "description": "Max violations to return (default 20)"},
					},
				},
			},
		},
	}
}

func (s *Server) toolsCall(name string, rawParams json.RawMessage) (interface{}, interface{}) {
	switch name {
	case "list_rules":
		return map[string]interface{}{
			"content": []map[string]interface{}{{
				"type": "text",
				"text": "Active rules: english_only, prefix_required, context_shield, status_dedup, semantic_dedup",
			}},
		}, nil
	case "get_violations":
		limit := 20
		var lp struct {
			Limit int `json:"limit"`
		}
		if json.Unmarshal(rawParams, &lp) == nil && lp.Limit > 0 {
			limit = lp.Limit
		}
		entries := s.readViolations(limit)
		b, _ := json.Marshal(entries)
		return map[string]interface{}{
			"content": []map[string]interface{}{{
				"type": "text",
				"text": string(b),
			}},
		}, nil
	default:
		return nil, map[string]interface{}{"code": -32602, "message": fmt.Sprintf("unknown tool: %s", name)}
	}
}

func (s *Server) readViolations(limit int) []map[string]interface{} {
	f, err := os.Open(s.violationsLog)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer f.Close()
	var all []map[string]interface{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e map[string]interface{}
		if json.Unmarshal(scanner.Bytes(), &e) == nil {
			all = append(all, e)
		}
	}
	if len(all) > limit {
		return all[len(all)-limit:]
	}
	return all
}
