package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// P17-15 rejections: array element move-out, a partial move from a custom-free
// type, whole use while a field is moved on some path, use after a discard on
// some path, and whole use after reinitializing only one moved field.
// Rules: rules/memory/copy_move.md — §§ 12–15; rules/memory/destruction.md — § 12.
func TestPartialStateRejections(t *testing.T) {
	path := "../../testdata/sema/partial_state_invalid.sec"
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
	var messages []string
	for _, err := range errors {
		messages = append(messages, err.ID+" "+err.Error())
	}
	text := strings.Join(messages, "\n")
	for _, want := range []string{
		"explicit indexed extraction",
		"S1085",
		"sub-place pair.left was moved on one possible execution path",
		"it was discarded on one possible execution path",
		"S1090",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if len(errors) != 5 {
		t.Errorf("got %d errors, want 5:\n%s", len(errors), text)
	}
}
