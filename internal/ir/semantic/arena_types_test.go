package semantic

import (
	"os"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Arena keeps one semantic identity across source receiver modes and target
// plans; no backing kind or physical layout is made into a nominal type.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§4,6,84–85.
func TestArenaSemanticTypes(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/semantic_ir/arena_type_forms.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{0, 32, 64} {
		p := parser.New(lexer.New(string(data)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		m := &Module{Version: Version, Identity: "main", Types: NewTypeTable(), PointerWidthBits: width}
		b := &builder{module: m, analyzer: a, maxPackage: 19}
		arena, err := b.internType(a.Types()["Arena"])
		if err != nil {
			t.Fatal(err)
		}
		if arena != m.Types.InternArena() {
			t.Fatal("Arena did not intern canonically")
		}
		for _, name := range []string{"Observe", "Mutate", "Owned"} {
			source := a.Functions()[name][0].Parameters[0].Type
			id, err := b.internType(source)
			if err != nil {
				t.Fatal(err)
			}
			typ, _ := m.Types.Lookup(id)
			if source.Kind == sema.ReferenceType {
				if typ.Kind != TypeReference || typ.Element != arena || typ.Mutable != source.ReferenceMutable {
					t.Fatal("reference lost Arena identity")
				}
			} else if id != arena {
				t.Fatal("owned mode created another Arena type")
			}
		}
		context := m.Types.InternAllocationContext()
		if context == arena || context != m.Types.InternAllocationContext() {
			t.Fatal("internal context identity is not distinct/canonical")
		}
		if err := Verify(m); err != nil {
			t.Fatal(err)
		}
		if ownedCopyClass(m, arena) != "move-only" || trivialDestruction(m, arena) {
			t.Fatal("Arena was treated as a trivial struct")
		}
		text := Format(m)
		if !strings.Contains(text, "Arena") || !strings.Contains(text, "allocation-context") || strings.Contains(text, "struct \"Arena\"") {
			t.Fatal(text)
		}
		for _, pkg := range []uint8{14, 15, 16, 17, 18} {
			old := &builder{module: &Module{Types: NewTypeTable()}, analyzer: a, maxPackage: pkg}
			if _, err := old.internType(a.Types()["Arena"]); err == nil || !strings.Contains(err.Error(), "Arena type (P19)") {
				t.Fatalf("package %d lost Arena gate: %v", pkg, err)
			}
		}
	}
}

// No backing/ABI/type parameter may specialize Arena or the internal context.
// The latter cannot be placed in user aggregates, references, Results or storage.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§84–85,146–148.
func TestArenaTypeVerifier(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Module, TypeID, TypeID)
	}{
		{"arena width", func(m *Module, a, c TypeID) { m.Types.types[a-1].BitWidth = 64 }},
		{"arena backing argument", func(m *Module, a, c TypeID) { m.Types.types[a-1].TypeArgs = []TypeID{a} }},
		{"arena nominal identity", func(m *Module, a, c TypeID) { m.Types.types[a-1].Identity = "borrowed" }},
		{"arena growth name", func(m *Module, a, c TypeID) { m.Types.types[a-1].Name = "GrowableArena" }},
		{"context source name", func(m *Module, a, c TypeID) { m.Types.types[c-1].Name = "Context" }},
		{"context provider parameter", func(m *Module, a, c TypeID) { m.Types.types[c-1].Element = a }},
		{"named context", func(m *Module, a, c TypeID) {
			m.Types.Intern(Type{Kind: TypeNamed, Name: "Wrapper", Identity: "main::Wrapper", Base: c})
		}},
		{"context array", func(m *Module, a, c TypeID) { m.Types.Intern(Type{Kind: TypeArray, Element: c, Length: "1"}) }},
		{"context result", func(m *Module, a, c TypeID) { m.Types.Intern(Type{Kind: TypeResult, Success: c, Error: a}) }},
		{"context ref", func(m *Module, a, c TypeID) { m.Types.Intern(Type{Kind: TypeReference, Element: c}) }},
		{"context place", func(m *Module, a, c TypeID) { m.Types.Intern(Type{Kind: TypePlace, Element: c}) }},
		{"context struct field", func(m *Module, a, c TypeID) {
			m.Structs = []StructDefinition{{Fields: []StructFieldDefinition{{Type: c}}}}
		}},
		{"context union payload", func(m *Module, a, c TypeID) {
			m.Unions = []UnionDefinition{{Variants: []UnionVariantDefinition{{Payload: c}}}}
		}},
		{"context union field", func(m *Module, a, c TypeID) {
			m.Unions = []UnionDefinition{{Variants: []UnionVariantDefinition{{PayloadFields: []UnionPayloadField{{Type: c}}}}}}
		}},
		{"context static", func(m *Module, a, c TypeID) { m.Statics = []Static{{Type: c}} }},
		{"context local", func(m *Module, a, c TypeID) { m.Functions = []*Function{{Storages: []Storage{{Type: c}}}} }},
		{"context returned", func(m *Module, a, c TypeID) { m.Functions = []*Function{{ReturnType: c}} }},
		{"context foreign", func(m *Module, a, c TypeID) {
			m.Functions = []*Function{{Extern: true, Parameters: []Parameter{{Value: Value{Type: c}}}}}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := &Module{Version: Version, Identity: "main", Types: NewTypeTable()}
			a, c := m.Types.InternArena(), m.Types.InternAllocationContext()
			test.change(m, a, c)
			if err := verifyArenaTypes(m); err == nil {
				t.Fatal("invalid Arena/context type accepted")
			}
		})
	}
}

// The internal type does not register any new source-language type name.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §85.
func TestArenaContextIsNotSourceVisible(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/semantic_ir/arena_context_source_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	if errors := a.Analyze(program); len(errors) == 0 {
		t.Fatal("compiler-internal allocation context became source-visible")
	}
	if _, visible := a.Types()["AllocationContext"]; visible {
		t.Fatal("internal type was registered in source types")
	}
}
