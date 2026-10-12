package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestBorrowConflictNavigation keeps original creation and the current holder
// separate after a move and identifies the back-edge origin in source loops.
// Rules: rules/memory/borrowing.md — §§8.2(2),32(4,8),33(3).
func TestBorrowConflictNavigation(t *testing.T) {
	const file = "../../testdata/tooling/borrow_holder_transfer_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	errors := NewAnalyzer().Analyze(program)
	if len(errors) != 2 {
		t.Fatal(errors)
	}
	for i, want := range []struct {
		use, origin, holder int
		label, name         string
	}{
		{6, 4, 5, "borrow origin", "second"},
		{14, 15, 11, "borrow origin from a previous loop iteration", "holder"},
	} {
		got := errors[i]
		links := got.RelatedLocations()
		if got.Line != want.use || len(links) != 2 || links[0].Line != want.origin || links[0].Label != want.label || links[1].Line != want.holder || links[1].Label != "borrow holder "+want.name+" keeps authority live" {
			t.Fatalf("navigation: %+v; %+v", got, links)
		}
		for _, link := range links {
			if link.File != file {
				t.Fatal(link)
			}
		}
	}
}

// TestBorrowConflictUnknownHolder never fabricates a source point for an
// internal temporary; the available origin remains navigable.
// Rules: rules/memory/borrowing.md — §§32(8),33(3).
func TestBorrowConflictUnknownHolder(t *testing.T) {
	a := NewAnalyzer()
	record := borrowRecord{Holder: "$temporary", Token: lexer.Token{File: "borrow.sec", Line: 2, Column: 3}, LoopCarried: true}
	a.addBorrowAuthorityConflict(Place{Root: "value"}, lexer.Token{File: "use.sec", Line: 4, Column: 5}, record, "conflict")
	links := a.errors[0].RelatedLocations()
	if len(links) != 1 || links[0].File != "borrow.sec" || !strings.Contains(links[0].Label, "previous loop iteration") {
		t.Fatal(links)
	}
}
