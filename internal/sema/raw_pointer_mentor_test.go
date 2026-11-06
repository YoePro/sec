package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestRawPointerMentorDiagnostics checks authorization and disproven validity
// separately, with practical help and provenance owned by semantic producers.
// Rules: rules/memory/raw_pointers.md §§5.2,7(8),11–14,18A,34(1–4).
func TestRawPointerMentorDiagnostics(t *testing.T) {
	for _, valid := range []bool{false, true} {
		name := "raw_pointer_mentor_invalid.sec"
		if valid {
			name = "raw_pointer_mentor_valid.sec"
		}
		path := "../../testdata/tooling/" + name
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), path))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		errors := NewAnalyzer().Analyze(program)
		if valid {
			if len(errors) != 0 {
				t.Fatalf("valid operations rejected: %v", errors)
			}
			continue
		}
		counts := map[string]int{}
		for _, e := range errors {
			counts[e.ID]++
			if e.Help == "" || e.File != path || e.Line < 1 || e.Column < 1 {
				t.Fatalf("missing mentor metadata: %+v", e)
			}
			switch e.ID {
			case diagnostics.RawPointerUnsafeRequired:
				if !strings.Contains(e.Message, "requires unsafe") || !strings.Contains(e.Help, "unsafe") {
					t.Fatal(e)
				}
				if strings.Contains(e.Message, "RawPtr.Read requires") && !strings.Contains(e.Help, "initialized") {
					t.Fatalf("lost registry contract: %+v", e)
				}
			case diagnostics.RawPointerInvalidOperation, diagnostics.RawPointerDanglingOrigin:
				if !strings.Contains(e.Help, "Adding unsafe cannot") {
					t.Fatalf("bad repair: %+v", e)
				}
			case diagnostics.RawPointerNullDereference:
				if !strings.Contains(e.Help, "does not make a proven null") {
					t.Fatal(e)
				}
			default:
				t.Fatalf("unexpected category: %+v", e)
			}
			if e.ID == diagnostics.RawPointerNullDereference || e.ID == diagnostics.RawPointerDanglingOrigin {
				related := e.RelatedLocations()
				if len(related) != 1 || related[0].File != path || related[0].Line >= e.Line {
					t.Fatalf("missing causal source link: %+v", e)
				}
			}
		}
		want := map[string]int{diagnostics.RawPointerUnsafeRequired: 11, diagnostics.RawPointerInvalidOperation: 21, diagnostics.RawPointerNullDereference: 6, diagnostics.RawPointerDanglingOrigin: 1}
		for id, n := range want {
			if counts[id] != n {
				t.Fatalf("counts %v; want %v", counts, want)
			}
		}
		if len(errors) != 39 {
			t.Fatalf("unexpected errors: %v", errors)
		}
	}
}
