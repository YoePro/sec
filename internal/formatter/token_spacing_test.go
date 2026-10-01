package formatter

import "testing"

func assertFormat(t *testing.T, input, want string) {
	t.Helper()
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong formatting:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("formatting is not idempotent:\n%s", again)
	}
}

// Binary, assignment, and initialization operators get one space on each side,
// unary operators and the consuming marker attach to their operand, a let
// type colon binds to its identifier, control-flow keywords take one space,
// and same-line opening braces follow one space, including "} else {".
//
// Rules:
//   - rules/tooling/formatter.md — §6(4)–(7) "Indentation and basic whitespace"
//   - rules/tooling/formatter.md — §8(1)–(3), §18(1), §18(12) braces and loop forms
//   - rules/tooling/formatter.md — §21(5) consuming call-site marker
func TestFormatCanonicalTokenSpacing(t *testing.T) {
	input := "module main\n\nfn F(items: int[3], x: int, ready: bool) int{\n" +
		"    let  y  :=  x+1\n" +
		"    let mut  z:int := x  *  2\n" +
		"    z+=1\n" +
		"    Use(<-  z)\n" +
		"    let moved :<-  z\n" +
		"    if  x==1{\n        return y\n    }else{\n        return -  y\n    }\n" +
		"    while  x>0   &&!ready  {\n        return y\n    }\n" +
		"    for  item  in  items{\n    }\n" +
		"    for{\n    }\n" +
		"    if x not  in  items {\n    }\n" +
		"    switch  x{\n    }\n" +
		"    return  match  x{\n        _ => 1<<2\n    }\n}\n"
	want := "module main\n\nfn F(items: int[3], x: int, ready: bool) int {\n" +
		"    let y := x + 1\n" +
		"    let mut z: int := x * 2\n" +
		"    z += 1\n" +
		"    Use(<-z)\n" +
		"    let moved :<- z\n" +
		"    if x == 1 {\n        return y\n    } else {\n        return -y\n    }\n" +
		"    while x > 0 && !ready {\n        return y\n    }\n" +
		"    for item in items {\n    }\n" +
		"    for {\n    }\n" +
		"    if x not in items {\n    }\n" +
		"    switch x {\n    }\n" +
		"    return match x {\n        _ => 1 << 2\n    }\n}\n"
	assertFormat(t, input, want)
}

// Spacing is decided only from parser roles and only inside one line: string
// contents, gaps holding a comment, line continuations, and typed declaration
// heads keep their source spelling.
//
// Rules:
//   - rules/tooling/formatter.md — §6(8), §12 "Comments", §29 "Formatter invariants"
func TestFormatTokenSpacingLeavesUnroledAndMultilineGaps(t *testing.T) {
	input := "module main\n\nfn F(a: int, b: int) int {\n" +
		"    let s := \"a+b==c\"\n" +
		"    let c := a /* note */ +b\n" +
		"    let total := a +\n" +
		"    b\n" +
		"    float: low := 1.0\n" +
		"    return a\n}\n"
	assertFormat(t, input, input)
}

// Decorative blank lines directly inside braces are removed, ordinary
// separation between statements remains, and multiple blank lines collapse.
//
// Rules:
//   - rules/tooling/formatter.md — §7(1)–(2) "Vertical spacing"
func TestFormatRemovesBlankLinesInsideBraces(t *testing.T) {
	input := "module main\n\n\n\nfn F() int {\n\n\n    let x := 1\n\n\n\n    if x == 1 {\n\n        return x\n\n    }\n\n    return x\n\n}\n"
	want := "module main\n\nfn F() int {\n    let x := 1\n\n    if x == 1 {\n        return x\n    }\n\n    return x\n}\n"
	assertFormat(t, input, want)
}

// The continuation lines of a multiline raw string are program data: their
// indentation, braces, and blank lines are preserved byte-for-byte and do not
// change the indentation of the surrounding code.
//
// Rules:
//   - rules/tooling/formatter.md — §6(8), §29 "Formatter invariants"
func TestFormatPreservesMultilineRawStrings(t *testing.T) {
	input := "module main\n\nfn F() string {\n        return `{\n\n  }\n}`\n}\n\nfn G() int {\nreturn 1\n}\n"
	want := "module main\n\nfn F() string {\n    return `{\n\n  }\n}`\n}\n\nfn G() int {\n    return 1\n}\n"
	assertFormat(t, input, want)
}
