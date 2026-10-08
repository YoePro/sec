package temporal_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

func analyzeTemporalCoreSource(t *testing.T, source string) (*sema.Analyzer, *ast.Program, []sema.Error) {
	t.Helper()
	const sourceFile = "sec/core/temporal_now_test.sec"
	parsed := parser.New(lexer.NewWithFile(source, sourceFile)).Parse()
	if parsed.HasErrors {
		t.Fatalf("parser errors: %+v", parsed.Diagnostics)
	}
	parsed.Program.SourceProvenance = map[string]ast.SourceProvenance{sourceFile: ast.SourceCore}
	analyzer := sema.NewAnalyzer()
	return analyzer, parsed.Program, analyzer.Analyze(parsed.Program)
}

// Rules:
//   - rules/compiler/compiler_known_members.md — "Private core UTC wall-clock intrinsic"
//   - rules/types/temporal.md — §3 "UTC wall-clock access"
func TestCompilerKnownNowIsTypedAndEffectfulInTrustedCore(t *testing.T) {
	analyzer, program, errors := analyzeTemporalCoreSource(t, fixture(t, "read_twice.sec"))
	assertSemaErrors(t, errors, nil)

	function, ok := program.Statements[1].(*ast.FunctionDeclaration)
	if !ok || len(function.Body.Statements) != 2 {
		t.Fatalf("unexpected program shape: %#v", program.Statements)
	}
	first := function.Body.Statements[0].(*ast.LetStatement).Value.(*ast.Identifier)
	second := function.Body.Statements[1].(*ast.ReturnStatement).Value.(*ast.Identifier)
	for _, identifier := range []*ast.Identifier{first, second} {
		known, found := analyzer.CompilerKnownValueAt(identifier.Token.File, identifier.Token.Line, identifier.Token.Column)
		if !found || known.ID != "CKV-TEMPORAL-NOW" || known.Result.Name != "datetime" ||
			known.RequiredCapability != "UTCWallClock" || len(known.Effects) != 1 ||
			known.Effects[0] != sema.EffectMayUseNondeterministicInput {
			t.Fatalf("_now fact = %+v, found %v", known, found)
		}
	}

	summary := analyzer.CallGraph().EffectSummary(callGraphNodeIDByName(t, analyzer.CallGraph(), "ReadTwice"))
	if len(summary.DirectEffects) != 2 {
		t.Fatalf("_now effects = %+v, want two distinct reads", summary.DirectEffects)
	}
	for _, effect := range summary.DirectEffects {
		if effect.Kind != sema.EffectMayUseNondeterministicInput || effect.Source.Lexeme != "_now" {
			t.Fatalf("unexpected _now effect: %+v", effect)
		}
	}
}

func TestTemporalCorePropertiesProjectOneNowRead(t *testing.T) {
	_, _, errors := analyzeTemporalCoreSource(t, fixture(t, "utc_properties.sec"))
	assertSemaErrors(t, errors, nil)
}

func TestCompilerKnownNowRejectsOrdinarySource(t *testing.T) {
	errors := analyzeSourceRaw(t, fixture(t, "now_untrusted_invalid.sec"))
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "_now is a compiler-internal value available only to loader-proven core source") {
		t.Fatalf("errors = %+v", errors)
	}
}

func TestCompilerKnownNowRejectsStaticInitialization(t *testing.T) {
	_, _, errors := analyzeTemporalCoreSource(t, fixture(t, "static_clock_invalid.sec"))
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "static initializer for Snapshot must be compile-time evaluable") {
		t.Fatalf("errors = %+v", errors)
	}
}

func TestCompilerKnownNowCannotBeDeclared(t *testing.T) {
	errors := analyzeSourceRaw(t, fixture(t, "now_declaration_invalid.sec"))
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "function _now is compiler-known and cannot be declared") {
		t.Fatalf("errors = %+v", errors)
	}
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../../../testdata/sema/temporal", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
func analyzeSourceRaw(t *testing.T, source string) []sema.Error {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, "application.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	return sema.NewAnalyzer().Analyze(program)
}
func assertSemaErrors(t *testing.T, errors []sema.Error, expected []string) {
	t.Helper()
	if len(errors) != len(expected) {
		t.Fatalf("errors = %v, want %v", errors, expected)
	}
	for i, err := range errors {
		if err.Error() != expected[i] {
			t.Fatalf("error = %v, want %s", err, expected[i])
		}
	}
}
func callGraphNodeIDByName(t *testing.T, graph *sema.CallGraph, name string) sema.CallableID {
	t.Helper()
	for _, node := range graph.Nodes() {
		if node.Name == name {
			return node.ID
		}
	}
	t.Fatalf("call graph does not contain %s", name)
	return ""
}
