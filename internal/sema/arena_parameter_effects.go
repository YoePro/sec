package sema

import (
	"sort"
	"strings"

	"sec/internal/ast"
)

// isArenaReferenceType reports a ref or ref mut Arena type.
func isArenaReferenceType(typ Type) bool {
	return typ.Kind == ReferenceType && typ.Element != nil && typ.Element.Name == "Arena"
}

func arenaReferenceParameterIndexes(function Function) []int {
	var indexes []int
	for index, parameter := range function.Parameters {
		if isArenaReferenceType(parameter.Type) || (parameter.Ref || parameter.MutableRef) && parameter.Type.Name == "Arena" {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func hasArenaReferenceParameter(function Function) bool {
	return len(arenaReferenceParameterIndexes(function)) > 0
}

func sortedArenaParameterResets(resets map[int]bool) []int {
	if len(resets) == 0 {
		return nil
	}
	indexes := make([]int, 0, len(resets))
	for index := range resets {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	return indexes
}

func equalIntSlices(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// recordArenaParameterReset notes that the current function resets the Arena
// behind one of its own Arena reference parameters by domain identity, even
// after the reference holder moves. Names alone cannot prove this relationship.
// Rules: rules/memory/arena.md — §§4.2,44,68(3)-(4).
func (a *Analyzer) recordArenaParameterReset(name string) {
	if a.currentArenaParameterResets == nil || a.inLambda {
		return
	}
	symbol, ok := a.symbols[name]
	if !ok || symbol.Type.ArenaDomainID == "" {
		return
	}
	for index, parameter := range a.currentFunctionMetadata.Parameters {
		if isArenaReferenceType(parameter.Type) && symbol.Type.ArenaDomainID == a.newArenaDomainID(parameter.Token, "parameter") {
			a.currentArenaParameterResets[index] = true
			return
		}
	}
}

// applyCalleeArenaParameterResets applies a validated call's Arena
// parameter summary to the caller: every argument bound to a parameter the
// callee may Reset passes the same Reset/Release dependency check as an
// explicit Reset and then advances its Arena's epoch, so storage allocated
// from that Arena before the call is stale afterwards. A forwarded caller
// parameter makes the caller reset that parameter too. Arguments whose Arena
// is not a visible binding (for example a field) are not resolved here.
//
// Rules:
//   - rules/memory/arena.md — §§36(1–4), 44(1–2), 68(3)–(4)
//   - rules/compiler/compiler_analysis.md — §14 summaries; rules/memory/lifetime_analysis.md — §28 "Separate compilation and summaries"
func (a *Analyzer) applyCalleeArenaParameterResets(callee Function, args []ast.Expression, preparedSpreadValues []bool) bool {
	if len(callee.ArenaParameterResets) == 0 {
		return true
	}
	for _, index := range callee.ArenaParameterResets {
		if index >= len(args) || index < len(preparedSpreadValues) && preparedSpreadValues[index] {
			continue
		}
		receiver, ok := explicitMoveSource(args[index]).(*ast.Identifier)
		if !ok {
			continue
		}
		symbol, ok := a.symbols[receiver.Value]
		if !ok {
			continue
		}
		var domain, owner string
		switch {
		case isArenaReferenceType(symbol.Type):
			domain, owner, _ = a.referencedArenaDomain(receiver, symbol)
			if owner == receiver.Value {
				a.recordArenaParameterReset(receiver.Value)
			}
		case symbol.Type.Name == "Arena":
			domain, owner = symbol.Type.ArenaDomainID, receiver.Value
			if domain == "" {
				domain = a.newArenaDomainID(symbol.Token, "owner")
				symbol.Type.ArenaDomainID = domain
				a.symbols[receiver.Value] = symbol
			}
		default:
			continue
		}
		if domain == "" {
			continue
		}
		if !a.summaryPass {
			before := len(a.errors)
			if a.checkArenaInvalidationDependencies("Reset", domain, owner, expressionToken(args[index])) {
				if len(a.errors) > before {
					diagnostic := &a.errors[len(a.errors)-1]
					diagnostic.Message = strings.Replace(diagnostic.Message, "cannot call Arena.Reset on "+owner,
						"cannot pass arena "+owner+" to "+callee.Name+", which may Reset it,", 1)
				}
				return false
			}
		}
		a.arenaGenerations[domain]++
	}
	return true
}
