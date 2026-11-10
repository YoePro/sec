package secmlir

import (
	"crypto/sha256"
	"fmt"

	"sec/internal/ir/semantic"
)

// resolvedFunctionSymbols assigns ordinary overloads deterministic local MLIR
// symbols from their canonical Semantic IR identity, independent of declaration
// order and physical argument carriers. It never resolves overloads or adds
// return/ownership dimensions to the source overload signature. A sole demanded
// specialization still uses its canonical identity, independently of how many
// sibling specializations are emitted. Unique ordinary names retain the local
// numbering; this is not a stable external Sec ABI.
// Rules: rules/declarations/functions.md — §§11,19–20;
// rules/compiler/semantic_ir.md — "Functions and calls";
// rules/mlir/sec_mlir_lowering.md — semantic identity preservation;
// rules/compiler/monomorphization.md — §§4.1,15.3.
func resolvedFunctionSymbols(module *semantic.Module) (map[semantic.FunctionID]string, error) {
	counts := map[string]int{}
	for _, function := range module.Functions {
		counts[function.Name]++
	}
	symbols := map[semantic.FunctionID]string{}
	used := map[string]semantic.FunctionID{}
	for index, function := range module.Functions {
		symbol := fmt.Sprintf("sec_fn_%d", index)
		if counts[function.Name] > 1 || function.Specialization != nil || function.TypePack != nil {
			symbol = fmt.Sprintf("sec_overload_%x", sha256.Sum256([]byte(function.ID)))
		}
		if previous, exists := used[symbol]; exists && previous != function.ID {
			return nil, fmt.Errorf("backend symbol collision for %s and %s", previous, function.ID)
		}
		used[symbol] = function.ID
		symbols[function.ID] = symbol
	}
	return symbols, nil
}
