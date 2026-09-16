package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

func TestMissingParameterTypesRetainParametersAndDeclarations(t *testing.T) {
	input, err := os.ReadFile("../../testdata/parser/missing_parameter_types_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	p := New(lexer.New(string(input)))
	result := p.Parse()
	if !result.HasErrors || len(result.Diagnostics) != 3 {
		t.Fatalf("wrong diagnostics: %#v", result.Diagnostics)
	}
	for index, diagnostic := range result.Diagnostics {
		if diagnostic.ID != diagnostics.ParserInvalidTypeReference {
			t.Fatalf("diagnostic %d ID = %q, want %q", index, diagnostic.ID, diagnostics.ParserInvalidTypeReference)
		}
	}

	if len(result.Program.Statements) != 4 {
		t.Fatalf("wrong declaration count. got=%d want=4", len(result.Program.Statements))
	}

	missing, ok := result.Program.Statements[0].(*ast.FunctionDeclaration)
	if !ok || missing.Name == nil || missing.Name.Value != "MissingFunction" {
		t.Fatalf("missing-type function was not retained: %#v", result.Program.Statements[0])
	}
	assertRecoveredMissingParameterType(t, missing.Parameters, "left", "right")
	if missing.Body == nil || len(missing.Body.Statements) != 1 {
		t.Fatalf("missing-type function body was not retained: %#v", missing)
	}

	container, ok := result.Program.Statements[1].(*ast.FunctionDeclaration)
	if !ok || container.Body == nil || len(container.Body.Statements) != 2 {
		t.Fatalf("lambda container was not retained: %#v", result.Program.Statements[1])
	}
	declaration, ok := container.Body.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("lambda declaration was not retained: %#v", container.Body.Statements[0])
	}
	lambda, ok := declaration.Value.(*ast.LambdaExpression)
	if !ok || lambda.Body == nil {
		t.Fatalf("lambda was not retained: %#v", declaration.Value)
	}
	assertRecoveredMissingParameterType(t, lambda.Parameters, "value", "fallback")

	missingFinal, ok := result.Program.Statements[2].(*ast.FunctionDeclaration)
	if !ok || missingFinal.Name == nil || missingFinal.Name.Value != "MissingFinal" || len(missingFinal.Parameters) != 1 {
		t.Fatalf("final missing-type parameter function was not retained: %#v", result.Program.Statements[2])
	}
	finalParameter := missingFinal.Parameters[0]
	if finalParameter.Type == nil || !finalParameter.Type.Invalid || finalParameter.Type.Recovery == nil {
		t.Fatalf("missing final parameter type was not retained: %#v", finalParameter)
	}

	following, ok := result.Program.Statements[3].(*ast.FunctionDeclaration)
	if !ok || following.Name == nil || following.Name.Value != "Following" {
		t.Fatalf("following declaration was not retained: %#v", result.Program.Statements[3])
	}
}

func assertRecoveredMissingParameterType(t *testing.T, parameters []*ast.Parameter, missingName, followingName string) {
	t.Helper()
	if len(parameters) != 2 {
		t.Fatalf("wrong parameter count. got=%d want=2", len(parameters))
	}
	missing := parameters[0]
	if missing.Name == nil || missing.Name.Value != missingName || missing.Type == nil || !missing.Type.Invalid || missing.Type.Recovery == nil {
		t.Fatalf("missing parameter type was not retained: %#v", missing)
	}
	if missing.Type.Recovery.DiagnosticID != diagnostics.ParserInvalidTypeReference {
		t.Fatalf("wrong recovery diagnostic: %#v", missing.Type.Recovery)
	}
	following := parameters[1]
	if following.Name == nil || following.Name.Value != followingName || following.Type == nil || following.Type.Name != "int" || following.Type.Invalid {
		t.Fatalf("following parameter was not retained: %#v", following)
	}
}
