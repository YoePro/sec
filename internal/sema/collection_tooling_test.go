package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Rules: collections.md §§13–15; compiler_known_members.md — Collection
// capacity members; lsp.md — One semantic source of truth.
func TestCollectionFactsResolvedAndDetached(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/collection_metadata.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "collection_metadata.sec"))
	program := p.Parse()
	if program.HasErrors {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	fn := a.Functions()["Inspect"][0]
	for i, param := range fn.Parameters {
		f, ok := CollectionFactsOf(param.Type)
		if i == 7 {
			if ok {
				t.Fatal("plain int advertised as collection")
			}
			continue
		}
		if !ok {
			t.Fatalf("no facts for %s", param.Name)
		}
		if i < 6 && f.Dynamic != (i%2 == 0) {
			t.Fatalf("capacity: %+v", f)
		}
		if i < 6 && !f.Dynamic && f.MaximumCapacity != []string{"8", "12", "16"}[i/2] {
			t.Fatal(f)
		}
		if f.Family == "map" && (f.KeyType != "string" || f.ValueType != "int" || f.CountUnit != "entries") {
			t.Fatal(f)
		}
		if f.Family == "set" && (f.ElementType != "string" || f.CountUnit != "distinct elements") {
			t.Fatal(f)
		}
		if (f.Family == "list") != (f.IterationOrder == "index order") {
			t.Fatal(f)
		}
		reserve, factory := false, false
		for _, member := range f.Members {
			if member.Name == "Reserve" {
				reserve = true
				if !member.StructuralMutation || member.Static || member.Result != "Result[void, CollectionError]" {
					t.Fatal(member)
				}
			}
			if member.Name == "WithCapacity" {
				factory = member.Static
			}
		}
		if reserve != f.Dynamic || factory != f.Dynamic {
			t.Fatalf("wrong reservation surface: %+v", f)
		}
		if !strings.Contains(f.Display(), "Len is runtime state") {
			t.Fatal(f.Display())
		}
		again, _ := CollectionFactsOf(param.Type)
		if !reflect.DeepEqual(f, again) {
			t.Fatal("non-deterministic facts")
		}
		f.Members[0].Name = "corrupted"
		after, _ := CollectionFactsOf(param.Type)
		if !reflect.DeepEqual(again, after) {
			t.Fatal("facts alias semantic state")
		}
	}
}

func TestCollectionFactsRejectUnsupportedShapes(t *testing.T) {
	for _, typ := range []Type{builtinType("int"), {Name: "List", TypeArgs: []Type{builtinType("int")}},
		{Name: "list"}, {Name: "map", TypeArgs: []Type{builtinType("int")}},
		{Name: "set", TypeArgs: []Type{builtinType("int")}, ConstArgs: []int64{0}}} {
		if f, ok := CollectionFactsOf(typ); ok {
			t.Fatalf("unsupported type: %+v -> %+v", typ, f)
		}
	}
}
