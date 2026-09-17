package formatter

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
)

func TestFormatContextualMatrixXWithoutRewritingIdentifiers(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/contextual_x.sec")
	if err != nil {
		t.Fatal(err)
	}
	got := Format(Source{Text: string(input)}, Options{}).Text
	for _, want := range []string{
		"let product := left x right",
		"let grouped := left x (right)",
		"fn Multiply(x: int,",
		"discard x\n",
		"discard x(1)",
		"discard holder.x",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("contextual x formatting missing %q:\n%s", want, got)
		}
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("contextual x formatting is not idempotent:\nfirst:\n%s\nsecond:\n%s", got, again)
	}
}

// rules/foundations/grammar.md "Collection and shaped types" and
// rules/tooling/formatter.md "Contextual set" require type-context formatting
// without rewriting the same spelling when it is an ordinary identifier.
func TestFormatContextualCollectionAndShapedTypeNames(t *testing.T) {
	input := "fn Use() void {\nlet values: list[ int, 8 ]\nlet lookup: map[ string, int ]\nlet flags: set[ string ]\nlet position: vector[ float64, 3 ]\nlet transform: matrix[ float32, 4, 4 ]\nlet image: tensor[ float32, 3, 224, 224 ]\nlet view: tensor_view[ float32, 3 ]\nlet set := 1\ndiscard set\n}\n"
	want := "fn Use() void {\n    let values: list[int, 8]\n    let lookup: map[string, int]\n    let flags: set[string]\n    let position: vector[float64, 3]\n    let transform: matrix[float32, 4, 4]\n    let image: tensor[float32, 3, 224, 224]\n    let view: tensor_view[float32, 3]\n    let set := 1\n    discard set\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong contextual type-name formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("contextual type-name formatting is not idempotent:\n%s", again)
	}
}

// rules/foundations/lexical_structure.md §§22–23 require the formatter to
// preserve contextual spellings and metadata while normalizing surrounding
// whitespace.
func TestFormatPreservesLexicalContextSpellings(t *testing.T) {
	input := "/** Packet docs */\ntype Count int multipleOf 2\ntype Wire struct {\nvalue: int `wire:\"value\"`,\n}\nfn Work() void {}\nfn Use() void {\nlet job := spawn thread Work()\nmatch job {\n_=>{}\n}\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	for _, spelling := range []string{
		"/** Packet docs */",
		"multipleOf 2",
		"`wire:\"value\"`",
		"spawn thread Work()",
		"_ => {}",
	} {
		if !strings.Contains(got, spelling) {
			t.Fatalf("formatter lost contextual spelling %q:\n%s", spelling, got)
		}
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("contextual lexical formatting is not idempotent:\nfirst:\n%s\nsecond:\n%s", got, again)
	}
}

func TestFormatReturnsAttachmentsForCanonicalOutput(t *testing.T) {
	result := Format(Source{Text: "fn Use() void {\n// leading\nlet value := 1 // trailing\n\n// detached\n\ndiscard value\n}\n"}, Options{})
	want := []ast.CommentPlacement{ast.CommentLeading, ast.CommentTrailing, ast.CommentDetached}
	if len(result.Comments) != len(want) {
		t.Fatalf("formatter comments = %#v\n%s", result.Comments, result.Text)
	}
	for index, placement := range want {
		if result.Comments[index].Placement != placement {
			t.Errorf("formatter attachment %d = %q, want %q", index, result.Comments[index].Placement, placement)
		}
	}
}

// rules/tooling/formatter.md "Increment" and "Decrement" require canonical
// compound-assignment output only for parser-confirmed statement aliases.
func TestFormatIncrementAndDecrementAliases(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/increment_decrement.sec")
	if err != nil {
		t.Fatal(err)
	}
	want := "module increment_decrement\n\nfn Update() int {\n    let mut value: int := 2\n    value += 1\n    value -= 1\n    return value\n}\n"
	got := Format(Source{Text: string(input)}, Options{}).Text
	if got != want {
		t.Fatalf("wrong alias formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("postfix alias formatting is not idempotent:\n%s", again)
	}
}

func TestFormatterPreservesInvalidIncrementExpression(t *testing.T) {
	input, err := os.ReadFile("../../testdata/parser/increment_decrement_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	got := Format(Source{Text: string(input)}, Options{}).Text
	if !strings.Contains(got, "let old := value++") || !strings.Contains(got, "value++ + 1") {
		t.Fatalf("formatter rewrote invalid expression aliases:\n%s", got)
	}
}

func TestFormatPreservesDefaultClauseAndPartialStructLiteral(t *testing.T) {
	input := "module main\n\n" +
		"type User string in [\"Admin\", \"User\"] default \"User\"\n\n" +
		"fn main() int {\n" +
		"let mut user: User\n" +
		"let position := Position { line: 10 }\n" +
		"return 0\n" +
		"}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if !strings.Contains(got, `type User string in ["Admin", "User"] default "User"`) {
		t.Fatalf("formatter changed explicit default or membership order:\n%s", got)
	}
	if !strings.Contains(got, "Position { line: 10 }") {
		t.Fatalf("formatter expanded partial struct literal:\n%s", got)
	}
}

func TestFormatPlacesNoCopyAttributeOnOwnLine(t *testing.T) {
	input := "@noCopy type SessionID struct {\nvalue: uint64,\n}\n"
	want := "@noCopy\ntype SessionID struct {\n    value: uint64,\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong @noCopy formatting:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatRemovesInitialByteOrderMark(t *testing.T) {
	got := Format(Source{Text: "\uFEFFmodule main\n"}, Options{}).Text
	if got != "module main\n" {
		t.Fatalf("formatter retained initial BOM: %q", got)
	}
}

func TestFormatImplExtension(t *testing.T) {
	input := "impl extends Vehicle {\nfn Stop() void {\n}\n}\n"
	want := "impl extends Vehicle {\n    fn Stop() void {\n    }\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong impl extension formatting:\n%s\nwant:\n%s", got, want)
	}
}

// rules/declarations/static.md, sections 3, 6, and 25.
func TestFormatRemovesOnlyRedundantModuleStatic(t *testing.T) {
	input := "static let Global: int := 1\n\nimpl Counter {\nstatic let Value: int := 2\nstatic let mut Total: int := 0\nstatic property Current: int {\nget { return Counter.Value }\n}\n}\n\nfn Use() void {\nstatic let Calls: int := 0\n}\n"
	want := "let Global: int := 1\n\nimpl Counter {\n    static let Value: int := 2\n    static let mut Total: int := 0\n    static property Current: int {\n        get { return Counter.Value }\n    }\n}\n\nfn Use() void {\n    static let Calls: int := 0\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("wrong static formatting:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatInitAndNewLifecycleSyntax(t *testing.T) {
	input := "impl Buffer {\ninit( size: uint, alignment: uint, ) AllocationError {\n}\n}\n\nfn Make() Result[Buffer, AllocationError] {\nreturn try new Buffer(4096, 16)\n}\n"
	want := "impl Buffer {\n    init(size: uint, alignment: uint) AllocationError {\n    }\n}\n\nfn Make() Result[Buffer, AllocationError] {\n    return try new Buffer(4096, 16)\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("wrong lifecycle formatting:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatConsumingFunctionParameter(t *testing.T) {
	input := "fn Test( -> a: int, b: string, ) void {\n}\n"
	want := "fn Test(-> a: int, b: string) void {\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("wrong consuming parameter formatting:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatNormalizesSingleLineCallSpacing(t *testing.T) {
	input := `fn NextToken() Token {
return self.token(                lookupIdent(literal),                literal,                line,                column,             )
}
`
	want := `fn NextToken() Token {
    return self.token(lookupIdent(literal), literal, line, column)
}
`
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("single-line call spacing was not normalized:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("single-line call formatting is not idempotent:\nfirst:\n%s\nsecond:\n%s", got, again)
	}
}

func TestFormatPreservesMultilineCallLayout(t *testing.T) {
	input := "fn Test() void {\nconsume(\nfirst,\nsecond,\n)\n}\n"
	want := "fn Test() void {\n    consume(\n        first,\n        second,\n    )\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("formatter changed multiline call layout:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatNormalizesSameLineDelimiterSpacing(t *testing.T) {
	input := `fn Render() void {
match self.Domain {
Some(domain) => {                 out += "; Domain=" + domain            }
None => {             }
}
let values := [         first,          Build("(", [ second, third ]),      ]
let item := Build(      Item {       Left: first, Right: second       },      third,    )
let grouped := (             first + second              )
}
`
	want := `fn Render() void {
    match self.Domain {
        Some(domain) => { out += "; Domain=" + domain }
        None => {}
    }
    let values := [first, Build("(", [second, third])]
    let item := Build(Item { Left: first, Right: second }, third)
    let grouped := (first + second)
}
`

	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("single-line delimiter spacing was not normalized:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("single-line delimiter formatting is not idempotent:\nfirst:\n%s\nsecond:\n%s", got, again)
	}
}

func TestFormatPreservesMultilineDelimiterPadding(t *testing.T) {
	input := "fn Test() void {\nconsume(\n    first,\n)\nlet values := [\n    first,\n]\nif ready {\n    Run()\n}\n}\n"
	want := "fn Test() void {\n    consume(\n        first,\n    )\n    let values := [\n        first,\n    ]\n    if ready {\n        Run()\n    }\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("formatter changed multiline delimiter layout:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatPreservesRegisterLayoutModifiers(t *testing.T) {
	input := "type Header register[16] msb-first big-endian {\nVersion: bit[4],\nPayload: bit[12],\n}\n"
	want := "type Header register[16] msb-first big-endian {\n    Version: bit[4],\n    Payload: bit[12],\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong register modifier formatting:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatPreservesRegisterFieldAccessModifiers(t *testing.T) {
	input := "type Device register[3] {\nReady: bit read-only,\nCommand: bit write-only,\nPending: bit write-one-clear,\n}\n"
	want := "type Device register[3] {\n    Ready: bit read-only,\n    Command: bit write-only,\n    Pending: bit write-one-clear,\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("wrong register field modifier formatting:\n%s\nwant:\n%s", got, want)
	}
}

// rules/tooling/formatter.md, General trailing-comment alignment. Nominal
// declaration items share one local comment column with a four-space gutter.
func TestFormatAlignsTrailingCommentsInNominalDeclarations(t *testing.T) {
	input := `enum test {
  a,   // kommentar 1
  longer, // kommentar 2
  c,        // kommentar 3
}

type Packet struct {
short: int, // field
longerName: string,      // text
plain: string,
}

type Device register[4] {
Ready: bit, // ready
Mode: bit[3],          // mode
}

type State union {
Idle // idle
Running // running
}
`
	want := `enum test {
    a,         // kommentar 1
    longer,    // kommentar 2
    c,         // kommentar 3
}

type Packet struct {
    short: int,            // field
    longerName: string,    // text
    plain: string,
}

type Device register[4] {
    Ready: bit,      // ready
    Mode: bit[3],    // mode
}

type State union {
    Idle       // idle
    Running    // running
}
`
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("trailing comments were not aligned:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("trailing-comment alignment is not idempotent:\nfirst:\n%s\nsecond:\n%s", got, again)
	}
}

func TestFormatKeepsTrailingCommentAlignmentGroupsLocal(t *testing.T) {
	input := `type Config struct {
ID: int, // identity
Name: string, // display

// Network settings.
Endpoint: string, // endpoint
VeryLongTimeoutName: int, // timeout
URL: string = "https://example.test/a//b", // URL
}
`
	want := `type Config struct {
    ID: int,         // identity
    Name: string,    // display

    // Network settings.
    Endpoint: string,                             // endpoint
    VeryLongTimeoutName: int,                     // timeout
    URL: string = "https://example.test/a//b",    // URL
}
`
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("local trailing-comment groups were formatted incorrectly:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatPreservesErrorEnumMarker(t *testing.T) {
	input := "enum ProtocolError uint16 error {\nInvalid = 1,\n}\n"
	want := "enum ProtocolError uint16 error {\n    Invalid = 1,\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("wrong error enum formatting:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatPreservesPayloadUnionErrorMarker(t *testing.T) {
	input := "type DetailedError union error {\nOpen {\nPath: string\n}\nRead(string)\n}\n"
	want := "type DetailedError union error {\n    Open {\n        Path: string\n    }\n    Read(string)\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("wrong error union formatting:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatPreservesUnionDefaultVariantMarker(t *testing.T) {
	input := "type State union {\nIdle default\nRunning\n}\n"
	want := "type State union {\n    Idle default\n    Running\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong union default formatting:\n%s\nwant:\n%s", got, want)
	}
}

func TestFixReversedTypeDeclarationOrder(t *testing.T) {
	input := "type struct User {\nname: string,\n}\n\n" +
		"type union State {\nReady\n}\n\n" +
		"type register Status[8] {\nReady: bit,\n_: bit[7],\n}\n"
	want := "type User struct {\n    name: string,\n}\n\n" +
		"type State union {\n    Ready\n}\n\n" +
		"type Status register[8] {\n    Ready: bit,\n    _: bit[7],\n}\n"

	if got := Format(Source{Text: input}, Options{}).Text; got == want || !strings.Contains(got, "type struct User") {
		t.Fatalf("ordinary formatting unexpectedly repaired declaration order:\n%s", got)
	}
	if got := Format(Source{Text: input}, Options{Fix: true}).Text; got != want {
		t.Fatalf("wrong fixed declaration order:\n%s\nwant:\n%s", got, want)
	}
}

func TestFixDoesNotRewriteContextualRegisterIdentifierWithoutRegisterShape(t *testing.T) {
	input := "type register Word\n"
	if got := Format(Source{Text: input}, Options{Fix: true}).Text; got != input {
		t.Fatalf("fix rewrote an unproven contextual register identifier: %q", got)
	}
}

func TestFormatPreservesCanonicalNumericFamilySuffixes(t *testing.T) {
	input := "fn Values() void {\nlet values := [8i, 8u, 8g, 8m, 65t, 65r, 0x41t, 0x10g, 0x10m]\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	for _, literal := range []string{"8i", "8u", "8g", "8m", "65t", "65r", "0x41t", "0x10g", "0x10m"} {
		if !strings.Contains(got, literal) {
			t.Fatalf("formatter lost canonical literal %s:\n%s", literal, got)
		}
	}
}

func TestFormatCompactsUnitExpressionWithoutReordering(t *testing.T) {
	input := "type Flux decimal<( kg * m ) / ( s ^ 2 * A )>\n"
	want := "type Flux decimal<(kg*m)/(s^2*A)>\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("unit expression formatting changed identity or order: %q, want %q", got, want)
	}
}

func TestFormatDoesNotTreatComparisonAsUnitExpression(t *testing.T) {
	input := "fn Compare(a: int, b: int, c: int, d: int) bool {\nreturn a < b / c > d\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if !strings.Contains(got, "return a < b / c > d") {
		t.Fatalf("comparison was rewritten as a unit expression: %q", got)
	}
}

func TestFormatTryHandlerAfterStructLiteralCallArgument(t *testing.T) {
	input := `fn NextToken() Token {
if invalid {
let token := self.readOne(ILLEGAL)
try self.diagnostics.Append(Diagnostic {
ID: "L1002",
Message: $"unexpected byte-order mark at {line}:{column}",
Primary: token,
}) {
Err(error) => { return }
}
return token
}
return self.readOne(ILLEGAL)
}
`
	want := `fn NextToken() Token {
    if invalid {
        let token := self.readOne(ILLEGAL)
        try self.diagnostics.Append(Diagnostic {
            ID: "L1002",
            Message: $"unexpected byte-order mark at {line}:{column}",
            Primary: token,
        }) {
            Err(error) => { return }
        }
        return token
    }
    return self.readOne(ILLEGAL)
}
`

	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong try-handler indentation:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("try-handler formatting is not idempotent:\nfirst:\n%s\nsecond:\n%s", got, again)
	}
}

func TestFormatCanonicalMatchPatterns(t *testing.T) {
	input := `fn Test(value: Shape) int {
match value {
Shape.Circle( ref mut circle )=> 1
Rectangle { width : w, height:ref h } where ready=>2
_=>0
}
}
`
	want := `fn Test(value: Shape) int {
    match value {
        Shape.Circle(ref mut circle) => 1
        Rectangle { width: w, height: ref h } where ready => 2
        _ => 0
    }
}
`
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("formatted match patterns:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatterDoesNotCanonicalizeInvalidExpressionPattern(t *testing.T) {
	input := "A | B=>1\n"
	if got := Format(Source{Text: input}, Options{Fix: true}).Text; got != input {
		t.Fatalf("formatter rewrote non-canonical match-like expression: %q", got)
	}
}

func TestFormatInterfaceReceiverSignatures(t *testing.T) {
	input := "interface Resource {\nmut fn Update( value: int, ) void\n-> fn Detach( ) int\nstatic fn Create( ) int\n}\n"
	want := "interface Resource {\n    mut fn Update(value: int) void\n    -> fn Detach() int\n    static fn Create() int\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("formatted interface signatures:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatCallableCapabilityTypes(t *testing.T) {
	input := "fn Apply( shared: fn( int, ) int, mutable: mut fn( int, ) int, consuming: -> fn( int, ) int, ) void {\n}\n"
	want := "fn Apply(shared: fn(int) int, mutable: mut fn(int) int, consuming: -> fn(int) int) void {\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("wrong callable capability formatting:\n%s\nwant:\n%s", got, want)
	}
}

// rules/errors/panic.md § 15.1 and rules/tooling/formatter.md "Assertion
// statements" require one space after the assertion-message comma.
func TestFormatAssertStatements(t *testing.T) {
	input := "fn Check(ready: bool, value: int) void {\nassert ready\nassert value > Select(1, 2),\"message, // preserved\" // invariant\n}\n"
	want := "fn Check(ready: bool, value: int) void {\n    assert ready\n    assert value > Select(1, 2), \"message, // preserved\" // invariant\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong assertion formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("assertion formatting is not idempotent:\n%s", again)
	}
}

// rules/errors/panic.md § 17 defines panic as a keyword statement followed by
// one ordinary string literal.
func TestFormatPanicStatement(t *testing.T) {
	input := "fn Fail() void {\npanic   \"failure\"\n}\n"
	want := "fn Fail() void {\n    panic \"failure\"\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong panic formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("panic formatting is not idempotent:\n%s", again)
	}
}

// TestFormatCheckedUnreachable verifies that the shared statement formatter
// preserves the canonical payload-free spelling and indentation idempotently.
//
// Rules:
//   - rules/errors/panic.md — § 16(1) "Checked unreachable"
func TestFormatCheckedUnreachable(t *testing.T) {
	input := "fn Stop() int {\nunreachable\n}\n"
	want := "fn Stop() int {\n    unreachable\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong checked-unreachable formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("checked-unreachable formatting is not idempotent:\n%s", again)
	}
}

// rules/tooling/formatter.md "Source model" and "Line comments" require
// source edits to distinguish real lexer comments from comment-like literal or
// block-comment text.
func TestTrailingCommentBoundaryUsesLexicalCST(t *testing.T) {
	tests := []struct {
		line    string
		code    string
		comment string
	}{
		{`value: string /* // not line */ // actual`, `value: string /* // not line */`, `// actual`},
		{"value := `// literal` // actual", "value := `// literal`", "// actual"},
		{`value := $"// {Read()}" // actual`, `value := $"// {Read()}"`, `// actual`},
		{`value := "// literal"`, `value := "// literal"`, ""},
	}
	for _, test := range tests {
		code, comment, found := splitTrailingLineComment(test.line)
		if code != test.code || comment != test.comment || found != (test.comment != "") {
			t.Errorf("split %q = (%q, %q, %v), want (%q, %q)", test.line, code, comment, found, test.code, test.comment)
		}
	}
}

func TestFormatAlignsCommentAfterInlineBlockComment(t *testing.T) {
	input := "type Config struct {\nShort: string /* // fake */, // real\nLonger: int, // other\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if !strings.Contains(got, "/* // fake */,    // real") {
		t.Fatalf("real line comment was not aligned after block comment:\n%s", got)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("block-comment alignment is not idempotent:\n%s", again)
	}
}
