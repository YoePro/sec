package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIPCErrorLSP consumes core enum facts for exact hover/completion and real
// declaration/variant navigation; no separate IPC type model is introduced.
// Rules: rules/concurrency/ipc.md — §§6,27,44,55,68,79;
// rules/tooling/lsp.md — Hover, Completion and Go to definition.
func TestIPCErrorLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/ipc/error_values_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "ipc.sec"))
	for _, diagnostic := range analyze(uri, source) {
		if diagnostic.Severity == 1 {
			t.Fatal(diagnostic)
		}
	}
	for _, test := range []struct {
		name, variant string
		variants      []string
	}{
		{"IPCError", "ProtocolFailure", []string{"Closed", "PeerClosed", "TransferFailed", "MaterializationFailed", "OutOfMemory", "ResourceLimit", "ProtocolFailure", "NativeFailure"}},
		{"SharedMemoryError", "MappingFailed", []string{"Closed", "InvalidSize", "PermissionDenied", "OutOfMemory", "ResourceLimit", "MappingFailed", "DuplicationFailed", "NativeFailure"}},
		{"IPCSyncError", "OwnerDied", []string{"Closed", "InvalidCount", "LimitExceeded", "OwnerDied", "PermissionDenied", "OutOfMemory", "ResourceLimit", "NativeFailure"}},
		{"IPCSharedError", "LayoutMismatch", []string{"Closed", "LayoutMismatch", "OwnerDied", "PermissionDenied", "OutOfMemory", "ResourceLimit", "MappingFailed", "DuplicationFailed", "NativeFailure"}},
		{"IPCAtomicError", "DuplicationFailed", []string{"PermissionDenied", "OutOfMemory", "ResourceLimit", "DuplicationFailed", "NativeFailure"}},
	} {
		offset := strings.Index(source, ") "+test.name) + 2
		hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
		if !ok || !strings.Contains(hover.Contents.Value, "enum "+test.name+" error") {
			t.Fatal(test.name, hover, ok)
		}
		for _, name := range test.variants {
			if !strings.Contains(hover.Contents.Value, name) {
				t.Fatal(name, hover)
			}
		}
		for _, index := range []int{offset, strings.Index(source, "return "+test.name+".") + len("return "+test.name+".")} {
			definitions := definitionsForSource(uri, source, offsetPosition(source, index))
			if len(definitions) != 1 || !strings.HasSuffix(definitions[0].URI, "/sec/core/ipc_errors.sec") {
				t.Fatal(test.name, definitions)
			}
		}
		variantOffset := strings.Index(source, test.name+"."+test.variant) + len(test.name) + 1
		hover, ok = hoverForSource(uri, source, offsetPosition(source, variantOffset))
		if !ok || !strings.Contains(hover.Contents.Value, test.name+"."+test.variant) {
			t.Fatal(test.name, hover, ok)
		}
		completion := source + "\nfn Complete() void {\n    discard " + test.name + ".\n}\n"
		items := completeSource(uri, completion, strings.LastIndex(completion, test.name+".\n")+len(test.name)+1)
		assertCompletionLabels(t, items, test.variants)
		if test.name == "IPCAtomicError" {
			for _, item := range items {
				if item.Label == "Closed" || item.Label == "OwnerDied" || item.Label == "Unsupported" {
					t.Fatal(items)
				}
			}
		}
	}
}
