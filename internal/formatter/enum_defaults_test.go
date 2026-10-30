package formatter

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestFormatEnumDefaultMarkers preserves semantic default placement and exact
// literal spellings while aligning complete member/default cells. Parsing the
// output also verifies that format and second pass retain every marked member.
// Rules: rules/declarations/enums.md — §§5,8.2;
// rules/tooling/formatter.md — §§15(4),29(1–3).
func TestFormatEnumDefaultMarkers(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/enum_defaults.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/enum_defaults.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{2, 4, 6} {
		options := Options{IndentationWidth: width}
		got := Format(Source{Text: string(input)}, options).Text
		if width == 4 && got != string(want) {
			t.Fatalf("format:\n%s\nwant:\n%s", got, want)
		}
		if again := Format(Source{Text: got}, options).Text; again != got {
			t.Fatal("not idempotent", again)
		}
		parsed := parser.New(lexer.New(got)).Parse()
		if parsed.HasErrors {
			t.Fatal(parsed.Diagnostics)
		}
		for _, statement := range parsed.Program.Statements {
			enum, ok := statement.(*ast.EnumDeclaration)
			if !ok {
				continue
			}
			if len(enum.Values) != 3 || !enum.Values[1].Default || enum.Values[0].Default || enum.Values[2].Default {
				t.Fatalf("lost default in %s", enum.Name.Value)
			}
		}
	}
}

// TestFormatEnumDefaultInitializerRecovery retains the parser's default and
// initializer despite the legacy colon error, including later members/source.
// Ordinary formatting preserves the invalid spelling for the migration error;
// it does not silently turn a rejected program into an accepted one.
// Rules: rules/declarations/enums.md — §§5,8.2;
// rules/tooling/formatter.md — §§25–26; rules/compiler/parser_recovery.md — "Enum recovery".
func TestFormatEnumDefaultInitializerRecovery(t *testing.T) {
	path := "../../testdata/formatter/enum_default_recovery_invalid.sec"
	input, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	formatted := Format(Source{Text: string(input)}, Options{}).Text
	if !strings.Contains(formatted, "Chosen default:") {
		t.Fatal("default marker spacing lost", formatted)
	}
	if again := Format(Source{Text: formatted}, Options{}).Text; again != formatted {
		t.Fatal("recovery formatting not idempotent", again)
	}
	for _, text := range []string{string(input), formatted} {
		parsed := parser.New(lexer.New(text)).Parse()
		if len(parsed.Diagnostics) != 1 || parsed.Diagnostics[0].ID != diagnostics.ParserLegacyEnumColonInitializer {
			t.Fatal(parsed.Diagnostics)
		}
		if len(parsed.Program.Statements) != 3 {
			t.Fatal("lost later declaration")
		}
		enum := parsed.Program.Statements[1].(*ast.EnumDeclaration)
		member := enum.Values[1]
		if len(enum.Values) != 3 || !member.Default || member.InitializerToken.Type != lexer.COLON || member.Initializer.(*ast.StringLiteral).Value != "b" {
			t.Fatalf("lost recovered member: %+v", member)
		}
		if fn := parsed.Program.Statements[2].(*ast.FunctionDeclaration); fn.Name.Value != "After" {
			t.Fatal("lost later function")
		}
	}
}
