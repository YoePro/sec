package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// The regex pattern contract is retained as a dedicated AST node in source
// order inside the sequential contract conjunction.
//
// Rules:
//   - rules/types/contracts.md — "Applicability" and "String and collection contracts"
//   - rules/foundations/grammar.md — "Type contracts" (sequential contracts)
func TestParseRegexContractRetainsPatternInContractOrder(t *testing.T) {
	parser := New(lexer.New("type Email string minLen 3 regex \"^[a-z]+$\" maxLen 64 default \"abc\"\n"))
	program := parser.ParseProgram()
	checkParserErrors(t, parser)
	declaration, ok := program.Statements[0].(*ast.TypeDeclStatement)
	if !ok {
		t.Fatalf("statement = %T, want type declaration", program.Statements[0])
	}
	list, ok := declaration.Contract.(*ast.ContractList)
	if !ok || len(list.Contracts) != 3 {
		t.Fatalf("contract = %#v, want three sequential contracts", declaration.Contract)
	}
	regex, ok := list.Contracts[1].(*ast.RegexContract)
	if !ok {
		t.Fatalf("second contract = %T, want *ast.RegexContract", list.Contracts[1])
	}
	pattern, ok := regex.Pattern.(*ast.StringLiteral)
	if !ok || pattern.Value != "^[a-z]+$" || regex.Token.Lexeme != "regex" {
		t.Fatalf("regex contract = %#v", regex)
	}
	if declaration.Default == nil {
		t.Fatal("default clause after the regex contract was not retained")
	}
}

// A missing pattern produces one parser diagnostic per declaration, keeps an
// invalid pattern node, and never consumes the next contract, default clause,
// or following declaration.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Type-contract recovery", "Bounded damage"
//   - rules/types/contracts.md — "String and collection contracts"
func TestParseRegexContractMissingPatternRecovers(t *testing.T) {
	source := "type First string regex minLen 3\ntype Second string regex default \"x\"\ntype Third string regex\nfn After() void {}\n"
	result := New(lexer.New(source)).Parse()
	if len(result.Diagnostics) != 3 {
		t.Fatalf("diagnostics = %+v, want 3", result.Diagnostics)
	}
	if len(result.Program.Statements) != 4 {
		t.Fatalf("statements = %d, want 4", len(result.Program.Statements))
	}
	first := result.Program.Statements[0].(*ast.TypeDeclStatement)
	list, ok := first.Contract.(*ast.ContractList)
	if !ok || len(list.Contracts) != 2 {
		t.Fatalf("first contract = %#v, want regex plus minLen", first.Contract)
	}
	regex := list.Contracts[0].(*ast.RegexContract)
	if _, invalid := regex.Pattern.(*ast.InvalidExpression); !invalid {
		t.Fatalf("missing pattern = %T, want *ast.InvalidExpression", regex.Pattern)
	}
	second := result.Program.Statements[1].(*ast.TypeDeclStatement)
	if second.Default == nil {
		t.Fatal("default clause after missing regex pattern was not retained")
	}
	if _, ok := result.Program.Statements[3].(*ast.FunctionDeclaration); !ok {
		t.Fatalf("following declaration = %T, want function", result.Program.Statements[3])
	}
}

// `regex` follows ordinary whitespace and continuation rules: a contract on
// the following physical line belongs to the declaration (MD-009).
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 10.12–10.14
//   - rules/foundations/grammar.md — "Type contracts"
func TestParseRegexContractOnFollowingLine(t *testing.T) {
	parser := New(lexer.New("type Email string\n    regex \"^[a-z]+$\"\nfn After() void {}\n"))
	program := parser.ParseProgram()
	checkParserErrors(t, parser)
	declaration := program.Statements[0].(*ast.TypeDeclStatement)
	if _, ok := declaration.Contract.(*ast.RegexContract); !ok {
		t.Fatalf("contract = %#v, want regex contract from the continuation line", declaration.Contract)
	}
	if len(program.Statements) != 2 {
		t.Fatalf("statements = %d, want 2", len(program.Statements))
	}
}
