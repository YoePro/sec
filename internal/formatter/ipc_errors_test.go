package formatter

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestFormatIPCErrors preserves the five exact error enum declarations and
// their closed variant order, including documentation, through a stable format.
// Rules: rules/concurrency/ipc.md — §§27,44,55,68,79;
// rules/tooling/formatter.md — Enum declarations and documentation comments.
func TestFormatIPCErrors(t *testing.T) {
	assertIPCCoreEnumFormatting(t, "ipc_errors.sec", 5, true)
}

// TestFormatCommandIOModes keeps the two four-variant mode declarations,
// Resource documentation and non-error category stable through formatting.
// Rules: rules/concurrency/ipc.md — §94;
// rules/concurrency/processes.md — §§5.12–5.13;
// rules/tooling/formatter.md — Enum declarations and documentation comments.
func TestFormatCommandIOModes(t *testing.T) {
	assertIPCCoreEnumFormatting(t, "command_io_modes.sec", 2, false)
}

func assertIPCCoreEnumFormatting(t *testing.T, file string, count int, errorType bool) {
	t.Helper()
	source, err := os.ReadFile("../../sec/core/" + file)
	if err != nil {
		t.Fatal(err)
	}
	formatted := Format(Source{Text: string(source)}, Options{}).Text
	if Format(Source{Text: formatted}, Options{}).Text != formatted {
		t.Fatal("not idempotent")
	}
	var before map[string][]string
	for _, text := range []string{string(source), formatted} {
		p := parser.New(lexer.New(text))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		shapes := map[string][]string{}
		for _, statement := range program.Statements {
			if enum, ok := statement.(*ast.EnumDeclaration); ok {
				if enum.ErrorType != errorType || len(enum.GenericParameters) != 0 {
					t.Fatal(enum)
				}
				for _, value := range enum.Values {
					shapes[enum.Name.Value] = append(shapes[enum.Name.Value], value.Name.Value)
				}
			}
		}
		if len(shapes) != count {
			t.Fatal(shapes)
		}
		if before == nil {
			before = shapes
		} else if !reflect.DeepEqual(before, shapes) {
			t.Fatal(shapes, before)
		}
		if len(program.Documentation) == 0 {
			t.Fatal("lost documentation")
		}
	}
}
