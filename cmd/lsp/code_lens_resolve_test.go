package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	lspserver "sec/internal/lsp/server"
)

func codeLensResolveFixture(t *testing.T) (*server, string, string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"native.sec", "native.linux.sec", "native.windows_invalid.sec"} {
		data, err := os.ReadFile(filepath.Join("../../testdata/lsp/code_lens_resolve", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "native.sec")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	uri, text := uriFromPath(path), string(data)
	s := &server{documentSnapshots: lspserver.NewDocuments()}
	s.documentSnapshots.Open(uri, 1, text)
	return s, uri, text
}

// Listing lenses must never perform cross-target analysis; resolve runs the
// existing shared conformance projection and preserves exact declaration identity.
// Rules: rules/tooling/lsp.md — "Interface conformance code lens", "Responsiveness model";
// rules/declarations/interfaces.md — §§6,12.
func TestCodeLensResolveDefersAnalysisAndPreservesReport(t *testing.T) {
	s, uri, text := codeLensResolveFixture(t)
	init := lspTestRequest(t, s, "initialize", map[string]any{})
	raw, _ := json.Marshal(init.Result)
	if !strings.Contains(string(raw), `"codeLensProvider":{"resolveProvider":true}`) {
		t.Fatal(string(raw))
	}
	interfaceConformanceCache.Lock()
	interfaceConformanceCache.key = "not-analyzed"
	interfaceConformanceCache.summary = nil
	interfaceConformanceCache.Unlock()
	lenses := lspTestDecode[[]codeLens](t, lspTestRequest(t, s, "textDocument/codeLens", codeLensParams{TextDocument: textDocumentIdentifier{URI: uri}}))
	if len(lenses) != 2 {
		t.Fatal(lenses)
	}
	interfaceConformanceCache.Lock()
	untouched := interfaceConformanceCache.key == "not-analyzed"
	interfaceConformanceCache.Unlock()
	if !untouched {
		t.Fatal("listing lenses invoked cross-target analysis")
	}
	eager := interfaceConformanceCodeLenses(uri, text, s.sourceOverlay())
	for i, lens := range lenses {
		if lens.Command != nil || lens.Data == nil {
			t.Fatal(lens)
		}
		// Never accept a client-supplied executable command as the result.
		lens.Command = &lspCommand{Title: "forged", Command: "forged"}
		resolved := lspTestDecode[codeLens](t, lspTestRequest(t, s, "codeLens/resolve", lens))
		if !reflect.DeepEqual(resolved.Range, eager[i].Range) || resolved.Command.Title != eager[i].Command.Title || resolved.Command.Command != eager[i].Command.Command || !reflect.DeepEqual(resolved.Data, lens.Data) {
			t.Fatal(resolved, eager[i])
		}
		if i == 0 && !strings.Contains(resolved.Command.Title, "windows-amd64: Size") {
			t.Fatal(resolved)
		}
		if i == 1 && !strings.Contains(resolved.Command.Title, "no implementation") {
			t.Fatal(resolved)
		}
		twice := lspTestDecode[codeLens](t, lspTestRequest(t, s, "codeLens/resolve", resolved))
		if !reflect.DeepEqual(twice, resolved) {
			t.Fatal(twice, resolved)
		}
	}
	// A changed implementation leaves declaration data valid but invalidates the
	// existing module cache, so resolve projects the current compiler facts.
	windows := filepath.Join(filepath.Dir(pathFromURI(uri)), "native.windows_invalid.sec")
	data, err := os.ReadFile(windows)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(data), "Size(extra: int)", "Size()", 1)
	if err := os.WriteFile(windows, []byte(changed), 0644); err != nil {
		t.Fatal(err)
	}
	resolved := lspTestDecode[codeLens](t, lspTestRequest(t, s, "codeLens/resolve", lenses[0]))
	if resolved.Command.Title != "✓ Handle conforms on 2 targets" {
		t.Fatal(resolved)
	}
	if !isDeferrableAnalysisRequest("codeLens/resolve") {
		t.Fatal("resolve bypasses the read-only worker queue")
	}
}

// Rejected resolve requests never move a lens to a same-named declaration or
// run commands supplied by the client. Queued changes/close supersede the item.
// Rules: rules/tooling/lsp.md — "Document snapshots", "Interface conformance code lens".
func TestCodeLensResolveRejectsMalformedAndStaleData(t *testing.T) {
	s, uri, text := codeLensResolveFixture(t)
	original := unresolvedInterfaceCodeLenses(uri, text)[0]
	for _, name := range []string{"missing data", "version", "name", "range", "closed", "changed"} {
		t.Run(name, func(t *testing.T) {
			s.documentSnapshots.Open(uri, 1, text)
			lens := original
			data := *lens.Data
			lens.Data = &data
			want := -32602
			switch name {
			case "missing data":
				lens.Data = nil
			case "version":
				data.Version++
			case "name":
				data.Interface = "NativeMore"
			case "range":
				lens.Range.Start.Character++
			case "closed":
				s.documentSnapshots.Close(uri)
				want = -32801
			case "changed":
				s.documentSnapshots.Change(uri, 2, "\n"+text)
				want = -32801
			}
			response := lspTestRequest(t, s, "codeLens/resolve", lens)
			if response.Error == nil || response.Error.Code != want {
				t.Fatal(response)
			}
		})
	}
	for _, invalid := range []any{nil, 1, map[string]any{"data": "bad"}} {
		response := lspTestRequest(t, s, "codeLens/resolve", invalid)
		if response.Error == nil || response.Error.Code != -32602 {
			t.Fatal(response)
		}
	}
	params, _ := json.Marshal(original)
	message := rpcMessage{Method: "codeLens/resolve", Params: params}
	for _, method := range []string{"textDocument/didChange", "textDocument/didClose"} {
		later, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": uri}})
		if !supersededByQueuedEdit(message, []rpcMessage{{Method: method, Params: later}}) {
			t.Fatal("stale resolve not superseded")
		}
	}
}

// Deferred conformance requests obey formatting priority and queued cancellation.
// Rules: rules/tooling/lsp.md — "Responsiveness model", request cancellation.
func TestCodeLensResolveQueuePriorityAndCancellation(t *testing.T) {
	message := rpcMessage{ID: json.RawMessage(`1`), Method: "codeLens/resolve"}
	if nextRequestIndex([]rpcMessage{message, {ID: json.RawMessage(`2`), Method: "textDocument/formatting"}}) != 1 {
		t.Fatal("resolve prevented formatting priority")
	}
	queue := newRequestQueue()
	queue.push(message)
	queue.push(rpcMessage{Method: "$/cancelRequest", Params: json.RawMessage(`{"id":1}`)})
	queue.close(nil)
	_, disposition, ok := queue.next()
	if !ok || disposition != dispositionCancelled {
		t.Fatal(disposition, ok)
	}
}
