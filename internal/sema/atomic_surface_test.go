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

// TestAtomicSemanticEligibilityIsTargetIndependent accepts the complete
// semantic scalar family on both target widths without promising native,
// lock-free or emulated support, and rejects representation-sized composites.
// Rules: rules/concurrency/atomics.md — §§8–10,27–30.
func TestAtomicSemanticEligibilityIsTargetIndependent(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, test := range []struct {
			file   string
			errors int
		}{
			{"atomic_surface_valid.sec", 0}, {"atomic_surface_invalid.sec", 15},
		} {
			data, err := os.ReadFile("../../testdata/sema/" + test.file)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.New(string(data)))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "linux", TargetArch: "test", PointerWidthBits: width, Endianness: layout.LittleEndian})
			errors := a.Analyze(program)
			if len(errors) != test.errors {
				t.Fatal(width, test.file, errors)
			}
			if test.errors > 0 {
				for _, diagnostic := range errors {
					if !strings.Contains(diagnostic.Message, "not supported") && !strings.Contains(diagnostic.Message, "unknown function or type value.") {
						t.Fatal(diagnostic)
					}
				}
			}
		}
	}
	for _, name := range []string{"float", "float32", "float64", "decimal", "decimal128", "string", "Option", "Result", "Task", "list", "map"} {
		if atomicElementTypeSupported(builtinType(name)) {
			t.Fatal("accepted", name)
		}
	}
}

// TestAtomicMemoryOrderCoreIdentity compares standalone registration with the
// real source declaration and keeps that identity reserved from project code.
// Rules: rules/concurrency/atomics.md — §§4–5.
func TestAtomicMemoryOrderCoreIdentity(t *testing.T) {
	const file = "../../sec/core/atomic.sec"
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
	core := a.Types()["MemoryOrder"]
	fallback := builtinType("MemoryOrder")
	want := []string{"Relaxed", "Acquire", "Release", "AcqRel", "SeqCst"}
	if !core.Intrinsic || !core.Named || core.Kind != EnumType || core.Module != "core" || !reflect.DeepEqual(core.EnumValues, want) || !sameConcreteType(core, fallback) {
		t.Fatal(core, fallback)
	}
	for _, name := range want {
		if core.EnumConsts[name].Token.File != file {
			t.Fatal("missing source", name, core.EnumConsts[name])
		}
	}
	_, errors := analyzeSourceWithAnalyzerRaw(t, "module main\nenum MemoryOrder { Other }\n")
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "compiler-known and cannot be redeclared") {
		t.Fatal(errors)
	}
}

// TestAtomicMemberCatalogUsesCanonicalNames makes lookup and tooling consume
// the same public spelling list, with no lowercase compatibility aliases.
// Rules: rules/concurrency/atomics.md — §§12–13,43–44.
func TestAtomicMemberCatalogUsesCanonicalNames(t *testing.T) {
	typ := builtinType("Atomic")
	typ.TypeArgs = []Type{builtinType("int128")}
	want := []string{"Load", "Store", "Swap", "CompareExchange", "FetchAdd", "FetchSub", "FetchAnd", "FetchOr", "FetchXor"}
	got := []string{}
	for _, member := range CompilerKnownMembersForType(typ, false) {
		if !strings.HasPrefix(member.ID, "CKM-ATOMIC-") {
			continue
		}
		got = append(got, member.Name)
		if len(member.LegacyNames) != 0 || !strings.Contains(member.Signature, "MemoryOrder") {
			t.Fatal(member)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
}
