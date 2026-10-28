package sema

import (
	"strings"
	"testing"
)

// A bare None argument takes its Option[T] type from the parameter in its
// position when every candidate accepting the argument count agrees, for
// ordinary calls and for `new` initializers; candidates that still disagree
// after all arguments are considered make the call ambiguous (S1147, MD-027).
//
// Rules:
//   - rules/foundations/language_philosophy.md — §16 "Infer, do not guess"
//   - rules/declarations/impl.md — initializer selection through `new`
//   - rules/types/types.md — "Contextual None and empty-literal arguments"
func TestBareNoneArgumentUsesAgreedParameterContext(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type Box struct {
    value: int,
}

impl Box {
    init(first: Option[int], second: int) {
        self.value = second
    }

    init(only: Option[string]) BuildError {
        self.value = 0
    }
}

enum BuildError error {
    Bad,
}

fn Take(first: Option[int], second: int) int {
    return second
}

fn Either(value: Option[int]) void {
}

fn Either(value: Option[string]) void {
}

fn Calls() Result[Box, BuildError] {
    discard Take(None, 1)
    let a := new Box(None, 1)
    discard a
    let b := try new Box(None)
    return Ok(b)
}

fn Ambiguous() void {
    Either(None)
}
`)
	assertSemaErrors(t, errors, []string{
		"ambiguous call to Either: None could be Option[int] or Option[string] at 41:12",
	})
}

// The shared builtin type table is built once for lookups, while every
// Analyzer still receives its own mutable copy.
func TestBuiltinTypeTableCopiesAreIndependent(t *testing.T) {
	first := builtinTypes()
	first["int"] = Type{Name: "changed", Kind: InvalidType}
	if builtinType("int").Kind != IntType || builtinTypes()["int"].Kind != IntType {
		t.Fatal("mutating an Analyzer copy changed the shared builtin type table")
	}
}

// MD-027 regression matrix: overload resolution considers all supplied
// arguments together before typing None and [] from the single selected
// parameter, for calls and `new` initializers; incompatible remaining
// candidates are ambiguous with the candidate types named; an explicitly
// typed binding disambiguates; return types neither distinguish overloads
// nor select one.
//
// Rules:
//   - rules/types/types.md — "Contextual None and empty-literal arguments"
//   - rules/declarations/functions.md — "Overload resolution"
//   - rules/corrections/applied/md027-md033-md036-md054-contextual-literals-formatter-configuration-correction-20261012.md — §§ 2, 5.1
func TestContextualLiteralArgumentsFollowOverloadResolution(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type Box struct {
    item: Option[int],
    items: int[],
}

impl Box {
    init(item: Option[int], items: int[]) {
        self.item = item
        self.items <- items
    }
}

fn AcceptOption(value: Option[int]) void {}
fn AcceptItems(values: int[]) void {}

fn Set(key: string, value: Option[int]) void {}
fn Set(key: int, value: Option[string]) void {}

fn Put(value: Option[int], key: string) void {}
fn Put(value: Option[string], key: int) void {}

fn Store(value: Option[int]) void {}
fn Store(value: Option[string]) void {}

fn Load(values: int[]) void {}
fn Load(values: string[]) void {}

fn Make(a: int) int {
    return a
}

fn Make(a: string) string {
    return a
}

fn Valid() void {
    AcceptOption(None)
    AcceptItems([])
    let box := new Box(None, [])
    discard box
    Set("count", None)
    Set(42, None)
    Put(None, "count")
    Put(None, 42)
    let number: Option[int] := None
    Store(number)
    let names: string[] := []
    Load(<-names)
}

fn Invalid() void {
    Store(None)
    Load([])
    let text: string := Make(1)
}
`)
	assertSemaErrors(t, errors, []string{
		"ambiguous call to Store: None could be Option[int] or Option[string] at 55:11",
		"ambiguous call to Load: [] could be int[] or string[] at 56:10",
		"cannot initialize string with int at 57:25",
	})
	if errors[0].ID != "S1147" || !strings.Contains(errors[0].Help, "let value: Option[int] := None") {
		t.Fatalf("ambiguity diagnostic lacks S1147 or its typed-binding help: %+v", errors[0])
	}
}

// Overloads differ by their complete input signature; a return type alone
// never makes two declarations distinct.
func TestReturnTypeAloneDoesNotDistinguishOverloads(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

fn Fun(a: int, b: string) void {}
fn Fun(a: string, b: int) void {}
fn Fun(a: int, b: int) void {}

fn Invalid(a: int, b: string) int {
    return 1
}

fn Invalid(a: int, b: string) string {
    return "one"
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, `duplicate function "Invalid" with same signature`) {
		t.Fatalf("errors = %v, want only the return-type-only duplicate rejected", errors)
	}
}

// A bare Some(...) argument is a contextual Option argument like None: its
// Option[T] comes from the parameter in that position, for ordinary calls,
// methods, middle positions and struct fields; a non-Option parameter is a
// reported mismatch (never silent), overloads differing only in the Option
// payload stay ambiguous, an uninstantiated generic parameter gives no
// context, and None() is explained (MD-102).
//
// Rules:
//   - rules/types/types.md — "Contextual None and empty-literal arguments"
//   - rules/foundations/language_philosophy.md — §16 "Infer, do not guess"
func TestBareSomeArgumentUsesParameterContext(t *testing.T) {
	valid := analyzeSourceRaw(t, `
module main

type Box struct {
    value: Option[int],
}

impl Box {
    fn Has(probe: Option[int]) bool {
        return true
    }
}

fn Pick(value: Option[int]) int {
    return match value {
        Some(v) => v
        None => 0
    }
}

fn Middle(a: int, value: Option[rune], b: int) int {
    return a + b
}

fn Calls(x: int, r: rune) int {
    let box := Box { value: Some(x) }
    discard box.Has(Some(2))
    discard Middle(1, Some(r), 2)
    return Pick(Some(x))
}
`)
	if len(valid) != 0 {
		t.Fatalf("valid Some arguments rejected: %v", valid)
	}
	invalid := analyzeSourceRaw(t, `
module main

fn Pick(value: Option[int]) int {
    return 0
}

fn Plain(value: int) int {
    return value
}

fn Store(value: Option[int]) void {
}

fn Store(value: Option[string]) void {
}

fn Process[T](value: Option[T]) void {
}

fn Calls() int {
    discard Pick(Some("text"))
    discard Plain(Some(1))
    discard Plain(None)
    Store(None)
    Process(None)
    return Pick(None())
}
`)
	for _, want := range []string{
		"payload must be int, got string",
		"argument 1 to Plain must be int, got Some(1) (an Option value)",
		"argument 1 to Plain must be int, got None (an Option value)",
		"ambiguous call to Store: None could be Option[int] or Option[string]",
		"cannot infer generic arguments for Process",
		"None is not a function and takes no arguments",
	} {
		found := false
		for _, err := range invalid {
			found = found || strings.Contains(err.Error(), want)
		}
		if !found {
			t.Errorf("missing %q in %v", want, invalid)
		}
	}
	if len(invalid) != 6 {
		t.Fatalf("want exactly 6 errors, got %d: %v", len(invalid), invalid)
	}
}
