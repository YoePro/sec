package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestOwnershipCallMentor separates missing explicit consuming markers from
// non-copyable ordinary arguments, including owning payload and Result contexts.
// Invalid construction must not make the still-owned source unavailable.
// Rules: rules/memory/copy_move.md — §§8.1–8.3,9.2,10.1–10.4,12,28.1–28.2.
func TestOwnershipCallMentor(t *testing.T) {
	for _, test := range []struct {
		file  string
		count int
	}{
		{"ownership_call_mentor_invalid.sec", 10}, {"ownership_call_mentor_valid.sec", 0},
	} {
		data, err := os.ReadFile("../../testdata/tooling/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), test.file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		errors := NewAnalyzer().Analyze(program)
		if len(errors) != test.count {
			t.Fatalf("%s: %+v", test.file, errors)
		}
		for i, got := range errors {
			if got.ID != diagnostics.ImplicitMoveDisallowed {
				t.Fatal(got)
			}
			links := got.RelatedLocations()
			if len(links) == 0 || links[len(links)-1].Label != "source ownership established here" {
				t.Fatal(got)
			}
			for _, link := range links {
				if link.File != test.file {
					t.Fatal(link)
				}
			}
			switch i {
			case 0, 1:
				if len(links) != 2 || links[0].Label != "parameter ownership contract" || !strings.Contains(got.Help, "takes ownership") || !strings.Contains(got.Help, "Write `<-") {
					t.Fatal(got)
				}
			case 2:
				if !strings.Contains(got.Message, "ordinary by-value") || !strings.Contains(got.Message, "copies its argument") || !strings.Contains(got.Help, "ref mut") || !strings.Contains(got.Help, "slice/view") || !strings.Contains(got.Help, "declare `->") || strings.Contains(got.Help, "Write `<-") {
					t.Fatal(got)
				}
			default:
				if !strings.Contains(got.Help, "owning field or payload") || !strings.Contains(got.Help, "Write `<-") || !strings.Contains(got.Help, "construction commits") {
					t.Fatal(got)
				}
			}
		}
	}
}

// TestBorrowOverlapMentor explains same/whole/field/dynamic and call-bounded
// overlap from Sema's canonical Place relationship, including deferred retention.
// Rules: rules/memory/borrowing.md — §§12–14,15.4,21,32(2–8),33(1).
func TestBorrowOverlapMentor(t *testing.T) {
	const file = "borrow_overlap_mentor_invalid.sec"
	data, err := os.ReadFile("../../testdata/tooling/" + file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	errors := NewAnalyzer().Analyze(program)
	if len(errors) != 5 {
		t.Fatal(errors)
	}
	for i, want := range []struct {
		line int
		text string
	}{
		{6, "contains"}, {12, "contains"}, {18, "may overlap"}, {25, "same storage"}, {30, "defer still needs"},
	} {
		got := errors[i]
		if got.Line != want.line || !strings.Contains(got.Help, want.text) || len(got.RelatedLocations()) == 0 {
			t.Fatal(got)
		}
		if i == 3 && got.RelatedLocationLabel() != "borrow prepared for argument 1" {
			t.Fatal(got)
		}
		if i == 4 && got.RelatedLocationLabel() != "defer retains this storage" {
			t.Fatal(got)
		}
	}
}

// TestBorrowAlternativeOverlapExplanation never describes an uncertain origin
// set as one proven storage location, including conservative legacy records.
// Rules: rules/memory/borrowing.md — §12(6–8),§32(2).
func TestBorrowAlternativeOverlapExplanation(t *testing.T) {
	place := Place{Root: "left", AlternativeOrigins: []Place{{Root: "right"}}}
	text := borrowOverlapExplanation(place, borrowRecord{Place: Place{Root: "left"}, Kind: mutableBorrow})
	if !strings.Contains(text, "possible Places {left, right}") || !strings.Contains(text, "may overlap") || strings.Contains(text, "same storage") {
		t.Fatal(text)
	}
	text = borrowOverlapExplanation(Place{Root: "value"}, borrowRecord{Root: "value", Kind: sharedBorrow})
	if !strings.Contains(text, "may overlap") || strings.Contains(text, "same storage") {
		t.Fatal(text)
	}
}
