package sema

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Source declarations and standalone fallbacks must denote one core identity,
// preserving metadata copyability and the TaskOutcome containment payload.
// Rules: rules/errors/panic.md §13(2–7); rules/concurrency/tasks.md §11.
func TestPanicMetadataCoreDeclarations(t *testing.T) {
	panicFile := "../../sec/core/panic.sec"
	data, err := os.ReadFile(panicFile)
	if err != nil {
		t.Fatal(err)
	}
	parse := func(file, source string) *ast.Program {
		t.Helper()
		p := parser.New(lexer.NewWithFile(source, file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		return program
	}
	for _, width := range []uint16{32, 64} {
		program := parse(panicFile, string(data))
		program.SourceProvenance = map[string]ast.SourceProvenance{panicFile: ast.SourceCore}
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		for _, name := range []string{"PanicID", "PanicInfo"} {
			typ := a.Types()[name]
			if !typ.Intrinsic || !typ.Declared || typ.Module != "core" || typ.DeclarationToken.File != panicFile || !implicitlyCopyable(typ) {
				t.Fatal(name, typ)
			}
			if !sameConcreteType(typ, builtinTypes()[name]) {
				t.Fatal("source and fallback identities differ", name)
			}
		}
		info := a.Types()["PanicInfo"]
		if len(info.Fields) != 5 {
			t.Fatal(info)
		}
		for _, field := range info.Fields {
			if field.Token.File != panicFile || !sameConcreteType(field.Type, a.Types()[field.Type.Name]) {
				t.Fatal("field lost canonical identity/source", field)
			}
		}
		for _, variant := range a.Types()["TaskOutcome"].UnionVariants {
			if variant.Name == "Panicked" && (variant.Payload == nil || !sameConcreteType(*variant.Payload, info)) {
				t.Fatal("fallback task payload differs", variant)
			}
		}
		for _, file := range []string{"../../sec/core/task.sec", "../../testdata/sema/panic_info_valid.sec"} {
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			program.Statements = append(program.Statements, parse(file, string(source)).Statements...)
			if strings.HasSuffix(file, "task.sec") {
				program.SourceProvenance[file] = ast.SourceCore
			}
		}
		a = NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal("core TaskOutcome and ordinary metadata reads", errors)
		}
		for _, variant := range a.Types()["TaskOutcome"].UnionVariants {
			if variant.Name == "Panicked" && (variant.Payload == nil || !sameConcreteType(*variant.Payload, a.Types()["PanicInfo"])) {
				t.Fatal("source task payload differs", variant)
			}
		}
	}
	for _, tc := range []struct{ old, new string }{
		{"uint32", "uint64"}, {`Line:\s+uint`, "Line: int"}, {`Function:\s+string`, "Extra: string"},
		{"type PanicInfo struct", "@noCopy\ntype PanicInfo struct"},
		{"type PanicInfo struct", "type PanicInfo[T] struct"},
		{`Column:\s+uint,`, "Column: uint,\n    Extra: uint,"},
	} {
		changed := regexp.MustCompile(tc.old).ReplaceAllString(string(data), tc.new)
		if changed == string(data) {
			t.Fatal("mutation was not applied", tc)
		}
		program := parse(panicFile, changed)
		program.SourceProvenance = map[string]ast.SourceProvenance{panicFile: ast.SourceCore}
		errors := NewAnalyzer().Analyze(program)
		if len(errors) != 1 || errors[0].ID != diagnostics.ReservedDeclarationName || !strings.Contains(errors[0].Message, "canonical core declaration") {
			t.Fatal(tc, errors)
		}
	}
	// A convincing filename and module declaration are not loader authority.
	errors := NewAnalyzer().Analyze(parse(panicFile, string(data)))
	if len(errors) != 2 {
		t.Fatal("untrusted declarations were accepted", errors)
	}
	for _, e := range errors {
		if e.ID != diagnostics.ReservedDeclarationName {
			t.Fatal(e)
		}
	}
}
