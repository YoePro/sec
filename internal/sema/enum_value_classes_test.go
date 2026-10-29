package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestEnumValueClassesAreOrderedAndDetached uses canonical enum alias identity,
// preserves distinct underlying families, and prevents mutable fact leakage.
// Rules: rules/declarations/enums.md — §7; rules/compiler/compiler_analysis.md — immutable facts.
func TestEnumValueClassesAreOrderedAndDetached(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/enum_value_classes.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	typ := a.Types()["Numbers"]
	classes := EnumValueClasses(typ)
	if len(classes) != 2 || !reflect.DeepEqual(classes[0].Members, []string{"One", "Alias"}) || classes[0].IntegerValue.String() != "1" || classes[1].IntegerValue.String() != "2" {
		t.Fatal(classes)
	}
	classes[0].Members[0] = "Changed"
	classes[0].IntegerValue.SetInt64(99)
	next := EnumValueClasses(typ)
	if next[0].Members[0] != "One" || next[0].IntegerValue.String() != "1" {
		t.Fatal("consumer changed canonical enum facts", next)
	}
	textType := a.Types()["Texts"]
	texts := EnumValueClasses(textType)
	if len(texts) != 2 || !reflect.DeepEqual(texts[0].Members, []string{"One", "Alias"}) || texts[0].StringValue == nil || *texts[0].StringValue != "one" || texts[1].StringValue == nil || *texts[1].StringValue != "" {
		t.Fatal(texts)
	}
	*texts[0].StringValue = "Changed"
	if next := EnumValueClasses(textType); *next[0].StringValue != "one" {
		t.Fatal("consumer changed canonical string class", next)
	}
}
