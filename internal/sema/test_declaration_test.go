package sema

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestTestDeclarationIdentityValidation verifies the frontend-only identity
// rules that do not require a TestCompilationPlan or executable test harness.
//
// Rules:
//   - rules/tooling/testing.md — §4.2 "Test declaration location"
//   - rules/tooling/testing.md — §5.4 "Name requirement"
//   - rules/tooling/testing.md — §6.2 "Unique names"
func TestTestDeclarationIdentityValidation(t *testing.T) {
	tests := []struct {
		fixture string
		wantID  string
	}{
		{"test_declaration_outside_test_file_invalid.sec", diagnostics.TestDeclarationOutsideTestFile},
		{"empty_test_name_invalid_test.sec", diagnostics.EmptyTestName},
		{"duplicate_test_name_invalid_test.sec", diagnostics.DuplicateTestIdentity},
		{"test_return_value_invalid_test.sec", diagnostics.TestReturnValue},
	}
	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			path := "../../testdata/sema/" + test.fixture
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			result := parser.New(lexer.NewWithFile(string(source), path)).Parse()
			if result.HasErrors {
				t.Fatalf("parser diagnostics = %+v", result.Diagnostics)
			}
			errors := NewAnalyzer().Analyze(result.Program)
			if len(errors) != 1 || errors[0].ID != test.wantID {
				t.Fatalf("errors = %+v, want one %s diagnostic", errors, test.wantID)
			}
			if test.wantID == diagnostics.DuplicateTestIdentity && errors[0].PreviousLine == 0 {
				t.Fatalf("duplicate diagnostic has no previous declaration: %+v", errors[0])
			}
		})
	}
}

// TestTestingOperationsResolveInTestContext verifies the implemented compiler-
// known namespace signatures and retained Sema operation facts.
//
// Rules:
//   - rules/tooling/testing.md — §11 "Compiler-known testing namespace"
//   - rules/tooling/testing.md — §§15–17 Log, Expect, and Require
func TestTestingOperationsResolveInTestContext(t *testing.T) {
	path := "../../testdata/sema/testing_expect_require_valid_test.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := parser.New(lexer.NewWithFile(string(source), path)).Parse()
	if result.HasErrors {
		t.Fatalf("parser diagnostics = %+v", result.Diagnostics)
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("testing operation errors = %+v", errors)
	}
	declaration := result.Program.Statements[1].(*ast.TestDeclaration)
	wants := []struct {
		kind         TestingOperationKind
		hasCondition bool
		hasMessage   bool
	}{
		{TestingOperationExpect, true, false},
		{TestingOperationExpect, true, true},
		{TestingOperationRequire, true, false},
		{TestingOperationRequire, true, true},
		{TestingOperationLog, false, true},
	}
	for index, want := range wants {
		statement := declaration.Body.Statements[index].(*ast.ExpressionStatement)
		call := statement.Expression.(*ast.CallExpression)
		operation, ok := analyzer.ResolvedTestingOperationOf(call)
		if !ok || operation.Kind != want.kind || operation.Test != declaration {
			t.Errorf("operation %d = %+v, found=%v, want %s in owning test", index, operation, ok, want.kind)
		}
		if (operation.Condition != nil) != want.hasCondition || (operation.Message != nil) != want.hasMessage {
			t.Errorf("operation %d operands = condition:%#v message:%#v", index, operation.Condition, operation.Message)
		}
	}
}

// TestTestingOperationDiagnostics verifies test-context availability and the
// implemented operation arity and operand-type diagnostics.
//
// Rules:
//   - rules/tooling/testing.md — §§11.3 and 15.1–17.2
//   - rules/tooling/diagnostics.txt — "Source-testing diagnostics"
func TestTestingOperationDiagnostics(t *testing.T) {
	t.Run("outside test context", func(t *testing.T) {
		path := "../../testdata/sema/testing_outside_context_invalid.sec"
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		result := parser.New(lexer.NewWithFile(string(source), path)).Parse()
		errors := NewAnalyzer().Analyze(result.Program)
		if len(errors) != 1 || errors[0].ID != diagnostics.TestingOutsideTestContext {
			t.Fatalf("outside-context errors = %+v", errors)
		}
	})

	t.Run("invalid arguments", func(t *testing.T) {
		path := "../../testdata/sema/testing_expect_require_invalid_test.sec"
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		result := parser.New(lexer.NewWithFile(string(source), path)).Parse()
		errors := NewAnalyzer().Analyze(result.Program)
		want := []string{
			diagnostics.InvalidTestingExpectArguments,
			diagnostics.InvalidTestingExpectArguments,
			diagnostics.InvalidTestingRequireArguments,
			diagnostics.InvalidTestingLogArguments,
			diagnostics.InvalidTestingLogArguments,
		}
		if len(errors) != len(want) {
			t.Fatalf("invalid-argument errors = %+v", errors)
		}
		for index, id := range want {
			if errors[index].ID != id {
				t.Errorf("error %d ID = %s, want %s: %+v", index, errors[index].ID, id, errors[index])
			}
		}
	})
}

// TestDistinctTestNamesAreAccepted verifies that valid test identities and
// ordinary bodies are analyzed in isolated scopes without registering tests as
// ordinary functions or producing module-scope errors.
//
// Rules:
//   - rules/tooling/testing.md — §5.7 "Not callable as an ordinary function"
//   - rules/tooling/testing.md — §6.2 "Unique names"
//   - rules/tooling/testing.md — §§9.1–9.3 body semantics and bare return
func TestDistinctTestNamesAreAccepted(t *testing.T) {
	fixtures := []string{
		"test_identity_valid_test.sec",
		"same_test_name_different_modules_valid_test.sec",
	}
	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			path := "../../testdata/sema/" + fixture
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			result := parser.New(lexer.NewWithFile(string(source), path)).Parse()
			if result.HasErrors {
				t.Fatalf("parser diagnostics = %+v", result.Diagnostics)
			}
			analyzer := NewAnalyzer()
			if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
				t.Fatalf("valid test identity errors = %+v", errors)
			}
			if _, registered := analyzer.Functions()["first"]; registered {
				t.Fatal("test identity was registered as an ordinary function")
			}
		})
	}
}

// TestOrdinaryTestBodySemanticsAreAnalyzed verifies that a test body uses the
// normal statement/type analyzer rather than only test-specific validation.
//
// Rules:
//   - rules/tooling/testing.md — §9.1 "Ordinary body semantics"
func TestOrdinaryTestBodySemanticsAreAnalyzed(t *testing.T) {
	path := "../../testdata/sema/test_body_semantics_invalid_test.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := parser.New(lexer.NewWithFile(string(source), path)).Parse()
	if result.HasErrors {
		t.Fatalf("parser diagnostics = %+v", result.Diagnostics)
	}
	errors := NewAnalyzer().Analyze(result.Program)
	if len(errors) != 1 || errors[0].Message != "assert condition must be bool, got int" {
		t.Fatalf("ordinary body errors = %+v", errors)
	}
}
