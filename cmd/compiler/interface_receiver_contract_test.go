package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInterfaceReceiverContractCLI rejects removed interface receivers before IR emission.
// Rules: rules/declarations/interfaces.md — §3.3.
func TestInterfaceReceiverContractCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/interface_receiver_contract_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, command := range []string{"check", "emit-ir"} {
		artifact := filepath.Join(t.TempDir(), "invalid.ir")
		args := []string{command, "../../testdata/parser/interface_consuming_receiver_invalid.sec", "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", artifact)
		}
		_, output, code = runCLIForDiagnostics(t, args...)
		if code != 2 {
			t.Fatal(code, output)
		}
		if doc := decodeOccurrenceDocument(t, output); doc.Summary.Errors != 2 {
			t.Fatal(output)
		}
		if !strings.Contains(output, "P2039") || !strings.Contains(output, "parser.consuming-interface-receiver") {
			t.Fatal(output)
		}
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatal("invalid interface receiver emitted an artifact", err)
		}
	}
}
