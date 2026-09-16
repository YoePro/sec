package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

func TestGenericParameterShadowsSameModuleType(t *testing.T) {
	for _, test := range []struct {
		file    string
		invalid []string
	}{
		{"generic_type_shadowing_valid.sec", nil},
		{"generic_type_shadowing_invalid.sec", []string{"Item", "Output", "State", "Value", "Later", "Owner"}},
	} {
		t.Run(test.file, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/names/" + test.file)
			if err != nil {
				t.Fatal(err)
			}
			_, errors := analyzeSourceWithAnalyzer(t, string(source))
			if len(errors) != len(test.invalid) {
				t.Fatalf("errors = %v, want %d shadowing errors", errors, len(test.invalid))
			}
			for _, name := range test.invalid {
				count := 0
				for _, diagnostic := range errors {
					if strings.Contains(diagnostic.Message, "generic parameter "+name+" shadows visible type "+name) {
						count++
						if diagnostic.ID != diagnostics.GenericParameterShadowsType || diagnostic.Line == 0 || diagnostic.Column == 0 || diagnostic.PreviousLine == 0 || diagnostic.PreviousColumn == 0 {
							t.Errorf("missing diagnostic or previous declaration: %+v", diagnostic)
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
