package sema

import (
	"os"
	"strings"
	"testing"
)

func TestGenericParameterCapitalization(t *testing.T) {
	for _, test := range []struct {
		file    string
		invalid []string
	}{
		{"generic_capitalization_valid.sec", nil},
		{"generic_capitalization_invalid.sec", []string{"element", "item", "entry", "result", "valueType", "nestedType", "_hidden", "__private", "å"}},
	} {
		t.Run(test.file, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/names/" + test.file)
			if err != nil {
				t.Fatal(err)
			}
			_, errors := analyzeSourceWithAnalyzer(t, string(source))
			if len(errors) != len(test.invalid) {
				t.Fatalf("errors = %v, want %d naming errors", errors, len(test.invalid))
			}
			for _, name := range test.invalid {
				count := 0
				for _, diagnostic := range errors {
					if strings.Contains(diagnostic.Message, "generic type parameter "+name+" must begin with an uppercase letter") {
						count++
						if diagnostic.ID != "S1027" || diagnostic.Line == 0 || diagnostic.Column == 0 || diagnostic.Help == "" {
							t.Errorf("missing diagnostic metadata: %+v", diagnostic)
						}
					}
				}
				if count != 1 {
					t.Errorf("%s: got %d diagnostics, want exactly one", name, count)
				}
			}
		})
	}
}
