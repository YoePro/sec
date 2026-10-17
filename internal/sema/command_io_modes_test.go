package sema

import (
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
)

// TestCommandIOModes keeps Resource in each exact, nominal non-error core enum
// on both scalar plans. Default is Inherit; Pipe and Resource stay distinct.
// Rules: rules/concurrency/ipc.md — §94;
// rules/concurrency/processes.md — §§5.12–5.13;
// rules/types/default_values.md — Enums.
func TestCommandIOModes(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		program := parseIPCErrorFixture(t, "../../sec/core/command_io_modes.sec")
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		for _, statement := range program.Statements {
			enum, ok := statement.(*ast.EnumDeclaration)
			if !ok {
				continue
			}
			name := enum.Name.Value
			typ, fallback := a.Types()[name], builtinType(name)
			want := []string{"Inherit", "Closed", "Pipe", "Resource"}
			if name == "CommandOutputMode" {
				want[1] = "Discard"
			}
			if !typ.Intrinsic || !typ.Named || typ.Module != "core" || typ.Kind != EnumType || typ.ErrorAssignable || typ.ExplicitlyNonCopyable || !sameConcreteType(typ, fallback) || !reflect.DeepEqual(typ.EnumValues, want) || !reflect.DeepEqual(fallback.EnumValues, want) {
				t.Fatal(name, typ, fallback)
			}
			if resolution := DefaultValueOf(typ); resolution.Kind != EnumDefault || resolution.Value.Lexeme != "Inherit" {
				t.Fatal(resolution)
			}
			if CopyClassificationOf(typ) != CopyTrivial {
				t.Fatal("mode must remain an ordinary copyable value", typ)
			}
			for i, value := range enum.Values {
				member := typ.EnumConsts[value.Name.Value]
				if member.Token != value.Name.Token || member.Value.Int64() != int64(i) {
					t.Fatal("lost declaration identity", member)
				}
			}
		}
		for _, fixture := range []struct {
			name   string
			errors int
		}{
			{"command_modes_valid.sec", 0}, {"command_modes_invalid.sec", 8}, {"command_modes_spoof_invalid.sec", 2},
		} {
			program := parseIPCErrorFixture(t, "../../testdata/sema/ipc/"+fixture.name)
			a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := a.Analyze(program); len(errors) != fixture.errors {
				t.Fatal(width, fixture.name, len(errors), errors)
			}
		}
	}
}

// TestCommandIOModesRejectMalformedCore keeps trusted declarations from
// dropping Resource or changing these ordinary enums into errors/other types.
// Rules: rules/concurrency/ipc.md — §94;
// rules/concurrency/processes.md — §§5.12–5.13.
func TestCommandIOModesRejectMalformedCore(t *testing.T) {
	const file = "../../testdata/sema/ipc/command_modes_core_shape_invalid.sec"
	program := parseIPCErrorFixture(t, file)
	program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
	if errors := NewAnalyzer().Analyze(program); len(errors) != 2 {
		t.Fatal(errors)
	}
	for _, name := range []string{"CommandInputMode", "CommandOutputMode"} {
		for _, change := range []func(*ast.EnumDeclaration){
			func(e *ast.EnumDeclaration) { e.ErrorType = true },
			func(e *ast.EnumDeclaration) { e.Values = e.Values[:3] },
			func(e *ast.EnumDeclaration) { e.Values[3].Name.Value = "Handle" },
			func(e *ast.EnumDeclaration) { e.Values[3].Default = true },
			func(e *ast.EnumDeclaration) { e.UnderlyingType = &ast.TypeReference{Name: "uint"} },
		} {
			program := parseIPCErrorFixture(t, "../../sec/core/command_io_modes.sec")
			for _, statement := range program.Statements {
				if enum, ok := statement.(*ast.EnumDeclaration); ok && enum.Name.Value == name {
					change(enum)
				}
			}
			a := NewAnalyzer()
			if errors := a.Analyze(program); len(errors) != 1 || !strings.Contains(errors[0].Message, "canonical core enum") {
				t.Fatal(name, errors)
			}
			if typ := a.Types()[name]; !reflect.DeepEqual(typ.EnumValues, commandIOModeVariants(name)) || typ.ErrorAssignable {
				t.Fatal(typ)
			}
		}
	}
}
