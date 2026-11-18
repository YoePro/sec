package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"sec/internal/lsp/protocol"
	lspserver "sec/internal/lsp/server"
)

// TestTargetStatusSnapshots distinguishes absent/stale/incomplete comparisons
// from a current clean result and retains distinct logical variant identities.
// Rules: rules/tooling/lsp.md — §§23.1–23.2, Document snapshots.
func TestTargetStatusSnapshots(t *testing.T) {
	_, uri, text := targetSelectionFixture(t)
	s := &server{documentSnapshots: lspserver.NewDocuments(), crossTarget: newCrossTargetStore()}
	options, err := lspTargetOptions(pathFromURI(uri), sourceOverlay{})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.targetStatusFor(uri, options); got.State != "unavailable" || len(got.Targets) != 3 {
		t.Fatal(got)
	}
	s.documentSnapshots.Open(uri, 1, text)
	if got := s.targetStatusFor(uri, options); got.State != "unavailable" || got.Targets[0].Current || got.Targets[0].Diagnostics != nil {
		t.Fatal("missing comparison became a clean result", got)
	}
	result := crossTargetResult{textHash: documentTextHash(text), active: options[1].diagnosticName(), perTarget: map[string][]diagnostic{}}
	for _, option := range options {
		name := option.diagnosticName()
		result.targets = append(result.targets, name)
		result.perTarget[name] = []diagnostic{}
	}
	result.perTarget[result.targets[0]] = []diagnostic{{Severity: 1, Code: "S1246", Message: "condition requires bool"}}
	s.crossTarget.results[uri] = result
	got := s.targetStatusFor(uri, options)
	if got.State != "current" || got.Version != 1 || len(got.Targets) != 3 || !got.Targets[1].Active || got.Targets[0].Option == nil {
		t.Fatal(got)
	}
	if len(got.Targets[0].Diagnostics) != 1 || got.Targets[1].Diagnostics == nil || len(got.Targets[1].Diagnostics) != 0 {
		t.Fatal("lost diagnostic or current clean comparison", got)
	}
	got.Targets[0].Diagnostics[0].Message = "changed"
	got.Targets[0].Option.OS = "changed"
	if again := s.targetStatusFor(uri, options); again.Targets[0].Diagnostics[0].Message != "condition requires bool" || again.Targets[0].Option.OS == "changed" {
		t.Fatal("presentation changed stored facts", again)
	}
	changedOptions := append([]lspTargetOption{}, options...)
	changedOptions[0].Variant = "replacement"
	if got := s.targetStatusFor(uri, changedOptions); got.State != "unavailable" {
		t.Fatal("obsolete manifest target retained", got)
	}
	delete(result.perTarget, result.targets[0])
	if got := s.targetStatusFor(uri, options); got.State != "unavailable" {
		t.Fatal("incomplete result became current", got)
	}
	result.perTarget[result.targets[0]] = []diagnostic{}
	s.documentSnapshots.Change(uri, 2, text+"\n")
	if got := s.targetStatusFor(uri, options); got.State != "unavailable" || got.Version != 2 {
		t.Fatal("dirty snapshot reused old diagnostics", got)
	}
	s.documentSnapshots.Close(uri)
	if got := s.targetStatusFor(uri, options); got.State != "unavailable" {
		t.Fatal("closed snapshot reused diagnostics", got)
	}
}

// TestTargetStatusProtocol publishes the real compiler's per-target results
// without altering selection or recomputing on a read-only status request.
// Rules: rules/tooling/lsp.md — §§23.1–23.2, Multi-target diagnostics.
func TestTargetStatusProtocol(t *testing.T) {
	_, uri, text := targetSelectionFixture(t)
	var out bytes.Buffer
	s := &server{out: &out, documentSnapshots: lspserver.NewDocuments(), crossTarget: newCrossTargetStore()}
	s.documentSnapshots.Open(uri, 3, text)
	result, ok := computeCrossTargetDiagnostics(uri, text, s.sourceOverlay())
	if !ok {
		t.Fatal("compiler comparison unavailable")
	}
	s.crossTarget.results[uri] = result
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}})
	if err := s.handle(rpcMessage{ID: json.RawMessage(`1`), Method: "sec/targetStatus", Params: params}); err != nil {
		t.Fatal(err)
	}
	response, err := protocol.ReadMessage(bufio.NewReader(bytes.NewReader(out.Bytes())))
	if err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
	raw, _ := json.Marshal(response.Result)
	var status targetStatusResult
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	if status.State != "current" || status.Version != 3 || len(status.Targets) != 3 || len(s.targetSelections) != 0 {
		t.Fatal(status)
	}
	for _, row := range status.Targets {
		overflow := false
		for _, d := range row.Diagnostics {
			overflow = overflow || strings.Contains(d.Message, "overflows int")
		}
		if !row.Current || row.Option == nil || overflow != (row.Option.PointerWidthBits == 32) {
			t.Fatal("compiler target result lost", row)
		}
	}
}

// Target-status is read-only, follows preceding synchronization, and is
// superseded by queued changes/close just like other document queries.
// Rules: rules/tooling/lsp.md — Responsiveness model, Document snapshots.
func TestTargetStatusQueueAndInvalidURI(t *testing.T) {
	request := rpcMessage{Method: "sec/targetStatus", ID: json.RawMessage(`1`), Params: json.RawMessage(`{"textDocument":{"uri":"file:///example.sec"}}`)}
	if !isDeferrableAnalysisRequest(request.Method) || nextRequestIndex([]rpcMessage{request, {Method: "textDocument/formatting"}}) != 1 {
		t.Fatal("status blocks formatting")
	}
	for _, method := range []string{"textDocument/didChange", "textDocument/didClose"} {
		if !supersededByQueuedEdit(request, []rpcMessage{{Method: method, Params: request.Params}}) {
			t.Fatal("stale status request not superseded", method)
		}
	}
	var out bytes.Buffer
	s := &server{out: &out}
	request.Params = json.RawMessage(`{"textDocument":{"uri":"https://example.com/file.sec"}}`)
	if err := s.handle(request); err != nil {
		t.Fatal(err)
	}
	response, err := protocol.ReadMessage(bufio.NewReader(bytes.NewReader(out.Bytes())))
	if err != nil || response.Error == nil || response.Error.Code != -32602 {
		t.Fatal(response, err)
	}
}
