package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestMutexOpaqueCoreDeclarations verifies one canonical opaque identity and
// no-copy policy in source and standalone Sema, preserving movable guard owners
// and nontrivial destruction independently of the protected type's traits.
// Rules: rules/concurrency/mutex.md — §§4–6,18,28,46;
// rules/memory/destruction.md — "Destruction classification".
func TestMutexOpaqueCoreDeclarations(t *testing.T) {
	const file = "../../sec/core/mutex.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	for _, name := range []string{"Mutex", "MutexGuard"} {
		typ := a.Types()[name]
		if !typ.Intrinsic || !typ.Named || !typ.ExplicitlyNonCopyable || typ.Module != "core" || typ.DeclarationToken.File != file || len(typ.GenericParameters) != 1 || typ.GenericParameters[0] != "T" || len(typ.Fields) != 0 || !sameConcreteType(typ, builtinType(name)) {
			t.Fatal(typ)
		}
		for _, element := range []Type{builtinType("int"), builtinType("string"), {Name: "Resource", Kind: StructType, ExplicitlyNonCopyable: true}} {
			instance := typ
			instance.TypeArgs = []Type{element}
			instance = a.instantiateGenericType(instance)
			want := CopyNonCopyable
			if name == "MutexGuard" {
				want = CopyMoveOnly
			}
			if CopyClassificationOf(instance) != want || TriviallyDestructible(instance) || DefaultValueOf(instance).Kind != NoDefault {
				t.Fatal(name, element.Name, instance)
			}
		}
	}
	for _, replacement := range []string{"type Mutex[T]", "@noCopy\ntype Mutex[U]", "@noCopy\ntype Mutex[T] struct {}", "enum Mutex { Other }", "interface Mutex {}"} {
		changed := strings.Replace(string(data), "@noCopy\ntype Mutex[T]", replacement, 1)
		p := parser.New(lexer.NewWithFile(changed, file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(replacement, p.Errors())
		}
		program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
		errors := NewAnalyzer().Analyze(program)
		if len(errors) == 0 || !strings.Contains(errors[0].Message, "requires its canonical opaque declaration") {
			t.Fatal(replacement, errors)
		}
	}
	// Merely writing module core does not grant the declaration authority.
	p = parser.New(lexer.NewWithFile(string(data), file))
	errors := NewAnalyzer().Analyze(p.ParseProgram())
	if len(errors) != 2 {
		t.Fatal(errors)
	}
}

// TestMutexNoCopySource checks moves, copies, shared-reference copies and the
// runtime-private construction boundary with ordinary user source.
// Rules: rules/concurrency/mutex.md — §§4–6,18,28.
func TestMutexNoCopySource(t *testing.T) {
	for _, test := range []struct {
		file   string
		errors int
	}{{"mutex_nocopy_valid.sec", 0}, {"mutex_nocopy_invalid.sec", 6}} {
		data, err := os.ReadFile("../../testdata/sema/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		_, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != test.errors {
			t.Fatal(test.file, errors)
		}
		if test.errors > 0 {
			for _, want := range []string{
				"explicitly forbids implicit copy through @noCopy",
				"cannot copy move-only value first",
				"use of moved value first",
				"Mutex has no source struct constructor",
				"MutexGuard has no source struct constructor",
				"unknown member Value on Mutex[int]",
			} {
				found := false
				for _, diagnostic := range errors {
					found = found || strings.Contains(diagnostic.Message, want)
				}
				if !found {
					t.Fatal("missing", want, errors)
				}
			}
		}
	}
}
