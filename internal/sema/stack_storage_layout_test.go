package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
)

// Storage evidence must follow the authoritative representation on each plan,
// including padding/array stride, and preserve unresolved descriptor/atomic
// boundaries. An object's bytes are not a proof of a callable frame.
// Rules: rules/analysis/stack_analysis.md §§8–9,41,44,48–58.
func TestStackStorageLayout(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/stack_storage_layout.sec")
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{}
	for _, arch := range []string{"amd64", "armv7"} {
		t.Run(arch, func(t *testing.T) {
			definition, _ := target.Find(target.Target{OS: "linux", Arch: arch})
			plan, err := definition.ScalarPlan()
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(data), "stack_storage_layout.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithScalarPlan(plan)
			if diagnostics := a.Analyze(program); len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			r, err := definition.LayoutResolver()
			if err != nil {
				t.Fatal(err)
			}
			for _, field := range a.types["Storage"].Fields {
				got, err := ResolveStackStorageLayout(r, field.Type)
				if err != nil {
					t.Fatal(err)
				}
				want := r.Resolve(LayoutRequirementOf(field.Type))
				if !reflect.DeepEqual(got.Representation, want) || got.PlanKey != r.Key().String() {
					t.Fatalf("%s differs from resolver", field.Name)
				}
				if want.Complete() {
					count, finite := got.Bytes.Bytes()
					if !finite || got.Bytes.Kind() != StackBoundExact || count.Cmp(want.SizeBytes) != 0 {
						t.Fatalf("%s: %s vs %v", field.Name, got.Bytes, want.SizeBytes)
					}
					if field.Name == "Inline" {
						expected := int64(24)
						if arch == "armv7" {
							expected = 16
						}
						if count.Int64() != expected {
							t.Fatalf("aggregate bytes %v, want %d", count, expected)
						}
					}
					// Neither count nor representation snapshots can mutate the resolver.
					count.SetInt64(999)
					got.Representation.SizeBytes.SetInt64(999)
					for i := range got.Layouts {
						if got.Layouts[i].SizeBytes != nil {
							got.Layouts[i].SizeBytes.SetInt64(777)
						}
					}
					again, _ := ResolveStackStorageLayout(r, field.Type)
					if !reflect.DeepEqual(again.Representation, want) {
						t.Fatal("mutable published evidence")
					}
				} else if got.Bytes.Kind() != StackBoundUnknown || len(want.Missing) == 0 {
					t.Fatalf("%s: unresolved layout has %s or no missing evidence", field.Name, got.Bytes)
				}
				before, _ := ResolveStackStorageLayout(r, field.Type)
				r.Resolve(LayoutRequirementOf(builtinType("int256")))
				after, _ := ResolveStackStorageLayout(r, field.Type)
				if !reflect.DeepEqual(before, after) {
					t.Fatal("unrelated layout changed object evidence")
				}
			}
			keys[r.Key().String()] = true
		})
	}
	if len(keys) != 2 {
		t.Fatal("plan identities conflated")
	}
	if _, err := ResolveStackStorageLayout(nil, builtinType("int")); err == nil {
		t.Fatal("missing resolver accepted")
	}
}

// Missing layout knowledge cannot become zero bytes. Custom target alignment
// changes must flow through the resolver, including padding in fixed arrays.
// Rules: rules/analysis/stack_analysis.md §§8–9,41; rules/memory/layout.md §§5,12.
func TestStackStorageLayoutMissingAndAlignment(t *testing.T) {
	plan := layout.ResolvedScalarPlan{TargetOS: "test", TargetArch: "test", PointerWidthBits: 64, Endianness: layout.LittleEndian}
	for _, alignment := range []uint64{0, 4, 16} {
		alignments := layout.ScalarAlignments{}
		if alignment != 0 {
			alignments[layout.StorageRepresentation{Class: layout.IntegerStorage, WidthBits: 32}] = alignment
		}
		r, err := layout.NewResolver(plan, alignments)
		if err != nil {
			t.Fatal(err)
		}
		typ := Type{Kind: StructType, Name: "Padded", Fields: []StructField{{Name: "First", Type: builtinType("int32")}, {Name: "Second", Type: builtinType("int32")}}}
		got, err := ResolveStackStorageLayout(r, typ)
		if err != nil {
			t.Fatal(err)
		}
		if alignment == 0 {
			if got.Bytes.Kind() != StackBoundUnknown || got.Representation.SizeBytes != nil {
				t.Fatal("missing alignment guessed")
			}
		} else {
			count, _ := got.Bytes.Bytes()
			if count.Uint64() != 2*alignment || got.Representation.AlignBytes.Uint64() != alignment {
				t.Fatalf("alignment %d: %+v", alignment, got)
			}
		}
		empty, err := ResolveStackStorageLayout(r, Type{Kind: StructType, Name: "Empty"})
		if err != nil {
			t.Fatal(err)
		}
		count, finite := empty.Bytes.Bytes()
		if !finite || count.Sign() != 0 || empty.Bytes.Kind() != StackBoundExact {
			t.Fatal("complete empty object lost exact zero")
		}
		unknown, err := ResolveStackStorageLayout(r, Type{Kind: UnionType, Name: "Pending"})
		if err != nil {
			t.Fatal(err)
		}
		if unknown.Bytes.Kind() != StackBoundUnknown || unknown.Representation.Completeness != layout.LayoutUnsupported {
			t.Fatal("unsupported union guessed")
		}
	}
}
