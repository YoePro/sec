package formatter

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Ownership markers retain their parser meaning through canonical formatting,
// including payloads, projections and mixed copy/move multiline captures.
// Rules: rules/memory/copy_move.md §29(3); rules/tooling/formatter.md §§16(11–12),21,29.
func TestFormatOwnershipMarkers(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/ownership_markers.sec")
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile("../../testdata/formatter/ownership_markers.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	before := ownershipSyntax(t, string(input))
	for _, width := range []int{2, 4, 6} {
		for _, crlf := range []bool{false, true} {
			for _, fix := range []bool{false, true} {
				t.Run(fmt.Sprintf("indent%d/crlf%t/fix%t", width, crlf, fix), func(t *testing.T) {
					source := string(input)
					if crlf {
						source = strings.ReplaceAll(source, "\n", "\r\n")
					}
					options := Options{IndentationWidth: width, Fix: fix}
					result := Format(Source{Text: source}, options)
					want := reindentOwnershipGolden(string(golden), width)
					if result.Malformed || result.Text != want {
						t.Fatalf("formatted ownership syntax (malformed=%t):\n%s\nwant:\n%s", result.Malformed, result.Text, want)
					}
					if after := ownershipSyntax(t, result.Text); !reflect.DeepEqual(before, after) {
						t.Fatalf("ownership syntax changed:\nbefore %v\nafter %v", before, after)
					}
					if again := Format(Source{Text: result.Text}, options); again.Text != result.Text || again.Malformed {
						t.Fatalf("second pass changed source:\n%s", again.Text)
					}
				})
			}
		}
	}
}

// Comments, literal contents and parser-invalid capture regions must never be
// reinterpreted as move syntax; ordinary calls and returns remain unmarked.
// Rules: rules/tooling/formatter.md §§21(7–9),25,29(2–4).
func TestFormatOwnershipMarkersPreservesTriviaAndUnmarkedValues(t *testing.T) {
	source := "fn Keep(value: int) int {\nlet text := \"capture(<-value)\"\nlet ordinary := capture(value)fn()int {\nreturn value\n}\nlet marked := capture /* capture origin */ ( <- /* source */ value ) fn()int {\nreturn <- /* return origin */ value\n}\nlet result := Own(value)\nlet temporary := Own(Create())\nreturn result\n}\n"
	before := ownershipSyntax(t, source)
	got := Format(Source{Text: source}, Options{}).Text
	for _, literal := range []string{"\"capture(<-value)\"", "capture(value) fn() int", "/* capture origin */", "/* source */", "/* return origin */", "Own(value)", "Own(Create())", "return result"} {
		if !strings.Contains(got, literal) {
			t.Fatalf("lost %q:\n%s", literal, got)
		}
	}
	if !reflect.DeepEqual(before, ownershipSyntax(t, got)) {
		t.Fatalf("changed ownership syntax:\n%s", got)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("non-idempotent comment formatting:\n%s", again)
	}
	broken := "fn Broken() void {\nlet owned := capture( <- value\n"
	if got := Format(Source{Text: broken}, Options{}); !got.Malformed || got.Text != broken {
		t.Fatalf("changed malformed region: %+v", got)
	}
}

// ownershipSyntax projects parser-owned ownership decisions without positions,
// so normalized whitespace and optional trailing commas do not mask a transfer
// becoming a copy, a changed operand, or a capture/parameter mode change.
// Rules: rules/memory/copy_move.md §§8–11,29(3); rules/tooling/formatter.md §29(2).
func ownershipSyntax(t *testing.T, source string) []string {
	t.Helper()
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("invalid source: %v\n%s", p.Errors(), source)
	}
	var result []string
	ast.Visit(program, func(node any) bool {
		switch node := node.(type) {
		case *ast.Parameter:
			result = append(result, fmt.Sprintf("parameter %s consuming=%t", node.Name.Value, node.Consuming))
		case *ast.ReturnStatement:
			result = append(result, fmt.Sprintf("return move=%t", node.MoveToken.Type == lexer.MOVE_ASSIGN))
		case *ast.LambdaExpression:
			for _, capture := range node.Captures {
				result = append(result, fmt.Sprintf("capture %s %s", capture.Mode, capture.Name.Value))
			}
		case *ast.PrefixExpression:
			result = append(result, "prefix "+node.Operator)
		case *ast.Identifier:
			result = append(result, "identifier "+node.Value)
		case *ast.LetStatement:
			result = append(result, "binding "+string(node.Ownership))
		}
		return true
	})
	return result
}

// The golden's only configuration-dependent layout here is indentation.
// Rules: rules/tooling/formatter.md §4(8),§6(1–3).
func reindentOwnershipGolden(source string, width int) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		lines[i] = strings.Repeat(" ", (len(line)-len(trimmed))/4*width) + trimmed
	}
	return strings.Join(lines, "\n")
}
