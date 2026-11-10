package main

import (
	"os"
	"strings"
	"testing"
)

// TestInterfaceReceiverContractLSP projects canonical parser diagnostics without publishing removed methods.
// Rules: rules/declarations/interfaces.md — §3.3; rules/tooling/lsp.md — diagnostics.
func TestInterfaceReceiverContractLSP(t *testing.T) {
	uri := "file:///interface-receivers/main.sec"
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"../../testdata/sema/interface_receiver_contract_valid.sec", 0}, {"../../testdata/parser/interface_consuming_receiver_invalid.sec", 2},
	} {
		data, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity != 1 {
				continue
			}
			count++
			if d.Code != "P2039" || !strings.Contains(d.Message, "cannot consume the complete receiver") {
				t.Fatal(d)
			}
		}
		if count != tc.errors {
			t.Fatal(tc.file, count)
		}
	}
}

// TestInterfaceReceiverCompletion excludes the rejected members from the shared
// semantic catalog instead of inventing an LSP-only interface contract.
// Rules: rules/declarations/interfaces.md — §§3.3,12.
func TestInterfaceReceiverCompletion(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/interface_consuming_receiver_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data) + "\nfn Complete(value: ref mut Resource) void {\n discard value.\n}\n"
	offset := strings.LastIndex(source, "value.") + len("value.")
	items := completeSource("file:///interface-receivers/main.sec", source, offset)
	assertCompletionLabels(t, items, []string{"Inspect", "Update", "Take"})
	for _, item := range items {
		if item.Label == "Detach" || item.Label == "Drop" {
			t.Fatal("invalid requirement exposed by completion", item)
		}
	}
}
