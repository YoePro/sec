package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// A try handler may carry a where guard, and a handler block may end with a
// contextual recovery-value expression; such an expression is accepted only as
// the block's final statement and not in nested blocks.
//
// Rules:
//   - rules/errors/errorhandling.md — §19 "Guards", §21.1 "Block handler result position"
func TestTryHandlerGuardsAndFinalBlockValues(t *testing.T) {
	input := "module main\nfn F() int {\n    return try Read() {\n        Err(e) where e == IOError.Busy => 1\n        Err(_) => {\n            Log(\"x\")\n            5\n        }\n    }\n}\n"
	result := New(lexer.New(input)).Parse()
	if result.HasErrors {
		t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
	}
	ret := result.Program.Statements[1].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement)
	try := ret.Value.(*ast.TryExpression)
	if len(try.Handlers) != 2 {
		t.Fatalf("handlers = %+v", try.Handlers)
	}
	guarded := try.Handlers[0]
	if guarded.Guard == nil || guarded.GuardToken.Type != lexer.WHERE || guarded.Guard.String() != "(e == IOError.Busy)" {
		t.Fatalf("guard = %#v", guarded.Guard)
	}
	block := try.Handlers[1].BlockBody
	if block == nil || len(block.Statements) != 2 {
		t.Fatalf("handler block = %+v", block)
	}
	final, ok := block.Statements[1].(*ast.ExpressionStatement)
	if !ok || final.Expression.String() != "5" {
		t.Fatalf("final value = %#v", block.Statements[1])
	}

	nonFinal := "module main\nfn F() int {\n    return try Read() {\n        Err(_) => {\n            5\n            Log(\"x\")\n        }\n    }\n}\n"
	result = New(lexer.New(nonFinal)).Parse()
	if !result.HasErrors || len(result.Diagnostics) == 0 || !strings.Contains(result.Diagnostics[0].Message, "unexpected token \"5\"") {
		t.Fatalf("non-final literal statement must stay invalid: %+v", result.Diagnostics)
	}
	nested := "module main\nfn F() void {\n    try Clear() {\n        Err(_) => {\n            if true {\n                5\n            }\n        }\n    }\n}\n"
	result = New(lexer.New(nested)).Parse()
	if !result.HasErrors {
		t.Fatal("a literal ending a nested block inside a handler must stay invalid")
	}
}

// Fallible setters and fallible setter requirements carry their explicit error
// type after the value parameter.
//
// Rules:
//   - rules/errors/errorhandling.md — §24 "Fallible property setters", §24.1 "Interfaces"
func TestParseFallibleSetterErrorContracts(t *testing.T) {
	input := "module main\ninterface Tunable {\n    property Speed: int {\n        try set value SpeedError\n    }\n}\nimpl Car {\n    property Speed: int {\n        try set value SpeedError {\n            _speed = value\n        }\n    }\n}\n"
	result := New(lexer.New(input)).Parse()
	if result.HasErrors {
		t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
	}
	requirement := result.Program.Statements[1].(*ast.InterfaceDeclaration).Properties[0]
	if !requirement.SetterFallible || requirement.SetterErrorType == nil || requirement.SetterErrorType.Name != "SpeedError" {
		t.Fatalf("requirement = %+v", requirement)
	}
	impl := result.Program.Statements[2].(*ast.ImplStatement)
	setter := impl.Members[0].(*ast.PropertyDeclaration).Setter
	if setter == nil || !setter.Fallible || setter.ErrorType == nil || setter.ErrorType.Name != "SpeedError" || setter.Body == nil {
		t.Fatalf("setter = %+v", setter)
	}
}
