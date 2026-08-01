package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	lspserver "sec/internal/lsp/server"
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

func TestAnalyzeLoadsUnicodePackage(t *testing.T) {
	source := `module main

import "unicode"

fn IsLetter(ch: rune) bool {
	return unicode.IsLetter(ch)
}
`

	diagnostics := analyze("file:///tmp/sec-lsp-unicode-test/main.sec", source)
	if len(diagnostics) == 0 {
		return
	}
	messages := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		messages = append(messages, diagnostic.Message)
	}
	t.Fatalf("analyze returned diagnostics for unicode import:\n%s", strings.Join(messages, "\n"))
}

func TestRespondIncludesNullResult(t *testing.T) {
	var out bytes.Buffer
	server := &server{out: &out}

	if err := server.respond(json.RawMessage("1"), nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"result":null`) {
		t.Fatalf("response should include explicit null result, got %q", out.String())
	}
}

func TestDidCloseRemovesSnapshotAndClearsDiagnostics(t *testing.T) {
	var out bytes.Buffer
	s := &server{
		out:               &out,
		documentSnapshots: lspserver.NewDocuments(),
		diagnosticTimers:  map[string]*time.Timer{},
	}
	uri := "file:///tmp/main.sec"
	s.documentSnapshots.Open(uri, 1, "module main\n")
	params, err := json.Marshal(didCloseParams{TextDocument: textDocumentIdentifier{URI: uri}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.handle(rpcMessage{JSONRPC: "2.0", Method: "textDocument/didClose", Params: params}); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.documentSnapshots.Snapshot(uri); ok {
		t.Fatal("didClose retained the document snapshot")
	}
	if !strings.Contains(out.String(), `"diagnostics":[]`) {
		t.Fatalf("didClose did not clear diagnostics: %q", out.String())
	}
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

func TestHoverShowsResolvedTypeDefault(t *testing.T) {
	source := `module main

type Port int range 1..65535 default 8080

fn Use(value: Port) void {
}
`
	offset := strings.LastIndex(source, "Port")
	result, ok := hoverForSource("", source, offsetPosition(source, offset))
	if !ok {
		t.Fatal("missing hover for Port")
	}
	if !strings.Contains(result.Contents.Value, "Default: `8080`") || !strings.Contains(result.Contents.Value, "Source: `explicit`") {
		t.Fatalf("hover does not expose explicit default: %s", result.Contents.Value)
	}
}

func TestCompletionShowsResolvedTypeDefault(t *testing.T) {
	source := `module main

type User string in ["Admin", "User"]

fn Use() void {
    let mut value: Us
}
`
	offset := strings.LastIndex(source, "Us") + len("Us")
	items := completeSource("", source, offset)
	for _, item := range items {
		if item.Label == "User" {
			if item.Detail != `string = "Admin"` {
				t.Fatalf("User completion detail = %q", item.Detail)
			}
			return
		}
	}
	t.Fatal("missing User completion")
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

func TestLSPSourceIncludePathsLoadsProjectImportsFromSecProjectRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".sec", "sec.toml"), []byte("[project]\nname = \"sample\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	sourceFile := filepath.Join(dir, "cmd", "sec", "main.sec")
	if err := os.MkdirAll(filepath.Dir(sourceFile), 0755); err != nil {
		t.Fatal(err)
	}
	imported := filepath.Join(dir, "lexer", "token.sec")
	if err := os.MkdirAll(filepath.Dir(imported), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(imported, []byte("module token\n"), 0644); err != nil {
		t.Fatal(err)
	}

	paths := sourceIncludePaths("lexer/token", sourceFile)
	if len(paths) != 1 || paths[0] != imported {
		t.Fatalf("project import paths = %#v, want %#v", paths, []string{imported})
	}
}

func TestLSPSourceIncludePathsLoadsIOPackageFiles(t *testing.T) {
	paths := sourceIncludePaths("io", "")
	wants := map[string]bool{
		"write.linux.amd64.sec": false,
		"file.linux.amd64.sec":  false,
	}
	for _, path := range paths {
		if _, exists := wants[filepath.Base(path)]; exists {
			wants[filepath.Base(path)] = true
		}
	}
	for path, found := range wants {
		if !found {
			t.Fatalf("io package missing %s: %#v", path, paths)
		}
	}
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

func TestOwnershipCodeActionsOfferExplicitMoveFixes(t *testing.T) {
	tests := []struct {
		name          string
		source        string
		line          int
		column        int
		expectedOld   string
		expectedNew   string
		expectedTitle string
	}{
		{
			name:          "declaration",
			source:        "let second := first\n",
			line:          0,
			column:        14,
			expectedOld:   ":=",
			expectedNew:   ":<-",
			expectedTitle: "Change := to :<- to move the value (source becomes unavailable)",
		},
		{
			name:          "assignment",
			source:        "destination = source\n",
			line:          0,
			column:        14,
			expectedOld:   "=",
			expectedNew:   "<-",
			expectedTitle: "Change = to <- to move the value (source becomes unavailable)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reported := diagnostic{
				Range: lspRange{Start: position{Line: tt.line, Character: tt.column}},
				Code:  diagnostics.ImplicitMoveDisallowed,
			}
			actions := ownershipCodeActions("file:///tmp/main.sec", tt.source, []diagnostic{reported})
			if len(actions) != 1 {
				t.Fatalf("got %d actions, want 1: %#v", len(actions), actions)
			}
			action := actions[0]
			if action.Title != tt.expectedTitle || action.Kind != "quickfix" {
				t.Fatalf("unexpected action: %#v", action)
			}
			edits := action.Edit.Changes["file:///tmp/main.sec"]
			if len(edits) != 1 {
				t.Fatalf("got edits %#v, want one", edits)
			}
			edit := edits[0]
			start := lineCharToOffset(tt.source, edit.Range.Start.Line, edit.Range.Start.Character)
			end := lineCharToOffset(tt.source, edit.Range.End.Line, edit.Range.End.Character)
			if got := tt.source[start:end]; got != tt.expectedOld {
				t.Fatalf("edit replaces %q, want %q", got, tt.expectedOld)
			}
			if edit.NewText != tt.expectedNew {
				t.Fatalf("edit text = %q, want %q", edit.NewText, tt.expectedNew)
			}
		})
	}
}

func TestOwnershipCodeActionsIgnoreUnrelatedDiagnostics(t *testing.T) {
	actions := ownershipCodeActions("file:///tmp/main.sec", "let second := first\n", []diagnostic{{
		Range: lspRange{Start: position{Line: 0, Character: 14}},
		Code:  diagnostics.UnhandledMustUseResult,
	}})
	if len(actions) != 0 {
		t.Fatalf("got unexpected actions: %#v", actions)
	}
}

func TestDocumentSymbolsIncludeOutlineDeclarations(t *testing.T) {
	source := `module main

type User struct {
	id: int,
}

fn Ready() bool {
	return true
}

impl User {
	property Name: string {
		get {
			return ""
		}
	}

	fn Clear() void {
	}
}
`

	symbols := documentSymbolsForSource("", source)
	assertDocumentSymbolNames(t, symbols, []string{"main", "User", "Ready", "impl User"})

	var implSymbol *documentSymbol
	for i := range symbols {
		if symbols[i].Name == "impl User" {
			implSymbol = &symbols[i]
			break
		}
	}
	if implSymbol == nil {
		t.Fatal("missing impl User outline symbol")
	}
	assertDocumentSymbolNames(t, implSymbol.Children, []string{"Name", "Clear"})
	assertDocumentSymbolRangesContainSelections(t, symbols)
}

func TestDocumentSymbolsIgnoreTypedNilStatements(t *testing.T) {
	statements := []ast.Statement{
		(*ast.ModuleStatement)(nil),
		(*ast.TypeDeclStatement)(nil),
		(*ast.UnitDeclStatement)(nil),
		(*ast.EnumDeclaration)(nil),
		(*ast.InterfaceDeclaration)(nil),
		(*ast.FunctionDeclaration)(nil),
		(*ast.StructStatement)(nil),
		(*ast.LetStatement)(nil),
		(*ast.LetGroupStatement)(nil),
		(*ast.ImplStatement)(nil),
	}

	for _, stmt := range statements {
		if symbol, ok := documentSymbolForStatement(stmt); ok {
			t.Fatalf("typed nil statement produced symbol: %+v", symbol)
		}
	}
}

func TestSemanticTokensClassifyLineObjects(t *testing.T) {
	source := `module main

fn Check(myVar: bool) bool {
	if myVar != true {
		return false
	}
	return true
}
`

	tokens := decodeSemanticTokens(semanticTokensForSource("", source))
	assertSemanticToken(t, tokens, 3, 1, 2, "keyword")  // if
	assertSemanticToken(t, tokens, 3, 4, 5, "variable") // myVar
	assertSemanticToken(t, tokens, 3, 10, 2, "operator")
	assertSemanticToken(t, tokens, 3, 13, 4, "keyword") // true
}

func TestSemanticTokensTolerateIncompleteExpressions(t *testing.T) {
	source := `module main

fn Check(ch: rune) bool {
	if ch == {
		return false
	}
	return true
}
`

	tokens := semanticTokensForSource("", source)
	if len(tokens.Data) == 0 {
		t.Fatal("expected lexical semantic tokens for incomplete source")
	}
}

func TestCompletionIncludesCompilerKnownLen(t *testing.T) {
	source := `module main

fn Count(text: string) int {
	return le
}
`

	offset := strings.Index(source, "return le") + len("return le")
	items := completeSource("", source, offset)
	assertCompletionLabels(t, items, []string{"len"})
}

func TestHoverUsesDocCommentAboveFunction(t *testing.T) {
	source := `module main

/**
 * Returns true when the system is ready.
 */
fn Ready() bool {
	return true
}
`

	pos := position{Line: 5, Character: strings.Index(sourceLine(source, 5), "Ready") + 1}
	hover, ok := hoverForSource("", source, pos)
	if !ok {
		t.Fatal("expected hover for documented function")
	}
	if hover.Contents.Kind != "markdown" {
		t.Fatalf("hover kind = %q, want markdown", hover.Contents.Kind)
	}
	if !strings.Contains(hover.Contents.Value, "Returns true when the system is ready.") {
		t.Fatalf("wrong hover contents: %q", hover.Contents.Value)
	}
}

func TestHoverResolvesSelfMembersAndMethods(t *testing.T) {
	source := `module main

type Reader struct {
	position: int,
	storage: EventStorage[int, 4],
}

impl Reader {
	event Changed using storage

	property Current: int {
		get {
			return self.position
		}
	}

	/**
	 * Reads two input values.
	 */
	fn readTwo(kind: int) bool {
		self.position += 2
		return kind == 0
	}

	fn Equal() bool {
		let current := self.Current
		let changed := self.Changed
		discard current
		discard changed
		return self.readTwo(1)
	}
}
`

	assertHoverContains := func(needle string, expected string) {
		t.Helper()
		line := 0
		for index, value := range strings.Split(source, "\n") {
			if strings.Contains(value, needle) {
				line = index
				break
			}
		}
		pos := position{Line: line, Character: strings.Index(sourceLine(source, line), needle) + len("self.") + 1}
		hover, ok := hoverForSource("", source, pos)
		if !ok {
			t.Fatalf("expected hover for %q", needle)
		}
		if !strings.Contains(hover.Contents.Value, expected) {
			t.Fatalf("hover for %q = %q, want %q", needle, hover.Contents.Value, expected)
		}
	}

	assertHoverContains("self.position", "field position: int")
	assertHoverContains("self.Current", "property Current: int")
	assertHoverContains("self.Changed", "event Changed:")
	assertHoverContains("self.readTwo", "fn Reader.readTwo(kind: int) bool")
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

func TestFindSelectorLHSSurvivesTypedNilNodes(t *testing.T) {
	text := "value."
	dotOffset := strings.Index(text, ".")

	if expr := findSelectorLHS((*ast.BlockStatement)(nil), text, dotOffset); expr != nil {
		t.Fatalf("typed nil block returned %T, want nil", expr)
	}
	block := &ast.BlockStatement{Statements: []ast.Statement{(*ast.LetStatement)(nil)}}
	if expr := findSelectorLHS(block, text, dotOffset); expr != nil {
		t.Fatalf("typed nil statement returned %T, want nil", expr)
	}
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

func TestCompletionReturnsRuneArrayToString(t *testing.T) {
	source := `module main

fn main() string {
	let runes: rune[2] := ['A', 'B']
	runes.
}
`

	items := completeSource("", source, strings.Index(source, "runes.")+len("runes."))
	assertCompletionLabels(t, items, []string{"ToString"})
}

func TestCompletionReturnsInferredMethodResultMembersInIncompleteIf(t *testing.T) {
	source := `module main

type StringBody struct {
	text: string,
	terminated: bool,
}

type Reader struct {
	position: int,
}

impl Reader {
	fn readStringBody(escaped: bool) StringBody {
		return StringBody{ text: "", terminated: escaped }
	}

	fn scan() void {
		let rv := self.readStringBody(false)
		if rv.
	}
}
`

	items := completeSource("", source, strings.Index(source, "rv.")+len("rv."))
	assertCompletionLabels(t, items, []string{"terminated", "text"})
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

func assertDocumentSymbolNames(t *testing.T, symbols []documentSymbol, names []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, symbol := range symbols {
		seen[symbol.Name] = true
	}
	for _, name := range names {
		if !seen[name] {
			t.Fatalf("missing document symbol %q in %+v", name, symbols)
		}
	}
}

func assertDocumentSymbolRangesContainSelections(t *testing.T, symbols []documentSymbol) {
	t.Helper()
	for _, symbol := range symbols {
		if comparePosition(symbol.SelectionRange.Start, symbol.Range.Start) < 0 ||
			comparePosition(symbol.SelectionRange.End, symbol.Range.End) > 0 {
			t.Fatalf("document symbol %q selectionRange %+v is not contained in range %+v", symbol.Name, symbol.SelectionRange, symbol.Range)
		}
		assertDocumentSymbolRangesContainSelections(t, symbol.Children)
	}
}

type decodedSemanticToken struct {
	Line      int
	Start     int
	Length    int
	TokenType string
}

func decodeSemanticTokens(tokens semanticTokens) []decodedSemanticToken {
	decoded := []decodedSemanticToken{}
	line := 0
	start := 0
	for i := 0; i+4 < len(tokens.Data); i += 5 {
		line += tokens.Data[i]
		if tokens.Data[i] == 0 {
			start += tokens.Data[i+1]
		} else {
			start = tokens.Data[i+1]
		}
		tokenType := ""
		if tokens.Data[i+3] >= 0 && tokens.Data[i+3] < len(semanticTokenTypes) {
			tokenType = semanticTokenTypes[tokens.Data[i+3]]
		}
		decoded = append(decoded, decodedSemanticToken{
			Line:      line,
			Start:     start,
			Length:    tokens.Data[i+2],
			TokenType: tokenType,
		})
	}
	return decoded
}

func assertSemanticToken(t *testing.T, tokens []decodedSemanticToken, line int, start int, length int, tokenType string) {
	t.Helper()
	for _, token := range tokens {
		if token.Line == line && token.Start == start && token.Length == length {
			if token.TokenType != tokenType {
				t.Fatalf("semantic token at %d:%d type = %q, want %q; tokens=%+v", line, start, token.TokenType, tokenType, tokens)
			}
			return
		}
	}
	t.Fatalf("missing semantic token at %d:%d length %d type %q; tokens=%+v", line, start, length, tokenType, tokens)
}

func sourceLine(source string, line int) string {
	lines := strings.Split(source, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}
	return lines[line]
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

func TestFormatSourceHandlesBootstrapLexerDelimiters(t *testing.T) {
	input := `fn NewWithFile(input: string,file: string) Result[Lexer, AllocationError] {
let runes := try input.ToRuneArray()

return Ok(Lexer {
input: runes
file: file
pos: 0
line: 1
column: 1
})
}

fn Next() Token {
return self.token(
lookupIdent(literal),
literal,
line,
column,
)
}

fn ReadBrace() Token {
switch ch {
case '{':
return self.readOne(LBRACE)
case '}':
return self.readOne(RBRACE)
}
return self.readOne(ILLEGAL)
}
`

	want := `fn NewWithFile(input: string, file: string) Result[Lexer, AllocationError] {
    let runes := try input.ToRuneArray()

    return Ok(Lexer {
        input: runes
        file: file
        pos: 0
        line: 1
        column: 1
    })
}

fn Next() Token {
    return self.token(
        lookupIdent(literal),
        literal,
        line,
        column,
    )
}

fn ReadBrace() Token {
    switch ch {
        case '{':
            return self.readOne(LBRACE)
        case '}':
            return self.readOne(RBRACE)
    }
    return self.readOne(ILLEGAL)
}
`

	if got := formatSource(input); got != want {
		t.Fatalf("formatSource() mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestFormatSourceNormalizesSingleLineFunctionParameters(t *testing.T) {
	input := `fn token(        typ: TokenType,        lexeme: string,        line: int,        column: int,) Token {
return Token{}
}
`

	want := `fn token(typ: TokenType, lexeme: string, line: int, column: int) Token {
    return Token{}
}
`

	if got := formatSource(input); got != want {
		t.Fatalf("formatSource() mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestFormatSourceNormalizesLetDeclarationListAndUnambiguousFuncTypo(t *testing.T) {
	input := `func token(        typ: TokenType,        lexeme: string,        line: int,        column: int,) Token {
let line := l.line,         column := l.column,         start := l.pos
func(callback)
}
`

	want := `fn token(typ: TokenType, lexeme: string, line: int, column: int) Token {
    let line := l.line, column := l.column, start := l.pos
    func(callback)
}
`

	if got := formatSource(input); got != want {
		t.Fatalf("formatSource() mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestFormatSourcePreservesExplicitMoveOperators(t *testing.T) {
	input := `fn Move() void {
let moved :<- source
target <- replacement
}
`

	want := `fn Move() void {
    let moved :<- source
    target <- replacement
}
`

	if got := formatSource(input); got != want {
		t.Fatalf("formatSource() mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestFormatSourceIndentsTypedDeclarationGroup(t *testing.T) {
	input := `module main

TokenType (
ILLEGAL := "ILLEGAL",
EOF := "EOF",
IDENT := "IDENT",
)

fn main() int {
return 0
}
`

	want := `module main

TokenType (
    ILLEGAL := "ILLEGAL",
    EOF := "EOF",
    IDENT := "IDENT",
)

fn main() int {
    return 0
}
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

func TestCompletionReturnsImplMembersForSelf(t *testing.T) {
	source := `module main

type Counter struct {
	value: int,
	storage: EventStorage[int, 4],
}

impl Counter {
	event Changed using storage

	property Current: int {
		get {
			return self.value
		}
	}

	fn advance() int {
		return self.value + 1
	}

	fn Complete() void {
		self.
	}
}
`

	offset := strings.LastIndex(source, "self.") + len("self.")
	items := completeSource("", source, offset)
	assertCompletionLabels(t, items, []string{"Changed", "Current", "Complete", "advance", "storage", "value"})

	for _, item := range items {
		if item.Label == "advance" && item.Detail != "int" {
			t.Fatalf("advance completion detail. got=%q want=int", item.Detail)
		}
	}
}
