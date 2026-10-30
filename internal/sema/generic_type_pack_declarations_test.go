package sema

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestTypePackDeclarationFacts keeps recognized packs distinct from a single
// generic type and exposes signatures while concrete expansion stays gated.
// Rules: rules/declarations/generics.md — §16A; MD-117 — §§2,3,9–10.
func TestTypePackDeclarationFacts(t *testing.T) {
	source, err := os.ReadFile("../../testdata/generics/type_pack_declarations/declarations.sec")
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.New(string(source))).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	analyzer := NewAnalyzer()
	errs := analyzer.Analyze(parsed.Program)
	if len(errs) != 4 {
		t.Fatalf("want boundaries only for unsupported owned/mutable/method declarations: %v", errs)
	}
	for _, err := range errs {
		if err.ID != diagnostics.GenericTypePackExpansionUnavailable || err.Help == "" {
			t.Fatal(err)
		}
	}
	count := 0
	ast.Visit(parsed.Program, func(node any) bool {
		if parameter, ok := node.(*ast.GenericParameter); ok {
			fact, ok := analyzer.GenericParameterFactAt(parameter.Name.Token)
			if !ok || fact.TypePack != parameter.TypePack {
				t.Fatalf("wrong declaration fact for %s: %+v", parameter.Name.Value, fact)
			}
			if fact.TypePack {
				count++
			}
		}
		return true
	})
	if count != 5 {
		t.Fatal("missing pack declaration facts", count)
	}
	for name, pack := range map[string]string{"Owned": "Types", "Shared": "Types", "Mutable": "Types", "Mixed": "Rest"} {
		functions := analyzer.Functions()[name]
		if len(functions) != 1 || functions[0].GenericTypePack != pack {
			t.Fatal(name, functions)
		}
		typ := functions[0].Parameters[len(functions[0].Parameters)-1].Type
		if typ.Kind == ReferenceType {
			typ = *typ.Element
		}
		if typ.Kind != GenericTypePackType {
			t.Fatalf("%s pack collapsed into runtime/single-type representation: %s", name, typ.Kind)
		}
	}
}

// TestTypePackHasNoRuntimeMemberSurface keeps a compile-time sequence distinct
// from sized/string-convertible runtime values, including pointwise ref modes.
// Rules: rules/declarations/generics.md — §16A(1–5); MD-117 — §2(4).
func TestTypePackHasNoRuntimeMemberSurface(t *testing.T) {
	pack := Type{Name: "Types", Kind: GenericTypePackType}
	for _, typ := range []Type{pack, {Kind: ReferenceType, Element: &pack}, {Kind: ReferenceType, ReferenceMutable: true, Element: &pack}} {
		if compilerKnownSizedType(typ) || compilerKnownToStringReceiver(typ) {
			t.Fatal("type sequence acquired a runtime member surface", typ.Kind)
		}
		if _, ok := NewAnalyzer().ResolvedTypeDeclarationLocation(typ); typ.Kind == GenericTypePackType && ok {
			t.Fatal("invented concrete declaration")
		}
	}
}

// TestTypePackDoesNotRelaxHomogeneousBorrow preserves the ordinary native
// variadic restriction when an unused type pack is declared beside ...ref int.
// Rules: MD-117 — §1(4), §3; rules/declarations/functions.md — §§28,36.
func TestTypePackDoesNotRelaxHomogeneousBorrow(t *testing.T) {
	source, err := os.ReadFile("../../testdata/generics/type_pack_declarations/homogeneous_borrow_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(source))
	if len(errors) != 2 {
		t.Fatal(errors)
	}
	found := false
	for _, err := range errors {
		found = found || err.Message == "variadic parameter cannot use ref type"
	}
	if !found {
		t.Fatal("homogeneous restriction was removed", errors)
	}
}
