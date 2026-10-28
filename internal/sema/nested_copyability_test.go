package sema

import (
	"os"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// Nested concrete instantiations derive copyability from every owned payload;
// two instantiations of one declaration are not an infinite recursive value.
// Rules: rules/memory/copy_move.md — §§4.1–4.3;
// rules/types/types.md — "Generic and parameterized types".
func TestNestedGenericCopyability(t *testing.T) {
	const dir = "../../testdata/sema/nested_copyability/"
	data, err := os.ReadFile(dir + "valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), dir+"valid.sec"))
	program := p.ParseProgram()
	if errors := p.Errors(); len(errors) != 0 {
		t.Fatal(errors)
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatalf("valid nested copies/move rejected: %v", errors)
	}
	for _, name := range []string{"CopyOption", "CopyBox", "CopyUnion", "Deep", "MoveOption", "MoveBox", "MoveUnion"} {
		typ := a.functions[name][0].Parameters[0].Type
		owns := strings.HasPrefix(name, "Move")
		want := CopyTrivial
		if owns {
			want = CopyMoveOnly
		}
		if got := CopyClassificationOf(typ); got != want {
			t.Errorf("%s copy=%s, want %s", name, got, want)
		}
		if TriviallyDestructible(typ) == owns {
			t.Errorf("%s lost nested destruction classification", name)
		}
		if EqualityComparable(typ) == owns {
			t.Errorf("%s lost nested equality classification", name)
		}
	}
	data, err = os.ReadFile(dir + "copy_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(data))
	if len(errors) != 3 {
		t.Fatalf("want three rejected copies, got %v", errors)
	}
	for _, e := range errors {
		if e.ID != "S1007" || !strings.Contains(e.Message, "move-only") {
			t.Errorf("wrong copy restriction: %+v", e)
		}
	}
}

// A genuine recursive nominal value still stops classification without
// manufacturing copying permission; unresolved generic payloads do too.
// Rules: rules/memory/copy_move.md — §§4,11;
// rules/memory/destruction.md — §29.
func TestGenericCopyabilityRecursionRemainsConditional(t *testing.T) {
	recursive := Type{Name: "Recursive", Kind: UnionType}
	recursive.UnionVariants = []UnionVariant{{Name: "Again", Payload: &recursive}}
	if got := CopyClassificationOf(recursive); got != CopyConditional {
		t.Fatalf("recursive copy=%s", got)
	}
	unresolved := Type{Name: "T", Kind: GenericType}
	outer := Type{Name: "Box", Kind: StructType, TypeArgs: []Type{unresolved}, Fields: []StructField{{Name: "value", Type: unresolved}}}
	if got := CopyClassificationOf(outer); got != CopyConditional {
		t.Fatalf("unresolved generic copy=%s", got)
	}
}
