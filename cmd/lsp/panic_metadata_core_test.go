package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LSP consumes the compiler's real core metadata and source declaration tokens.
// Rules: rules/errors/panic.md §13(3–7); rules/tooling/lsp.md — Navigation.
func TestPanicMetadataCoreLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/panic_info_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	if errors := analyze(uri, source); len(errors) != 0 {
		t.Fatal(errors)
	}
	locations := typeDefinitionsForSource(uri, source, offsetPosition(source, strings.Index(source, "PanicInfo")+1))
	if len(locations) != 1 || !strings.HasSuffix(locations[0].URI, "/sec/core/panic.sec") {
		t.Fatal("PanicInfo did not resolve to core source", locations)
	}
}
