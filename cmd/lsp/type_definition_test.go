package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lspserver "sec/internal/lsp/server"
)

// Type-definition navigation follows Sema's binding and expression types for
// parameters, locals, members, and call results.
//
// Rules:
//   - rules/tooling/lsp.md — "Navigation and references" and A.15
func TestTypeDefinitionUsesResolvedSemaType(t *testing.T) {
	source := `module main

type Widget struct {
    value: int,
}

type Box struct {
    item: Widget,
}

fn Build() Widget {
    return Widget { value: 1 }
}

fn Select(box: Box, parameter: Widget) Widget {
    let local: Widget := parameter
    let built := Build()
    discard local
    discard built
    return box.item
}
`
	wantOffset := strings.Index(source, "Widget struct")
	want := offsetPosition(source, wantOffset)
	for name, offset := range map[string]int{
		"parameter use":  strings.Index(source, "parameter\n"),
		"local use":      strings.Index(source, "discard local") + len("discard "),
		"call result":    strings.Index(source, "Build()\n") + 1,
		"member":         strings.LastIndex(source, "item"),
		"type reference": strings.Index(source, "parameter: Widget") + len("parameter: ") + 1,
	} {
		t.Run(name, func(t *testing.T) {
			locations := typeDefinitionsForSource("", source, offsetPosition(source, offset))
			if len(locations) != 1 || locations[0].Range.Start != want {
				t.Fatalf("type definition at %s = %+v, want Widget at %+v", name, locations, want)
			}
		})
	}
}

func TestTypeDefinitionReturnsNothingForBuiltinType(t *testing.T) {
	source := "module main\n\nfn Value(number: int) int { return number }\n"
	offset := strings.LastIndex(source, "number") + 1
	if locations := typeDefinitionsForSource("", source, offsetPosition(source, offset)); len(locations) != 0 {
		t.Fatalf("builtin int unexpectedly has a source type definition: %+v", locations)
	}
}

func TestTypeDefinitionResolvesAcrossSameModuleFiles(t *testing.T) {
	dir := t.TempDir()
	typePath := filepath.Join(dir, "widget.sec")
	typeSource := "module sample\n\ntype Widget struct { value: int, }\n"
	if err := os.WriteFile(typePath, []byte(typeSource), 0644); err != nil {
		t.Fatal(err)
	}
	usePath := filepath.Join(dir, "use.sec")
	useSource := "module sample\n\nfn Use(value: Widget) Widget { return value }\n"
	offset := strings.LastIndex(useSource, "value") + 1
	locations := typeDefinitionsForSource(uriFromPath(usePath), useSource, offsetPosition(useSource, offset))
	want := offsetPosition(typeSource, strings.Index(typeSource, "Widget"))
	if len(locations) != 1 || locations[0].URI != uriFromPath(typePath) || locations[0].Range.Start != want {
		t.Fatalf("cross-file type definition = %+v, want %s at %+v", locations, uriFromPath(typePath), want)
	}
}

func TestTypeDefinitionProtocolSurface(t *testing.T) {
	var initialize bytes.Buffer
	initializing := &server{out: &initialize}
	if err := initializing.handle(rpcMessage{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "initialize", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(initialize.String(), `"typeDefinitionProvider":true`) {
		t.Fatalf("initialize does not advertise type definitions: %s", initialize.String())
	}
	if !isDeferrableAnalysisRequest("textDocument/typeDefinition") {
		t.Fatal("type definition request is not classified as read-only analysis")
	}

	source := "module main\n\ntype Widget struct { value: int, }\nfn Use(value: Widget) Widget { return value }\n"
	uri := "file:///type-definition.sec"
	position := offsetPosition(source, strings.LastIndex(source, "value")+1)
	params, err := json.Marshal(definitionParams{TextDocument: textDocumentIdentifier{URI: uri}, Position: position})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	s := &server{
		out:                  &output,
		documentSnapshots:    lspserver.NewDocuments(),
		diagnosticTimers:     map[string]*time.Timer{},
		diagnosticGeneration: map[string]uint64{},
		diagnosticDelay:      time.Hour,
	}
	s.documentSnapshots.Open(uri, 1, source)
	if err := s.handle(rpcMessage{JSONRPC: "2.0", ID: json.RawMessage("2"), Method: "textDocument/typeDefinition", Params: params}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"id":2`) || !strings.Contains(output.String(), `"uri":"file:///type-definition.sec"`) {
		t.Fatalf("type-definition protocol response = %s", output.String())
	}
}
