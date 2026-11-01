package main

import (
	"strings"
	"testing"
)

const stringToolingURI = "file:///tmp/sec-lsp-string-operators/main.sec"

func stringHoverAt(t *testing.T, source, marker string, delta int) string {
	t.Helper()
	hover, ok := hoverForSource(stringToolingURI, source, offsetPosition(source, strings.Index(source, marker)+delta))
	if !ok {
		t.Fatalf("no hover at %q", marker)
	}
	return hover.Contents.Value
}

// Hover separates panic mode, try mode, the protected operand and the invalid
// unhandled operand, from Sema's plan.
// Rules: rules/foundations/operators.md (MD-039); correction MD-004-MD-039 § 6(6).
func TestStringOperatorHoverModes(t *testing.T) {
	source := `module main

fn Ordinary(name: string) string {
    return "a" + name + "b"
}

fn Protected(name: string) Result[string, StringError] {
    return Ok(try "c" + name)
}

fn Folded() string {
    return "d" + "e"
}

fn Append(name: string, count: int) Result[string, StringError] {
    let mut text := "x"
    text += name
    try text += name
    try text += count.ToString()
    return Ok(text)
}
`
	for _, item := range []struct {
		marker string
		delta  int
		want   string
	}{
		{`"a" + name`, 4, "**panics** with `StringMaterializationFailure`"},
		{`name + "b"`, 5, "**panics**"},
		{`"c" + name`, 4, "reports a failed materialization as `StringError.Allocation`"},
		{`"d" + "e"`, 4, "Compile-time text"},
		{"text += name\n    try", 5, "**panics**"},
		{"try text += name", 9, "`StringError.Allocation`"},
		{"try text += count", 9, "operand and the materialization as one"},
	} {
		if got := stringHoverAt(t, source, item.marker, item.delta); !strings.Contains(got, item.want) {
			t.Errorf("%s: want %q, got %s", item.marker, item.want, got)
		}
	}
}

func stringActionTitles(t *testing.T, source string, start, end position) []string {
	t.Helper()
	reported := analyze(stringToolingURI, source)
	titles := []string{}
	for _, action := range stringTryCodeActions(stringToolingURI, source, lspRange{Start: start, End: end}, reported, sourceOverlay{}) {
		titles = append(titles, action.Kind+": "+action.Title)
	}
	return titles
}

// A fix is offered only when the edited source proves it, so a function that
// cannot carry StringError gets none.
func TestStringTryCodeActions(t *testing.T) {
	far := position{Line: 0, Character: 0}
	operand := "module main\nfn A(c: int) Result[string, StringError] {\n    let mut t := \"x\"\n    t += c.ToString()\n    return Ok(t)\n}\n"
	titles := strings.Join(stringActionTitles(t, operand, far, far), "\n")
	for _, want := range []string{"Write try before the assignment", "Write try before the operand: the append may still panic"} {
		if !strings.Contains(titles, want) {
			t.Errorf("want %q in:\n%s", want, titles)
		}
	}
	concat := "module main\nfn A(c: int) Result[string, StringError] {\n    return Ok(\"x\" + c.ToString())\n}\n"
	if titles := strings.Join(stringActionTitles(t, concat, far, far), "\n"); !strings.Contains(titles, "Write try before the expression") {
		t.Errorf("concat: %s", titles)
	}
	noChannel := "module main\nfn A(c: int) string {\n    return \"x\" + c.ToString()\n}\n"
	if titles := stringActionTitles(t, noChannel, far, far); len(titles) != 0 {
		t.Errorf("unverifiable fix offered: %v", titles)
	}
	ordinary := "module main\nfn A(n: string) Result[string, StringError] {\n    let mut t := \"x\" + n\n    t += n\n    return Ok(t)\n}\n"
	plus := offsetPosition(ordinary, strings.Index(ordinary, "+ n"))
	if titles := strings.Join(stringActionTitles(t, ordinary, plus, plus), "\n"); !strings.Contains(titles, "refactor.rewrite: Write try: report a failed materialization") {
		t.Errorf("+ refactor: %s", titles)
	}
	target := offsetPosition(ordinary, strings.Index(ordinary, "t += n"))
	if titles := strings.Join(stringActionTitles(t, ordinary, target, target), "\n"); !strings.Contains(titles, "refactor.rewrite: Write try: report a failed append") {
		t.Errorf("+= refactor: %s", titles)
	}
	if titles := stringActionTitles(t, ordinary, far, far); len(titles) != 0 {
		t.Errorf("refactor outside its range: %v", titles)
	}
	panicking := "module main\nfn A(n: string) string {\n    return \"x\" + n\n}\n"
	plus = offsetPosition(panicking, strings.Index(panicking, "+ n"))
	if titles := stringActionTitles(t, panicking, plus, plus); len(titles) != 0 {
		t.Errorf("refactor that adds an error: %v", titles)
	}
}
