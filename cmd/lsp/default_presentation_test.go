package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	"sec/internal/formatter"
)

// TestLSPPresentsEveryDefaultForm pins hover value/provenance and completion
// values before and after formatting, including all resolver kinds and types
// that have no default. The manifest is shared with formatter/Sema verification.
// Rules: rules/types/default_values.md — "LSP", "Formatter";
// rules/tooling/lsp.md — hover and completion.
func TestLSPPresentsEveryDefaultForm(t *testing.T) {
	input, err := os.ReadFile("../../testdata/defaults/default_presentation.sec")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile("../../testdata/defaults/default_presentation.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name   string
		Value  *string
		Source string
	}
	if err := json.Unmarshal(manifest, &cases); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{string(input), formatter.Format(formatter.Source{Text: string(input)}, formatter.Options{}).Text} {
		items := completeSource("", source+"\nfn Query(value: ", len(source+"\nfn Query(value: "))
		completion := map[string]string{}
		for _, item := range items {
			completion[item.Label] = item.Detail
		}
		for i, entry := range cases {
			offset := strings.Index(source, "value"+strconv.Itoa(i))
			if offset < 0 {
				t.Fatal("missing hover site", entry.Name)
			}
			result, ok := hoverForSource("", source, offsetPosition(source, offset))
			if !ok {
				t.Fatal("missing hover", entry.Name)
			}
			if entry.Name == "DefaultList" || entry.Name == "DefaultBoundedList" || entry.Name == "DefaultInheritedList" {
				if !strings.Contains(result.Contents.Value, "value"+strconv.Itoa(i)+": "+entry.Name+"\n") {
					t.Fatal("nongeneric nominal hover gained carrier parameters", result.Contents.Value)
				}
			}
			detail, exists := completion[entry.Name]
			if !exists {
				t.Fatal("missing completion", entry.Name)
			}
			if entry.Value == nil {
				if !strings.Contains(result.Contents.Value, "Default: _none_") || strings.Contains(detail, " = ") {
					t.Fatal(entry.Name, "fabricated default", result, detail)
				}
				continue
			}
			if !strings.Contains(result.Contents.Value, "Default: `"+*entry.Value+"`") || !strings.Contains(result.Contents.Value, "Source: `"+entry.Source+"`") || !strings.HasSuffix(detail, " = "+*entry.Value) {
				t.Fatalf("%s hover=%s completion=%q, want %q/%s", entry.Name, result.Contents.Value, detail, *entry.Value, entry.Source)
			}
		}
	}
}
