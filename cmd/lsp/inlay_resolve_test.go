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

func inlayResolveFixture(t *testing.T) (*server, string, string) {
	t.Helper()
	data, err := os.ReadFile("../../testdata/lsp/inlay_resolve/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	s := &server{documentSnapshots: lspserver.NewDocuments()}
	s.documentSnapshots.Open(uri, 1, string(data))
	return s, uri, string(data)
}

func listResolvableInlayHints(t *testing.T, s *server, uri string) []inlayHint {
	t.Helper()
	return lspTestDecode[[]inlayHint](t, lspTestRequest(t, s, "textDocument/inlayHint", inlayHintParams{
		TextDocument: textDocumentIdentifier{URI: uri}, Range: allLines(),
	}))
}

// TestInlayHintResolve checks actual framed protocol round-trips, stable labels,
// selected parameter/type declarations and repeat resolve from compiler facts.
// Rules: rules/tooling/lsp.md — "Inlay hints", "Navigation and references";
// LSP 3.17 InlayHint and inlayHint/resolve.
func TestInlayHintResolve(t *testing.T) {
	s, uri, text := inlayResolveFixture(t)
	initialized := lspTestRequest(t, s, "initialize", map[string]any{
		"capabilities": map[string]any{"textDocument": map[string]any{"inlayHint": map[string]any{
			"resolveSupport": map[string]any{"properties": []string{"tooltip", "label.tooltip", "label.location"}},
		}}},
	})
	raw, _ := json.Marshal(initialized.Result)
	if !strings.Contains(string(raw), `"inlayHintProvider":{"resolveProvider":true}`) {
		t.Fatal(string(raw))
	}
	for _, diagnostic := range analyze(uri, text) {
		if diagnostic.Severity == 1 {
			t.Fatalf("invalid fixture: %+v", diagnostic)
		}
	}
	hints := listResolvableInlayHints(t, s, uri)
	if len(hints) == 0 {
		t.Fatal("no hints")
	}
	checked := map[string]bool{}
	for _, hint := range hints {
		if hint.Tooltip != nil || len(hint.LabelParts) != 1 || hint.LabelParts[0].Tooltip != nil || hint.LabelParts[0].Location != nil || hint.Data == nil {
			t.Fatalf("listing did not defer decorations: %+v", hint)
		}
		// Decorations sent back by the client are replaced by compiler facts.
		forged := hint
		forged.LabelParts = append([]inlayHintLabelPart(nil), hint.LabelParts...)
		forged.Tooltip = &markupContent{Kind: "markdown", Value: "forged"}
		forged.LabelParts[0].Location = &location{URI: "file:///forged.sec"}
		resolved := lspTestDecode[inlayHint](t, lspTestRequest(t, s, "inlayHint/resolve", forged))
		if !sameInlayHintIdentity(resolved, hint) || resolved.Tooltip == nil || resolved.Tooltip.Value == "forged" || resolved.LabelParts[0].Tooltip == nil {
			t.Fatalf("incorrect resolved hint: %+v", resolved)
		}
		if hint.Label == ": int" && resolved.LabelParts[0].Location != nil {
			t.Fatal("builtin type acquired a guessed physical declaration")
		}
		var declaration string
		switch hint.Label {
		case ": Item":
			declaration = "type Item"
		case "amount:":
			declaration = "amount: int"
		case "enabled:":
			declaration = "enabled: bool"
		case "ref", "defaults: Value: 0":
			checked[hint.Label] = true
		}
		if declaration != "" {
			token := declaration
			if declaration == "type Item" {
				token = "Item"
			}
			start := strings.Index(text, declaration) + strings.Index(declaration, token)
			want := offsetPosition(text, start)
			target := resolved.LabelParts[0].Location
			if target == nil || target.URI != uri || target.Range.Start != want {
				t.Fatalf("%s: target=%+v, want %+v", hint.Label, target, want)
			}
			checked[hint.Label] = true
		}
		twice := lspTestDecode[inlayHint](t, lspTestRequest(t, s, "inlayHint/resolve", resolved))
		if !reflect.DeepEqual(twice, resolved) {
			t.Fatal("resolve is not idempotent")
		}
	}
	for _, label := range []string{": Item", "amount:", "enabled:", "ref", "defaults: Value: 0"} {
		if !checked[label] {
			t.Errorf("missing %s", label)
		}
	}
	if !isDeferrableAnalysisRequest("inlayHint/resolve") {
		t.Fatal("resolve bypasses worker queue")
	}
}

// TestInlayHintResolveRejectsStaleAndForgedItems prevents old offsets from
// acquiring a new target after edits, close, configuration or label changes.
// Rules: rules/tooling/lsp.md — "Inlay hints", "Document snapshots".
func TestInlayHintResolveRejectsStaleAndForgedItems(t *testing.T) {
	s, uri, text := inlayResolveFixture(t)
	original := listResolvableInlayHints(t, s, uri)[0]
	for _, name := range []string{"data", "version", "position", "label", "parts", "closed", "changed", "disabled"} {
		t.Run(name, func(t *testing.T) {
			s.documentSnapshots.Open(uri, 1, text)
			s.inlayHints = nil
			hint := original
			data := *hint.Data
			hint.Data = &data
			hint.LabelParts = append([]inlayHintLabelPart(nil), hint.LabelParts...)
			want := -32602
			switch name {
			case "data":
				hint.Data = nil
			case "version":
				hint.Data.Version++
			case "position":
				hint.Position.Line++
			case "label":
				hint.LabelParts[0].Value += "forged"
			case "parts":
				hint.LabelParts = []inlayHintLabelPart{{Value: ":"}, {Value: " Item"}}
			case "closed":
				s.documentSnapshots.Close(uri)
				want = -32801
			case "changed":
				s.documentSnapshots.Change(uri, 2, "\n"+text)
				want = -32801
			case "disabled":
				settings := inlayHintSettings{}
				s.inlayHints = &settings
			}
			response := lspTestRequest(t, s, "inlayHint/resolve", hint)
			if response.Error == nil || response.Error.Code != want {
				t.Fatalf("%+v", response)
			}
		})
	}
	for _, invalid := range []any{nil, 1, map[string]any{"label": 5}, map[string]any{"label": []any{map[string]any{"value": ""}}}} {
		response := lspTestRequest(t, s, "inlayHint/resolve", invalid)
		if response.Error == nil || response.Error.Code != -32602 {
			t.Fatal(response)
		}
	}
	params, _ := json.Marshal(original)
	for _, method := range []string{"textDocument/didChange", "textDocument/didClose"} {
		later, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": uri}})
		if !supersededByQueuedEdit(rpcMessage{Method: "inlayHint/resolve", Params: params}, []rpcMessage{{Method: method, Params: later}}) {
			t.Fatal("queued edit did not supersede resolve")
		}
	}
}

// TestInlayHintResolveEagerFallbackAndImportedTypes checks property-by-property
// negotiation and imported type provenance without name-based LSP resolution.
// Rules: rules/tooling/lsp.md — "Inlay hints", "Navigation and references".
func TestInlayHintResolveEagerFallbackAndImportedTypes(t *testing.T) {
	s, uri, _ := inlayResolveFixture(t)
	for _, properties := range [][]string{nil, {"tooltip", "label.location"}} {
		s.configureInlayResolve(mustJSON(t, map[string]any{"capabilities": map[string]any{"textDocument": map[string]any{"inlayHint": map[string]any{"resolveSupport": map[string]any{"properties": properties}}}}}))
		for _, hint := range listResolvableInlayHints(t, s, uri) {
			if (hint.Tooltip == nil) != s.inlayResolve.tooltip || hint.LabelParts[0].Tooltip == nil {
				t.Fatal(hint)
			}
			if hint.Label == ": Item" && (hint.LabelParts[0].Location == nil) != s.inlayResolve.labelLocation {
				t.Fatal(hint)
			}
		}
	}
	uri, source, catalogURI, catalog := genericNavigationFixture(t, "use")
	s.documentSnapshots.Open(uri, 1, source)
	s.inlayResolve = inlayResolveSupport{}
	found := false
	for _, hint := range listResolvableInlayHints(t, s, uri) {
		if hint.Label != ": catalog.Item" {
			continue
		}
		found = true
		target := hint.LabelParts[0].Location
		want := offsetPosition(catalog, strings.Index(catalog, "type Item")+len("type "))
		if target == nil || target.URI != catalogURI || target.Range.Start != want {
			t.Fatalf("imported type target=%+v", target)
		}
	}
	if !found {
		t.Fatalf("missing inferred imported field type: %+v", listResolvableInlayHints(t, s, uri))
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
