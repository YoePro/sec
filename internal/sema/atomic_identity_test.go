package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestAtomicCoreIdentity checks source-visible nominal identity, opaque storage,
// contextual/inferred construction and the compiler-owned public API on both
// target widths. A module header alone cannot grant privileged core authority.
// Rules: rules/concurrency/atomics.md — §§4,7,11–13,31–32,43–45;
// rules/declarations/impl.md — §§2–3.
func TestAtomicCoreIdentity(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, test := range []struct {
			file   string
			errors int
		}{
			{"atomic_identity_valid.sec", 0},
			{"atomic_identity_invalid.sec", 14},
			{"atomic_identity_spoof_invalid.sec", 1},
		} {
			data, err := os.ReadFile("../../testdata/sema/" + test.file)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(data), test.file))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "linux", TargetArch: "test", PointerWidthBits: width, Endianness: layout.LittleEndian})
			errors := a.Analyze(program)
			if len(errors) != test.errors {
				t.Fatalf("%d %s: %d: %v", width, test.file, len(errors), errors)
			}
			template := a.Types()["Atomic"]
			if !template.Intrinsic || !template.Named || template.Module != "core" || len(template.GenericParameters) != 1 || len(template.Fields) != 0 {
				t.Fatal(template)
			}
			if test.errors != 0 {
				if len(a.Functions()["Atomic.Load"]) != 0 {
					t.Fatal("untrusted impl replaced the atomic operation")
				}
				continue
			}
			facts, ok := AtomicTypeFactsOf(template)
			if !ok || !facts.Template || facts.Element.Name != "T" || len(facts.Members) != 9 {
				t.Fatal(facts, ok)
			}
			for _, member := range facts.Members {
				if strings.HasPrefix(member.Name, "Fetch") && !strings.Contains(member.Documentation, "only for eligible unconstrained integer T") {
					t.Fatal(member)
				}
			}
			constructors := 0
			for _, statement := range program.Statements {
				var bindings []*ast.LetStatement
				if binding, ok := statement.(*ast.LetStatement); ok {
					bindings = append(bindings, binding)
				}
				if fn, ok := statement.(*ast.FunctionDeclaration); ok {
					for _, child := range fn.Body.Statements {
						if binding, ok := child.(*ast.LetStatement); ok {
							bindings = append(bindings, binding)
						}
					}
				}
				for _, binding := range bindings {
					call, ok := binding.Value.(*ast.CallExpression)
					if !ok || callExpressionName(call) != "Atomic" {
						continue
					}
					typ, ok := a.ResolvedTypeOf(call)
					if !ok || !isAtomicType(typ) || !typ.Named || typ.Module != "core" || len(typ.Fields) != 0 || len(typ.GenericParameters) != 0 {
						t.Fatal(typ, ok)
					}
					name := call.Callee.(*ast.Identifier)
					use, found := a.AtomicTypeAt(name.Token.File, name.Token.Line, name.Token.Column)
					if !found || !sameConcreteType(use, typ) {
						t.Fatal(use, found, typ)
					}
					facts, ok := AtomicTypeFactsOf(typ)
					if !ok || facts.Template || len(facts.Members) != 9 {
						t.Fatal(facts, ok)
					}
					want := "Count"
					if binding.Name.Value == "Requests" {
						want = "uint64"
					}
					if fn, ok := statement.(*ast.FunctionDeclaration); ok && fn.Name.Value == "Contextual" {
						want = "uint64"
					}
					if facts.Element.Name != want {
						t.Fatal(binding.Name.Value, facts.Element, want)
					}
					constructors++
				}
			}
			if constructors != 4 {
				t.Fatal(constructors)
			}
			for _, typ := range []Type{{Name: "Atomic", Kind: StructType}, {Name: "Other", Intrinsic: true, Named: true, Module: "core"}} {
				if _, ok := AtomicTypeFactsOf(typ); ok {
					t.Fatal("noncanonical atomic facts", typ)
				}
			}
		}
	}
}
