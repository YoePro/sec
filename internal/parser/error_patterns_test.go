package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// MD-029 ErrPattern forms parse into one structured pattern in match arms and
// try handlers; the braces of a field pattern are not a handler list or arm
// body, and later arms and handlers survive (M029-15).
//
// Rules:
//   - rules/foundations/grammar.md — "Try expression" (ErrPattern)
//   - rules/compiler/parser_recovery.md — "Match recovery", "Error patterns"
func TestErrorPatternForms(t *testing.T) {
	p := New(lexer.New(`module main

fn ViaMatch(flag: bool) int {
    match Open(flag) {
        Ok(value) => return value
        Err(FileError.NotFound { Code: code, Path: ref path }) => return code
        Err(.NotFound {}) => return 1
        Err(.InvalidEncoding(position)) => return 2
        Err(FileError.InvalidEncoding(_)) => return 3
        Err(.PermissionDenied) => return 4
    }
}

fn ViaTry(flag: bool) int {
    return try Open(flag) {
        Err(.NotFound { Code }) => Code
        Err(FileError.InvalidEncoding(ref position)) => 1
        Err(FileError.PermissionDenied) => 2
        Err(other) => 3
    }
}
`))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	var arms []string
	var handlers []string
	for _, statement := range program.Statements {
		function, ok := statement.(*ast.FunctionDeclaration)
		if !ok || function.Body == nil {
			continue
		}
		for _, inner := range function.Body.Statements {
			switch inner := inner.(type) {
			case *ast.MatchStatement:
				for _, arm := range inner.Match.Arms {
					arms = append(arms, arm.Pattern.String())
				}
			case *ast.ReturnStatement:
				try, ok := inner.Value.(*ast.TryExpression)
				if !ok {
					continue
				}
				for _, handler := range try.Handlers {
					if handler.ErrPattern != nil {
						handlers = append(handlers, handler.ErrPattern.String())
					} else {
						handlers = append(handlers, handler.Pattern.String())
					}
				}
			}
		}
	}
	wantArms := []string{
		"Ok(value)",
		"Err(FileError.NotFound { Code: code, Path: ref path })",
		"Err(.NotFound {})",
		"Err(.InvalidEncoding(position))",
		"Err(FileError.InvalidEncoding(_))",
		"Err(.PermissionDenied)",
	}
	wantHandlers := []string{
		"Err(.NotFound { Code })",
		"Err(FileError.InvalidEncoding(ref position))",
		"Err(FileError.PermissionDenied)",
		"Err(other)",
	}
	if len(arms) != len(wantArms) || len(handlers) != len(wantHandlers) {
		t.Fatalf("arms %q handlers %q", arms, handlers)
	}
	for i := range wantArms {
		if arms[i] != wantArms[i] {
			t.Errorf("arm %d = %q, want %q", i, arms[i], wantArms[i])
		}
	}
	for i := range wantHandlers {
		if handlers[i] != wantHandlers[i] {
			t.Errorf("handler %d = %q, want %q", i, handlers[i], wantHandlers[i])
		}
	}
}
