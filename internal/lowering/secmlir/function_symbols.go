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
//
// An imported declaration, and with export every defined Sec function, uses
// the binary symbol identity of its canonical FunctionID instead, so one
// module's object defines exactly the symbol another module's declaration
// references. It is derived from the identity alone, never from an import
// alias, file order or host (linking.md §§ 9, 15, 19).
func resolvedFunctionSymbols(module *semantic.Module, export bool) (map[semantic.FunctionID]string, error) {
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
		// A specialization is instantiated by every module that demands
		// it, so it keeps its local symbol and, in an exported unit, private
		// linkage instead of a shared one (linking.md § 24).
		if function.Imported || export && !function.Extern && function.Specialization == nil && function.TypePack == nil {
			symbol = BinarySymbol(function.ID)
		}
		if previous, exists := used[symbol]; exists && previous != function.ID {
			return nil, fmt.Errorf("backend symbol collision for %s and %s", previous, function.ID)
		}
		used[symbol] = function.ID
		symbols[function.ID] = symbol
	}
	return symbols, nil
}

// BinarySymbol is the module-independent native symbol of a Sec function:
// a collision-resistant encoding of its canonical FunctionID.
func BinarySymbol(id semantic.FunctionID) string {
	return fmt.Sprintf("sec_sym_%x", sha256.Sum256([]byte(id)))
}
