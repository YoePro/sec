package parser

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Every canonical prefix in the fixture must be accepted both by Pratt parsing
// and its shared start predicate, including unit references and explicit moves.
// Rules: rules/foundations/grammar.md — "Primary expressions", "Prefix expressions";
// rules/compiler/parser_recovery.md — Appendix A.8.
func TestGrammarMetadataAgreesWithExpressionAndAssignmentParsing(t *testing.T) {
	source, err := os.ReadFile("../../testdata/parser/grammar_metadata_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := New(lexer.New(string(source)))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("canonical fixture: %v", result.Diagnostics)
	}
	for _, line := range strings.Split(string(source), "\n") {
		if _, value, ok := strings.Cut(line, ":="); ok {
			token := lexer.New(strings.TrimSpace(value)).NextToken()
			if !p.isExpressionStart(token.Type) {
				t.Errorf("accepted expression prefix %q is absent from shared metadata", token.Lexeme)
			}
		}
	}
	// Each unambiguous top-level source spelling must be available to recovery
	// too; the final attributed declaration exercises its leading @ rather than fn.
	for _, line := range strings.Split(string(source), "\n") {
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "}") {
			continue
		}
		token := lexer.New(line).NextToken()
		if !isDeclarationStart(token.Type) {
			t.Errorf("top-level declaration %q is missing from shared metadata", line)
		}
	}
	assignments := result.Program.Statements[2].(*ast.FunctionDeclaration).Body.Statements
	if len(assignments) != len(assignmentOperators()) {
		t.Fatalf("assignment coverage = %d, inventory = %d", len(assignments), len(assignmentOperators()))
	}
	seen := map[lexer.TokenType]bool{}
	for _, statement := range assignments {
		assignment, ok := statement.(*ast.AssignmentStatement)
		if !ok {
			t.Fatalf("assignment parsed as %T", statement)
		}
		if !p.isAssignmentOperator(lexer.New(assignment.Operator).NextToken().Type) {
			t.Errorf("accepted operator %q missing from shared inventory", lexer.New(assignment.Operator).NextToken().Type)
		}
		seen[lexer.New(assignment.Operator).NextToken().Type] = true
	}
	for _, operator := range assignmentOperators() {
		if !seen[operator] {
			t.Errorf("operator %q not exercised", operator)
		}
	}
	for _, spelling := range []string{":=", "==", "<=", "++", "--"} {
		if p.isAssignmentOperator(lexer.New(spelling).NextToken().Type) {
			t.Errorf("%q is not an assignment operator", spelling)
		}
	}
}

// Invalid predecessors must not swallow identifier-led impl members or
// attributes, nor assert/panic/reference statement starts on following lines.
// Rules: rules/compiler/parser_recovery.md — "Impl-member recovery", "Statement recovery".
func TestGrammarMetadataPreservesRecoverySiblings(t *testing.T) {
	source, err := os.ReadFile("../../testdata/parser/grammar_metadata_recovery_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.New(string(source))).Parse()
	if !result.HasErrors {
		t.Fatal("invalid fixture was accepted")
	}
	if len(result.Program.Statements) != 5 {
		t.Fatalf("lost declarations: %#v", result.Program.Statements)
	}
	impl := result.Program.Statements[1].(*ast.ImplStatement)
	if len(impl.Members) != 6 {
		t.Fatalf("impl members = %#v", impl.Members)
	}
	for index, want := range map[int]string{1: "*ast.InitDeclaration", 3: "*ast.EventDeclaration", 5: "*ast.FunctionDeclaration"} {
		if got := fmt.Sprintf("%T", impl.Members[index]); got != want {
			t.Errorf("member %d = %s, want %s", index, got, want)
		}
	}
	unit := result.Program.Statements[2].(*ast.ImplStatement)
	if len(unit.Members) != 2 {
		t.Fatalf("unit members = %#v", unit.Members)
	}
	if _, ok := unit.Members[1].(*ast.UnitMetadataDeclaration); !ok {
		t.Fatalf("lost metadata member: %T", unit.Members[1])
	}
	function := result.Program.Statements[3].(*ast.FunctionDeclaration)
	if len(function.Body.Statements) != 6 {
		t.Fatalf("statement siblings = %#v", function.Body.Statements)
	}
	if _, ok := function.Body.Statements[1].(*ast.AssertStatement); !ok {
		t.Fatalf("lost assert: %T", function.Body.Statements[1])
	}
	if _, ok := function.Body.Statements[3].(*ast.PanicStatement); !ok {
		t.Fatalf("lost panic: %T", function.Body.Statements[3])
	}
}

// Contracts retain one lexer-owned inventory and contextual words remain names
// away from the syntactic role in which the parser recognizes them.
// Rules: rules/foundations/lexical_structure.md §§7.2–7.3,9;
// rules/foundations/grammar.md — "Type contracts", "Contextual spellings".
func TestGrammarMetadataContextualAndContractBoundaries(t *testing.T) {
	p := New(lexer.New(""))
	for _, word := range lexer.ContractStartWords() {
		token := lexer.New(word).NextToken()
		if !p.isContractStart(token) {
			t.Errorf("contract %q missing", word)
		}
		if p.isContractStart(lexer.Token{Type: lexer.STRING, Lexeme: word}) {
			t.Errorf("string %q became a contract", word)
		}
	}
	for _, word := range []string{"range", "in"} {
		if !p.isContractStart(lexer.New(word).NextToken()) {
			t.Errorf("keyword contract %q missing", word)
		}
	}
	for _, word := range []string{"init", "event", "set", "test", "is", "not", "x", "detach", "unreachable", "step", "using"} {
		token := lexer.New(word).NextToken()
		if token.Type != lexer.IDENT {
			t.Errorf("contextual %q was reserved", word)
		}
		candidate := New(lexer.New("fn F(" + word + ": int) int { return " + word + " }"))
		if candidate.Parse().HasErrors {
			t.Errorf("ordinary name %q: %v", word, candidate.Errors())
		}
	}
	if p.isImplMemberStart(lexer.Token{Type: lexer.IDENT, Lexeme: "arbitrary"}) {
		t.Fatal("arbitrary identifier became a reliable impl recovery boundary")
	}
}

// Statement/declaration prefixes inside a damaged function body do not become
// false module-scope recovery boundaries or cause a stray closing-brace error.
// Rules: rules/compiler/parser_recovery.md — "Enclosing-boundary safety",
// "Declaration-header recovery", "Prefer grammar context".
func TestGrammarMetadataKeepsNestedDeclarationInsideRecovery(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/malformed_signature_nested_declaration_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.New(string(data))).Parse()
	if len(result.Diagnostics) != 2 {
		t.Fatalf("expected independent signature and lexer errors only: %+v", result.Diagnostics)
	}
	found := false
	for _, statement := range result.Program.Statements {
		if _, binding := statement.(*ast.LetStatement); binding {
			t.Fatal("nested let escaped into module scope")
		}
		if function, ok := statement.(*ast.FunctionDeclaration); ok && function.Name.Value == "After" {
			found = true
		}
	}
	if !found {
		t.Fatal("lost following declaration")
	}
}
