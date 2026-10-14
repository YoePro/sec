package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"sec/internal/lsp/protocol"
	"testing"
)

// lspTestRequest verifies one routed request through the framed JSON-RPC transport.
func lspTestRequest(t *testing.T, s *server, method string, params any) protocol.Message {
	t.Helper()
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	s.out = &out
	if err := s.handle(rpcMessage{ID: json.RawMessage(`1`), Method: method, Params: data}); err != nil {
		t.Fatal(err)
	}
	result, err := protocol.ReadMessage(bufio.NewReader(&out))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func lspTestDecode[T any](t *testing.T, response protocol.Message) T {
	t.Helper()
	if response.Error != nil {
		t.Fatal(response.Error)
	}
	data, err := json.Marshal(response.Result)
	if err != nil {
		t.Fatal(err)
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
