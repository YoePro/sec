package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestIPCErrorIdentities checks standalone/core agreement, closed sets, source
// provenance, Result/error-root use and nominal separation on both plan widths.
// Rules: rules/concurrency/ipc.md — §§6,27,44,55,68,79.
func TestIPCErrorIdentities(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		core := parseIPCErrorFixture(t, "../../sec/core/ipc_errors.sec")
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errors := a.Analyze(core); len(errors) != 0 {
			t.Fatal(errors)
		}
		for _, statement := range core.Statements {
			enum, ok := statement.(*ast.EnumDeclaration)
			if !ok {
				continue
			}
			name := enum.Name.Value
			typ, fallback := a.Types()[name], builtinType(name)
			if !typ.Intrinsic || !typ.Named || !typ.Declared || !typ.ErrorAssignable || typ.Module != "core" || typ.Kind != EnumType || !sameConcreteType(typ, fallback) || !reflect.DeepEqual(typ.EnumValues, fallback.EnumValues) || typ.EnumDefault != fallback.EnumValues[0] {
				t.Fatal(name, typ, fallback)
			}
			for i, value := range enum.Values {
				member := typ.EnumConsts[value.Name.Value]
				if member.Token != value.Name.Token || member.Value.Int64() != int64(i) {
					t.Fatal("lost variant identity", member)
				}
			}
		}
		for _, fixture := range []struct {
			name   string
			errors int
		}{
			{"errors_valid.sec", 0}, {"error_values_valid.sec", 0}, {"errors_invalid.sec", 8}, {"errors_spoof_invalid.sec", 5},
		} {
			program := parseIPCErrorFixture(t, "../../testdata/sema/ipc/"+fixture.name)
			a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := a.Analyze(program); len(errors) != fixture.errors {
				t.Fatal(width, fixture.name, len(errors), errors)
			}
		}
	}
}

// TestIPCErrorMalformedCore keeps trusted source from silently changing the
// portable error contract; malformed declarations retain the exact fallback.
// Rules: rules/concurrency/ipc.md — §§6,27,44,55,68,79.
func TestIPCErrorMalformedCore(t *testing.T) {
	const invalidShape = "../../testdata/sema/ipc/errors_core_shape_invalid.sec"
	program := parseIPCErrorFixture(t, invalidShape)
	program.SourceProvenance = map[string]ast.SourceProvenance{invalidShape: ast.SourceCore}
	if errors := NewAnalyzer().Analyze(program); len(errors) != 5 {
		t.Fatal("non-enum declarations replaced reserved errors", errors)
	}
	for _, name := range []string{"IPCError", "SharedMemoryError", "IPCSyncError", "IPCSharedError", "IPCAtomicError"} {
		for _, change := range []func(*ast.EnumDeclaration){
			func(e *ast.EnumDeclaration) { e.ErrorType = false },
			func(e *ast.EnumDeclaration) { e.Values[0].Name.Value = "Wrong" },
			func(e *ast.EnumDeclaration) { e.Values = e.Values[1:] },
			func(e *ast.EnumDeclaration) { e.Values[0].Default = true },
			func(e *ast.EnumDeclaration) { e.UnderlyingType = &ast.TypeReference{Name: "uint"} },
		} {
			program := parseIPCErrorFixture(t, "../../sec/core/ipc_errors.sec")
			for _, statement := range program.Statements {
				if enum, ok := statement.(*ast.EnumDeclaration); ok && enum.Name.Value == name {
					change(enum)
				}
			}
			a := NewAnalyzer()
			if errors := a.Analyze(program); len(errors) != 1 || !strings.Contains(errors[0].Message, "canonical core error enum") {
				t.Fatal(name, errors)
			}
			if typ := a.Types()[name]; !reflect.DeepEqual(typ.EnumValues, ipcErrorVariants(name)) || !typ.ErrorAssignable {
				t.Fatal(typ)
			}
		}
	}
}

func parseIPCErrorFixture(t *testing.T, file string) *ast.Program {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	if strings.Contains(file, "sec/core/") {
		program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
	}
	return program
}
