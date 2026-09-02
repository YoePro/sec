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

func TestAnalyzeLoadsDeclarationsFromSameModuleFiles(t *testing.T) {
	dir := t.TempDir()
	errorPath := filepath.Join(dir, "error.sec")
	errorSource := `module sample

enum OverflowError {
	Overflow,
}
`
	if err := os.WriteFile(errorPath, []byte(errorSource), 0644); err != nil {
		t.Fatal(err)
	}
	intPath := filepath.Join(dir, "int.sec")
	intSource := `module sample

fn Checked() Result[int, OverflowError] {
	return Ok(1)
}
`

	for _, got := range analyze(uriFromPath(intPath), intSource) {
		if strings.Contains(got.Message, "unknown type OverflowError") {
			t.Fatalf("same-module declaration was not loaded: %+v", got)
		}
	}
}

func TestAnalyzeCoreIntLoadsOverflowErrorFromErrorFile(t *testing.T) {
	intPath, err := filepath.Abs(filepath.Join("..", "..", "sec", "core", "int.sec"))
	if err != nil {
		t.Fatal(err)
	}
	intSource, err := os.ReadFile(intPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range analyze(uriFromPath(intPath), string(intSource)) {
		if strings.Contains(got.Message, "unknown type OverflowError") {
			t.Fatalf("sec/core/int.sec did not see sec/core/error.sec: %+v", got)
		}
	}
}

func TestAnalyzeUsesOpenSiblingSnapshotBeforeDisk(t *testing.T) {
	dir := t.TempDir()
	errorPath := filepath.Join(dir, "error.sec")
	if err := os.WriteFile(errorPath, []byte("module sample\n\nenum OldError { Old }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	usePath := filepath.Join(dir, "use.sec")
	useSource := "module sample\n\nfn Checked() Result[int, NewError] { return Ok(1) }\n"
	overlay := sourceOverlay{
		normalizedSourcePath(errorPath): "module sample\n\nenum NewError { New }\n",
	}

	for _, got := range analyze(uriFromPath(usePath), useSource, overlay) {
		if strings.Contains(got.Message, "unknown type NewError") {
			t.Fatalf("open sibling snapshot was not used: %+v", got)
		}
	}
}

func TestAnalyzeDoesNotLoadDifferentModuleSibling(t *testing.T) {
	dir := t.TempDir()
	siblingPath := filepath.Join(dir, "other.sec")
	if err := os.WriteFile(siblingPath, []byte("module other\n\nenum ForeignError { Foreign }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	usePath := filepath.Join(dir, "use.sec")
	useSource := "module sample\n\nfn Checked() Result[int, ForeignError] { return Ok(1) }\n"
	diagnostics := analyze(uriFromPath(usePath), useSource)
	for _, got := range diagnostics {
		if strings.Contains(got.Message, "unknown type ForeignError") {
			return
		}
	}
	t.Fatalf("different-module sibling leaked into analysis: %+v", diagnostics)
}

func TestDefinitionResolvesSameModuleDeclaration(t *testing.T) {
	dir := t.TempDir()
	errorPath := filepath.Join(dir, "error.sec")
	errorSource := "module sample\n\nenum OverflowError { Overflow }\n"
	if err := os.WriteFile(errorPath, []byte(errorSource), 0644); err != nil {
		t.Fatal(err)
	}
	usePath := filepath.Join(dir, "use.sec")
	useSource := "module sample\n\nfn Checked() Result[int, OverflowError] { return Ok(1) }\n"
	use := strings.Index(useSource, "OverflowError")

	locations := definitionsForSource(uriFromPath(usePath), useSource, offsetPosition(useSource, use))
	if len(locations) != 1 {
		t.Fatalf("definitions = %+v, want same-module declaration", locations)
	}
	want := offsetPosition(errorSource, strings.Index(errorSource, "OverflowError"))
	if locations[0].URI != uriFromPath(errorPath) || locations[0].Range.Start != want {
		t.Fatalf("definition = %+v, want %s at %+v", locations[0], uriFromPath(errorPath), want)
	}
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

func TestInitializeAdvertisesDefinitionProvider(t *testing.T) {
	var out bytes.Buffer
	server := &server{out: &out}
	if err := server.handle(rpcMessage{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "initialize", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"definitionProvider":true`) {
		t.Fatalf("initialize response does not advertise definitions: %q", out.String())
	}
	if !strings.Contains(out.String(), `"documentHighlightProvider":true`) {
		t.Fatalf("initialize response does not advertise document highlights: %q", out.String())
	}
	if !strings.Contains(out.String(), `"callHierarchyProvider":true`) {
		t.Fatalf("initialize response does not advertise call hierarchy: %q", out.String())
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

func TestDidOpenRepublishesSameModuleDiagnosticsWithOverlay(t *testing.T) {
	dir := t.TempDir()
	usePath := filepath.Join(dir, "use.sec")
	useURI := uriFromPath(usePath)
	useSource := "module sample\n\nfn Checked() Result[int, NewError] { return Ok(1) }\n"
	errorPath := filepath.Join(dir, "error.sec")

	var out bytes.Buffer
	s := &server{
		out:               &out,
		documentSnapshots: lspserver.NewDocuments(),
		diagnosticTimers:  map[string]*time.Timer{},
	}
	s.documentSnapshots.Open(useURI, 1, useSource)
	params, err := json.Marshal(didOpenParams{TextDocument: textDocumentItem{
		URI:     uriFromPath(errorPath),
		Version: 1,
		Text:    "module sample\n\nenum NewError { New }\n",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.handle(rpcMessage{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: params}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), useURI) {
		t.Fatalf("opening a sibling did not republish the existing document: %q", out.String())
	}
	if strings.Contains(out.String(), "unknown type NewError") {
		t.Fatalf("republished diagnostics ignored the open sibling snapshot: %q", out.String())
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

func TestHoverShowsCompilerOwnedUnitQuantityFacts(t *testing.T) {
	source := `module main

unit m physical
impl m {
    Dimension: [length^1]
    Kind: length
    Scale: 1 / 1000
}

fn Use(distance: m) void {
}
`
	offset := strings.LastIndex(source, "distance")
	result, ok := hoverForSource("", source, offsetPosition(source, offset))
	if !ok || !strings.Contains(result.Contents.Value, "Unit identity: named (`m`)") ||
		!strings.Contains(result.Contents.Value, "Kind: `length`") ||
		!strings.Contains(result.Contents.Value, "Exact scale: `1/1000`") {
		t.Fatalf("unit hover = %+v, %v", result, ok)
	}
}

func TestHoverUsesCompilerOwnedCallableCapabilityFacts(t *testing.T) {
	source := `module main

fn Apply(mutable: mut fn(int) int, consuming: -> fn() int) int {
    discard mutable(1)
    return consuming()
}
`
	mutableOffset := strings.LastIndex(source, "mutable")
	mutable, ok := hoverForSource("", source, offsetPosition(source, mutableOffset))
	if !ok || !strings.Contains(mutable.Contents.Value, "mutable: mut fn(int) int") ||
		!strings.Contains(mutable.Contents.Value, "Callable capability: `mut fn`") ||
		!strings.Contains(mutable.Contents.Value, "mutable/exclusive callable access") {
		t.Fatalf("mutable callable hover = %+v, %v", mutable, ok)
	}

	consumingOffset := strings.LastIndex(source, "consuming")
	consuming, ok := hoverForSource("", source, offsetPosition(source, consumingOffset))
	if !ok || !strings.Contains(consuming.Contents.Value, "consuming: -> fn() int") ||
		!strings.Contains(consuming.Contents.Value, "Callable capability: `-> fn`") ||
		!strings.Contains(consuming.Contents.Value, "successful invocation consumes the callable value") {
		t.Fatalf("consuming callable hover = %+v, %v", consuming, ok)
	}
}

func TestHoverUsesCompilerOwnedRegisterFieldAccess(t *testing.T) {
	source := `module main

type Device register[2] {
    Ready: bit read-only,
    Command: bit write-only,
}

fn Read(device: ref Device) bool {
    return device.Ready
}
`
	offset := strings.LastIndex(source, "Ready")
	result, ok := hoverForSource("", source, offsetPosition(source, offset))
	if !ok || !strings.Contains(result.Contents.Value, "register field Ready: bool") ||
		!strings.Contains(result.Contents.Value, "Access: `read-only`") {
		t.Fatalf("register access hover = %+v, %v", result, ok)
	}
}

func TestNestedRegisterHoverAndCheckedConversionUseSemaFacts(t *testing.T) {
	source := `module main

type Outer register[4] { Flags: Flags }
type Flags register[4] { Ready: bit read-only, _: bit[3] }

fn Inspect(value: ref Outer, raw: uint16) void {
    let ready := value.Flags.Ready
    let checked := Outer(raw)
}
`
	readyOffset := strings.LastIndex(source, "Ready")
	ready, ok := hoverForSource("", source, offsetPosition(source, readyOffset))
	if !ok || !strings.Contains(ready.Contents.Value, "register field Ready: bool") || !strings.Contains(ready.Contents.Value, "Access: `read-only`") {
		t.Fatalf("nested register hover = %+v, %v", ready, ok)
	}
	checkedOffset := strings.Index(source, "checked")
	checked, ok := hoverForSource("", source, offsetPosition(source, checkedOffset))
	if !ok || !strings.Contains(checked.Contents.Value, "checked: Result[Outer, ArithmeticError]") {
		t.Fatalf("checked conversion hover = %+v, %v", checked, ok)
	}
}

func TestHoverAbbreviatesLargeArrayDefault(t *testing.T) {
	source := `module main

fn Use() void {
    let mut values: int[2048]
    values[0] = 1
}
`
	offset := strings.Index(source, "values")
	result, ok := hoverForSource("", source, offsetPosition(source, offset))
	if !ok {
		t.Fatal("missing hover for large fixed array")
	}
	if !strings.Contains(result.Contents.Value, "Default: `[0, ...]`") {
		t.Fatalf("large array hover is not abbreviated: %s", result.Contents.Value)
	}
	if strings.Contains(result.Contents.Value, "0, 0, 0, 0, 0, 0, 0, 0") {
		t.Fatalf("large array hover rendered the full default: %s", result.Contents.Value)
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
    return int(amd64.SysCall.Write)
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
	return value.IsEmpty
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

func TestStructuredParserDiagnosticUsesFocusedCodeAndTokenRange(t *testing.T) {
	token := lexer.Token{Type: lexer.ASSIGN, Lexeme: "=", Line: 5, Column: 16}
	diagnostic := structuredParserDiagnostic(parser.Diagnostic{
		ID:         diagnostics.ParserInvalidAssignmentExpr,
		Message:    "assignment in while condition at 5:16",
		Primary:    token,
		Unexpected: &token,
	})

	if diagnostic.Code != diagnostics.ParserInvalidAssignmentExpr {
		t.Fatalf("wrong diagnostic code. got=%q want=%s", diagnostic.Code, diagnostics.ParserInvalidAssignmentExpr)
	}
	if diagnostic.Range.Start.Line != 4 || diagnostic.Range.Start.Character != 15 {
		t.Fatalf("wrong diagnostic start: %+v", diagnostic.Range.Start)
	}
}

func TestAnalyzePublishesFocusedWhileAssignmentDiagnostic(t *testing.T) {
	source := `module main

fn main() void {
    let mut running: bool := false
    while running = true {
        break
    }
}
`
	diagnosticsResult := analyze("file:///tmp/main.sec", source)
	if len(diagnosticsResult) != 1 {
		t.Fatalf("diagnostics = %+v, want one parser diagnostic", diagnosticsResult)
	}
	if diagnosticsResult[0].Code != diagnostics.ParserInvalidAssignmentExpr {
		t.Fatalf("diagnostic code = %q, want %s", diagnosticsResult[0].Code, diagnostics.ParserInvalidAssignmentExpr)
	}
	if diagnosticsResult[0].Range.Start.Line != 4 || diagnosticsResult[0].Range.Start.Character != 18 {
		t.Fatalf("diagnostic range = %+v", diagnosticsResult[0].Range)
	}
}

// TestAnalyzePublishesMoveOnlyIndexedReturnDiagnostic proves that the LSP uses
// the canonical ownership diagnostic for an invalid ordinary element read at a
// transferring return boundary.
//
// Rules:
//   - rules/collections/collections.md — "Move-only indexed reads"
//   - rules/tooling/lsp.md — semantic diagnostics
func TestAnalyzePublishesMoveOnlyIndexedReturnDiagnostic(t *testing.T) {
	source, err := os.ReadFile("../../testdata/semantic_ir/package14_unsupported/move_only_read_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	reported := analyze("file:///tmp/sec-lsp-array-ownership/main.sec", string(source))
	if len(reported) != 1 {
		t.Fatalf("diagnostics = %+v, want one ownership diagnostic", reported)
	}
	diagnostic := reported[0]
	if diagnostic.Code != diagnostics.ImplicitMoveDisallowed {
		t.Fatalf("diagnostic code = %q, want %s", diagnostic.Code, diagnostics.ImplicitMoveDisallowed)
	}
	want := "cannot return Token element values[0] by ordinary indexing because it is not implicitly copyable"
	if !strings.Contains(diagnostic.Message, want) || !strings.Contains(diagnostic.Message, "help: move the containing collection or return a reference to the element") {
		t.Fatalf("diagnostic message = %q, want ownership message and help", diagnostic.Message)
	}
	if diagnostic.Range.Start.Line != 6 || diagnostic.Range.Start.Character != 17 {
		t.Fatalf("diagnostic start = %+v, want line 6 character 17", diagnostic.Range.Start)
	}
}

// TestAnalyzePublishesMoveOnlyIndexedResultReturnDiagnostics proves that both
// terminal Result arms retain the ordinary-index ownership restriction and
// publish the canonical diagnostic through the LSP.
//
// Rules:
//   - rules/collections/collections.md — "Move-only indexed reads"
//   - rules/memory/copy_move.md — §10.3 "Result construction"
//   - rules/tooling/lsp.md — semantic diagnostics
func TestAnalyzePublishesMoveOnlyIndexedResultReturnDiagnostics(t *testing.T) {
	source, err := os.ReadFile("../../testdata/semantic_ir/package14_unsupported/move_only_result_read_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	reported := analyze("file:///tmp/sec-lsp-array-ownership/result.sec", string(source))
	if len(reported) != 2 {
		t.Fatalf("diagnostics = %+v, want Ok and Err ownership diagnostics", reported)
	}
	want := []struct {
		typeName  string
		line      int
		character int
	}{
		{typeName: "Token", line: 11, character: 20},
		{typeName: "MoveError", line: 15, character: 21},
	}
	for index, diagnostic := range reported {
		if diagnostic.Code != diagnostics.ImplicitMoveDisallowed {
			t.Fatalf("diagnostic %d code = %q, want %s", index, diagnostic.Code, diagnostics.ImplicitMoveDisallowed)
		}
		message := "cannot return " + want[index].typeName + " element values[0] by ordinary indexing because it is not implicitly copyable"
		if !strings.Contains(diagnostic.Message, message) || !strings.Contains(diagnostic.Message, "help: move the containing collection or return a reference to the element") {
			t.Fatalf("diagnostic %d message = %q, want ownership message and help", index, diagnostic.Message)
		}
		if diagnostic.Range.Start.Line != want[index].line || diagnostic.Range.Start.Character != want[index].character {
			t.Fatalf("diagnostic %d start = %+v, want line %d character %d", index, diagnostic.Range.Start, want[index].line, want[index].character)
		}
	}
}

// TestAnalyzePackage14UnsupportedFixturesRespectFrontendBoundary keeps LSP
// diagnostics distinct from the later Package 14 lowering capability boundary:
// frontend-invalid ownership operations are reported, while valid arrays,
// element borrows, slices and replacement remain valid source programs.
//
// Rules:
//   - rules/collections/collections.md — "Move-only indexed reads"
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — section 103
//   - rules/tooling/lsp.md — semantic diagnostics
func TestAnalyzePackage14UnsupportedFixturesRespectFrontendBoundary(t *testing.T) {
	directory := "../../testdata/semantic_ir/package14_unsupported"
	frontendErrors := map[string]string{
		"move_only_spread_invalid.sec":      "unsupported consuming element reads",
		"element_move_out_invalid.sec":      "explicit indexed extraction is not implemented",
		"move_only_result_read_invalid.sec": "cannot return Token element values[0] by ordinary indexing",
	}
	frontendValid := []string{
		"array_to_slice_invalid.sec",
		"dynamic_array_invalid.sec",
		"mutable_element_borrow_invalid.sec",
		"nontrivial_destruction_invalid.sec",
		"nontrivial_replacement_invalid.sec",
		"shared_element_borrow_invalid.sec",
		"slice_value_invalid.sec",
	}
	for file, expected := range frontendErrors {
		t.Run(file, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(directory, file))
			if err != nil {
				t.Fatal(err)
			}
			reported := analyze("file:///tmp/sec-lsp-p14-boundary/"+file, string(source))
			if len(reported) == 0 {
				t.Fatalf("missing frontend diagnostic containing %q", expected)
			}
			assertDiagnosticMessage(t, reported, expected)
		})
	}
	for _, file := range frontendValid {
		t.Run(file, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(directory, file))
			if err != nil {
				t.Fatal(err)
			}
			if reported := analyze("file:///tmp/sec-lsp-p14-boundary/"+file, string(source)); len(reported) != 0 {
				t.Fatalf("valid source was confused with a lowering boundary: %+v", reported)
			}
		})
	}
}

func TestAnalyzeContinuesDeclarationAnalysisAfterRecoveredBodyError(t *testing.T) {
	source := `module core

impl string {
    fn Compare(other: string) int {
        let minimum := if true {
            1
        } else {
            2
        }
        return 0
    }

    fn StartsWith(prefix: string) bool { return true }
    fn StartsWith(prefix: string) bool { return false }
}`

	reported := analyze("file:///tmp/sec/core/recovery.sec", source)
	if len(reported) != 2 {
		t.Fatalf("diagnostics = %+v, want the parser error and duplicate signature only", reported)
	}
	assertDiagnosticMessage(t, reported, `no prefix parse function for "IF"`)
	assertDiagnosticMessage(t, reported, `duplicate function "string.StartsWith" with same signature`)
}

func TestTooManyArgumentsHighlightsFirstExtraExpression(t *testing.T) {
	source := `module core

fn Only(value: int) int { return value }

fn Use(second: int) int {
    return Only(1, second + 3)
}`

	reported := analyze("file:///tmp/sec/core/range.sec", source)
	for _, diagnostic := range reported {
		if !strings.Contains(diagnostic.Message, "function Only expects 1 arguments, got 2") {
			continue
		}
		want := lspRange{
			Start: position{Line: 5, Character: 19},
			End:   position{Line: 5, Character: 29},
		}
		if diagnostic.Range != want {
			t.Fatalf("extra argument range = %+v, want %+v", diagnostic.Range, want)
		}
		return
	}
	t.Fatalf("missing too-many-arguments diagnostic in %+v", reported)
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

impl extends User {
	fn Reset() void {
	}
}
`

	symbols := documentSymbolsForSource("", source)
	assertDocumentSymbolNames(t, symbols, []string{"main", "User", "Ready", "impl User", "impl extends User"})

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

func TestSemanticTokensClassifyNoCopyAttribute(t *testing.T) {
	source := `module main

@noCopy
type SessionID int
`

	tokens := decodeSemanticTokens(semanticTokensForSource("", source))
	assertSemanticToken(t, tokens, 2, 1, 6, "modifier")
}

func TestSemanticTokensClassifyImmutableBindingsAsReadonlyVariables(t *testing.T) {
	source := `module main

let Global := 1

fn Check() int {
    let local := Global
    let mut mutable := local
    mutable += 1
    return local + mutable
}
`

	tokens := decodeSemanticTokens(semanticTokensForSource("", source))
	assertSemanticTokenWithModifier(t, tokens, 2, 4, 6, "variable", "readonly")     // Global declaration
	assertSemanticTokenWithModifier(t, tokens, 5, 8, 5, "variable", "readonly")     // local declaration
	assertSemanticTokenWithModifier(t, tokens, 5, 17, 6, "variable", "readonly")    // Global use
	assertSemanticTokenWithoutModifier(t, tokens, 6, 12, 7, "variable", "readonly") // mutable declaration
	assertSemanticTokenWithModifier(t, tokens, 6, 23, 5, "variable", "readonly")    // local use
	assertSemanticTokenWithoutModifier(t, tokens, 7, 4, 7, "variable", "readonly")  // mutable write
	assertSemanticTokenWithModifier(t, tokens, 8, 11, 5, "variable", "readonly")    // local return use
	assertSemanticTokenWithoutModifier(t, tokens, 8, 19, 7, "variable", "readonly") // mutable return use
}

func TestSemanticTokensUseFunctionParameterBindingMutability(t *testing.T) {
	source := `module main

fn Normalize(value: int, shared: ref int, exclusive: ref mut int) int {
    value = 0
    return value
}
`

	tokens := decodeSemanticTokens(semanticTokensForSource("", source))
	assertSemanticTokenWithoutModifier(t, tokens, 2, 13, len("value"), "variable", "readonly")
	assertSemanticTokenWithModifier(t, tokens, 2, 25, len("shared"), "variable", "readonly")
	assertSemanticTokenWithModifier(t, tokens, 2, 42, len("exclusive"), "variable", "readonly")
	assertSemanticTokenWithoutModifier(t, tokens, 3, 4, len("value"), "variable", "readonly")
	assertSemanticTokenWithoutModifier(t, tokens, 4, 11, len("value"), "variable", "readonly")
}

func TestFunctionHoverShowsConsumingParameter(t *testing.T) {
	source := `module main

fn Consume(-> value: int) void {}

fn Use() void {
    Consume(1)
}
`
	offset := strings.LastIndex(source, "Consume") + 1
	hover, ok := hoverForSource("", source, offsetPosition(source, offset))
	if !ok || !strings.Contains(hover.Contents.Value, "fn Consume(-> value: int) void") {
		t.Fatalf("consuming function hover = %+v, %v", hover, ok)
	}
}

func TestSemanticTokensClassifyContextualSet(t *testing.T) {
	source := `module main

interface Writable {
	property Value: int {
		set next
	}
}

type Holder struct {
	value: int,
}

impl Holder {
	property Value: int {
		get { return value }
		set next { value = next }
	}
}

fn Use() void {
	let set := 1
	discard set
}
`
	tokens := decodeSemanticTokens(semanticTokensForSource("", source))
	assertSemanticToken(t, tokens, 4, 2, len("set"), "keyword")
	assertSemanticToken(t, tokens, 4, 6, len("next"), "parameter")
	assertSemanticToken(t, tokens, 15, 2, len("set"), "keyword")
	assertSemanticTokenWithModifier(t, tokens, 20, 5, len("set"), "variable", "readonly")
	assertSemanticTokenWithModifier(t, tokens, 21, 9, len("set"), "variable", "readonly")
}

func TestLifecycleInitAndNewTooling(t *testing.T) {
	source := "type Buffer struct {\n" +
		"    value: int,\n" +
		"}\n\n" +
		"impl Buffer {\n" +
		"    init(value: int) {\n" +
		"        self.value = value\n" +
		"    }\n" +
		"}\n\n" +
		"fn Make() Buffer {\n" +
		"    return new Buffer(1)\n" +
		"}\n"

	tokens := decodeSemanticTokens(semanticTokensForSource("", source))
	assertSemanticToken(t, tokens, 5, 4, len("init"), "keyword")
	assertSemanticToken(t, tokens, 11, 11, len("new"), "keyword")

	symbols := documentSymbolsForSource("", source)
	var impl *documentSymbol
	for index := range symbols {
		if symbols[index].Name == "impl Buffer" {
			impl = &symbols[index]
			break
		}
	}
	if impl == nil {
		t.Fatal("missing impl outline symbol")
	}
	assertDocumentSymbolNames(t, impl.Children, []string{"init"})
	assertDocumentSymbolRangesContainSelections(t, symbols)

	initOffset := strings.Index(source, "init(") + 1
	initHover, ok := hoverForSource("", source, offsetPosition(source, initOffset))
	if !ok || !strings.Contains(initHover.Contents.Value, "init(value: int)") || !strings.Contains(initHover.Contents.Value, "Constructs: `Buffer`") {
		t.Fatalf("initializer hover = %+v, %v", initHover, ok)
	}
	newOffset := strings.Index(source, "new Buffer") + 1
	newHover, ok := hoverForSource("", source, offsetPosition(source, newOffset))
	if !ok || !strings.Contains(newHover.Contents.Value, "Construction error: _none_") {
		t.Fatalf("construction hover = %+v, %v", newHover, ok)
	}
	argumentOffset := strings.Index(source, "Buffer(1)") + len("Buffer(")
	help, ok := signatureHelpForSource("", source, offsetPosition(source, argumentOffset))
	if !ok || len(help.Signatures) != 1 || help.Signatures[0].Label != "init(value: int)" || help.ActiveParameter != 0 {
		t.Fatalf("initializer signature help = %+v, %v", help, ok)
	}

	memberSource := "type Buffer struct {}\nimpl Buffer {\n    ini"
	memberItems := completeSource("", memberSource, len(memberSource))
	assertCompletionLabels(t, memberItems, []string{"init"})
	for _, item := range memberItems {
		if item.Label == "new" {
			t.Fatalf("new must not be offered as an impl member: %+v", memberItems)
		}
	}
	expressionSource := "fn Make() void {\n    ne"
	expressionItems := completeSource("", expressionSource, len(expressionSource))
	assertCompletionLabels(t, expressionItems, []string{"new"})
}

func TestReferencesConnectInitializerDeclarationAndConstruction(t *testing.T) {
	dir := t.TempDir()
	declarationPath := filepath.Join(dir, "buffer.sec")
	usePath := filepath.Join(dir, "make.sec")
	declarationSource := "module sample\n\ntype Buffer struct { value: int, }\nimpl Buffer {\n    init(value: int) { self.value = value }\n}\n"
	useSource := "module sample\n\nfn Make() Buffer { return new Buffer(1) }\n"
	if err := os.WriteFile(declarationPath, []byte(declarationSource), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(usePath, []byte(useSource), 0644); err != nil {
		t.Fatal(err)
	}
	declarationOffset := strings.Index(declarationSource, "init(") + 1
	references := referencesForSource(uriFromPath(declarationPath), declarationSource, offsetPosition(declarationSource, declarationOffset), true)
	if len(references) != 2 {
		t.Fatalf("initializer references = %+v; want declaration and new construction", references)
	}
	if references[0].URI != uriFromPath(declarationPath) || references[1].URI != uriFromPath(usePath) {
		t.Fatalf("initializer reference URIs = %+v", references)
	}
}

func TestInitializerHoverSeparatesConstructionErrorFromReturnType(t *testing.T) {
	source := `module sample

type BuildError enum error { Invalid }
type Resource struct { value: int, }
impl Resource {
    init(value: int) BuildError { self.value = value }
}
fn Open(value: int) Result[Resource, BuildError] {
    return Ok(try new Resource(value))
}
`
	offset := strings.Index(source, "new Resource") + 1
	hover, ok := hoverForSource("", source, offsetPosition(source, offset))
	if !ok || !strings.Contains(hover.Contents.Value, "Construction error: `BuildError`") ||
		!strings.Contains(hover.Contents.Value, "Requires `try` or local handling: `yes`") {
		t.Fatalf("fallible construction hover = %+v, %v", hover, ok)
	}
	if strings.Contains(hover.Contents.Value, ") BuildError") {
		t.Fatalf("construction error was rendered as an initializer return type: %s", hover.Contents.Value)
	}
}

func TestSemanticTokensPreferStructFieldOverSameNamedMethod(t *testing.T) {
	source := `module main

type State struct {
    Diagnostics: int,
}

type Lexer struct {
    diagnostics: int,
}

impl Lexer {
    fn Snapshot() State {
        return State {
            Diagnostics: self.diagnostics
        }
    }

    fn Diagnostics() int {
        return self.diagnostics
    }
}
`

	tokens := decodeSemanticTokens(semanticTokensForSource("", source))
	assertSemanticToken(t, tokens, 3, 4, 11, "property")   // field declaration
	assertSemanticToken(t, tokens, 13, 12, 11, "property") // struct literal field
	assertSemanticToken(t, tokens, 17, 7, 11, "method")    // method declaration

	fieldOffset := strings.Index(source, "Diagnostics: self") + 1
	hover, ok := hoverForSource("", source, offsetPosition(source, fieldOffset))
	if !ok || !strings.Contains(hover.Contents.Value, "field Diagnostics: int") {
		t.Fatalf("struct literal field hover = %+v, %v; want field Diagnostics", hover, ok)
	}
	if strings.Contains(hover.Contents.Value, "fn Lexer.Diagnostics") {
		t.Fatalf("struct literal field hover resolved the same-named method: %s", hover.Contents.Value)
	}
}

// rules/declarations/struct.md section 4 and rules/tooling/lsp.md's Hover
// requirements expose resolved tag metadata without decoding consumer values.
func TestStructFieldHoverPreservesOpenTagMetadata(t *testing.T) {
	source := `module main

type Packet struct {
    Payload: string ` + "`wire:\"signed\\\"little\" json:\"wide value\"`" + `,
}

fn Read(packet: Packet) string {
    return packet.Payload
}
`
	use := strings.LastIndex(source, "Payload") + 1
	hover, ok := hoverForSource("", source, offsetPosition(source, use))
	if !ok {
		t.Fatal("missing struct field hover")
	}
	want := "field Payload: string `wire:\"signed\\\"little\" json:\"wide value\"`"
	if !strings.Contains(hover.Contents.Value, want) {
		t.Fatalf("struct field hover = %q, want %q", hover.Contents.Value, want)
	}
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

func TestCompletionIncludesAllCompilerKnownGlobalFunctions(t *testing.T) {
	source := "module main\n\nfn Use() void {\n\tSi\n\tfi\n}\n"
	sizeItems := completeSource("", source, strings.Index(source, "Si")+len("Si"))
	assertCompletionLabels(t, sizeItems, []string{"SizeOf"})
	fillItems := completeSource("", source, strings.Index(source, "fi")+len("fi"))
	assertCompletionLabels(t, fillItems, []string{"fill"})
}

func TestCompletionExcludesCompilerInternalFunctions(t *testing.T) {
	source := "module main\n\nfn Use() void {\n\t__String\n}\n"
	items := completeSource("", source, strings.Index(source, "__String")+len("__String"))
	for _, item := range items {
		if item.Label == "__StringSliceUnchecked" {
			t.Fatal("compiler-internal string slice operation leaked into completion")
		}
	}
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

func TestHoverShowsCallGraphReachabilityAndRecursion(t *testing.T) {
	source := `module main

fn alpha() void {
	beta()
}

fn beta() void {
	alpha()
}

fn main() void {
	alpha()
}
`
	use := strings.LastIndex(source, "alpha")
	hover, ok := hoverForSource("file:///tmp/sec-lsp-call-graph-hover/main.sec", source, offsetPosition(source, use))
	if !ok {
		t.Fatal("expected hover for alpha")
	}
	for _, expected := range []string{
		"**Call graph**",
		"Incoming: `2` call sites from `2` callables",
		"Outgoing: `1` call sites to `1` callables",
		"Reachable from: `program-entry`",
		"Same-stack recursion: `alpha`, `beta`",
	} {
		if !strings.Contains(hover.Contents.Value, expected) {
			t.Fatalf("hover does not contain %q: %s", expected, hover.Contents.Value)
		}
	}
}

func TestHoverReportsCallableOutsideActiveRootReachability(t *testing.T) {
	source := `module main

fn dead() void {}

fn main() void {
	if false {
		dead()
	}
}
`
	declaration := strings.Index(source, "dead()")
	hover, ok := hoverForSource("file:///tmp/sec-lsp-call-graph-dead/main.sec", source, offsetPosition(source, declaration))
	if !ok {
		t.Fatal("expected hover for dead")
	}
	if !strings.Contains(hover.Contents.Value, "Reachability: no active root in the current analysis") {
		t.Fatalf("wrong dead-callable hover: %s", hover.Contents.Value)
	}
}

func TestHoverShowsArenaEffectsAndSynchronousAllocationPath(t *testing.T) {
	source := `module main

fn Allocate() void {
	let arena := Arena.WithCapacity(64u)
}

fn Forward() void {
	Allocate()
}

fn Worker() void {
	let arena := Arena.Growable(64u)
}

fn Spawner() void {
	let worker := spawn Worker()
	detach worker
}
`
	uri := "file:///tmp/sec-lsp-arena-hover/main.sec"
	allocateDeclaration := strings.Index(source, "Allocate() void")
	hover, ok := hoverForSource(uri, source, offsetPosition(source, allocateDeclaration))
	if !ok || !strings.Contains(hover.Contents.Value, "Direct Arena effects: `create-owned`") {
		t.Fatalf("Allocate hover does not show direct Arena effect: %+v", hover)
	}

	forwardDeclaration := strings.Index(source, "Forward() void")
	hover, ok = hoverForSource(uri, source, offsetPosition(source, forwardDeclaration))
	if !ok || !strings.Contains(hover.Contents.Value, "May allocate: `yes`") || !strings.Contains(hover.Contents.Value, "Allocation path: `Forward` -> `Allocate`") {
		t.Fatalf("Forward hover does not show allocation cause path: %+v", hover)
	}

	spawnerDeclaration := strings.Index(source, "Spawner() void")
	hover, ok = hoverForSource(uri, source, offsetPosition(source, spawnerDeclaration))
	if !ok {
		t.Fatal("expected Spawner hover")
	}
	if strings.Contains(hover.Contents.Value, "May allocate: `yes`") {
		t.Fatalf("spawned Worker allocation leaked into Spawner hover: %s", hover.Contents.Value)
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

func TestDefinitionsResolveLocalsParametersFunctionsAndTypes(t *testing.T) {
	source := `module main

type Count int

fn Increment(value: Count) Count {
	return value + 1
}

fn Use() Count {
	let current: Count := Increment(1)
	return Increment(current)
}
`
	uri := "file:///tmp/sec-lsp-definition/main.sec"

	tests := []struct {
		name            string
		useOffset       int
		declarationText string
	}{
		{name: "parameter", useOffset: strings.Index(source, "return value") + len("return "), declarationText: "value: Count"},
		{name: "local", useOffset: strings.LastIndex(source, "current"), declarationText: "current: Count"},
		{name: "function", useOffset: strings.LastIndex(source, "Increment"), declarationText: "Increment(value"},
		{name: "type", useOffset: strings.Index(source, "fn Use() Count") + len("fn Use() "), declarationText: "Count int"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			locations := definitionsForSource(uri, source, offsetPosition(source, test.useOffset))
			if len(locations) != 1 {
				t.Fatalf("definitions = %+v, want one location", locations)
			}
			want := offsetPosition(source, strings.Index(source, test.declarationText))
			if locations[0].URI != uri || locations[0].Range.Start != want {
				t.Fatalf("definition = %+v, want %s at %+v", locations[0], uri, want)
			}
		})
	}
}

func TestDefinitionKeepsBindingsSeparateAcrossFunctions(t *testing.T) {
	source := `module main

fn First(value: int) int {
	return value
}

fn Second(value: int) int {
	return value
}
`
	uri := "file:///tmp/sec-lsp-definition-shadow/main.sec"

	firstUse := strings.Index(source, "return value") + len("return ")
	first := definitionsForSource(uri, source, offsetPosition(source, firstUse))
	if len(first) != 1 {
		t.Fatalf("first definitions = %+v, want one location", first)
	}
	wantFirst := offsetPosition(source, strings.Index(source, "value: int"))
	if first[0].Range.Start != wantFirst {
		t.Fatalf("first definition starts at %+v, want %+v", first[0].Range.Start, wantFirst)
	}

	secondUse := strings.LastIndex(source, "value")
	second := definitionsForSource(uri, source, offsetPosition(source, secondUse))
	if len(second) != 1 {
		t.Fatalf("second definitions = %+v, want one location", second)
	}
	wantSecond := offsetPosition(source, strings.LastIndex(source, "value: int"))
	if second[0].Range.Start != wantSecond {
		t.Fatalf("second definition starts at %+v, want %+v", second[0].Range.Start, wantSecond)
	}
}

func TestDefinitionSelectsResolvedOverload(t *testing.T) {
	source := `module main

fn Parse(value: int) int {
	return value
}

fn Parse(value: string) string {
	return value
}

fn Use() string {
	return Parse("value")
}
`
	uri := "file:///tmp/sec-lsp-definition-overload/main.sec"
	use := strings.LastIndex(source, "Parse")
	locations := definitionsForSource(uri, source, offsetPosition(source, use))
	if len(locations) != 1 {
		t.Fatalf("definitions = %+v, want selected overload", locations)
	}
	second := strings.Index(source[strings.Index(source, "fn Parse")+1:], "fn Parse") + strings.Index(source, "fn Parse") + 1 + len("fn ")
	want := offsetPosition(source, second)
	if locations[0].Range.Start != want {
		t.Fatalf("definition starts at %+v, want second overload at %+v", locations[0].Range.Start, want)
	}
}

func TestDefinitionsResolveSelfMembers(t *testing.T) {
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

	fn advance() int {
		self.position += 1
		return self.position
	}

	fn Read() int {
		let current := self.Current
		let changed := self.Changed
		discard changed
		return self.advance() + current
	}
}
`
	uri := "file:///tmp/sec-lsp-definition-self/main.sec"
	tests := []struct {
		use         string
		declaration string
	}{
		{use: "self.position +=", declaration: "position: int"},
		{use: "self.Current", declaration: "Current: int"},
		{use: "self.Changed", declaration: "Changed using"},
		{use: "self.advance()", declaration: "advance() int"},
	}
	for _, test := range tests {
		t.Run(test.use, func(t *testing.T) {
			use := strings.LastIndex(source, test.use) + len("self.")
			locations := definitionsForSource(uri, source, offsetPosition(source, use))
			if len(locations) != 1 {
				t.Fatalf("definitions = %+v, want one location", locations)
			}
			want := offsetPosition(source, strings.Index(source, test.declaration))
			if locations[0].Range.Start != want {
				t.Fatalf("definition starts at %+v, want %+v", locations[0].Range.Start, want)
			}
		})
	}
}

func TestDefinitionResolvesImportedProjectFunction(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".sec", "sec.toml"), []byte("[project]\nname = \"definition-test\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	helperPath := filepath.Join(dir, "helpers.sec")
	helperSource := `module helpers

fn Build() int {
	return 1
}
`
	if err := os.WriteFile(helperPath, []byte(helperSource), 0644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.sec")
	mainSource := `module main

import "helpers"

fn Use() int {
	return helpers.Build()
}
`
	uri := uriFromPath(mainPath)
	use := strings.Index(mainSource, "Build")
	locations := definitionsForSource(uri, mainSource, offsetPosition(mainSource, use))
	if len(locations) != 1 {
		t.Fatalf("definitions = %+v, want imported function", locations)
	}
	want := offsetPosition(helperSource, strings.Index(helperSource, "Build"))
	if locations[0].URI != uriFromPath(helperPath) || locations[0].Range.Start != want {
		t.Fatalf("definition = %+v, want %s at %+v", locations[0], uriFromPath(helperPath), want)
	}
	if got := locations[0].Range.End.Character - locations[0].Range.Start.Character; got != len("Build") {
		t.Fatalf("qualified imported definition range length = %d, want %d", got, len("Build"))
	}
}

func TestDefinitionResolvesCoreAndStdlibDeclarations(t *testing.T) {
	source := `module main

import "unicode"

fn Check(text: string, ch: rune) bool {
	return text.IsEmpty || unicode.IsLetter(ch)
}
`
	uri := "file:///tmp/sec-lsp-definition-library/main.sec"
	tests := []struct {
		name       string
		use        string
		fileSuffix string
	}{
		{name: "core method", use: "IsEmpty", fileSuffix: filepath.Join("sec", "core", "string.sec")},
		{name: "stdlib function", use: "IsLetter", fileSuffix: filepath.Join("sec", "stdlib", "unicode", "unicode.sec")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			use := strings.Index(source, test.use)
			locations := definitionsForSource(uri, source, offsetPosition(source, use))
			if len(locations) != 1 {
				t.Fatalf("definitions = %+v, want one library declaration", locations)
			}
			path := pathFromURI(locations[0].URI)
			if !strings.HasSuffix(path, test.fileSuffix) {
				t.Fatalf("definition URI = %q, want suffix %q", locations[0].URI, test.fileSuffix)
			}
		})
	}
}

func TestDefinitionSurvivesIncompleteSource(t *testing.T) {
	source := "module main\n\nfn Use() void {\n\tself.\n"
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("definitionsForSource panicked: %v", recovered)
		}
	}()
	if locations := definitionsForSource("file:///tmp/sec-lsp-definition-incomplete/main.sec", source, position{Line: 3, Character: 6}); len(locations) != 0 {
		t.Fatalf("incomplete source definitions = %+v, want none", locations)
	}
}

func TestDocumentHighlightsClassifyLocalReadsAndWrites(t *testing.T) {
	source := `module main

fn Use(input: int) int {
	let mut value := input
	value += 1
	return value
}
`
	uri := "file:///tmp/sec-lsp-highlight/main.sec"
	use := strings.LastIndex(source, "value")
	highlights := documentHighlightsForSource(uri, source, offsetPosition(source, use))
	if len(highlights) != 3 {
		t.Fatalf("highlights = %+v, want declaration, write, and read", highlights)
	}
	wantKinds := []int{documentHighlightWrite, documentHighlightWrite, documentHighlightRead}
	for index, want := range wantKinds {
		if highlights[index].Kind != want {
			t.Fatalf("highlight %d kind = %d, want %d", index, highlights[index].Kind, want)
		}
	}
}

func TestDocumentHighlightsKeepShadowedBindingsSeparate(t *testing.T) {
	source := `module main

fn First(value: int) int {
	return value
}

fn Second(value: int) int {
	return value
}
`
	uri := "file:///tmp/sec-lsp-highlight-shadow/main.sec"
	use := strings.LastIndex(source, "value")
	highlights := documentHighlightsForSource(uri, source, offsetPosition(source, use))
	if len(highlights) != 2 {
		t.Fatalf("highlights = %+v, want only the second parameter and use", highlights)
	}
	wantDeclaration := offsetPosition(source, strings.LastIndex(source, "value: int"))
	if highlights[0].Range.Start != wantDeclaration {
		t.Fatalf("first highlight starts at %+v, want %+v", highlights[0].Range.Start, wantDeclaration)
	}
}

func TestDocumentHighlightsResolveSelfMemberReadsAndWrites(t *testing.T) {
	source := `module main

type Counter struct {
	value: int,
}

impl Counter {
	fn Advance() int {
		self.value += 1
		return self.value
	}
}
`
	uri := "file:///tmp/sec-lsp-highlight-member/main.sec"
	use := strings.LastIndex(source, "value")
	highlights := documentHighlightsForSource(uri, source, offsetPosition(source, use))
	if len(highlights) != 3 {
		t.Fatalf("highlights = %+v, want field declaration, write, and read", highlights)
	}
	wantKinds := []int{documentHighlightWrite, documentHighlightWrite, documentHighlightRead}
	for index, want := range wantKinds {
		if highlights[index].Kind != want {
			t.Fatalf("highlight %d kind = %d, want %d", index, highlights[index].Kind, want)
		}
	}
}

func TestDocumentHighlightsSurviveIncompleteSource(t *testing.T) {
	source := "module main\n\nfn Use() void {\n\tself.\n"
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("documentHighlightsForSource panicked: %v", recovered)
		}
	}()
	highlights := documentHighlightsForSource("file:///tmp/sec-lsp-highlight-incomplete/main.sec", source, position{Line: 3, Character: 6})
	if len(highlights) != 0 {
		t.Fatalf("incomplete source highlights = %+v, want none", highlights)
	}
}

func TestCallHierarchyReportsIncomingAndOutgoingDirectCalls(t *testing.T) {
	source := `module main

fn leaf() int {
	return 1
}

fn middle() int {
	return leaf() + leaf()
}

fn top() int {
	return middle()
}
`
	uri := "file:///tmp/sec-lsp-call-hierarchy/main.sec"
	middleDeclaration := strings.Index(source, "middle() int")
	items := callHierarchyItemsForSource(uri, source, offsetPosition(source, middleDeclaration))
	if len(items) != 1 || items[0].Name != "middle" {
		t.Fatalf("prepare items = %+v, want middle", items)
	}

	incoming := callHierarchyIncomingCallsForSource(uri, source, items[0].Data.NodeID)
	if len(incoming) != 1 || incoming[0].From.Name != "top" || len(incoming[0].FromRanges) != 1 {
		t.Fatalf("incoming calls = %+v, want one call from top", incoming)
	}

	outgoing := callHierarchyOutgoingCallsForSource(uri, source, items[0].Data.NodeID)
	if len(outgoing) != 1 || outgoing[0].To.Name != "leaf" || len(outgoing[0].FromRanges) != 2 {
		t.Fatalf("outgoing calls = %+v, want two call sites grouped under leaf", outgoing)
	}
}

func TestCallHierarchyResolvesStaticMethods(t *testing.T) {
	source := `module main

type Reader struct {
	value: int,
}

impl Reader {
	fn read() int {
		return self.value
	}
}

fn use(reader: Reader) int {
	return reader.read()
}
`
	uri := "file:///tmp/sec-lsp-call-hierarchy-method/main.sec"
	useDeclaration := strings.Index(source, "use(reader")
	items := callHierarchyItemsForSource(uri, source, offsetPosition(source, useDeclaration))
	if len(items) != 1 {
		t.Fatalf("prepare items = %+v, want use", items)
	}
	outgoing := callHierarchyOutgoingCallsForSource(uri, source, items[0].Data.NodeID)
	if len(outgoing) != 1 || outgoing[0].To.Name != "read" || outgoing[0].To.Kind != 6 {
		t.Fatalf("method outgoing calls = %+v, want Reader.read method", outgoing)
	}
}

func TestCallHierarchyAndHoverPreserveSpawnExecutionRelations(t *testing.T) {
	source := `module main

fn TaskWork() void {}

fn ThreadWork() void {}

fn main() void {
	let taskHandle := spawn task TaskWork()
	let threadHandle := spawn thread ThreadWork()
	detach taskHandle
	detach threadHandle
}
`
	uri := "file:///tmp/sec-lsp-call-hierarchy-spawn/main.sec"
	mainDeclaration := strings.Index(source, "main() void")
	items := callHierarchyItemsForSource(uri, source, offsetPosition(source, mainDeclaration))
	if len(items) != 1 {
		t.Fatalf("prepare items = %+v, want main", items)
	}
	outgoing := callHierarchyOutgoingCallsForSource(uri, source, items[0].Data.NodeID)
	if len(outgoing) != 2 {
		t.Fatalf("spawn outgoing = %+v, want task and thread", outgoing)
	}
	relations := map[string]sema.CallExecutionRelation{}
	for _, call := range outgoing {
		relations[call.To.Name] = call.To.Data.Execution
		if !strings.Contains(call.To.Detail, strings.ReplaceAll(string(call.To.Data.Execution), "-", " ")) {
			t.Fatalf("call hierarchy detail does not preserve execution: %+v", call.To)
		}
	}
	if relations["TaskWork"] != sema.CallExecutionSpawnTask || relations["ThreadWork"] != sema.CallExecutionSpawnThread {
		t.Fatalf("spawn relations = %+v", relations)
	}

	hover, ok := hoverForSource(uri, source, offsetPosition(source, mainDeclaration))
	if !ok || !strings.Contains(hover.Contents.Value, "spawn task: `1`, spawn thread: `1`") {
		t.Fatalf("main hover does not expose spawn edges: %+v", hover)
	}
	taskDeclaration := strings.Index(source, "TaskWork()")
	hover, ok = hoverForSource(uri, source, offsetPosition(source, taskDeclaration))
	if !ok || !strings.Contains(hover.Contents.Value, "Reachable from: `program-entry`, `task-entry`") {
		t.Fatalf("task hover does not expose derived root: %+v", hover)
	}
}

func TestCallHierarchySurvivesIncompleteSource(t *testing.T) {
	source := "module main\n\nfn use() void {\n\tmissing(\n"
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("callHierarchyItemsForSource panicked: %v", recovered)
		}
	}()
	items := callHierarchyItemsForSource("file:///tmp/sec-lsp-call-hierarchy-incomplete/main.sec", source, position{Line: 3, Character: 2})
	if len(items) != 0 {
		t.Fatalf("incomplete source items = %+v, want none", items)
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
	created: datetime,
	elapsed: duration,
	wall: time,
}
`

	items := completeSource("", source, strings.Index(source, "RawPtr")+len("Ra"))
	assertCompletionLabels(t, items, []string{"RawPtr"})

	items = completeSource("", source, strings.Index(source, "list")+len("li"))
	assertCompletionLabels(t, items, []string{"list"})

	items = completeSource("", source, strings.Index(source, "datetime")+len("date"))
	assertCompletionLabels(t, items, []string{"date", "datetime"})

	items = completeSource("", source, strings.Index(source, "duration")+len("dur"))
	assertCompletionLabels(t, items, []string{"duration"})

	items = completeSource("", source, strings.LastIndex(source, "time")+len("ti"))
	assertCompletionLabels(t, items, []string{"time"})
}

func TestCompletionIncludesRawPtrMembers(t *testing.T) {
	source := `module main

fn Use(ptr: RawPtr[byte]) void {
	ptr.
}
`

	items := completeSource("", source, strings.Index(source, "ptr.")+len("ptr."))
	assertCompletionLabels(t, items, []string{"Offset", "AddBytes", "Difference", "VolatileRead", "VolatileWrite"})
	wantDetail := map[string]string{"VolatileRead": "unsafe byte (effectful)", "VolatileWrite": "unsafe void (effectful)"}
	for _, item := range items {
		if detail, wanted := wantDetail[item.Label]; wanted && item.Detail != detail {
			t.Fatalf("volatile completion %s detail = %q", item.Label, item.Detail)
		}
	}
}

func TestRawPtrVolatileHoverUsesCompilerOwnedEffects(t *testing.T) {
	source := `module main

fn Use(ptr: RawPtr[byte]) byte {
	unsafe {
		return ptr.VolatileRead()
	}
}
`
	hoverOffset := strings.Index(source, "VolatileRead") + 2
	hover, ok := hoverForSource("", source, offsetPosition(source, hoverOffset))
	if !ok || !strings.Contains(hover.Contents.Value, "unsafe method VolatileRead: byte") ||
		!strings.Contains(hover.Contents.Value, "CKM-RAWPTR-VOLATILE-READ") ||
		!strings.Contains(hover.Contents.Value, "Effects: `volatile-read`") {
		t.Fatalf("volatile compiler-known hover = %+v, %v", hover, ok)
	}
}

func TestCompletionIncludesCompilerKnownMembers(t *testing.T) {
	source := "module main\n\nfn Use(text: string) void {\n\ttext.\n}\n"
	textItems := completeSource("", source, strings.Index(source, "text.")+len("text."))
	assertCompletionLabels(t, textItems, []string{"Len", "Ptr", "SizeOf", "ToByteArray", "ToCharArray", "ToRuneArray", "ToString"})

	source = "module main\n\nfn Use(values: rune[2]) void {\n\tvalues.\n}\n"
	valueItems := completeSource("", source, strings.Index(source, "values.")+len("values."))
	assertCompletionLabels(t, valueItems, []string{"IsEmpty", "Len", "Ptr", "SizeOf", "ToString"})

	source = "module main\n\nfn Use(values: int[]) void {\n\tvalues.\n}\n"
	dynamicItems := completeSource("", source, strings.Index(source, "values.")+len("values."))
	assertCompletionLabels(t, dynamicItems, []string{"Append", "Clear", "IsEmpty", "Len", "Ptr", "RemoveAt", "SizeOf", "ToString"})

	source = "module main\n\nfn Use(view: ref mut int[]) void {\n\tview.\n}\n"
	sliceItems := completeSource("", source, strings.Index(source, "view.")+len("view."))
	assertCompletionLabels(t, sliceItems, []string{"Fill", "IsEmpty", "Len", "Ptr", "Reverse", "SizeOf", "ToString"})

	source = "module main\n\nfn Use(values: list[int]) void {\n\tvalues.\n}\n"
	listItems := completeSource("", source, strings.Index(source, "values.")+len("values."))
	assertCompletionLabels(t, listItems, []string{"Append", "Capacity", "Clear", "Contains", "IndexOf", "Insert", "IsEmpty", "Len", "Ptr", "Remove", "RemoveAt", "Reverse", "SizeOf", "Sort", "SortBy", "ToString"})
	source = "module main\n\nfn Use(entries: map[int, string]) void {\n\tentries.\n}\n"
	mapItems := completeSource("", source, strings.Index(source, "entries.")+len("entries."))
	assertCompletionLabels(t, mapItems, []string{"Clear", "ContainsKey", "IsEmpty", "Len", "Remove", "ToString"})
	source = "module main\n\nfn Use(unique: set[int]) void {\n\tunique.\n}\n"
	setItems := completeSource("", source, strings.Index(source, "unique.")+len("unique."))
	assertCompletionLabels(t, setItems, []string{"Add", "Clear", "Contains", "Difference", "Intersection", "IsEmpty", "Len", "Remove", "SymmetricDifference", "ToString", "Union"})

	source = "module main\n\nfn Use() void {\n\tlet mut arena: Arena := Arena {}\n\tarena.\n}\n"
	arenaItems := completeSource("", source, strings.Index(source, "arena.")+len("arena."))
	assertCompletionLabels(t, arenaItems, []string{"Alloc", "New", "Ptr", "Release", "Reset", "SizeOf"})
}

func TestCompilerKnownMembersUseSemaFactsForTokensAndHover(t *testing.T) {
	source := `module main

fn Use(values: int[]) uint {
	values.Clear()
	return values.Len
}
`
	tokens := decodeSemanticTokens(semanticTokensForSource("", source))
	clearStart := strings.Index(sourceLine(source, 3), "Clear")
	lenStart := strings.Index(sourceLine(source, 4), "Len")
	assertSemanticToken(t, tokens, 3, clearStart, len("Clear"), "method")
	assertSemanticToken(t, tokens, 4, lenStart, len("Len"), "property")

	hoverOffset := strings.LastIndex(source, "Len") + 1
	hover, ok := hoverForSource("", source, offsetPosition(source, hoverOffset))
	if !ok || !strings.Contains(hover.Contents.Value, "property Len: uint") || !strings.Contains(hover.Contents.Value, "CKM-LEN-ARRAY") {
		t.Fatalf("compiler-known hover = %+v, %v", hover, ok)
	}
}

func TestCompletionIncludesStaticCompilerKnownMembers(t *testing.T) {
	source := "module main\n\nfn Use() void {\n\tint32.\n}\n"
	integerItems := completeSource("", source, strings.Index(source, "int32.")+len("int32."))
	assertCompletionLabels(t, integerItems, []string{"Bits", "Max", "Min", "SizeOf"})

	source = "module main\n\nfn Use() void {\n\tstring.\n}\n"
	stringItems := completeSource("", source, strings.Index(source, "string.")+len("string."))
	assertCompletionLabels(t, stringItems, []string{"FromByteArray", "FromRuneArray", "SizeOf"})

	source = "module main\n\nfn Use() void {\n\tArena.\n}\n"
	arenaItems := completeSource("", source, strings.Index(source, "Arena.")+len("Arena."))
	assertCompletionLabels(t, arenaItems, []string{"FromBuffer", "Growable", "SizeOf", "WithCapacity"})
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
	Modifiers []string
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
		modifiers := []string{}
		for index, modifier := range semanticTokenModifiers {
			if tokens.Data[i+4]&(1<<index) != 0 {
				modifiers = append(modifiers, modifier)
			}
		}
		decoded = append(decoded, decodedSemanticToken{
			Line:      line,
			Start:     start,
			Length:    tokens.Data[i+2],
			TokenType: tokenType,
			Modifiers: modifiers,
		})
	}
	return decoded
}

func assertSemanticTokenWithModifier(t *testing.T, tokens []decodedSemanticToken, line int, start int, length int, tokenType string, modifier string) {
	t.Helper()
	for _, token := range tokens {
		if token.Line != line || token.Start != start || token.Length != length {
			continue
		}
		if token.TokenType != tokenType || !containsString(token.Modifiers, modifier) {
			t.Fatalf("semantic token at %d:%d = %q %+v, want %q with %q; tokens=%+v", line, start, token.TokenType, token.Modifiers, tokenType, modifier, tokens)
		}
		return
	}
	t.Fatalf("missing semantic token at %d:%d length %d type %q with modifier %q; tokens=%+v", line, start, length, tokenType, modifier, tokens)
}

func assertSemanticTokenWithoutModifier(t *testing.T, tokens []decodedSemanticToken, line int, start int, length int, tokenType string, modifier string) {
	t.Helper()
	for _, token := range tokens {
		if token.Line != line || token.Start != start || token.Length != length {
			continue
		}
		if token.TokenType != tokenType || containsString(token.Modifiers, modifier) {
			t.Fatalf("semantic token at %d:%d = %q %+v, want %q without %q; tokens=%+v", line, start, token.TokenType, token.Modifiers, tokenType, modifier, tokens)
		}
		return
	}
	t.Fatalf("missing semantic token at %d:%d length %d type %q without modifier %q; tokens=%+v", line, start, length, tokenType, modifier, tokens)
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
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

func TestFormatSourceAlignsDeclarationTrailingComments(t *testing.T) {
	input := `enum Status {
New, // new
InProgress,   // active
Done,        // done
}
`
	want := `enum Status {
    New,           // new
    InProgress,    // active
    Done,          // done
}
`
	if got := formatSource(input); got != want {
		t.Fatalf("LSP formatter did not use shared comment alignment:\n%s\nwant:\n%s", got, want)
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

// rules/declarations/static.md, sections 8 and 11.
func TestCompletionSeparatesStaticAndInstanceMembers(t *testing.T) {
	declarations := `module main

type Counter struct { value: int }

impl Counter {
	let Maximum: int := 100
    static property Current: int { get { return Counter.Maximum } }
    property Value: int { get { return self.value } }
    static fn Make() Counter { return Counter { value: 0 } }
    fn Read() int { return self.value }
}
`
	source := declarations + "\nfn Use() void {\n    Counter.\n}\n"
	staticOffset := strings.Index(source, "Counter.\n") + len("Counter.")
	assertCompletionLabels(t, completeSource("", source, staticOffset), []string{"Current", "Make", "Maximum", "SizeOf"})
	source = declarations + "\nfn Use(counter: Counter) void {\n    counter.\n}\n"
	instanceOffset := strings.Index(source, "counter.\n") + len("counter.")
	assertCompletionLabels(t, completeSource("", source, instanceOffset), []string{"Read", "SizeOf", "Value", "value"})
}

// rules/declarations/static.md section 6: compatibility static let produces
// one information diagnostic and canonical associated let shares completion.
func TestAssociatedLetCompletionAndRedundantStaticInformation(t *testing.T) {
	source := `module main

type Program string

impl Program {
	let OneCare := "Zebra OneCare"
	static let VIQ := "Z1C+VIQ"
}

fn Use() void {
	Program.
}
`
	offset := strings.Index(source, "Program.\n") + len("Program.")
	assertCompletionLabels(t, completeSource("", source, offset), []string{"OneCare", "SizeOf", "VIQ"})

	items := analyze("", source)
	found := false
	for _, item := range items {
		if item.Code == diagnostics.RedundantAssociatedStatic {
			found = true
			if item.Severity != 3 || !strings.Contains(item.Message, "static is redundant on immutable associated declaration VIQ") {
				t.Fatalf("wrong redundant-static LSP diagnostic: %+v", item)
			}
		}
	}
	if !found {
		t.Fatalf("missing redundant-static LSP diagnostic: %+v", items)
	}
}

func TestCompletionReturnsStringBackedEnumMembers(t *testing.T) {
	source := `module main

enum Program string {
	OneCare = "Zebra OneCare",
	VIQ = "Z1C+VIQ",
}

fn Use() void {
	Program.
}
`
	offset := strings.Index(source, "Program.\n") + len("Program.")
	items := completeSource("", source, offset)
	assertCompletionLabels(t, items, []string{"OneCare", "VIQ"})
	for _, item := range items {
		if item.Label == "OneCare" && (item.Kind != 20 || item.Detail != "Program") {
			t.Fatalf("string enum completion = %+v, want enum member of Program", item)
		}
	}
}

func TestCompletionReturnsEnumMembersAndUnionVariantsAfterTypeDot(t *testing.T) {
	enumSource := `module main

enum Method {
    GET,
    HEAD,
    POST,
    PUT,
    DELETE,
    CONNECT,
    OPTIONS,
    TRACE,
    PATCH,
}

fn FromString(In: string) Method {
    let mut mt: Method
    mt = Method.
    return mt
}
`
	enumOffset := strings.Index(enumSource, "Method.\n") + len("Method.")
	enumItems := completeSource("", enumSource, enumOffset)
	assertCompletionLabels(t, enumItems, []string{"CONNECT", "DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT", "TRACE"})
	for _, item := range enumItems {
		if item.Label == "PATCH" && (item.Kind != 20 || item.Detail != "Method") {
			t.Fatalf("PATCH completion = %+v, want enum member of Method", item)
		}
	}

	unionSource := `module main

type Response union {
    Success(string),
    Empty,
    Detailed {
        code: int,
    },
}

fn Build() void {
    Response.
}
`
	unionOffset := strings.Index(unionSource, "Response.\n") + len("Response.")
	unionItems := completeSource("", unionSource, unionOffset)
	assertCompletionLabels(t, unionItems, []string{"Detailed", "Empty", "Success"})
}

func TestLSPAnalysisDepthFromProjectManifest(t *testing.T) {
	dir := t.TempDir()
	manifestDir := filepath.Join(dir, ".sec")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(dir, "main.sec")
	if err := os.WriteFile(sourcePath, []byte("module main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		config string
		want   sema.AnalysisDepth
	}{
		{"[project]\nname = \"test\"\n", sema.AnalysisInteractive},
		{"[analysis]\nlsp_depth = \"interactive\"\n", sema.AnalysisInteractive},
		{"[analysis]\nlsp_depth = \"standard\"\n", sema.AnalysisStandard},
		{"[analysis]\nlsp_depth = \"deep\"\n", sema.AnalysisDeep},
		{"[analysis]\nlsp_depth = \"unsupported\"\n", sema.AnalysisInteractive},
	} {
		if err := os.WriteFile(filepath.Join(manifestDir, "sec.toml"), []byte(test.config), 0644); err != nil {
			t.Fatal(err)
		}
		if got := lspAnalysisDepth(sourcePath); got != test.want {
			t.Fatalf("lspAnalysisDepth with %q = %q, want %q", test.config, got, test.want)
		}
	}
}
