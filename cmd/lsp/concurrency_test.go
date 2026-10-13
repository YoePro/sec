package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"testing"
	"time"

	"sec/internal/lsp/protocol"
	lspserver "sec/internal/lsp/server"
)

// Read-only requests run on worker goroutines while document changes stay
// ordered: every request is answered exactly once, results for the same
// document share one analysis without data races, and the run loop waits for
// its workers before returning (run with -race).
//
// Rules:
//   - rules/tooling/lsp.md — "Responsiveness model"
func TestServerAnswersConcurrentRequestsExactlyOnce(t *testing.T) {
	var input bytes.Buffer
	write := func(message any) {
		if err := protocol.WriteMessage(&input, message); err != nil {
			t.Fatal(err)
		}
	}
	documents := map[string]string{
		"file:///first.sec":  "module main\n\ntype Pair struct {\n    left: int,\n    right: int,\n}\n\nfn Sum(pair: Pair) int {\n    let total := pair.left + pair.right\n    return total\n}\n",
		"file:///second.sec": "module other\n\nfn Twice(value: int) int {\n    return value * 2\n}\n",
	}
	write(map[string]any{"jsonrpc": "2.0", "id": 0, "method": "initialize", "params": map[string]any{"capabilities": map[string]any{}}})
	for uri, text := range documents {
		write(map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "sec", "version": 1, "text": text}}})
	}
	id := 1
	expected := map[int]bool{0: true}
	request := func(method string, params map[string]any) {
		write(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
		expected[id] = true
		id++
	}
	for round := 0; round < 3; round++ {
		for uri := range documents {
			document := map[string]any{"uri": uri}
			position := map[string]any{"line": 8, "character": 10}
			request("textDocument/semanticTokens/full", map[string]any{"textDocument": document})
			request("textDocument/hover", map[string]any{"textDocument": document, "position": position})
			request("textDocument/inlayHint", map[string]any{"textDocument": document, "range": map[string]any{"start": map[string]any{"line": 0, "character": 0}, "end": map[string]any{"line": 20, "character": 0}}})
			request("textDocument/documentHighlight", map[string]any{"textDocument": document, "position": position})
			request("textDocument/completion", map[string]any{"textDocument": document, "position": position})
			request("textDocument/documentSymbol", map[string]any{"textDocument": document})
		}
		request("workspace/symbol", map[string]any{"query": "Sum"})
		request("textDocument/formatting", map[string]any{"textDocument": map[string]any{"uri": "file:///second.sec"}, "options": map[string]any{"tabSize": 4, "insertSpaces": true}})
		write(map[string]any{"jsonrpc": "2.0", "method": "textDocument/didChange", "params": map[string]any{"textDocument": map[string]any{"uri": "file:///first.sec", "version": round + 2}, "contentChanges": []any{map[string]any{"text": documents["file:///first.sec"] + "\n"}}}})
	}
	request("textDocument/hover", map[string]any{"textDocument": map[string]any{"uri": "file:///first.sec"}, "position": map[string]any{"line": 8, "character": 10}})

	reader, writer := io.Pipe()
	s := &server{
		in:                bufio.NewReader(&input),
		out:               writer,
		documentSnapshots: lspserver.NewDocuments(),
		diagnosticTimers:  map[string]*time.Timer{},
		diagnosticDelay:   time.Hour,
	}
	done := make(chan error, 1)
	go func() {
		done <- s.run()
		writer.Close()
	}()
	answered := map[int]int{}
	output := bufio.NewReader(reader)
	for {
		raw, err := protocol.ReadMessage(output)
		if err != nil {
			break
		}
		if raw.Method != "" || len(raw.ID) == 0 {
			continue
		}
		var responseID int
		if json.Unmarshal(raw.ID, &responseID) == nil {
			answered[responseID]++
		}
	}
	if err := <-done; err != nil {
		t.Fatalf("run = %v", err)
	}
	for requestID := range expected {
		if answered[requestID] != 1 {
			t.Errorf("request %d answered %d times", requestID, answered[requestID])
		}
	}
	if len(answered) != len(expected) {
		t.Errorf("answered %d distinct requests, sent %d", len(answered), len(expected))
	}
}
