package main

import (
	"encoding/json"
	"testing"
)

func queueRequest(method string, id int, uri string) rpcMessage {
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}})
	raw, _ := json.Marshal(id)
	return rpcMessage{JSONRPC: "2.0", ID: raw, Method: method, Params: params}
}

func queueNotification(method string, params any) rpcMessage {
	raw, _ := json.Marshal(params)
	return rpcMessage{JSONRPC: "2.0", Method: method, Params: raw}
}

// A queued request the client cancels is answered RequestCancelled, and an
// analysis request whose document already has a later queued edit or close is
// answered ContentModified instead of costing a full stale analysis; requests
// without a later edit, and requests for other documents, are handled.
//
// Rules:
//   - rules/tooling/lsp.md — "Responsiveness model", "Document synchronization"
func TestRequestQueueDropsCancelledAndSupersededRequests(t *testing.T) {
	queue := newRequestQueue()
	queue.push(queueRequest("textDocument/hover", 1, "file:///a.sec"))
	queue.push(queueRequest("textDocument/semanticTokens/full", 2, "file:///a.sec"))
	queue.push(queueRequest("textDocument/hover", 3, "file:///b.sec"))
	queue.push(queueNotification("$/cancelRequest", map[string]int{"id": 3}))
	queue.push(queueNotification("$/cancelRequest", map[string]int{"id": 99}))
	queue.push(queueNotification("textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": "file:///a.sec", "version": 2}}))
	queue.push(queueRequest("textDocument/hover", 4, "file:///a.sec"))
	queue.close(nil)

	want := []struct {
		method      string
		disposition queueDisposition
	}{
		{"textDocument/hover", dispositionContentModified},
		{"textDocument/semanticTokens/full", dispositionContentModified},
		{"textDocument/hover", dispositionCancelled},
		{"textDocument/didChange", dispositionHandle},
		{"textDocument/hover", dispositionHandle},
	}
	for index, expected := range want {
		message, disposition, ok := queue.next()
		if !ok || message.Method != expected.method || disposition != expected.disposition {
			t.Fatalf("message %d = %s/%d (%t), want %s/%d", index, message.Method, disposition, ok, expected.method, expected.disposition)
		}
	}
	if _, _, ok := queue.next(); ok {
		t.Fatal("the cancel notifications were queued as messages")
	}
	if len(queue.cancelled) != 0 {
		t.Fatalf("cancellations accumulated: %v", queue.cancelled)
	}
}

// Within one epoch a document state is analyzed once and shared; a new epoch
// or different text analyzes again, and nothing is cached while disabled.
func TestAnalysisCacheSharesOneAnalysisPerEpoch(t *testing.T) {
	uri := "file:///cache.sec"
	text := "module main\n\nfn F() int {\n    return 1\n}\n"
	_, disabledA, releaseA := cachedLSPAnalysis(uri, text, parseProgramForLSP(uri, text), sourceOverlay{})
	releaseA()
	_, disabledB, releaseB := cachedLSPAnalysis(uri, text, parseProgramForLSP(uri, text), sourceOverlay{})
	releaseB()
	if disabledA == disabledB {
		t.Fatal("analysis was cached while the cache is disabled")
	}
	enableAnalysisCache()
	defer disableAnalysisCache()
	_, first, releaseFirst := cachedLSPAnalysis(uri, text, parseProgramForLSP(uri, text), sourceOverlay{})
	releaseFirst()
	_, second, releaseSecond := cachedLSPAnalysis(uri, text, parseProgramForLSP(uri, text), sourceOverlay{})
	releaseSecond()
	if first != second {
		t.Fatal("the same document state was analyzed twice in one epoch")
	}
	changed := text + "\n"
	_, other, releaseOther := cachedLSPAnalysis(uri, changed, parseProgramForLSP(uri, changed), sourceOverlay{})
	releaseOther()
	if other == second {
		t.Fatal("different text reused a cached analysis")
	}
	advanceAnalysisEpoch()
	_, next, releaseNext := cachedLSPAnalysis(uri, text, parseProgramForLSP(uri, text), sourceOverlay{})
	releaseNext()
	if next == second {
		t.Fatal("a new epoch reused a cached analysis")
	}
}
