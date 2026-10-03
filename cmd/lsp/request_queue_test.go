package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"sec/internal/lsp/protocol"
	lspserver "sec/internal/lsp/server"
)

func queuedMessage(method string, id int) rpcMessage {
	message := rpcMessage{Method: method}
	if id > 0 {
		message.ID = json.RawMessage(strconv.Itoa(id))
	}
	return message
}

// Formatting moves ahead of read-only analysis requests queued before it, but
// never ahead of document synchronization or any other message, so it always
// formats the text every earlier edit produced.
//
// Rules:
//   - rules/tooling/lsp.md — "Responsiveness model", "Document synchronization"
func TestFormattingOvertakesOnlyQueuedAnalysisRequests(t *testing.T) {
	tests := []struct {
		name    string
		methods []string
		want    int
	}{
		{name: "after analysis requests", methods: []string{"textDocument/hover", "textDocument/semanticTokens/full", "textDocument/formatting"}, want: 2},
		{name: "will save wait until", methods: []string{"textDocument/completion", "textDocument/willSaveWaitUntil"}, want: 1},
		{name: "never ahead of an edit", methods: []string{"textDocument/hover", "textDocument/didChange", "textDocument/formatting"}, want: 0},
		{name: "head is not analysis", methods: []string{"textDocument/didChange", "textDocument/formatting"}, want: 0},
		{name: "no formatting", methods: []string{"textDocument/hover", "textDocument/definition"}, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items := make([]rpcMessage, 0, len(test.methods))
			for index, method := range test.methods {
				items = append(items, queuedMessage(method, index+1))
			}
			if got := nextRequestIndex(items); got != test.want {
				t.Fatalf("next index = %d, want %d", got, test.want)
			}
		})
	}

	queue := newRequestQueue()
	for index, method := range []string{"textDocument/hover", "textDocument/hover", "textDocument/formatting"} {
		queue.push(queuedMessage(method, index+1))
	}
	queue.close(nil)
	order := []string{}
	for {
		message, ok := queue.next()
		if !ok {
			break
		}
		order = append(order, message.Method)
	}
	if strings.Join(order, ",") != "textDocument/formatting,textDocument/hover,textDocument/hover" {
		t.Fatalf("handled order = %v", order)
	}
}

// The reader goroutine feeds every message to the handler and end of input
// ends the server loop cleanly.
func TestServerRunAnswersQueuedRequestsUntilEOF(t *testing.T) {
	var input bytes.Buffer
	uri := "file:///queue.sec"
	messages := []any{
		map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": uri, "version": 1, "text": "fn  Main() int {\nreturn 0\n}\n"}}},
		map[string]any{"jsonrpc": "2.0", "id": 1, "method": "textDocument/documentSymbol", "params": map[string]any{"textDocument": map[string]any{"uri": uri}}},
		map[string]any{"jsonrpc": "2.0", "id": 2, "method": "textDocument/formatting", "params": map[string]any{"textDocument": map[string]any{"uri": uri}, "options": map[string]any{"tabSize": 4, "insertSpaces": true}}},
	}
	for _, message := range messages {
		if err := protocol.WriteMessage(&input, message); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	s := &server{
		in:                bufio.NewReader(&input),
		out:               &output,
		documentSnapshots: lspserver.NewDocuments(),
		diagnosticTimers:  map[string]*time.Timer{},
		diagnosticDelay:   time.Hour,
	}
	if err := s.run(); err != nil {
		t.Fatalf("run = %v", err)
	}
	text := output.String()
	if !strings.Contains(text, `"id":1`) || !strings.Contains(text, `"id":2`) {
		t.Fatalf("missing responses:\n%s", text)
	}
}
