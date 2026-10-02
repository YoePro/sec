package sema

import (
	"fmt"
	"math/big"
	"strings"

	"sec/internal/diagnostics"
)

// defaultRepresentable reports whether an explicit default constant has a
// value form of the declared type and fits the base type before any contract
// is considered.
//
// Rules:
//   - rules/types/default_values.md — "Explicit type defaults" (must be representable by the type)
//   - rules/types/default_values.md — "Diagnostics" (types.default-not-representable)
func defaultRepresentable(typ Type, value DefaultConstant) bool {
	if !defaultConstantCompatible(typ, value) {
		return false
	}
	base := typ
	base.Contracts = nil
	return defaultConstantSatisfies(base, value)
}

// firstViolatedContract returns the first contract, in declaration order, that
// value does not satisfy. Each contract is evaluated alone through the same
// satisfaction query used by default resolution.
//
// Rules:
//   - rules/types/contracts.md — "Composition" (contracts pass in source order)
//   - rules/types/contracts.md — "Diagnostics" (point to the relevant contract)
func firstViolatedContract(typ Type, value DefaultConstant) (Contract, bool) {
	for _, contract := range typ.Contracts {
		single := typ
		single.Contracts = []Contract{contract}
		if !defaultConstantSatisfies(single, value) {
			return contract, true
		}
	}
	return nil, false
}

// describeContract renders a semantic contract in canonical source spelling
// for diagnostic notes.
//
// Rule: rules/foundations/grammar.md — "Type contracts".
func describeContract(contract Contract) string {
	switch contract := contract.(type) {
	case RangeContract:
		operator := ".."
		if contract.Exclusive {
			operator = "..<"
		}
		return "range " + rangeBoundText(contract.Min, contract.MinLexeme) + operator + rangeBoundText(contract.Max, contract.MaxLexeme)
	case MembershipContract:
		values := make([]string, 0, len(contract.Values))
		for _, value := range contract.Values {
			values = append(values, value.Lexeme)
		}
		return "in [" + strings.Join(values, ", ") + "]"
	case MultipleOfContract:
		if contract.Value == nil {
			return "multipleOf"
		}
		return "multipleOf " + contract.Value.String()
	case LengthContract:
		return contract.Name + " " + contract.Value.String()
	case MarkerContract:
		return contract.Name
	case RegexContract:
		return fmt.Sprintf("regex %q", contract.Pattern)
	default:
		return "a type contract"
	}
}

func rangeBoundText(integer *big.Int, lexeme string) string {
	if lexeme != "" {
		return lexeme
	}
	if integer != nil {
		return integer.String()
	}
	return ""
}

// ambiguousImplicitDefault reports the two equally near valid values when the
// nearest-to-zero rule has no unique result for a type without an explicit
// default, so callers can explain why no implicit default exists.
//
// Rules:
//   - rules/types/default_values.md — "Ambiguous nearest-to-zero values"
//   - rules/types/default_values.md — "Compile-time resolution" (Ambiguous implicit default)
func ambiguousImplicitDefault(typ Type) (string, string, bool) {
	if typ.ExplicitDefault != nil || typ.InvalidExplicitDefault {
		return "", "", false
	}
	for _, contract := range typ.Contracts {
		if _, ok := contract.(MembershipContract); ok {
			return "", "", false
		}
	}
	switch typ.Kind {
	case IntType, UintType:
		if integerSatisfiesContracts(typ, big.NewInt(0)) {
			return "", "", false
		}
		positive, positiveOK := nearestIntegerDefault(typ, true)
		negative, negativeOK := nearestIntegerDefault(typ, false)
		if positiveOK && negativeOK && new(big.Int).Abs(positive).Cmp(new(big.Int).Abs(negative)) == 0 {
			return negative.String(), positive.String(), true
		}
	}
	return "", "", false
}

// noDefaultDiagnostic selects the registered diagnostic for a storage site
// that needs an implicit default its type cannot supply. Ambiguous nearest
// values keep their own stable identity and explanation instead of the site's
// generic no-default diagnostic. ok is false when the fallback applies.
//
// Rules:
//   - rules/types/default_values.md — "Diagnostics" (types.ambiguous-implicit-default)
//   - rules/types/default_values.md — "Ambiguous nearest-to-zero values"
func noDefaultDiagnostic(typ Type) (id string, help string, message string, ok bool) {
	negative, positive, ambiguous := ambiguousImplicitDefault(typ)
	if !ambiguous {
		return "", "", "", false
	}
	name := typeDisplayName(typ)
	return diagnostics.AmbiguousImplicitDefault,
		fmt.Sprintf("declare an explicit default on %s; %s and %s are equally near zero", name, negative, positive),
		fmt.Sprintf("%s has no unique implicit default", name),
		true
}

// ContractDisplays returns the resolved contracts of typ, including inherited
// contracts, in source order and canonical spelling for tooling presentation.
//
// Rules:
//   - rules/tooling/lsp.md — "Hover" (contracts)
//   - rules/types/contracts.md — "Composition"
func ContractDisplays(typ Type) []string {
	displays := make([]string, 0, len(typ.Contracts))
	for _, contract := range typ.Contracts {
		displays = append(displays, describeContract(contract))
	}
	return displays
}
