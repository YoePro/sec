package main

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

func TestAnalyzeLoadsFmtPackageAndTransitiveImports(t *testing.T) {
	source := `module main

import "fmt"

fn main() void {
    fmt.Println("Hello")
}
`

	diagnostics := analyze("file:///tmp/sec-lsp-import-test/main.sec", source)
	if len(diagnostics) == 0 {
		return
	}
	messages := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		messages = append(messages, diagnostic.Message)
	}
	t.Fatalf("analyze returned diagnostics for fmt package and transitive io import:\n%s", strings.Join(messages, "\n"))
}

func TestAnalyzeRecognizesBitBackedEnumSyntax(t *testing.T) {
	source := `module main

enum Flag: bit {
    Clear = 0,
    Set = 1,
}

enum Mode: bit[2] {
    Off = 0,
    On = 1,
}
`

	diagnostics := analyze("file:///tmp/sec-lsp-bit-test/main.sec", source)
	if len(diagnostics) == 0 {
		return
	}
	messages := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		messages = append(messages, diagnostic.Message)
	}
	t.Fatalf("analyze returned diagnostics for valid bit-backed enums:\n%s", strings.Join(messages, "\n"))
}

func TestAnalyzeGroupedPackageImportsAndShortNames(t *testing.T) {
	source := `module main

import (
    "platform/linux/amd64"
    "fmt"
)

fn main() int {
    fmt.Println("grouped")
    return int(amd64.sysCall.Write)
}
`

	diagnostics := analyze("file:///tmp/sec-lsp-grouped-import-test/main.sec", source)
	if len(diagnostics) == 0 {
		return
	}
	messages := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		messages = append(messages, diagnostic.Message)
	}
	t.Fatalf("analyze returned diagnostics for grouped package imports:\n%s", strings.Join(messages, "\n"))
}

func TestAnalyzeAllowsCoreStringImpl(t *testing.T) {
	source := `module string

impl string {
	fn Len() uint {
		return self.len
	}
}
`

	diagnostics := analyze("file:///tmp/sec-lsp-core-test/sec/core/string.sec", source)
	if len(diagnostics) == 0 {
		return
	}
	messages := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		messages = append(messages, diagnostic.Message)
	}
	t.Fatalf("analyze returned diagnostics for core string impl:\n%s", strings.Join(messages, "\n"))
}

func TestAnalyzeLoadsCoreLibrary(t *testing.T) {
	source := `module main

fn IsBlank(value: string) bool {
	return value.IsEmpty()
}
`

	diagnostics := analyze("file:///tmp/sec-lsp-core-load-test/main.sec", source)
	if len(diagnostics) == 0 {
		return
	}
	messages := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		messages = append(messages, diagnostic.Message)
	}
	t.Fatalf("analyze returned diagnostics for core method without import:\n%s", strings.Join(messages, "\n"))
}

func TestSemaDiagnosticIncludesCodeAndHelp(t *testing.T) {
	diagnostic := semaDiagnostic(sema.Error{
		ID:       diagnostics.LargeValueParameter,
		Severity: diagnostics.SeverityInformation,
		Help:     "Pass the parameter by shared reference.",
		Message:  "parameter \"frame\" passes large value Frame by value",
		Line:     4,
		Column:   12,
	}, 2)

	if diagnostic.Code != diagnostics.LargeValueParameter {
		t.Fatalf("wrong diagnostic code. got=%q want=%s", diagnostic.Code, diagnostics.LargeValueParameter)
	}
	if diagnostic.Severity != 3 {
		t.Fatalf("wrong diagnostic severity. got=%d want=3", diagnostic.Severity)
	}
	if !strings.Contains(diagnostic.Message, "parameter \"frame\" passes large value Frame by value") {
		t.Fatalf("missing diagnostic message. got=%q", diagnostic.Message)
	}
	if !strings.Contains(diagnostic.Message, "help: Pass the parameter by shared reference.") {
		t.Fatalf("missing diagnostic help. got=%q", diagnostic.Message)
	}
}

func TestParserDiagnosticIncludesCode(t *testing.T) {
	diagnostic := parserDiagnostic("no prefix parse function for \"}\" at 3:1")

	if diagnostic.Code != diagnostics.ParserSyntaxError {
		t.Fatalf("wrong diagnostic code. got=%q want=%s", diagnostic.Code, diagnostics.ParserSyntaxError)
	}
	if diagnostic.Severity != 1 {
		t.Fatalf("wrong diagnostic severity. got=%d want=1", diagnostic.Severity)
	}
}

func TestCompletionSurvivesIncompleteFunctionWithURI(t *testing.T) {
	source := `module main

fn `

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("completeSource panicked for incomplete function: %v", r)
		}
	}()

	items := completeSource("file:///tmp/sec-lsp-incomplete/main.sec", source, len(source))
	assertCompletionLabels(t, items, []string{"fn", "return", "struct"})
}

func TestProgramContainsCoreSourceSkipsNilStatements(t *testing.T) {
	program := &ast.Program{Statements: []ast.Statement{
		(*ast.FunctionDeclaration)(nil),
	}}

	if lspProgramContainsCoreSource(program) {
		t.Fatal("nil statement should not be treated as core source")
	}
}

func TestFindSelectorLHS(t *testing.T) {
	text := `module main

fn main() void {
    let value := foo.bar
}
`

	l := lexer.New(text)
	p := parser.New(l)
	program := p.ParseProgram()
	if program == nil {
		t.Fatal("expected parsed program")
	}

	dotOffset := strings.Index(text, ".")
	if dotOffset < 0 {
		t.Fatal("expected dot in source")
	}
	expr := findSelectorLHS(program, text, dotOffset)
	if expr == nil {
		t.Fatal("expected expression before dot")
	}

	if got := expr.String(); got != "foo" {
		t.Fatalf("expected selector lhs foo, got %q", got)
	}
}

func TestCompletionReturnsStructMembersAfterDot(t *testing.T) {
	source := `module main

type Point struct {
	start: int,
	size: int,
	count: int,
}

impl Point {
	property span: int {
		get {
			return size
		}
	}

	fn scale() int {
		return 1
	}
}

fn main() void {
	let point := Point{ start: 0, size: 1, count: 2 }
	point.
}
`

	items := completeSource("", source, strings.Index(source, "point.")+len("point."))
	assertCompletionLabels(t, items, []string{"count", "scale", "size", "span", "start"})
}

func TestCompletionFiltersStructMembersAfterDotPrefix(t *testing.T) {
	source := `module main

type Point struct {
	start: int,
	size: int,
	count: int,
}

fn main() void {
	let point := Point{ start: 0, size: 1, count: 2 }
	point.s
}
`

	items := completeSource("", source, strings.Index(source, "point.s")+len("point.s"))
	assertCompletionLabels(t, items, []string{"size", "start"})
	assertNoCompletionLabel(t, items, "count")
}

func TestCompletionReturnsGlobalSymbolsForPrefix(t *testing.T) {
	source := `module main

type Sensor struct {
	value: int,
}

fn sample() int {
	return 1
}

fn send() void {
}

fn main() void {
	let speed := 1
	s
}
`

	items := completeSource("", source, strings.LastIndex(source, "\ts")+2)
	assertCompletionLabels(t, items, []string{"Sensor", "sample", "send", "spawn", "speed", "static", "struct", "switch"})
}

func TestCompletionPrefersTypeFormsInTypeDeclaration(t *testing.T) {
	source := `module main

type Sample s
`

	items := completeSource("", source, strings.Index(source, "type Sample s")+len("type Sample s"))
	assertCompletionLabels(t, items, []string{"struct"})
	assertNoCompletionLabel(t, items, "switch")
}

func TestCompletionIncludesIntrinsicTypesInTypePosition(t *testing.T) {
	source := `module main

type Holder struct {
	ptr: RawPtr[byte],
	values: list[int],
}
`

	items := completeSource("", source, strings.Index(source, "RawPtr")+len("Ra"))
	assertCompletionLabels(t, items, []string{"RawPtr"})

	items = completeSource("", source, strings.Index(source, "list")+len("li"))
	assertCompletionLabels(t, items, []string{"list"})
}

func TestCompletionIncludesRawPtrMembers(t *testing.T) {
	source := `module main

fn Use(ptr: RawPtr[byte]) void {
	ptr.
}
`

	items := completeSource("", source, strings.Index(source, "ptr.")+len("ptr."))
	assertCompletionLabels(t, items, []string{"Offset", "AddBytes", "Difference"})
}

func TestCompletionIncludesContractModifiers(t *testing.T) {
	cases := []struct {
		source string
		labels []string
	}{
		{"type PageSize int m", []string{"multipleOf"}},
		{"type Tags string[] n", []string{"notEmpty"}},
		{"type Tags string[] u", []string{"unique"}},
		{"type Measurement float f", []string{"finite"}},
		{"type OddNumber int o", []string{"odd"}},
		{"type EvenNumber int e", []string{"even"}},
	}

	for _, tc := range cases {
		source := "module main\n\n" + tc.source
		items := completeSource("", source, len(source))
		assertCompletionLabels(t, items, tc.labels)
	}
}

func TestCompletionIncludesSpawnModifiers(t *testing.T) {
	cases := []struct {
		source string
		labels []string
	}{
		{"spawn t", []string{"task", "thread"}},
		{"spawn p", []string{"process"}},
	}

	for _, tc := range cases {
		source := "module main\n\nfn main() void {\n\tlet worker := " + tc.source + "\n}\n"
		items := completeSource("", source, strings.Index(source, tc.source)+len(tc.source))
		assertCompletionLabels(t, items, tc.labels)
	}
}

func TestAnalyzeReportsUniqueContractOnString(t *testing.T) {
	source := `module main

type Bad string unique
`

	diagnostics := analyze("file:///tmp/sec-lsp-contract-test/main.sec", source)
	assertDiagnosticMessage(t, diagnostics, "unique contract does not apply to string")
}

func TestCompletionFiltersBoolReturnValues(t *testing.T) {
	source := `module main

fn Check() bool {
	return true
}

fn main(ready: bool, name: string) bool {
	return /*cursor*/
}
`

	offset := strings.Index(source, "/*cursor*/")
	source = strings.Replace(source, "/*cursor*/", "", 1)
	items := completeSource("", source, offset)
	assertCompletionLabels(t, items, []string{"Check", "false", "ready", "true"})
	assertNoCompletionLabel(t, items, "fallthrough")
	assertNoCompletionLabel(t, items, "name")
	assertNoCompletionLabel(t, items, "string")
}

func TestCompletionFiltersStringReturnValues(t *testing.T) {
	source := `module main

fn Title() string {
	return "sec"
}

fn main(ready: bool, title: string) string {
	return t
}
`

	items := completeSource("", source, strings.Index(source, "return t")+len("return t"))
	assertCompletionLabels(t, items, []string{"Title", "title"})
	assertNoCompletionLabel(t, items, "true")
	assertNoCompletionLabel(t, items, "fallthrough")
	assertNoCompletionLabel(t, items, "ready")
}

func TestFormatSourceIndentsSwitchCaseBodies(t *testing.T) {
	input := `module main

fn Classify(value: int) int {
switch value {
case 0:
return 0
case 1, 2:
if value == 1 {
return 10
}
return 20
default:
// Unknown value.
return -1
}
}
`

	want := `module main

fn Classify(value: int) int {
    switch value {
        case 0:
            return 0
        case 1, 2:
            if value == 1 {
                return 10
            }
            return 20
        default:
            // Unknown value.
            return -1
    }
}
`

	if got := formatSource(input); got != want {
		t.Fatalf("formatSource() mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestFormatSourceIndentsSelectBranches(t *testing.T) {
	input := `fn Use(rx: Receiver[int], tx: Sender[int]) void {
select {
value := rx.Receive() => {
discard value
}
tx.Send(1) => {
return
}
after 10 => {
return
}
default => {
return
}
}
}
`

	want := `fn Use(rx: Receiver[int], tx: Sender[int]) void {
    select {
        value := rx.Receive() => {
            discard value
        }
        tx.Send(1) => {
            return
        }
        after 10 => {
            return
        }
        default => {
            return
        }
    }
}
`

	if got := formatSource(input); got != want {
		t.Fatalf("formatSource() mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func assertCompletionLabels(t *testing.T, items []completionItem, labels []string) {
	t.Helper()
	got := map[string]bool{}
	for _, item := range items {
		got[item.Label] = true
	}
	for _, label := range labels {
		if !got[label] {
			t.Fatalf("missing completion %q in %+v", label, items)
		}
	}
}

func assertNoCompletionLabel(t *testing.T, items []completionItem, label string) {
	t.Helper()
	for _, item := range items {
		if item.Label == label {
			t.Fatalf("unexpected completion %q in %+v", label, items)
		}
	}
}

func assertDiagnosticMessage(t *testing.T, diagnostics []diagnostic, message string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, message) {
			return
		}
	}
	t.Fatalf("missing diagnostic containing %q in %+v", message, diagnostics)
}

func TestFormatSourceIndentsGroupedImports(t *testing.T) {
	input := `module main

import (
"fmt"
sys "platform/linux/amd64"
)
`

	want := `module main

import (
    "fmt"
    sys "platform/linux/amd64"
)
`

	if got := formatSource(input); got != want {
		t.Fatalf("formatSource() mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestFormatSourceIndentsNestedSwitchCaseBodies(t *testing.T) {
	input := `fn Nested(outer: int, inner: bool) int {
switch outer {
case 1:
switch inner {
case true:
return 10
case false:
return 11
}
default:
return 0
}
}
`

	want := `fn Nested(outer: int, inner: bool) int {
    switch outer {
        case 1:
            switch inner {
                case true:
                    return 10
                case false:
                    return 11
            }
        default:
            return 0
    }
}
`

	if got := formatSource(input); got != want {
		t.Fatalf("formatSource() mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}
