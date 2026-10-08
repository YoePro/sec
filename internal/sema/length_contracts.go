package sema

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/sema/collectionshape"
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

	var bounds collectionshape.Bounds
	valid := true
	for _, contract := range typ.Contracts {
		switch contract := contract.(type) {
		case MarkerContract:
			if contract.Name == "notEmpty" {
				valid = bounds.Add(contract.Name, nil) && valid
			}
		case LengthContract:
			if contract.Value != nil {
				valid = bounds.Add(contract.Name, contract.Value) && valid
			}
		}
	}
	if fixedLength, ok := exactFixedArrayLength(typ); ok {
		valid = bounds.Add("exactLen", fixedLength) && valid
	}
	if !valid {
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
	return collectionshape.Satisfies(length, contract.Name, contract.Value)
}
