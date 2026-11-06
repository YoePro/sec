package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// A continuing arm block of a match expression must end with a value
// expression assignable to the result; a void or missing final value, an
// incompatible type and a copied move-only arm local are rejected.
// Rules: rules/control-flow/flowcontrol_match.md — §§ 21–22.
func TestMatchArmBlockValues(t *testing.T) {
	path := "../../testdata/sema/match_arm_blocks_invalid.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), path))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	var messages []string
	for _, err := range NewAnalyzer().Analyze(program) {
		messages = append(messages, err.ID+" "+err.Error())
	}
	text := strings.Join(messages, "\n")
	for _, want := range []string{
		"its final expression is void",
		"end it with a value expression",
		"match arms must produce compatible types, got string and int",
		"S1007",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	valid, err := os.ReadFile("../../testdata/semantic_ir/match_arm_blocks.sec")
	if err != nil {
		t.Fatal(err)
	}
	p = parser.New(lexer.NewWithFile(string(valid), "valid.sec"))
	if errors := NewAnalyzer().Analyze(p.ParseProgram()); len(errors) != 0 {
		t.Fatal(errors)
	}
}
