package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestCollectionNormalOutcomes checks the canonical distinction between normal
// absence/rejection and structural failure, including negative named indexes,
// nominal Option payloads and the same facts at every analysis depth.
// Rules: rules/collections/collections.md — §§6.9,13.6–13.10,14.7,15.3,16.
func TestCollectionNormalOutcomes(t *testing.T) {
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for _, invalid := range []bool{false, true} {
			suffix := "valid"
			want := 0
			if invalid {
				suffix, want = "invalid", 5
			}
			file := "../../testdata/sema/collection_outcomes_" + suffix + ".sec"
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(data), file))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			if errs := a.Analyze(program); len(errs) != want {
				t.Fatal(depth, suffix, errs)
			}
			if invalid {
				continue
			}
			count := 0
			walkASTValue(reflect.ValueOf(program), func(value any) {
				call, ok := value.(*ast.CallExpression)
				if !ok {
					return
				}
				member := call.Callee.(*ast.MemberExpression)
				typ, known := a.ResolvedTypeOf(call)
				if !known {
					t.Fatal(call)
				}
				count++
				switch member.Property.Value {
				case "RemoveAt", "IndexOf":
					if !isOptionType(typ) {
						t.Fatal(typ)
					}
				case "Remove":
					if member.Object.String() == "mapping" {
						if !isOptionType(typ) || typ.TypeArgs[0].Name != "Item" {
							t.Fatal(typ)
						}
					} else if typ.Kind != BoolType {
						t.Fatal(typ)
					}
				case "Add", "Insert":
					if typ.Kind != ResultType || typ.TypeArgs[0].Kind != BoolType || typ.TypeArgs[1].Name != "CollectionError" {
						t.Fatal(typ)
					}
				default:
					if typ.Kind != BoolType {
						t.Fatal(typ)
					}
				}
			})
			if count != 12 {
				t.Fatal(count)
			}
		}
	}
}

// TestCollectionOutcomeRegistry verifies the same instantiated return types and
// explanatory contracts on dynamic/bounded families and synthetic definitions.
// Rules: rules/collections/collections.md — §§13.6–13.10,14.7,15.3,16;
// rules/compiler/compiler_known_members.md — "Synthetic definitions".
func TestCollectionOutcomeRegistry(t *testing.T) {
	item := Type{Name: "Item", Kind: StructType, Named: true}
	for _, bounded := range []bool{false, true} {
		for _, family := range []string{"list", "map", "set"} {
			typ := builtinTypes()[family]
			typ.TypeArgs = []Type{item}
			if family == "map" {
				typ.TypeArgs = []Type{builtinType("string"), item}
			}
			if bounded {
				typ.ConstArgs = []int64{4}
			}
			count := 0
			for _, member := range CompilerKnownMembersForType(typ, false) {
				if member.Name != "Remove" && member.Name != "RemoveAt" && member.Name != "IndexOf" && member.Name != "Contains" && member.Name != "ContainsKey" && member.Name != "Add" && member.Name != "Insert" {
					continue
				}
				count++
				switch member.ID {
				case "CKM-LIST-REMOVEAT", "CKM-MAP-REMOVE":
					if !isOptionType(member.Result) || member.Result.TypeArgs[0].Name != "Item" {
						t.Fatal(member)
					}
				case "CKM-LIST-INDEXOF":
					if !isOptionType(member.Result) || member.Result.TypeArgs[0].Name != "uint" {
						t.Fatal(member)
					}
				case "CKM-LIST-INSERT", "CKM-SET-ADD":
					if member.Result.Kind != ResultType || member.Result.TypeArgs[0].Kind != BoolType || member.Result.TypeArgs[1].Name != "CollectionError" {
						t.Fatal(member)
					}
				default:
					if member.Result.Kind != BoolType {
						t.Fatal(member)
					}
				}
				if !strings.Contains(member.Documentation, "not an Error") && !strings.Contains(member.Documentation, "only structural/resource failure uses CollectionError") {
					t.Fatal(member)
				}
				if !strings.Contains(CompilerKnownSyntheticDefinition(member), member.Documentation) {
					t.Fatal("definition diverges", member)
				}
			}
			if count != map[string]int{"list": 5, "map": 2, "set": 3}[family] {
				t.Fatal(family, count)
			}
		}
	}
}
