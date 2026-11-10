package sema

import (
	"os"
	"reflect"
	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"testing"
)

// TestCommandConfigurationError checks the exact nominal error identity,
// InvalidIOResource, ordinary copy/default behavior and trusted refinement.
// Rules: rules/concurrency/processes.md — §5.9; rules/concurrency/ipc.md — §97;
// rules/types/default_values.md — Enums.
func TestCommandConfigurationError(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, tc := range []struct {
			name    string
			trusted bool
			errors  int
		}{
			{"valid.sec", false, 0}, {"invalid.sec", false, 3},
			{"core.sec", true, 0}, {"core_invalid.sec", true, 1},
			{"core.sec", false, 1},
		} {
			file := "../../testdata/sema/ipc/command_configuration/" + tc.name
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(data), file))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			if tc.trusted {
				program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
			}
			a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := a.Analyze(program); len(errors) != tc.errors {
				t.Fatal(width, tc, errors)
			}
			typ := a.Types()["CommandConfigurationError"]
			want := []string{"InvalidState", "InvalidWorkingDirectory", "NotFound", "NotDirectory", "PermissionDenied", "InvalidEnvironmentName", "InvalidEnvironmentValue", "InvalidIOResource", "OutOfMemory"}
			if typ.Kind != EnumType || !typ.Named || !typ.Intrinsic || !typ.ErrorAssignable || typ.Module != "core" || !reflect.DeepEqual(typ.EnumValues, want) || CopyClassificationOf(typ) != CopyTrivial {
				t.Fatal(typ)
			}
			if result := DefaultValueOf(typ); result.Kind != EnumDefault || result.Value.Lexeme != "InvalidState" {
				t.Fatal(result)
			}
			if tc.trusted && tc.errors == 0 {
				if typ.EnumConsts["InvalidIOResource"].Token.File != file || typ.DeclarationToken.File != file {
					t.Fatal("lost canonical source locations", typ)
				}
			}
		}
	}
}
