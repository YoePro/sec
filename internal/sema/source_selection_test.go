package sema

import (
	"sort"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// analyzeSelected parses each file separately, as the compiler does, merges
// the statements and analyzes them under a source selection.
func analyzeSelected(t *testing.T, sources map[string]string, selection SourceSelection) (*Analyzer, []Error) {
	t.Helper()
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	program := &ast.Program{}
	for _, name := range names {
		p := parser.New(lexer.NewWithFile(sources[name], name))
		parsed := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("%s: %v", name, p.Errors())
		}
		program.Statements = append(program.Statements, parsed.Statements...)
	}
	analyzer := NewAnalyzer()
	analyzer.SetSourceSelection(selection)
	return analyzer, analyzer.Analyze(program)
}

func linuxAmd64(parameters map[string]CompileTimeParameter) SourceSelection {
	return SourceSelection{Target: map[string]string{"os": "linux", "arch": "amd64"}, Parameters: parameters}
}

// File-level and statement-level @target and @when select the active source
// of a plan: a non-matching file and non-matching declarations stay parsed
// but are absent from Sema, so their names do not resolve, while matching
// variants of one symbol coexist in the source.
// Rules: rules/foundations/attributes.md — §§ 13, 15–17, 21, 26–28.
func TestSourceSelectionActivatesMatchingSource(t *testing.T) {
	sources := map[string]string{
		"main.sec": `module main

@target(os: "linux")
fn Open() int {
    return 1
}

@target(os: "windows")
fn Open() int {
    return 2
}

@when(config.audit)
fn Audit() int {
    return 3
}

@when(!config.audit && config.level == 2)
fn Quiet() int {
    return 4
}

@target(device: "controller-a")
let Port: int := 5

fn main() int {
    return Open() + Linux() + Quiet()
}
`,
		"linux.sec":   "@target(os: \"linux\", arch: \"amd64\")\n\nmodule main\n\nfn Linux() int {\n    return 10\n}\n",
		"windows.sec": "@target(os: \"windows\")\n\nmodule main\n\nfn Windows() int {\n    return 20\n}\n",
	}
	parameters := map[string]CompileTimeParameter{"audit": {Kind: "bool", Bool: false}, "level": {Kind: "integer", Text: "2"}}
	analyzer, errors := analyzeSelected(t, sources, linuxAmd64(parameters))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	excluded := []string{}
	for _, statement := range analyzer.ExcludedDeclarations() {
		if name, _, ok := selectionIdentity(statement); ok {
			excluded = append(excluded, name)
		}
	}
	sort.Strings(excluded)
	if strings.Join(excluded, ",") != "Audit,Open,Port,Windows" {
		t.Errorf("excluded = %v", excluded)
	}
	for _, statement := range analyzer.ActiveProgram().Statements {
		if function, ok := statement.(*ast.FunctionDeclaration); ok && function.Name.Value == "Windows" {
			t.Error("an excluded file reached the active program")
		}
	}
	// The windows plan sees the other variant; the use of Linux(), defined
	// only in the linux file, is a target-aware no-active-variant error
	// (attributes.md § 24).
	_, errors = analyzeSelected(t, sources, SourceSelection{Target: map[string]string{"os": "windows", "arch": "amd64"}, Parameters: parameters})
	if len(errors) != 1 || errors[0].ID != diagnostics.NoActiveVariant ||
		errors[0].Message != "`Linux` is unavailable for this compilation plan (os = windows, arch = amd64); available variants: os = linux, arch = amd64" {
		t.Errorf("windows plan errors = %v", errors)
	}
}

// Variants active together are rejected instead of resolved by specificity,
// mutually exclusive variants must keep one public shape, and an @when
// condition must name a declared, correctly typed parameter.
// Rules: rules/foundations/attributes.md — §§ 22, 23, 27, 28.
func TestSourceSelectionVariantDiagnostics(t *testing.T) {
	source := `module main

@target(os: "linux")
fn Open() int {
    return 1
}

@target(os: "linux", arch: "amd64")
fn Open() int {
    return 2
}

@target(os: "linux")
fn Read(n: int) int {
    return n
}

@target(os: "windows")
fn Read(n: uint) int {
    return 0
}

@when(config.telemetry)
fn Send() void {
}

@when(config.name)
fn Named() void {
}
`
	parameters := map[string]CompileTimeParameter{"name": {Kind: "string", Text: "shop"}}
	_, errors := analyzeSelected(t, map[string]string{"main.sec": source}, linuxAmd64(parameters))
	want := []struct{ id, text string }{
		{diagnostics.UnknownConfigParameter, "unknown compile-time parameter config.telemetry"},
		{diagnostics.UnknownConfigParameter, "@when condition config.name is string, not bool"},
		{diagnostics.OverlappingTargetVariants, "multiple active target variants define `Open` for this compilation plan (os = linux, arch = amd64)"},
		{diagnostics.IncompatibleVariantShape, "target variant of `Read` has public shape fn[0](uint) int, but the variant at 14:4 has fn[0](int) int"},
	}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v", errors)
	}
	for index, expected := range want {
		if errors[index].ID != expected.id || !strings.Contains(errors[index].Message, expected.text) {
			t.Errorf("error %d = %s %q, want %s %q", index, errors[index].ID, errors[index].Message, expected.id, expected.text)
		}
	}
}

// Source selection is plan-time evaluation: @when never calls user code and
// @target selectors are string literals, so selecting the active source
// graph cannot depend on that graph (no phase cycle), while range bounds,
// defaults and static initializers in the same program are semantic CTE
// positions. Every required position is classified.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 4, 5, 8; rules/foundations/attributes.md — §§ 13, 28.
func TestPlanTimeSelectionNeverExecutesUserCode(t *testing.T) {
	source := `module main

fn Enabled() bool {
    return true
}

fn Limit() int {
    return 10
}

@when(Enabled())
fn Hook() void {
}

@target(os: "linux")
type Level int range 0..Limit() default Limit()

let Base: int := Limit()

fn main() int {
    static let local: int := Limit()
    return Base + local
}
`
	analyzer, errors := analyzeSelected(t, map[string]string{"main.sec": source}, linuxAmd64(nil))
	if len(errors) != 1 || errors[0].ID != diagnostics.AttributeArgumentNotCompileTime ||
		!strings.Contains(errors[0].Message, "argument of @when must be known while the compilation is planned; a function call needs run-time execution") {
		t.Fatalf("errors = %v", errors)
	}
	program := analyzer.ActiveProgram()
	contexts := map[string]string{}
	for root, context := range analyzer.compileTimeContexts {
		contexts[root.String()+" @"+context.Construct] = string(context.Class)
	}
	for key, class := range map[string]string{
		"Enabled() @@when condition":     string(PlanTimeRequiredContext),
		"\"linux\" @@target os selector": string(PlanTimeRequiredContext),
		"Limit() @range upper bound":     string(SemanticCompileTimeRequiredContext),
		"0 @range lower bound":           string(SemanticCompileTimeRequiredContext),
		"Limit() @explicit type default": string(SemanticCompileTimeRequiredContext),
		"Limit() @static initializer":    string(SemanticCompileTimeRequiredContext),
	} {
		if contexts[key] != class {
			t.Errorf("context %q = %q, want %q (all: %v)", key, contexts[key], class, contexts)
		}
	}
	if program == nil {
		t.Fatal("no active program")
	}
}
