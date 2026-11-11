package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// textDocument/implementation navigates from an interface to its explicitly
// conforming types and from an interface method to each implementing method,
// at the declaration and at a use, from Sema conformance facts; other
// symbols have no implementation result.
// Rules: rules/tooling/lsp.md — § 14(1); rules/declarations/interfaces.md — §§ 4–6.
func TestImplementationNavigation(t *testing.T) {
	text := `module main

interface Figure {
    fn Area() int
}

type Square struct {
    side: int,
}

impl Square implements Figure {
    fn Area() int {
        return self.side * self.side
    }
}

type Line struct {
    length: int,
}

impl Line implements Figure {
    fn Area() int {
        return 0
    }
}

fn Measure[T: Figure](figure: T) int {
    return figure.Area()
}

fn Main() int {
    return Measure(Square { side: 2 })
}
`
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	at := func(marker string, skip int) []location {
		idx := strings.Index(text, marker)
		if idx < 0 {
			t.Fatalf("marker %q missing", marker)
		}
		return implementationsForSource(uri, text, offsetPosition(text, idx+skip))
	}
	lines := func(locations []location) []int {
		out := []int{}
		for _, location := range locations {
			out = append(out, location.Range.Start.Line+1)
		}
		return out
	}
	if got := lines(at("interface Figure", 10)); len(got) != 2 || got[0] != 7 || got[1] != 17 {
		t.Errorf("interface implementations = %v, want lines 7 and 17", got)
	}
	if got := lines(at("fn Area() int\n}", 3)); len(got) != 2 || got[0] != 12 || got[1] != 22 {
		t.Errorf("method implementations = %v, want lines 12 and 22", got)
	}
	if got := lines(at("T: Figure", 3)); len(got) != 2 {
		t.Errorf("implementations at an interface use = %v", got)
	}
	if got := at("type Square", 5); len(got) != 0 {
		t.Errorf("non-interface implementations = %v", got)
	}
}
