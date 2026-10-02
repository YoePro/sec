package sema

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// isLengthContractName identifies the integer-valued string and collection
// contracts defined by the canonical contract rulebook.
//
// Rule: rules/types/contracts.md — "String and collection contracts".
func isLengthContractName(name string) bool {
	switch name {
	case "minLen", "maxLen", "exactLen":
		return true
	default:
		return false
	}
}

// checkLengthContractSetConsistency intersects inherited and local length
// contracts. A fixed array contributes its statically known extent, while
// notEmpty contributes the lower bound one.
//
// Rule: rules/types/contracts.md — "Composition" and
// "String and collection contracts".
func (a *Analyzer) checkLengthContractSetConsistency(typ Type, contractNode ast.Contract) {
	if contractNode == nil || (typ.Kind != StringType && !a.isCollectionContractType(typ)) {
		return
	}

	var token lexer.Token
	typeName := typeDisplayName(typ)
	if typ.Named && typ.Name != "" {
		typeName = typ.Name
	}
	hasCurrentLengthContract := false
	for _, contract := range flattenASTContracts(contractNode) {
		marker, ok := contract.(*ast.MarkerContract)
		if !ok || (!isLengthContractName(marker.Name) && marker.Name != "notEmpty") {
			continue
		}
		token = marker.Token
		hasCurrentLengthContract = true
	}
	if !hasCurrentLengthContract {
		return
	}

	lower := big.NewInt(0)
	var upper *big.Int
	var exact *big.Int
	for _, contract := range typ.Contracts {
		switch contract := contract.(type) {
		case MarkerContract:
			if contract.Name == "notEmpty" && lower.Sign() == 0 {
				lower.SetInt64(1)
			}
		case LengthContract:
			if contract.Value == nil {
				continue
			}
			switch contract.Name {
			case "minLen":
				if contract.Value.Cmp(lower) > 0 {
					lower.Set(contract.Value)
				}
			case "maxLen":
				if upper == nil || contract.Value.Cmp(upper) < 0 {
					upper = new(big.Int).Set(contract.Value)
				}
			case "exactLen":
				if exact != nil && contract.Value.Cmp(exact) != 0 {
					a.addErrorAtTokenWithMetadata(token, diagnostics.UnsatisfiableContractSet, "remove or relax one of the conflicting contracts", "length contracts cannot be satisfied together for %s", typeName)
					return
				}
				exact = new(big.Int).Set(contract.Value)
			}
		}
	}

	if fixedLength, ok := exactFixedArrayLength(typ); ok {
		exactConflict := exact != nil && exact.Cmp(fixedLength) != 0
		belowMinimum := fixedLength.Cmp(lower) < 0
		aboveMaximum := upper != nil && fixedLength.Cmp(upper) > 0
		if exactConflict || belowMinimum || aboveMaximum {
			a.addErrorAtTokenWithMetadata(token, diagnostics.UnsatisfiableContractSet, "remove or relax one of the conflicting contracts", "length contracts cannot be satisfied together for %s", typeName)
		}
		return
	}
	exactOutsideBounds := exact != nil && (exact.Cmp(lower) < 0 || upper != nil && exact.Cmp(upper) > 0)
	emptyBounds := upper != nil && lower.Cmp(upper) > 0
	if exactOutsideBounds || emptyBounds {
		a.addErrorAtTokenWithMetadata(token, diagnostics.UnsatisfiableContractSet, "remove or relax one of the conflicting contracts", "length contracts cannot be satisfied together for %s", typeName)
	}
}

// stringLengthSatisfiesContract uses the same byte-count unit as the
// compiler-known string.Len property and trusted core's ByteLen projection.
func stringLengthSatisfiesContract(value string, contract LengthContract) bool {
	return knownLengthSatisfiesContract(new(big.Int).SetUint64(uint64(len(value))), contract)
}

// knownLengthSatisfiesContract compares one exact, nonnegative semantic length
// with a represented length contract without narrowing to host integer width.
//
// Rules:
//   - rules/types/contracts.md — "String and collection contracts"
func knownLengthSatisfiesContract(length *big.Int, contract LengthContract) bool {
	if length == nil || length.Sign() < 0 || contract.Value == nil {
		return false
	}
	switch contract.Name {
	case "minLen":
		return length.Cmp(contract.Value) >= 0
	case "maxLen":
		return length.Cmp(contract.Value) <= 0
	case "exactLen":
		return length.Cmp(contract.Value) == 0
	default:
		return false
	}
}
