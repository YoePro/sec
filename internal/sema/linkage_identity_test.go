package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// @link_name changes only the foreign linkage identity of an extern
// declaration: Sec code calls it by its Sec name, its ABI family and
// effects stay those of the declaration, a second declaration with the same
// Sec signature is a duplicate whatever its link name, two Sec declarations
// cannot share one foreign symbol, and @link_name is not accepted on Sec
// definitions.
// Rules: rules/compiler/linking.md — §§ 6, 11; rules/analysis/call_graph.md — § 19(2).
func TestLinkNameIsOnlyForeignLinkageIdentity(t *testing.T) {
	valid := `module main

@link_name("c_abs")
extern "C" fn Abs(value: int32) int32

fn Use() int32 {
    return Abs(-3)
}
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, valid)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	functions := analyzer.functions["Abs"]
	if len(functions) != 1 || functions[0].LinkName != "c_abs" || functions[0].ABI != "C" || functions[0].Name != "Abs" {
		t.Fatalf("Abs = %+v", functions)
	}
	if len(analyzer.functions["c_abs"]) != 0 {
		t.Error("the link name became a Sec name")
	}

	duplicate := strings.Replace(valid, "fn Use()", "@link_name(\"other_abs\")\nextern \"C\" fn Abs(value: int32) int32\n\nfn Use()", 1)
	errors = analyzeSourceRaw(t, duplicate)
	if len(errors) == 0 || errors[0].ID != diagnostics.DuplicateDeclaration {
		t.Errorf("same Sec signature with another link name: %v", errors)
	}
	shared := strings.Replace(valid, "fn Use()", "@link_name(\"c_abs\")\nextern \"C\" fn Magnitude(value: int32) int32\n\nfn Use()", 1)
	errors = analyzeSourceRaw(t, shared)
	if len(errors) == 0 || !strings.Contains(errors[0].Message, "duplicate extern symbol \"c_abs\"") {
		t.Errorf("two Sec declarations sharing one foreign symbol: %v", errors)
	}

	p := parser.New(lexer.NewWithFile("module main\n\n@link_name(\"exported\")\nfn Local() int {\n    return 1\n}\n", "main.sec"))
	p.ParseProgram()
	if joined := strings.Join(p.Errors(), "\n"); !strings.Contains(joined, "@link_name may only annotate an extern declaration") {
		t.Errorf("@link_name on a Sec definition: %v", p.Errors())
	}
}

// Sec 0.1 has no source-level weak linkage: no attribute, keyword or
// declaration form selects weak, link-once or optional linkage; such
// spellings are unknown attributes.
// Rules: rules/compiler/linking.md — § 25.
func TestNoSourceLevelWeakLinkage(t *testing.T) {
	for _, spelling := range []string{"@weak", "@linkonce", "@linkage(\"weak\")", "@optional"} {
		p := parser.New(lexer.NewWithFile("module main\n\n"+spelling+"\nextern \"C\" fn Hook() void\n", "main.sec"))
		p.ParseProgram()
		if joined := strings.Join(p.Errors(), "\n"); !strings.Contains(joined, "unknown attribute") {
			t.Errorf("%s: errors = %v", spelling, p.Errors())
		}
	}
	for _, info := range parser.CompilerKnownAttributes() {
		if strings.Contains(strings.ToLower(info.Name), "weak") || strings.Contains(strings.ToLower(info.Name), "link") && info.Name != "link_name" {
			t.Errorf("attribute %s introduces a linkage mechanism", info.Name)
		}
	}
}
