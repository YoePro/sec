package sema

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestStringEnumSwitchAndDefaults shares the closed alias-normalized domain
// with switch flow and semantic default initialization, including nonzero text.
// Rules: rules/declarations/enums.md — §§7–8,13;
// rules/control-flow/flowcontrol_switch.md — §§20,24,32.
func TestStringEnumSwitchAndDefaults(t *testing.T) {
	path := "../../testdata/tooling/string_enum_surfaces/valid.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(source), path)).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	analyzer := NewAnalyzer()
	if errs := analyzer.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatal(errs)
	}
	for name, want := range map[string]string{"Status": "Status.Active", "Implicit": "Implicit.First"} {
		value, kind, ok := DefaultValueDisplay(analyzer.Types()[name])
		if !ok || kind != EnumDefault || value != want {
			t.Fatalf("%s default = %s (%s), want %s", name, value, kind, want)
		}
	}
	checked := false
	ast.Visit(parsed.Program, func(node any) bool {
		if sw, ok := node.(*ast.SwitchStatement); ok {
			checked = true
			flow, ok := analyzer.ResolvedSwitchFlowOf(sw)
			if !ok || !flow.Exhaustive || flow.HasUnmatchedPath || len(flow.Clauses) != 3 {
				t.Fatalf("missing alias-normalized exhaustiveness: %+v", flow)
			}
		}
		return true
	})
	if !checked {
		t.Fatal("switch not checked")
	}
	source, err = os.ReadFile("../../testdata/tooling/string_enum_surfaces/duplicate_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errs := analyzeSourceRaw(t, string(source))
	if len(errs) != 1 || errs[0].ID != diagnostics.DuplicateSwitchCase || errs[0].Line != 11 || errs[0].PreviousLine != 9 {
		t.Fatal(errs)
	}
}
