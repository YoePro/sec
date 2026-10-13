package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	lspserver "sec/internal/lsp/server"
)

// TestLSPTestSelectionMatchesCLI compares actual CLI discovery and selection
// with the editor endpoint for names containing path separators and regex syntax.
// Rules: rules/tooling/testing.md — §§6,39.1–39.4;
// rules/compiler/compiler_pipeline.md — §11(3–5), §92(2).
func TestLSPTestSelectionMatchesCLI(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/identity_selection_valid_test.sec")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "identity_test.sec")
	if err := os.WriteFile(file, data, 0644); err != nil {
		t.Fatal(err)
	}
	uri, text := uriFromPath(file), string(data)
	for _, pattern := range []string{"", "^one", `^one\.\*$`, "absent"} {
		result, err := discoverTests(uri, text, pattern, sourceOverlay{})
		if err != nil || !result.CompilationValid {
			t.Fatal(result, err)
		}
		cmd := exec.Command("go", "run", "../compiler", "test", "--list", "--json", "--run", pattern, file)
		output, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Modules []struct {
				Tests []struct {
					Module, Name string
					Identity     []string
				}
			}
		}
		if err := json.Unmarshal(output, &report); err != nil {
			t.Fatal(err, string(output))
		}
		if len(report.Modules) != 1 || len(report.Modules[0].Tests) != len(result.Tests) {
			t.Fatal(report, result)
		}
		for i, value := range result.Tests {
			cli := report.Modules[0].Tests[i]
			if value.Module != cli.Module || value.Name != cli.Name || !reflect.DeepEqual(value.Identity, cli.Identity) || value.Location.Range.End.Character <= value.Location.Range.Start.Character {
				t.Fatal(value, cli)
			}
			if strings.Contains(value.Name, "𐐀") && value.Location.Range.End.Character-value.Location.Range.Start.Character != len(utf16.Encode([]rune("\""+value.Name+"\""))) {
				t.Fatal("incorrect UTF-16 test range", value)
			}
		}
	}
	var output bytes.Buffer
	s := &server{out: &output, documentSnapshots: lspserver.NewDocuments()}
	s.documentSnapshots.Open(uri, 2, strings.Replace(text, "second", "third", 1))
	params, _ := json.Marshal(testDiscoveryParams{TextDocument: textDocumentIdentifier{URI: uri}, Run: "^third$"})
	if err := s.handle(rpcMessage{Method: "sec/tests", ID: json.RawMessage("7"), Params: params}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"third"`) || strings.Contains(output.String(), `"second"`) {
		t.Fatal(output.String())
	}
	if _, err := discoverTests(uri, text, "[", sourceOverlay{}); err == nil {
		t.Fatal("invalid selection accepted")
	}
	invalidData, err := os.ReadFile("../../testdata/tooling/identity_selection_invalid_test.sec")
	if err != nil {
		t.Fatal(err)
	}
	invalid, err := discoverTests(uri, string(invalidData), "", sourceOverlay{})
	if err != nil || invalid.CompilationValid || len(invalid.Tests) != 0 {
		t.Fatal(invalid, err)
	}
}
