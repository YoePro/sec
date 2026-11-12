package main

import (
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// The editor consumes the compiler's unsafe-operation registry: hover
// explains the operation and its caller obligations, a missing context gets
// an exact add-unsafe quick fix that the reanalysis confirms, unnecessary
// unsafe is published as Information with a remove fix, and a call that
// stays invalid is never offered the wrap.
// Rules: rules/memory/unsafe.md — §§ 47(4), 50(5), 52(1)–(5).
func TestUnsafeHoverAndCodeActions(t *testing.T) {
	text := `module main

unsafe fn Danger(value: int) int {
    return value
}

fn Safe(value: int) int {
    return value
}

fn Missing() int {
    return Danger(1)
}

fn Extra() int {
    return unsafe Safe(2)
}

fn Reader(pointer: RawPtr[int]) int {
    return unsafe pointer.Read()
}
`
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	published := analyze(uri, text)
	var missing, unnecessary *diagnostic
	for index := range published {
		switch published[index].Code {
		case diagnostics.UnsafeCallRequiresContext:
			missing = &published[index]
		case diagnostics.UnsafeUnnecessary:
			unnecessary = &published[index]
		}
	}
	if missing == nil || unnecessary == nil || unnecessary.Severity != 3 {
		t.Fatalf("diagnostics = %+v", published)
	}
	for _, item := range []*diagnostic{missing, unnecessary} {
		actions := unsafeCodeActions(uri, text, []diagnostic{*item}, sourceOverlay{})
		if len(actions) != 1 {
			t.Fatalf("%s actions = %+v", item.Code, actions)
		}
		edit := actions[0].Edit.Changes[uri][0]
		start := lineCharToOffset(text, edit.Range.Start.Line, edit.Range.Start.Character)
		end := lineCharToOffset(text, edit.Range.End.Line, edit.Range.End.Character)
		edited := text[:start] + edit.NewText + text[end:]
		switch item.Code {
		case diagnostics.UnsafeCallRequiresContext:
			if !strings.Contains(edited, "return unsafe Danger(1)") {
				t.Errorf("wrap edit:\n%s", edited)
			}
		default:
			if !strings.Contains(edited, "return Safe(2)") || strings.Contains(edited, "unsafe Safe") {
				t.Errorf("remove edit:\n%s", edited)
			}
		}
	}
	index := strings.Index(text, "Read()")
	hover, _ := hoverForSource(uri, text, offsetPosition(text, index))
	for _, want := range []string{"**Unsafe operation** (`raw-pointer-access`)", "`RawPtr.Read` reads or writes storage through a raw pointer; authorized by an unsafe context", "caller obligations:", "it does not make a compiler-proven invalid operation valid"} {
		if !strings.Contains(hover.Contents.Value, want) {
			t.Errorf("hover lacks %q:\n%s", want, hover.Contents.Value)
		}
	}
}
