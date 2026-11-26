package main

import (
	"os"
	"sec/internal/platform/projectconfig"
	"strings"
	"testing"
)

// Unsaved LSP manifests and harness plans use the same canonical profile and
// provider rules; rejected activation cannot become supported through tooling.
func TestProjectWorkspaceLSPValidationParity(t *testing.T) {
	data, err := os.ReadFile("../../testdata/projects/model/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{
		"\n[variant.linux-amd64.allocation]\nmode='noalloc'\n",
		"\n[variant.linux-amd64.allocation]\nmode='hosted'\nprovider='sec.system-heap.v1'\n",
		"\n[variant.linux-amd64.allocation]\nunknown=true\n",
	} {
		text := string(data) + suffix
		_, shared := projectconfig.Resolve(text)
		editor := planSelectionDiagnostics(text)
		if (len(shared) == 0) != (len(editor) == 0) {
			t.Fatalf("different validation: %v %v", shared, editor)
		}
		for _, diagnostic := range editor {
			found := false
			for _, canonical := range shared {
				if strings.Contains(canonical.Message, diagnostic.Message) {
					found = true
				}
			}
			if !found {
				t.Fatal(shared, editor)
			}
		}
	}
}
