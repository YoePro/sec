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

// TestAtomicOperationApplicability exercises the complete common/fetch surface
// on real source, including inherited constraints and preserved nominal results,
// independently of target width. Method type arguments are never part of the API.
// Rules: rules/concurrency/atomics.md — §§8,12–13,21–22,27–31,43–45.
func TestAtomicOperationApplicability(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, test := range []struct {
			file   string
			errors int
		}{
			{"atomic_applicability_valid.sec", 0},
			{"atomic_applicability_invalid.sec", 49},
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
				t.Fatal(width, test.file, errors)
			}
			unsupported, typeargs := 0, 0
			for _, diagnostic := range errors {
				switch {
				case strings.Contains(diagnostic.Message, "not supported for Atomic["):
					unsupported++
				case strings.Contains(diagnostic.Message, "does not take type arguments"):
					typeargs++
				default:
					t.Fatal(diagnostic)
				}
			}
			if test.errors > 0 {
				if unsupported != 40 || typeargs != 9 {
					t.Fatal(unsupported, typeargs)
				}
				continue
			}
			for _, statement := range program.Statements {
				fn, ok := statement.(*ast.FunctionDeclaration)
				if !ok {
					continue
				}
				last := fn.Body.Statements[len(fn.Body.Statements)-1].(*ast.ReturnStatement)
				got, ok := a.ResolvedTypeOf(last.Value)
				declared := a.Functions()[fn.Name.Value][0].ReturnType
				if !ok || !sameConcreteType(got, declared) {
					t.Fatal(fn.Name.Value, got, declared)
				}
				element := declared
				atomic := builtinType("Atomic")
				atomic.TypeArgs = []Type{element}
				names := []string{}
				for _, member := range CompilerKnownMembersForType(atomic, false) {
					if strings.HasPrefix(member.ID, "CKM-ATOMIC-") {
						names = append(names, member.Name)
					}
				}
				want := []string{"Load", "Store", "Swap", "CompareExchange"}
				switch fn.Name.Value {
				case "Signed", "Unsigned", "Named", "NamedChain", "Inferred":
					want = append(want, "FetchAdd", "FetchSub", "FetchAnd", "FetchOr", "FetchXor")
				}
				if !reflect.DeepEqual(names, want) {
					t.Fatal(fn.Name.Value, names, want)
				}
			}
		}
	}
}
