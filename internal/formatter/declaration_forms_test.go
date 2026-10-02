package formatter

import (
	"os"
	"testing"
)

// Every declaration form owned by the types rulebook, plus the declaration
// headers that carry those types, formats to the canonical golden output from
// irregular spacing, and the golden output is a fixed point.
//
// Rules:
//   - rules/types/types.md — "Variable declarations", "Multiple let declarators",
//     "Type-first declarations", "Parenthesized immutable type-first groups",
//     "Named types and contracts", "Safe references", "Raw pointers",
//     "Function types", "First-class collections", "First-class shaped types",
//     "Hardware layout types", "Units", "Core generic result and option types"
//   - rules/tooling/formatter.md — §3, §6, §7(6), §9, §13, §15, §16
func TestFormatEveryTypeDeclarationForm(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/declaration_forms.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/declaration_forms.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	got := Format(Source{Text: string(input)}, Options{}).Text
	if got != string(want) {
		t.Fatalf("declaration forms formatted as:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("declaration-form formatting is not idempotent:\n%s", again)
	}
}

// Type-first declarations keep the colon attached to the type or `mut`
// instead of the previous `int mut : name` output.
//
// Rules:
//   - rules/types/types.md — "Type-first declarations"
//   - rules/tooling/formatter.md — §6(4) declaration colon
func TestFormatTypeFirstDeclarationColonBindsLeft(t *testing.T) {
	input := "fn Use() void {\n    int mut: a\n    int   mut :   b, c\n    uint32 mut: d := 1\n}\n"
	want := "fn Use() void {\n    int mut: a\n    int mut: b, c\n    uint32 mut: d := 1\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("type-first declarations formatted as:\n%s\nwant:\n%s", got, want)
	}
}

// Trailing comments in nominal declaration blocks are aligned against the
// final role-aligned columns, so a second pass is a fixed point.
//
// Rule: rules/tooling/formatter.md — §12 trailing comments, §29 idempotence.
func TestTrailingCommentsRealignAfterRoleAlignment(t *testing.T) {
	input, err := os.ReadFile("../../sec/platform/linux/arm64/syscall_numbers.sec")
	if err != nil {
		t.Skip("platform source not available")
	}
	first := Format(Source{Text: string(input)}, Options{}).Text
	if second := Format(Source{Text: first}, Options{}).Text; second != first {
		t.Fatal("syscall number table is not a formatting fixed point")
	}
}

// Empty collection literals format as `list[T] {}` with one space before an
// empty brace pair.
//
// Rule: rules/collections/collections.md — §13.3 canonical explicit empty forms.
func TestFormatEmptyListLiterals(t *testing.T) {
	input := "fn F() void {\n    let d := list[int,   4]   {}\n    let e: list[ int ] := list[int]{ }\n}\n"
	want := "fn F() void {\n    let d := list[int, 4] {}\n    let e: list[int] := list[int] {}\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("list literals formatted as:\n%s\nwant:\n%s", got, want)
	}
}

// The formatter writes LF line endings for CRLF, bare CR, and mixed input; no
// formatter configuration currently selects another convention. Malformed
// source keeps its original bytes, including line endings.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §2 "Line endings"
//   - rules/tooling/formatter.md — §25 "Malformed and incomplete source"
func TestFormatWritesLFLineEndings(t *testing.T) {
	want := "module main\n\nfn F() void {\n}\n"
	for name, input := range map[string]string{
		"CRLF":    "module main\r\n\r\nfn F() void {\r\n}\r\n",
		"bare CR": "module main\r\rfn F() void {\r}\r",
		"mixed":   "module main\r\n\nfn F() void {\r}\n",
	} {
		if got := Format(Source{Text: input}, Options{}).Text; got != want {
			t.Errorf("%s: formatted %q, want %q", name, got, want)
		}
	}
}

// Foreign-qualified names stay one contiguous token sequence while the
// surrounding annotation colon receives its canonical spacing.
//
// Rules: rules/platform/ffi.md §5-6; rules/tooling/formatter.md §6(4).
func TestFormatKeepsForeignQualifiedNamesContiguous(t *testing.T) {
	input := "module main\nextern \"C\" fn GetVersion(flags: C::uint) C::int\nfn Use() void {\n    let v:C::int := C::int(1)\n    let size: c::stddef::size_t := 0\n}\n"
	want := "module main\nextern \"C\" fn GetVersion(flags: C::uint) C::int\nfn Use() void {\n    let v: C::int := C::int(1)\n    let size: c::stddef::size_t := 0\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("foreign names formatted as:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: want}, Options{}).Text; again != want {
		t.Fatalf("formatting is not idempotent:\n%s", again)
	}
}

// A free lifecycle member formats as `free {` with an ordinary indented body
// and is a fixed point.
//
// Rules:
//   - rules/declarations/impl.md — §19 "`free`"
//   - rules/tooling/formatter.md — §3
func TestFormatFreeLifecycleMember(t *testing.T) {
	input := "module main\n\nimpl Handle {\n    free   {\n  Release(self.raw)\n    }\n}\n"
	want := "module main\n\nimpl Handle {\n    free {\n        Release(self.raw)\n    }\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("free formatted as:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("free formatting is not idempotent:\n%s", again)
	}
}

// Error-handling syntax formats canonically from irregular spacing and the
// result is a fixed point: error enum and union markers, representation
// enums, fallible setter contracts, direct try handlers with where guards and
// block values, Option None handlers, borrowed and consuming projections,
// fallible assignment with and without handlers, and return try. No semantic
// try or match is inserted or removed.
//
// Rules:
//   - rules/tooling/formatter.md — §19(1)–(6) "try"
//   - rules/errors/errorhandling.md — §§3, 6, 15, 19, 21.1, 23, 24, 26
func TestFormatErrorHandlingForms(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/errorhandling_forms.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/errorhandling_forms.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	got := Format(Source{Text: string(input)}, Options{}).Text
	if got != string(want) {
		t.Fatalf("error-handling forms formatted as:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("error-handling formatting is not idempotent:\n%s", again)
	}
}
