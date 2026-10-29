package secmlir

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestOverloadSymbolsPreserveSemanticIdentity verifies emitted definitions and
// calls across source declaration order and target widths. int8/uint8 both
// become signless i8 after scalar lowering; their Sec overloads stay distinct.
// Rules: rules/declarations/functions.md — §§19–20;
// rules/mlir/sec_mlir_lowering.md — semantic identity preservation.
func TestOverloadSymbolsPreserveSemanticIdentity(t *testing.T) {
	const fixture = "../../../testdata/build/native/overload_identity.sec"
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start, end := strings.Index(source, "fn Select"), strings.Index(source, "fn main")
	declarations := strings.Split(strings.TrimSpace(source[start:end]), "\n")
	for i, j := 0, len(declarations)-1; i < j; i, j = i+1, j-1 {
		declarations[i], declarations[j] = declarations[j], declarations[i]
	}
	reordered := source[:start] + strings.Join(declarations, "\n") + "\n\n" + source[end:]
	var baseline map[string]string
	definition := regexp.MustCompile(`(?m)^  func.func @([^ (]+).*sec.function_id = "([^"]+)"`)
	for _, width := range []uint16{32, 64} {
		for _, input := range []string{source, reordered} {
			p := parser.New(lexer.NewWithFile(input, fixture))
			program := p.ParseProgram()
			if errors := p.Errors(); len(errors) != 0 {
				t.Fatal(errors)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(testPlan(width))
			if errors := analyzer.Analyze(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(program, analyzer, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 14})
			if err != nil {
				t.Fatal(err)
			}
			output, err := Emit(module, testPlan(width))
			if err != nil {
				t.Fatal(err)
			}
			text := string(output)
			symbols := map[string]string{}
			for _, match := range definition.FindAllStringSubmatch(text, -1) {
				symbols[match[2]] = match[1]
			}
			used := map[string]bool{}
			for _, typ := range []string{"int8", "uint8", "int16"} {
				id := "main::Select(" + typ + ")"
				symbol := symbols[id]
				if symbol == "" || used[symbol] {
					t.Fatal("semantic overload identity collapsed", symbols)
				}
				used[symbol] = true
				if baseline != nil && symbol != baseline[id] {
					t.Fatal("source order/target changed overload symbol", symbols, baseline)
				}
				if !strings.Contains(text, "callee = @"+symbol+"}") {
					t.Fatal("resolved call does not use definition's symbol", id)
				}
			}
			if baseline == nil {
				baseline = symbols
			}
			// Reverse the IR declaration sequence independently as well: the
			// emitter consumes identities, never positional declaration indexes.
			for i, j := 0, len(module.Functions)-1; i < j; i, j = i+1, j-1 {
				module.Functions[i], module.Functions[j] = module.Functions[j], module.Functions[i]
			}
			again, err := Emit(module, testPlan(width))
			if err != nil {
				t.Fatal(err)
			}
			for _, match := range definition.FindAllStringSubmatch(string(again), -1) {
				if strings.HasPrefix(match[2], "main::Select(") && match[1] != baseline[match[2]] {
					t.Fatal("IR order changed overload identity", match)
				}
			}
		}
	}
}
