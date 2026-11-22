package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func typeContracts(t *testing.T, source string) (map[string][]ast.Contract, []string) {
	t.Helper()
	p := New(lexer.New(source))
	result := p.Parse()
	contracts := map[string][]ast.Contract{}
	for _, statement := range result.Program.Statements {
		if decl, ok := statement.(*ast.TypeDeclStatement); ok && decl.Name != nil {
			var flat func(ast.Contract)
			flat = func(c ast.Contract) {
				if list, ok := c.(*ast.ContractList); ok {
					for _, item := range list.Contracts {
						flat(item)
					}
					return
				}
				if c != nil {
					contracts[decl.Name.Value] = append(contracts[decl.Name.Value], c)
				}
			}
			flat(decl.Contract)
		}
	}
	return contracts, p.Errors()
}

// MD-049 grammar.md § 17(10), lexical_structure.md § 7.3.1: `scale N` is a
// contextual contract in any order with range; `scale` stays an ordinary
// identifier, including as a range bound (T26, T53, T55).
func TestScaleContractParses(t *testing.T) {
	contracts, errs := typeContracts(t, `module main

let scale := 3

type Money decimal scale 2
type Rate decimal128 range 0..1 scale 6
type Open decimal scale 2 range 0..
type Bound decimal range 0..scale
type Steps decimal scale (0 - 3) range ..<0
`)
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	scaleOf := func(name string) ast.Expression {
		for _, contract := range contracts[name] {
			if marker, ok := contract.(*ast.MarkerContract); ok && marker.Name == "scale" {
				return marker.Value
			}
		}
		return nil
	}
	for _, name := range []string{"Money", "Rate", "Open", "Steps"} {
		if scaleOf(name) == nil {
			t.Fatalf("%s has no scale contract: %v", name, contracts[name])
		}
	}
	if len(contracts["Open"]) != 2 || len(contracts["Rate"]) != 2 {
		t.Fatalf("contract counts: Open %d, Rate %d", len(contracts["Open"]), len(contracts["Rate"]))
	}
	if scaleOf("Bound") != nil {
		t.Fatal("range 0..scale must read scale as the upper bound")
	}
	bound, ok := contracts["Bound"][0].(*ast.RangeContract)
	if !ok || bound.Max == nil || bound.Max.String() != "scale" {
		t.Fatalf("Bound = %#v", contracts["Bound"])
	}
}

// A missing argument is a recoverable contract node that does not consume
// the following range or default clause.
func TestScaleContractRecovery(t *testing.T) {
	contracts, errs := typeContracts(t, "module main\n\ntype Money decimal scale range 0..10\n")
	if !strings.Contains(strings.Join(errs, "\n"), "missing scale contract argument") {
		t.Fatalf("errors: %v", errs)
	}
	found := false
	for _, contract := range contracts["Money"] {
		if _, ok := contract.(*ast.RangeContract); ok {
			found = true
		}
	}
	if !found {
		t.Fatalf("range clause lost: %v", contracts["Money"])
	}
}

// Owner decision 2026-11-22 (contracts.md § 16(6), parser_recovery.md
// § 34.1(2)): a range without any bound constrains nothing and is an error,
// superseding MD-049 test vector T11.
func TestRangeWithoutBoundsIsRejected(t *testing.T) {
	for _, source := range []string{"type Broad decimal range ..", "type Broad decimal range ..<", "type Broad int range .."} {
		_, errs := typeContracts(t, "module main\n\n"+source+"\n")
		if !strings.Contains(strings.Join(errs, "\n"), "range contract must have at least one bound") {
			t.Fatalf("%s: errors %v", source, errs)
		}
	}
}
