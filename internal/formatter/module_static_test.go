package formatter

import (
	"os"
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// Module static removal respects lexical trivia and AST declaration scope;
// canonical output is shared with the CLI and LSP and is a fixed point.
// Rules: rules/declarations/static.md — §§3,25;
// rules/tooling/formatter.md — §§6,25,28–29.
func TestFormatModuleStatic(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/module_static.sec")
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile("../../testdata/formatter/module_static.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{2, 4, 6} {
		lines := strings.Split(string(golden), "\n")
		for i, line := range lines {
			spaces := len(line) - len(strings.TrimLeft(line, " "))
			// Comment decoration is lexical trivia, not a structural indent.
			if spaces >= 4 {
				lines[i] = strings.Repeat(" ", spaces/4*width) + line[spaces:]
			}
		}
		want := strings.Join(lines, "\n")
		for _, fix := range []bool{false, true} {
			for _, crlf := range []bool{false, true} {
				input := string(data)
				if crlf {
					input = strings.ReplaceAll(input, "\n", "\r\n")
				}
				options := Options{Fix: fix, IndentationWidth: width}
				got := Format(Source{Text: input}, options)
				if got.Malformed || got.Text != want {
					t.Fatalf("width=%d fix=%v crlf=%v:\n%s\nwant:\n%s", width, fix, crlf, got.Text, want)
				}
				if again := Format(Source{Text: got.Text}, options); again.Text != got.Text {
					t.Fatalf("not idempotent:\n%s", again.Text)
				}
				p := parser.New(lexer.New(got.Text))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				for _, statement := range program.Statements {
					switch statement := statement.(type) {
					case *ast.LetStatement:
						if statement.Static {
							t.Fatal("redundant modifier retained")
						}
					case *ast.LetGroupStatement:
						for _, binding := range statement.Lets {
							if binding.Static {
								t.Fatal("group modifier retained")
							}
						}
					}
				}
			}
		}
	}
}

// Recovered declarations do not authorize token deletion, even for an earlier
// valid declaration. Trivia between static and let remains byte-for-byte intact.
// Rules: rules/tooling/formatter.md — §§6,25;
// rules/declarations/static.md — §25.
func TestModuleStaticRemovalBoundaries(t *testing.T) {
	invalid := "static let Good := 1\nlet broken :=\n"
	if got := formatModuleStatic(invalid); got != invalid {
		t.Fatalf("rewrote recovered source: %q", got)
	}
	input := "/* static let Comment := 0 */ static /* keep */ let Value := \"static let Text\"\n"
	want := "/* static let Comment := 0 */ /* keep */ let Value := \"static let Text\"\n"
	if got := formatModuleStatic(input); got != want {
		t.Fatalf("trivia changed: %q", got)
	}
}
