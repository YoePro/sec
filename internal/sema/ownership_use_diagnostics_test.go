package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestOwnershipUseOriginMentor checks actual ownership actions, the initial
// owner and repair guidance, retaining the distinction between definite,
// conditional and partial unavailability and Result projection inspection.
// Rules: rules/memory/copy_move.md — §28.1; rules/memory/destruction.md — §33.2(1).
func TestOwnershipUseOriginMentor(t *testing.T) {
	for _, test := range []struct {
		file  string
		count int
	}{
		{"ownership_use_origin_invalid.sec", 9}, {"ownership_use_origin_valid.sec", 0},
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
			t.Fatal(errors)
		}
		for i, want := range []struct {
			use, action, owner int
			id, label          string
		}{
			{11, 10, 9, "S1088", "ownership moved here"},
			{17, 16, 15, "S4001", "ownership discarded here"},
			{22, 21, 20, "S1088", "ownership consumed by a call here"},
			{27, 26, 25, "S1088", "ownership moved here"},
			{33, 31, 30, "S1088", "ownership consumed by Result.Ok() here"},
			{38, 36, 35, "S1088", "ownership consumed by Result.Err() here"},
			{42, 41, 40, "S1089", "ownership action on a possibly unavailable path"},
			{46, 45, 44, "S1090", "ownership moved here"},
			{53, 52, 51, "S1088", "ownership released here"},
		} {
			if test.count == 0 {
				break
			}
			got := errors[i]
			links := got.RelatedLocations()
			if got.Line != want.use || got.ID != want.id || len(links) != 2 || links[0].Line != want.action || links[0].Label != want.label || links[1].Line != want.owner || links[1].Label != "source ownership established here" {
				t.Fatal(got, links)
			}
			if got.Help == "" || !strings.Contains(got.Help, "owned") && !strings.Contains(got.Help, "owns") {
				t.Fatal(got)
			}
			if i == 4 && !strings.Contains(got.Help, "OkRef") || i == 5 && !strings.Contains(got.Help, "ErrRef") || i == 6 && !strings.Contains(got.Help, "not proof that every path") {
				t.Fatal(got)
			}
		}
	}
}

// TestOwnershipUseReturnAndMixedOrigins checks source-mapped provenance across
// files, a represented return action, and conservative witnesses at path joins.
// Missing owner locations do not produce fabricated navigation points.
// Rules: rules/memory/ownership.md — §6; rules/memory/destruction.md — §33.2(1).
func TestOwnershipUseReturnAndMixedOrigins(t *testing.T) {
	for _, reason := range []string{"returned", "moved|discarded"} {
		a := NewAnalyzer()
		a.moveReasons = map[string]string{"value": reason}
		place := Place{Root: "value", RootToken: lexer.Token{File: "owner.sec", Line: 2, Column: 3}}
		a.addUnavailableOwnershipDiagnostic(place, "value", lexer.Token{File: "use.sec", Line: 8, Column: 9}, lexer.Token{File: "transfer.sec", Line: 4, Column: 5}, diagnostics.UseAfterMove, "use the value before that operation", "unavailable value")
		got := a.errors[0]
		links := got.RelatedLocations()
		if len(links) != 2 || links[0].File != "transfer.sec" || links[1].File != "owner.sec" || got.File != "use.sec" {
			t.Fatal(got)
		}
		if reason == "returned" && links[0].Label != "ownership returned here" {
			t.Fatal(got)
		}
		if reason != "returned" && (!strings.Contains(got.Help, "one witness") || links[0].Label != "earlier ownership action on one continuing path") {
			t.Fatal(got)
		}
	}
	a := NewAnalyzer()
	a.moveReasons = map[string]string{"value": "moved"}
	a.addUnavailableOwnershipDiagnostic(Place{Root: "value"}, "value", lexer.Token{Line: 8, Column: 9}, lexer.Token{Line: 4, Column: 5}, diagnostics.UseAfterMove, "use it before the move", "unavailable value")
	if len(a.errors[0].RelatedLocations()) != 1 {
		t.Fatal(a.errors[0])
	}
}
