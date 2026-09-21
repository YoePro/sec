package sema

import (
	"os"
	"strings"
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

// TestResolvedTestMetadataRetainsStructuredIdentity verifies that frontend
// test identity and source location are explicit facts rather than generated
// callable or linker names.
//
// Rules:
//   - rules/tooling/testing.md — § 6 "Test identity"
//   - rules/tooling/testing.md — § 33.1 "Explicit semantic identity"
func TestResolvedTestMetadataRetainsStructuredIdentity(t *testing.T) {
	const path = "metadata_test.sec"
	source := `module test_metadata
test "first case" {}
test "second case" {}
`
	result := parser.New(lexer.NewWithFile(source, path)).Parse()
	if result.HasErrors {
		t.Fatalf("parser diagnostics = %+v", result.Diagnostics)
	}
	analyzer := NewAnalyzer()
	assertSemaErrors(t, analyzer.Analyze(result.Program), nil)

	resolved := analyzer.ResolvedTests()
	if len(resolved) != 2 {
		t.Fatalf("resolved tests = %+v, want two", resolved)
	}
	first := resolved[0]
	if first.Identity.Module != "test_metadata" || len(first.Identity.Path) != 1 || first.Identity.Path[0] != "first case" {
		t.Fatalf("first identity = %+v", first.Identity)
	}
	if first.Name != "first case" || first.Source.File != path || first.Source.Line != 2 || first.Source.Column == 0 {
		t.Fatalf("first metadata = %+v", first)
	}
	declaration := result.Program.Statements[1].(*ast.TestDeclaration)
	byDeclaration, ok := analyzer.ResolvedTestMetadataOf(declaration)
	if !ok || byDeclaration.Identity.Module != first.Identity.Module || byDeclaration.Name != first.Name {
		t.Fatalf("metadata by declaration = %+v, %v", byDeclaration, ok)
	}

	resolved[0].Identity.Path[0] = "mutated"
	again := analyzer.ResolvedTests()
	if again[0].Identity.Path[0] != "first case" {
		t.Fatalf("resolved test identity leaked mutable path storage: %+v", again[0].Identity)
	}
}

// TestTestingEqualityOperations validates the frontend signatures and retained
// expected/actual operands without assuming a test runner or lowering exists.
// Rules: rules/tooling/testing.md — §§11.4 and 18 "Equality expectations".
func TestTestingEqualityOperations(t *testing.T) {
	path := "testing_equality_valid_test.sec"
	source := `module testing_equality
test "equality" {
	testing.ExpectEqual(1, 1)
	testing.ExpectEqual(true == true, true, "comparison result")
	testing.RequireEqual("expected", "actual")
	testing.RequireEqual(3, 3, "context")
}`
	result := parser.New(lexer.NewWithFile(source, path)).Parse()
	if result.HasErrors {
		t.Fatalf("parser diagnostics = %+v", result.Diagnostics)
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("testing equality errors = %+v", errors)
	}
	declaration := result.Program.Statements[1].(*ast.TestDeclaration)
	wants := []TestingOperationKind{
		TestingOperationExpectEqual, TestingOperationExpectEqual,
		TestingOperationRequireEqual, TestingOperationRequireEqual,
	}
	for index, want := range wants {
		call := declaration.Body.Statements[index].(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
		operation, ok := analyzer.ResolvedTestingOperationOf(call)
		if !ok || operation.Kind != want || operation.Test != declaration ||
			operation.Expected != call.Arguments[0] || operation.Actual != call.Arguments[1] {
			t.Errorf("operation %d = %+v, found=%v, want %s and original operands", index, operation, ok, want)
		}
		if (operation.Message != nil) != (len(call.Arguments) == 3) {
			t.Errorf("operation %d message = %#v", index, operation.Message)
		}
	}
}

func TestTestingEqualityOperationDiagnostics(t *testing.T) {
	tests := []struct {
		name, call, wantID string
	}{
		{"missing actual", "testing.ExpectEqual(1)", diagnostics.InvalidTestingExpectEqualArguments},
		{"excess arguments", "testing.RequireEqual(1, 1, \"message\", 2)", diagnostics.InvalidTestingRequireEqualArguments},
		{"incomparable operands", "testing.ExpectEqual(true, 1)", diagnostics.InvalidTestingExpectEqualArguments},
		{"invalid message", "testing.RequireEqual(1, 1, false)", diagnostics.InvalidTestingRequireEqualArguments},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := "module testing_equality\ntest \"invalid\" {\n" + test.call + "\n}"
			result := parser.New(lexer.NewWithFile(source, "testing_equality_invalid_test.sec")).Parse()
			if result.HasErrors {
				t.Fatalf("parser diagnostics = %+v", result.Diagnostics)
			}
			errors := NewAnalyzer().Analyze(result.Program)
			if len(errors) != 1 || errors[0].ID != test.wantID {
				t.Fatalf("errors = %+v, want one %s", errors, test.wantID)
			}
		})
	}
	t.Run("outside test context", func(t *testing.T) {
		source := "module testing_equality\nfn F() void { testing.ExpectEqual(1, 1) }"
		result := parser.New(lexer.NewWithFile(source, "testing_equality_invalid.sec")).Parse()
		if result.HasErrors {
			t.Fatalf("parser diagnostics = %+v", result.Diagnostics)
		}
		errors := NewAnalyzer().Analyze(result.Program)
		if len(errors) != 1 || errors[0].ID != diagnostics.TestingOutsideTestContext || !strings.Contains(errors[0].Message, "ExpectEqual") {
			t.Fatalf("outside-context errors = %+v", errors)
		}
	})
}

// TestTestingTerminationOperations validates the compiler-known signatures
// and operation facts, leaving runtime termination and cleanup to later stages.
// Rules: rules/tooling/testing.md — §§11.3–11.4 and 12–14.
func TestTestingTerminationOperations(t *testing.T) {
	path := "../../testdata/sema/testing_termination_valid_test.sec"
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
		t.Fatalf("termination operation errors = %+v", errors)
	}
	wants := []TestingOperationKind{TestingOperationPass, TestingOperationFail, TestingOperationSkip}
	for index, kind := range wants {
		declaration := result.Program.Statements[index+1].(*ast.TestDeclaration)
		call := declaration.Body.Statements[0].(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
		operation, ok := analyzer.ResolvedTestingOperationOf(call)
		if !ok || operation.Kind != kind || operation.Test != declaration ||
			(operation.Message != nil) != (kind != TestingOperationPass) {
			t.Errorf("operation %d = %+v, found=%v, want %s with correct message", index, operation, ok, kind)
		}
	}
}

func TestTestingTerminationOperationDiagnostics(t *testing.T) {
	path := "../../testdata/sema/testing_termination_invalid_test.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := parser.New(lexer.NewWithFile(string(source), path)).Parse()
	if result.HasErrors {
		t.Fatalf("parser diagnostics = %+v", result.Diagnostics)
	}
	errors := NewAnalyzer().Analyze(result.Program)
	wants := []string{"Pass", "Fail", "Fail", "Skip", "Skip"}
	if len(errors) != len(wants) {
		t.Fatalf("errors = %+v, want %d diagnostics", errors, len(wants))
	}
	for index, operation := range wants {
		if errors[index].ID != diagnostics.InvalidTestingTerminationArguments || !strings.Contains(errors[index].Message, operation) {
			t.Errorf("error %d = %+v, want S1044 for %s", index, errors[index], operation)
		}
	}

	path = "../../testdata/sema/testing_termination_outside_invalid.sec"
	source, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result = parser.New(lexer.NewWithFile(string(source), path)).Parse()
	if result.HasErrors {
		t.Fatalf("outside parser diagnostics = %+v", result.Diagnostics)
	}
	errors = NewAnalyzer().Analyze(result.Program)
	if len(errors) != 1 || errors[0].ID != diagnostics.TestingOutsideTestContext {
		t.Fatalf("outside-context errors = %+v", errors)
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
