package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// TestOrdinaryReceiverContract preserves shared copies and mutable member
// transfers without exposing a consuming receiver or conflating callable values.
// Rules: rules/memory/ownership.md — §§8–9;
// rules/declarations/functions.md — §§22,24–25.
func TestOrdinaryReceiverContract(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/ordinary_receiver_contract_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	method := a.functions["Container.ReleaseField"][0]
	if !method.ReceiverMutable || functionCallableCapability(method) != CallableMutable {
		t.Fatal(method)
	}
	read := a.functions["Number.Read"][0]
	if read.ReceiverMutable || functionCallableCapability(read) != CallableShared {
		t.Fatal(read)
	}
	invoke := a.functions["Invoke"][0]
	if !invoke.Parameters[0].Consuming || invoke.Parameters[0].Type.FunctionCapability != CallableConsuming {
		t.Fatal(invoke)
	}
	data, err = os.ReadFile("../../testdata/sema/ordinary_receiver_contract_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	_, errors = analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 3 {
		t.Fatal(errors)
	}
	for _, e := range errors {
		if !strings.Contains(e.Message, "ordinary method cannot consume complete self") {
			t.Fatal(e)
		}
	}
}

// TestOrdinaryReceiverConstructedAST rejects a legacy receiver mode even when
// source parsing is bypassed; the rejected declaration has no callable identity.
// Rules: rules/memory/ownership.md — §9; functions.md — §22.
func TestOrdinaryReceiverConstructedAST(t *testing.T) {
	data, err := os.ReadFile("../../testdata/semantic_ir/receiver_value_copy.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	var method *ast.FunctionDeclaration
	for _, stmt := range program.Statements {
		if impl, ok := stmt.(*ast.ImplStatement); ok {
			for _, member := range impl.Members {
				if fn, ok := member.(*ast.FunctionDeclaration); ok && fn.Name.Value == "Read" {
					method = fn
					break
				}
			}
			if method != nil {
				break
			}
		}
	}
	if method == nil {
		t.Fatal("fixture method missing")
	}
	statements := program.Statements[:0]
	for _, stmt := range program.Statements {
		if fn, ok := stmt.(*ast.FunctionDeclaration); ok && fn.Name.Value == "Use" {
			continue
		}
		statements = append(statements, stmt)
	}
	program.Statements = statements
	method.ReceiverCapability = ast.ReceiverConsuming
	a := NewAnalyzer()
	errors := a.Analyze(program)
	found := false
	for _, e := range errors {
		if e.ID == diagnostics.OrdinaryConsumingReceiver && e.Help != "" {
			found = true
		}
	}
	if !found || len(errors) != 1 || len(a.functions["Counter.Read"]) != 0 {
		t.Fatal(errors)
	}
}
