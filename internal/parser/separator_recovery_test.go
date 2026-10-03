package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

func missingSeparatorRepairs(result ParseResult) []RecoveryEvent {
	repairs := []RecoveryEvent{}
	for _, event := range result.Recovery {
		if event.Kind == RecoveryInsertMissingToken && event.After.Type != "" {
			repairs = append(repairs, event)
		}
	}
	return repairs
}

func TestMissingCommaBetweenLineSeparatedItemsIsRepairedWithoutCascade(t *testing.T) {
	source := `module main

type Point struct {
    x: int // first
    y: int
    z: int,
}

fn Sum(
    a: int
    b: int,
) int {
    return a + b
}

fn main() void {
    let total := Sum(
        1
        2,
    )
    let values := [
        1
        2,
    ]
}
`
	result := New(lexer.New(source)).Parse()
	if len(result.Diagnostics) != 5 {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.ID != diagnostics.ParserMissingToken || diagnostic.Help == "" {
			t.Fatalf("diagnostic = %#v", diagnostic)
		}
	}
	repairs := missingSeparatorRepairs(result)
	wantAfter := []string{"int", "int", "int", "1", "1"}
	wantLine := []int{4, 5, 10, 18, 22}
	if len(repairs) != len(wantAfter) {
		t.Fatalf("repairs = %#v", repairs)
	}
	for index, repair := range repairs {
		if repair.After.Lexeme != wantAfter[index] || repair.After.Line != wantLine[index] {
			t.Fatalf("repair %d after %q at line %d", index, repair.After.Lexeme, repair.After.Line)
		}
	}
	var point *ast.TypeDeclStatement
	for _, statement := range result.Program.Statements {
		if declaration, ok := statement.(*ast.TypeDeclStatement); ok && declaration.Name.Value == "Point" {
			point = declaration
		}
	}
	if point == nil || point.StructType == nil || len(point.StructType.Fields) != 3 {
		t.Fatalf("Point fields were not retained: %#v", point)
	}
}

func TestMissingCommaOnSameLineIsNotRepaired(t *testing.T) {
	result := New(lexer.New("module main\n\ntype Pair struct {\n    left: int /* gap */ right: int\n}\n")).Parse()
	if len(result.Diagnostics) == 0 {
		t.Fatal("same-line adjacency must stay invalid")
	}
	if repairs := missingSeparatorRepairs(result); len(repairs) != 0 {
		t.Fatalf("same-line adjacency must not be repaired: %#v", repairs)
	}
}
