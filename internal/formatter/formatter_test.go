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

// Multiple generic constraints use a parser-owned CST role for canonical
// operator spacing; ordinary bitwise-and expressions remain untouched.
//
// Rules:
//   - rules/declarations/generics.md — §12 "Multiple constraints"
//   - rules/tooling/formatter.md — §5 "Required syntax representation"
//   - rules/tooling/formatter.md — §6 "Horizontal whitespace"
func TestFormatGenericConstraintConjunctionsFromCST(t *testing.T) {
	input := "fn Save[T:First&Comparable, U: Printable](value: T, mask: T) void {\nlet masked := value&mask\ndiscard masked\n}\n\ntype Box[T: Item  &  Printable] struct {\nvalue T\n}\n"
	// The bitwise & is a binary operator and receives §6(5) spacing through its
	// own role; only the constraint & uses the conjunction role.
	want := "fn Save[T:First & Comparable, U: Printable](value: T, mask: T) void {\n    let masked := value & mask\n    discard masked\n}\n\ntype Box[T: Item & Printable] struct {\n    value T\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong generic constraint formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("generic constraint formatting is not idempotent:\n%s", again)
	}
}

// TestFormatOptionIfBinding preserves the sole canonical if payload-binding
// spelling and remains idempotent.
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §12 "State tests"
//   - rules/corrections/applied/formatter-errorhandling-correction-20260824.md — Option binding example
func TestFormatOptionIfBinding(t *testing.T) {
	input := "fn Read(option: Option[int]) int {\nif option is Some( value ) {\nreturn value\n}\nreturn 0\n}\n"
	want := "fn Read(option: Option[int]) int {\n    if option is Some(value) {\n        return value\n    }\n    return 0\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong Option if binding formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("Option if binding formatting is not idempotent:\n%s", again)
	}
}

// TestFormatOwnershipAvailabilityTests preserves contextual ownership queries
// without rewriting them as Option/null state tests.
//
// Rules:
//   - rules/memory/ownership.md — §21 "is available and is not available"
//   - rules/tooling/formatter.md — semantic spelling preservation
func TestFormatOwnershipAvailabilityTests(t *testing.T) {
	input := "fn Check(value: int) void {\nif value is available{\n}\nif value is not available{\n}\n}\n"
	want := "fn Check(value: int) void {\n    if value is available {\n    }\n    if value is not available {\n    }\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong availability formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("availability formatting is not idempotent:\n%s", again)
	}
}

// TestFormatUnionStateTests keeps the body brace of an if or while whose
// condition ends in a union state word separated from that word.
//
// Rules:
//   - rules/tooling/formatter.md — § 8(1), § 18 "Control flow"
//   - rules/declarations/unions.md — §8 "`is` tests for union state and active variant"
func TestFormatUnionStateTests(t *testing.T) {
	input := "fn Check(state: State) void {\nif state is Idle{\n}\nwhile state is State.Running{\n}\nif state is empty{\n}\n}\n"
	want := "fn Check(state: State) void {\n    if state is Idle {\n    }\n    while state is State.Running {\n    }\n    if state is empty {\n    }\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong state-test formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("state-test formatting is not idempotent:\n%s", again)
	}
}

// TestFormatTopLevelTestDeclaration covers the testing rulebook's canonical
// header, ordinary body formatting, exact name preservation, and the fact that
// test remains an ordinary identifier outside declaration shape.
//
// Rules:
//   - rules/tooling/testing.md — §5 "Test declaration syntax"
//   - rules/tooling/testing.md — §41 "Formatter requirements"
func TestFormatTopLevelTestDeclaration(t *testing.T) {
	input := "test   \"keeps  spacing \\\"and escapes\\\"\"{\ntesting.Expect( true , \"message\" )\n}\n\nfn test(test: string) void {\nreturn\n}\n"
	want := "test \"keeps  spacing \\\"and escapes\\\"\" {\n    testing.Expect(true, \"message\")\n}\n\nfn test(test: string) void {\n    return\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong test declaration formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("test declaration formatting is not idempotent:\n%s", again)
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
		"_ => {",
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

// Ordinary formatting conservatively preserves a malformed document until
// CST recovery ranges can isolate independently safe surrounding regions.
//
// Rules:
//   - rules/tooling/formatter.md — §25 "Malformed and incomplete source"
func TestFormatPreservesMalformedSourceByteForByte(t *testing.T) {
	for name, input := range map[string]string{
		"incomplete delimiters": "fn Broken() void {\r\n\tlet values := [1, 2\r\n    // unfinished\r\n",
		"lexical error":         "fn Broken() void {\n\tdiscard \xff\n}\n",
		"unmatched closer":      "fn Broken() void }\n",
	} {
		t.Run(name, func(t *testing.T) {
			result := Format(Source{Text: input}, Options{})
			if !result.Malformed {
				t.Fatal("malformed source was reported as format-safe")
			}
			if result.Text != input {
				t.Fatalf("malformed source changed:\n got %q\nwant %q", result.Text, input)
			}
		})
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

// Parser-owned enum assignment anchors align contiguous explicit-value groups
// while comments, blank lines, and implicit members terminate a group.
//
// Rules:
//   - rules/tooling/formatter.md — §9(8–10) alignment boundaries
//   - rules/tooling/formatter.md — §15(4) enum explicit-value assignments
func TestFormatAlignsExplicitEnumValueAssignmentsFromCST(t *testing.T) {
	input := "enum Status uint8 {\nA=1\nLongName   =   2\nC =3\n// separate\nD=4\nWideName=5\n\nImplicit\nE=6\n}\n"
	want := "enum Status uint8 {\n    A        = 1\n    LongName = 2\n    C        = 3\n    // separate\n    D        = 4\n    WideName = 5\n\n    Implicit\n    E = 6\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong enum assignment alignment:\n%s\nwant:\n%s", got, want)
	}
	if second := Format(Source{Text: got}, Options{}).Text; second != got {
		t.Fatalf("enum assignment alignment is not idempotent:\nfirst:\n%s\nsecond:\n%s", got, second)
	}
}

func TestFormatDropsEnumAlignmentBeyondMaximumPadding(t *testing.T) {
	input := "enum Status uint8 {\nA       =1\nExceptionallyLongStatusName=2\n}\n"
	want := "enum Status uint8 {\n    A = 1\n    ExceptionallyLongStatusName = 2\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("enum assignment padding limit failed:\n%s\nwant:\n%s", got, want)
	}
}

// Attribute-to-declaration layout is parser-owned and formatted from concrete
// tokens rather than from an attribute-name-specific line rewrite.
//
// Rules:
//   - rules/tooling/formatter.md — §16(14) attributes occupy their own line
//   - rules/tooling/formatter.md — §16(17) attributes attach without a blank line
func TestFormatPlacesNoPanicAttributeImmediatelyBeforeDeclaration(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"same line": {
			input: "@noPanic   fn Safe() void {\n}\n",
			want:  "@noPanic\nfn Safe() void {\n}\n",
		},
		"blank line": {
			input: "@noPanic\n\nfn Safe() void {\n}\n",
			want:  "@noPanic\nfn Safe() void {\n}\n",
		},
		"method indentation": {
			input: "impl Worker {\n@noPanic fn Run() void {\n}\n}\n",
			want:  "impl Worker {\n    @noPanic\n    fn Run() void {\n    }\n}\n",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := Format(Source{Text: test.input}, Options{}).Text
			if got != test.want {
				t.Fatalf("wrong attached attribute formatting:\n%s\nwant:\n%s", got, test.want)
			}
			if second := Format(Source{Text: got}, Options{}).Text; second != got {
				t.Fatalf("attached attribute formatting is not idempotent:\nfirst:\n%s\nsecond:\n%s", got, second)
			}
		})
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
	want := "let Global: int := 1\n\nimpl Counter {\n    static let Value: int := 2\n    static let mut Total: int := 0\n    static property Current: int {\n        get {\n            return Counter.Value\n        }\n    }\n}\n\nfn Use() void {\n    static let Calls: int := 0\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("wrong static formatting:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatInitAndNewLifecycleSyntax(t *testing.T) {
	input := "impl Buffer {\ninit ( size: uint, alignment: uint, ) AllocationError {\n}\n}\n\nfn Make() Result[Buffer, AllocationError] {\nreturn try new Buffer(4096, 16)\n}\n"
	want := "impl Buffer {\n    init(size: uint, alignment: uint) AllocationError {\n    }\n}\n\nfn Make() Result[Buffer, AllocationError] {\n    return try new Buffer(4096, 16)\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("wrong lifecycle formatting:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatCallableParameterListOpenersFromCST(t *testing.T) {
	input := "fn Apply ( value: int, callback: fn(int) int ) int {\nlet transform := fn ( item: int ) int {\nreturn item\n}\nreturn callback(transform(value))\n}\n"
	want := "fn Apply(value: int, callback: fn(int) int) int {\n    let transform := fn(item: int) int {\n        return item\n    }\n    return callback(transform(value))\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong CST callable parameter-list formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("CST callable parameter-list formatting is not idempotent:\n%s", again)
	}
}

// Multiline callable parameter lists use the canonical trailing comma while
// single-line signatures remain governed by their separate compact rule.
//
// Rules:
//   - rules/tooling/formatter.md — §11(2) multiline trailing commas
//   - rules/tooling/formatter.md — §16(2) multiline parameter lists
func TestFormatAddsTrailingCommaToMultilineCallableParameters(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"function": {
			input: "fn Connect(\nhost: string,\nport: uint16\n) void {\n}\n",
			want:  "fn Connect(\n    host: string,\n    port: uint16,\n) void {\n}\n",
		},
		"initializer": {
			input: "impl Buffer {\ninit(\nsize: uint\n) AllocationError {\n}\n}\n",
			want:  "impl Buffer {\n    init(\n        size: uint,\n    ) AllocationError {\n    }\n}\n",
		},
		"lambda": {
			input: "fn Build() void {\nlet transform := fn(\nvalue: int\n) int {\nreturn value\n}\n}\n",
			want:  "fn Build() void {\n    let transform := fn(\n        value: int,\n    ) int {\n        return value\n    }\n}\n",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := Format(Source{Text: test.input}, Options{}).Text
			if got != test.want {
				t.Fatalf("wrong multiline parameter formatting:\n%s\nwant:\n%s", got, test.want)
			}
			if second := Format(Source{Text: got}, Options{}).Text; second != got {
				t.Fatalf("multiline parameter formatting is not idempotent:\nfirst:\n%s\nsecond:\n%s", got, second)
			}
		})
	}
}

// A complete multiline generic type-argument list gets its canonical trailing
// comma from parser-owned brackets. Single-line generics and arrays are not
// rewritten by this rule.
//
// Rules:
//   - rules/tooling/formatter.md — §11(1–2) trailing commas
//   - rules/tooling/formatter.md — §15(6–8) multiline generic lists
func TestFormatAddsTrailingCommaToMultilineTypeArguments(t *testing.T) {
	input := "fn Use(\nvalue: Result[\nValue,\nError\n],\ncompact: Result[Value, Error],\narray: Value[4]\n) void {\n}\n"
	want := "fn Use(\n    value: Result[\n        Value,\n        Error,\n    ],\n    compact: Result[Value, Error],\n    array:   Value[4],\n) void {\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong multiline generic argument formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("multiline generic argument formatting is not idempotent:\n%s", again)
	}
}

// Compatible multiline callable parameters align their parser-owned type
// anchors. Blank lines split groups and excessive padding falls back to
// ordinary one-space declaration formatting.
//
// Rules:
//   - rules/tooling/formatter.md — §9(1–2), §9(8–10)
//   - rules/tooling/formatter.md — §16(2–3)
func TestFormatAlignsMultilineCallableParametersFromCST(t *testing.T) {
	input := "fn Connect(\nhost: string,\ncertificatePath:Path,\nport:uint16,\n) void {\n}\n"
	want := "fn Connect(\n    host:            string,\n    certificatePath: Path,\n    port:            uint16,\n) void {\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong callable parameter alignment:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("callable parameter alignment is not idempotent:\n%s", again)
	}
}

func TestFormatDropsCallableParameterAlignmentBeyondPaddingLimit(t *testing.T) {
	input := "fn Wide(\na:int,\nextraordinarilyLongParameter: string,\n) void {\n}\n"
	want := "fn Wide(\n    a: int,\n    extraordinarilyLongParameter: string,\n) void {\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("excessive callable parameter alignment was not dropped:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatCallableParameterBlankLinesSplitAlignmentGroups(t *testing.T) {
	input := "fn Grouped(\na:int,\n\nlongName:string,\nb:bool,\n) void {\n}\n"
	want := "fn Grouped(\n    a: int,\n\n    longName: string,\n    b:        bool,\n) void {\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("parameter blank line did not split alignment groups:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("blank-split parameter alignment is not idempotent:\n%s", again)
	}
}

// Rules:
//   - rules/tooling/formatter.md — §11(2) multiline trailing commas
//   - rules/tooling/formatter.md — §16(12) multiline capture lists
func TestFormatAddsTrailingCommaToMultilineLambdaCaptures(t *testing.T) {
	input := "fn Build() void {\nlet closure := capture(\nfirst,\n<-second\n) fn() int {\nreturn first + second\n}\n}\n"
	want := "fn Build() void {\n    let closure := capture(\n        first,\n        <-second,\n    ) fn() int {\n        return first + second\n    }\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong multiline capture formatting:\n%s\nwant:\n%s", got, want)
	}
	if second := Format(Source{Text: got}, Options{}).Text; second != got {
		t.Fatalf("multiline capture formatting is not idempotent:\nfirst:\n%s\nsecond:\n%s", got, second)
	}
}

// Ordinary executable blocks are always multiline, including empty bodies;
// aggregate literals and empty structural declarations are distinct syntax.
//
// Rules:
//   - rules/tooling/formatter.md — §8(3–8) brace placement
//   - rules/tooling/formatter.md — §15(1–2) structural declarations and literals
func TestFormatExpandsSingleLineExecutableBlocks(t *testing.T) {
	input := "type Marker struct {}\n\nfn Run(ready: bool) void { if ready { Start() } else { Stop() } }\n\nfn Cleanup() void { defer { Close() } }\n\nfn Empty() void {}\n"
	want := "type Marker struct {}\n\nfn Run(ready: bool) void {\n    if ready {\n        Start()\n    } else {\n        Stop()\n    }\n}\n\nfn Cleanup() void {\n    defer {\n        Close()\n    }\n}\n\nfn Empty() void {\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong executable-block formatting:\n%s\nwant:\n%s", got, want)
	}
	if second := Format(Source{Text: got}, Options{}).Text; second != got {
		t.Fatalf("executable-block formatting is not idempotent:\nfirst:\n%s\nsecond:\n%s", got, second)
	}
}

func TestFormatLeavesSingleLineAggregateLiteralCompact(t *testing.T) {
	input := "type Pair struct { Left: int, Right: int }\n\nfn Build() Pair { return Pair { Left: 1, Right: 2 } }\n"
	// rules/tooling/formatter.md — §15(1) makes the structural declaration
	// multiline, while §15(2) lets the aggregate literal stay single-line.
	want := "type Pair struct {\n    Left:  int,\n    Right: int,\n}\n\nfn Build() Pair {\n    return Pair { Left: 1, Right: 2 }\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("aggregate literal was treated as an executable block:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatDeclarationGroupSeparatorsFromCST(t *testing.T) {
	input := "fn Build() void {\nlet first := Pair(1,2) ,second := 3  ,  third := 4\nfloat: low := 1.0 ,high := 2.0\n}\n"
	want := "fn Build() void {\n    let first := Pair(1, 2), second := 3, third := 4\n    float: low := 1.0, high := 2.0\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong CST declaration-group formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("CST declaration-group formatting is not idempotent:\n%s", again)
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
        Some(domain) => {
            out += "; Domain=" + domain
        }
        None => {
        }
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
	input := "type Device register[4] {\nReady: bit read-only,\nCommand: bit write-only,\nPending: bit write-one-clear,\nEvent: bit read-clear,\n}\n"
	want := "type Device register[4] {\n    Ready:   bit read-only,\n    Command: bit write-only,\n    Pending: bit write-one-clear,\n    Event:   bit read-clear,\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("wrong register field modifier formatting:\n%s\nwant:\n%s", got, want)
	}
}

// Nominal declaration items share one local comment column one standard space
// after the widest code cell; trailing comments do not end field alignment
// groups, and register fields align like struct fields (MD-013).
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§ 5.7–5.23
//   - rules/tooling/formatter.md — § 9(2), § 12(4)–(6), § 22(1), § 22(4)
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
    a,      // kommentar 1
    longer, // kommentar 2
    c,      // kommentar 3
}

type Packet struct {
    short:      int,    // field
    longerName: string, // text
    plain:      string,
}

type Device register[4] {
    Ready: bit,    // ready
    Mode:  bit[3], // mode
}

type State union {
    Idle    // idle
    Running // running
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
    ID:   int,    // identity
    Name: string, // display

    // Network settings.
    Endpoint: string,                          // endpoint
    VeryLongTimeoutName: int,                  // timeout
    URL: string = "https://example.test/a//b", // URL
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
		"type Status register[8] {\n    Ready: bit,\n    _:     bit[7],\n}\n"

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

// Foreign function keywords belong to the opt-in Language Corrections layer.
// Ordinary CLI/LSP formatting must not repair them merely because the intended
// declaration happens to be locally recognizable.
//
// Rules:
//   - rules/tooling/formatter.md — §26 "Language Corrections model"
//   - rules/tooling/formatter.md — §27(1) foreign function keywords
func TestForeignFunctionKeywordCorrectionIsOptIn(t *testing.T) {
	input := "func Parse() void {\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != input {
		t.Fatalf("ordinary formatting applied a language correction: %q", got)
	}

	want := "fn Parse() void {\n}\n"
	got := Format(Source{Text: input}, Options{Fix: true}).Text
	if got != want {
		t.Fatalf("opt-in correction = %q, want %q", got, want)
	}
	if again := Format(Source{Text: got}, Options{Fix: true}).Text; again != got {
		t.Fatalf("opt-in correction is not idempotent: %q", again)
	}
}

func TestForeignFunctionKeywordCorrectionRejectsAmbiguousShapes(t *testing.T) {
	for _, input := range []string{
		"let func := callback\n",
		"func Parse()\n",
		"func(value)\n",
	} {
		if got := Format(Source{Text: input}, Options{Fix: true}).Text; got != input {
			t.Fatalf("ambiguous foreign function spelling %q corrected to %q", input, got)
		}
	}
}

// Redundant nested parentheses are a CST-proven, opt-in Language Correction.
// The inner pair is removed so a surrounding call delimiter is preserved.
//
// Rules:
//   - rules/tooling/formatter.md — §17(13) parenthesis preservation
//   - rules/tooling/formatter.md — §27(19) redundant expression parentheses
func TestRedundantNestedParenthesesCorrectionIsOptIn(t *testing.T) {
	input := "fn Read(value: int) int {\nreturn Transform((value))\n}\n"
	ordinary := "fn Read(value: int) int {\n    return Transform((value))\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != ordinary {
		t.Fatalf("ordinary formatting removed parentheses:\n%s\nwant:\n%s", got, ordinary)
	}

	want := "fn Read(value: int) int {\n    return Transform(value)\n}\n"
	got := Format(Source{Text: input}, Options{Fix: true}).Text
	if got != want {
		t.Fatalf("redundant-parentheses correction:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{Fix: true}).Text; again != got {
		t.Fatalf("redundant-parentheses correction is not idempotent:\n%s", again)
	}
}

func TestRedundantEmptyNestedParenthesesCorrection(t *testing.T) {
	if got := Format(Source{Text: "(())\n"}, Options{Fix: true}).Text; got != "()\n" {
		t.Fatalf("double empty parentheses corrected to %q, want %q", got, "()\n")
	}
}

func TestRedundantParenthesesCorrectionPreservesUnsafeShapes(t *testing.T) {
	for _, input := range []string{
		"return (left + right) * scale\n",
		"return (/* attached */(value))\n",
		"return \"((value))\"\n",
	} {
		if got := Format(Source{Text: input}, Options{Fix: true}).Text; got != input {
			t.Fatalf("unsafe parenthesis shape %q corrected to %q", input, got)
		}
	}
}

// Rules: rules/tooling/formatter.md §27(17–18).
func TestRedundantControlConditionParenthesesCorrection(t *testing.T) {
	input := "fn Check(ready: bool, fallback: bool, enabled: bool, value: int) void {\nif (ready) {\nreturn\n}\nwhile (ready) {\nbreak\n}\nswitch (value) {\ndefault:\nreturn\n}\nif (ready || fallback) && enabled {\nreturn\n}\n}\n"
	ordinary := "fn Check(ready: bool, fallback: bool, enabled: bool, value: int) void {\n    if (ready) {\n        return\n    }\n    while (ready) {\n        break\n    }\n    switch (value) {\n        default:\n            return\n    }\n    if (ready || fallback) && enabled {\n        return\n    }\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != ordinary {
		t.Fatalf("ordinary formatting removed control parentheses:\n%s\nwant:\n%s", got, ordinary)
	}

	want := "fn Check(ready: bool, fallback: bool, enabled: bool, value: int) void {\n    if ready {\n        return\n    }\n    while ready {\n        break\n    }\n    switch value {\n        default:\n            return\n    }\n    if (ready || fallback) && enabled {\n        return\n    }\n}\n"
	got := Format(Source{Text: input}, Options{Fix: true}).Text
	if got != want {
		t.Fatalf("control-parentheses correction:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{Fix: true}).Text; again != got {
		t.Fatalf("control-parentheses correction is not idempotent:\n%s", again)
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
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("unit expression formatting changed identity or order: %q, want %q", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("CST unit expression formatting is not idempotent: %q, want %q", again, got)
	}
}

func TestFormatDoesNotTreatComparisonAsUnitExpression(t *testing.T) {
	input := "fn Compare(a: int, b: int, c: int, d: int) bool {\nreturn a < b / c > d\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if !strings.Contains(got, "return a < b / c > d") {
		t.Fatalf("comparison was rewritten as a unit expression: %q", got)
	}
}

// TestFormatCanonicalizesUnitMetadataNames verifies that migration spellings
// become PascalCase only in a parsed unit impl and that formatting is stable.
//
// Rules:
//   - rules/types/units.md — "Unit metadata", canonical PascalCase names
//   - rules/types/units.md — "Formatter requirements"
func TestFormatCanonicalizesUnitMetadataNames(t *testing.T) {
	input := `unit Meter decimal physical

impl Meter {
long_name: "Meter"
SYMBOL: "m"
base_unit: true
STATUS: active
dimension: [length^1]
KIND: length
scale: 1
SYSTEM: SI
transform: linear
OFFSET: 0
origin: zero
log_base: 10
log_factor: 10
REFERENCE: 1
}

impl Ordinary {
long_name: "ordinary"
}
`
	want := `unit Meter decimal physical

impl Meter {
    LongName: "Meter"
    Symbol: "m"
    BaseUnit: true
    Status: active
    Dimension: [length^1]
    Kind: length
    Scale: 1
    System: SI
    Transform: linear
    Offset: 0
    Origin: zero
    LogBase: 10
    LogFactor: 10
    Reference: 1
}

impl Ordinary {
    long_name: "ordinary"
}
`
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong unit metadata formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("unit metadata formatting is not idempotent:\nfirst:\n%s\nsecond:\n%s", got, again)
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
            Err(error) => {
                return
            }
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
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("formatted match patterns:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("CST match formatting is not idempotent:\n%s", again)
	}
}

func TestFormatMatchPatternDelimitersFromCST(t *testing.T) {
	input := "fn Read(value: Shape) int {\nreturn match value {\nShape.Circle ( ref mut circle )=>1\nRectangle{width : w ,height:ref h }=>2\n}\n}\n"
	want := "fn Read(value: Shape) int {\n    return match value {\n        Shape.Circle(ref mut circle) => 1\n        Rectangle { width: w, height: ref h } => 2\n    }\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong CST match-pattern delimiter formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("CST match-pattern delimiter formatting is not idempotent:\n%s", again)
	}
}

func TestFormatMultilineMatchFieldPatternKeepsClosingIndentation(t *testing.T) {
	input := "fn Read(value: Shape) int {\nreturn match value {\nRectangle {\nwidth: w,\nheight: h,\n}=>1\n}\n}\n"
	want := "fn Read(value: Shape) int {\n    return match value {\n        Rectangle {\n            width: w,\n            height: h,\n        } => 1\n    }\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("multiline match-pattern indentation changed:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatMatchArmCommentsRemainValidTrivia(t *testing.T) {
	input := "fn Choose(value: Choice) int {\nreturn match value {\n//first arm\nChoice.Some(item)=>item //trailing\n//between\nChoice.None=>0\n//final\n}\n}\n"
	want := "fn Choose(value: Choice) int {\n    return match value {\n        // first arm\n        Choice.Some(item) => item // trailing\n        // between\n        Choice.None => 0\n        // final\n    }\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong match-comment formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("match-comment formatting is not idempotent:\n%s", again)
	}
}

func TestFormatSwitchCaseHeadersFromCST(t *testing.T) {
	input := "fn Classify(value: int) int {\nswitch value {\ncase 1 ,  3,5 :\nreturn 1\ndefault :\nreturn 0\n}\n}\n"
	want := "fn Classify(value: int) int {\n    switch value {\n        case 1, 3, 5:\n            return 1\n        default:\n            return 0\n    }\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong CST switch-header formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("CST switch-header formatting is not idempotent:\n%s", again)
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

// rules/errors/panic.md § 17 defines panic as a keyword statement with an
// optional ordinary string literal.
func TestFormatPanicStatement(t *testing.T) {
	input := "fn Fail() void {\npanic   \"failure\"\n}\nfn Bare() void {\npanic\n}\n"
	want := "fn Fail() void {\n    panic \"failure\"\n}\nfn Bare() void {\n    panic\n}\n"
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
	if !strings.Contains(got, "/* // fake */, // real") {
		t.Fatalf("real line comment was not aligned after block comment:\n%s", got)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("block-comment alignment is not idempotent:\n%s", again)
	}
}

func TestFormatStandaloneMultilineBlockCommentsFromCST(t *testing.T) {
	input := "fn Example() void {\n/* First paragraph.\nSecond line.\n\n\nThird paragraph. */\n}\n"
	want := "fn Example() void {\n    /*\n     * First paragraph.\n     * Second line.\n     *\n     * Third paragraph.\n     */\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong multiline block-comment formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("multiline block-comment formatting is not idempotent:\n%s", again)
	}
}

func TestFormatStandaloneDocumentationBlockCommentRetainsForm(t *testing.T) {
	input := "/** Summary.\nMore detail. */\nfn Example() void {}\n"
	want := "/**\n * Summary.\n * More detail.\n */\nfn Example() void {\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong documentation block-comment formatting:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatBlockCommentsPreservesSingleLineInlineAndNestedForms(t *testing.T) {
	input := "fn Example() void {\nlet value := 1 /* inline */\n/* outer /* nested */ remains */\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	for _, want := range []string{"let value := 1 /* inline */", "/* outer /* nested */ remains */"} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatter changed preserved block comment %q:\n%s", want, got)
		}
	}
}

func TestFormatBlockCommentKeepsPreformattedContentOpaqueToIndentation(t *testing.T) {
	input := "fn Example() void {\n/*\n * example:\n *     if ready {\n *         Use()\n *     }\n */\nlet value := 1\n}\n"
	want := "fn Example() void {\n    /*\n     * example:\n     *     if ready {\n     *         Use()\n     *     }\n     */\n    let value := 1\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("preformatted block comment affected structural indentation:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("preformatted block-comment formatting is not idempotent:\n%s", again)
	}
}

func TestFormatLineCommentMarginsFromCST(t *testing.T) {
	input := "fn Example() void {\n//first   paragraph\n//     second\n//    \nlet value := 1 //trailing note\n}\n"
	want := "fn Example() void {\n    // first   paragraph\n    // second\n    //\n    let value := 1 // trailing note\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong line-comment formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("line-comment formatting is not idempotent:\n%s", again)
	}
}

func TestFormatLineCommentsUsesOnlyLexerCommentTokens(t *testing.T) {
	input := "fn Example() void {\nlet url := \"https://example.test/a//b\"\nlet raw := `//not a comment`\n/* // block text */\n/// ordinary\n}\n"
	want := "fn Example() void {\n    let url := \"https://example.test/a//b\"\n    let raw := `//not a comment`\n    /* // block text */\n    // / ordinary\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("line-comment token scoping failed:\n%s\nwant:\n%s", got, want)
	}
}

// Parser-owned CST roles keep range and slice operators compact and give the
// contextual for-range step keyword exactly one surrounding space.
//
// Rules:
//   - rules/tooling/formatter.md — §17(12) compact slicing
//   - rules/tooling/formatter.md — §23(5–8) ranges and step
func TestFormatRangesAndStepFromCST(t *testing.T) {
	input := "fn Visit(values: int[], step: int) void {\nfor index in 0  ..<  values.Len    step    2 {\nlet window := values[index  ..  index + 2]\nlet ordinary := step\n}\n}\n"
	want := "fn Visit(values: int[], step: int) void {\n    for index in 0..<values.Len step 2 {\n        let window := values[index..index + 2]\n        let ordinary := step\n    }\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong CST range formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("CST range formatting is not idempotent:\n%s", again)
	}
}

// Struct field types align from parser-owned CST anchors. Comments and blank
// lines split groups, and excessive padding disables alignment for the whole
// candidate group.
//
// Rules:
//   - rules/tooling/formatter.md — §9(5) struct-field alignment
//   - rules/tooling/formatter.md — §9(8–10) boundaries and padding limit
func TestFormatAlignsStructFieldTypesFromCST(t *testing.T) {
	input := "type Endpoint struct {\nHost:       string,\nPort:uint16,\nTimeout: Duration,\n\n// separate group\nID:uint,\nDisplayName:string,\n}\n"
	want := "type Endpoint struct {\n    Host:    string,\n    Port:    uint16,\n    Timeout: Duration,\n\n    // separate group\n    ID:          uint,\n    DisplayName: string,\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong CST struct-field alignment:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("CST struct-field alignment is not idempotent:\n%s", again)
	}
}

func TestFormatDropsStructFieldAlignmentBeyondPaddingLimit(t *testing.T) {
	input := "type Wide struct {\nA: int,\nExtremelyLongFieldName: string,\n}\n"
	want := "type Wide struct {\n    A: int,\n    ExtremelyLongFieldName: string,\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("excessive struct-field alignment was not dropped:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatAlignsStructFieldTagsAsSecondaryAnchors(t *testing.T) {
	input := "type Endpoint struct {\nHost:string    `json:\"host\"`,\nTimeout:Duration `json:\"timeout\"`,\n}\n"
	want := "type Endpoint struct {\n    Host:    string   `json:\"host\"`,\n    Timeout: Duration `json:\"timeout\"`,\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong CST struct-field tag alignment:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("CST struct-field tag alignment is not idempotent:\n%s", again)
	}
}

func TestFormatDropsOnlyStructFieldTagColumnBeyondPaddingLimit(t *testing.T) {
	input := "type Tagged struct {\nShort: int                 `wire:\"short\"`,\nLong: ExtremelyLongTypeName `wire:\"long\"`,\n}\n"
	want := "type Tagged struct {\n    Short: int `wire:\"short\"`,\n    Long:  ExtremelyLongTypeName `wire:\"long\"`,\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("excessive secondary tag alignment did not fall back locally:\n%s\nwant:\n%s", got, want)
	}
}

// Rules:
//   - rules/tooling/formatter.md — §18(7) switch case alternatives, §23(5) "Range operators remain compact"
func TestFormatOpenRangeSwitchCasesKeepCaseKeywordSpacing(t *testing.T) {
	input := "fn F(value: int) int {\nswitch value {\ncase..<0:\nreturn -1\ncase ..< 0:\nreturn -2\ncase 5..  :\nreturn 2\ncase 1 .. 3:\nreturn 1\ndefault:\nreturn 0\n}\n}\n"
	want := "fn F(value: int) int {\n    switch value {\n        case ..<0:\n            return -1\n        case ..<0:\n            return -2\n        case 5..:\n            return 2\n        case 1..3:\n            return 1\n        default:\n            return 0\n    }\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("Format() =\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("second pass changed output:\n%s", again)
	}
}

func TestFormatSwitchValidFixtureIsIdempotent(t *testing.T) {
	source, err := os.ReadFile("../../testdata/switch_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	first := Format(Source{Text: string(source)}, Options{}).Text
	if second := Format(Source{Text: first}, Options{}).Text; second != first {
		t.Fatal("testdata/switch_valid.sec is not idempotent under formatting")
	}
}

// A single-line parameter list has no alignment group, so every parameter gets
// exactly one space after its colon, including ownership-prefixed types; the
// result is a fixed point.
//
// Rules:
//   - rules/tooling/formatter.md — § 6(4), § 16(2)
func TestFormatNormalizesSingleLineParameterSpacing(t *testing.T) {
	input := "fn Copy(destination: ref mut Writer, source:      ref mut Reader) int {\n    return 0\n}\n\nfn A(x:      int, y:   int) int {\n    return 0\n}\n"
	want := "fn Copy(destination: ref mut Writer, source: ref mut Reader) int {\n    return 0\n}\n\nfn A(x: int, y: int) int {\n    return 0\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("single-line parameters formatted as:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("single-line parameter spacing is not idempotent:\n%s", again)
	}
}
