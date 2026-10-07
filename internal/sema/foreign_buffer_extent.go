package sema

import (
	"fmt"
	"reflect"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// ForeignExtentUnit and ForeignBufferAccess are supplied by trusted contracts;
// empty access means that the contract does not supply an access direction.
// Rules: rules/analysis/pitfall_analysis.md — "Canonical foreign extent relationships".
type ForeignExtentUnit string
type ForeignBufferAccess string

const (
	ForeignExtentElements  ForeignExtentUnit   = "elements"
	ForeignExtentBytes     ForeignExtentUnit   = "bytes"
	ForeignBufferRead      ForeignBufferAccess = "read"
	ForeignBufferWrite     ForeignBufferAccess = "write"
	ForeignBufferReadWrite ForeignBufferAccess = "read-write"
)

// ForeignBufferExtentRelation identifies zero-based declared parameter
// positions, independently of their names. Source identifies the trusted
// metadata evidence, not a guessed relationship at a call site.
// Rules: rules/analysis/pitfall_analysis.md — "FFI pitfall analysis",
// "Canonical foreign extent relationships"; rules/platform/ffi.md — §3.
type ForeignBufferExtentRelation struct {
	PointerArgument, ExtentArgument int
	ExtentUnit                      ForeignExtentUnit
	AccessMode                      ForeignBufferAccess
	Source                          lexer.Token
}

type foreignExtentTarget struct {
	Declaration                 sourceTokenKey
	Module, Name, ABI, LinkName string
}
type foreignExtentSignature struct {
	Parameters                 []Type
	Ref, MutableRef, Consuming []bool
	Return                     Type
}
type foreignExtentContract struct {
	Signature foreignExtentSignature
	Relations []ForeignBufferExtentRelation
}

// ForeignBufferExtentContractStore is a producer boundary for already resolved,
// trusted foreign metadata. It defines no source annotation/import syntax.
// Declaration identity and exact resolved signature protect against overloads,
// shadowing, ABI changes and stale metadata after source replacement.
// Rules: rules/analysis/pitfall_analysis.md — "FFI pitfall analysis";
// rules/platform/ffi.md — §§2–3.
type ForeignBufferExtentContractStore struct {
	contracts map[foreignExtentTarget]foreignExtentContract
}

// Record validates the whole relationship set before replacing a declaration's
// contract. Unsupported units, positions, parameter kinds, variadics and
// contradictory duplicate pairs cannot become canonical facts.
// Rules: rules/analysis/pitfall_analysis.md — "Canonical foreign extent relationships";
// rules/platform/ffi.md — §§2–3, 45.
func (s *ForeignBufferExtentContractStore) Record(function Function, relations []ForeignBufferExtentRelation) error {
	if s == nil || !function.Extern || function.Token.Line <= 0 || function.Token.Column <= 0 || function.Name == "" || (function.ABI != "C" && function.ABI != "system" && function.ABI != "Sec") || len(function.GenericParameters) != 0 || len(relations) == 0 {
		return fmt.Errorf("foreign extent metadata requires a resolved extern declaration and explicit relations")
	}
	for _, parameter := range function.Parameters {
		if parameter.Variadic {
			return fmt.Errorf("foreign extent metadata requires fixed parameter positions")
		}
	}
	seen := map[[2]int]bool{}
	for _, relation := range relations {
		if relation.PointerArgument < 0 || relation.ExtentArgument < 0 || relation.PointerArgument >= len(function.Parameters) || relation.ExtentArgument >= len(function.Parameters) || relation.PointerArgument == relation.ExtentArgument {
			return fmt.Errorf("foreign extent metadata has invalid parameter positions")
		}
		pointer := function.Parameters[relation.PointerArgument]
		extent := function.Parameters[relation.ExtentArgument]
		if pointer.Type.Kind != RawPtrType && pointer.Type.Kind != ReferenceType && !pointer.Ref {
			return fmt.Errorf("foreign extent pointer parameter must carry a pointer or borrow")
		}
		if !isIntegerType(extent.Type) || extent.Ref || extent.MutableRef || extent.Type.Kind == ReferenceType {
			return fmt.Errorf("foreign extent parameter must carry an integer quantity")
		}
		if relation.ExtentUnit != ForeignExtentElements && relation.ExtentUnit != ForeignExtentBytes {
			return fmt.Errorf("foreign extent metadata requires an explicit supported unit")
		}
		if relation.AccessMode != "" && relation.AccessMode != ForeignBufferRead && relation.AccessMode != ForeignBufferWrite && relation.AccessMode != ForeignBufferReadWrite {
			return fmt.Errorf("foreign extent metadata has unsupported access direction")
		}
		if (relation.AccessMode == ForeignBufferWrite || relation.AccessMode == ForeignBufferReadWrite) && (pointer.Ref || pointer.Type.Kind == ReferenceType) && !pointer.MutableRef && !pointer.Type.ReferenceMutable {
			return fmt.Errorf("foreign extent write contract cannot target a shared borrow")
		}
		pair := [2]int{relation.PointerArgument, relation.ExtentArgument}
		if seen[pair] || relation.Source.Line <= 0 || relation.Source.Column <= 0 {
			return fmt.Errorf("foreign extent metadata requires unique pairs and explicit evidence source")
		}
		seen[pair] = true
	}
	if s.contracts == nil {
		s.contracts = map[foreignExtentTarget]foreignExtentContract{}
	}
	s.contracts[foreignExtentTargetOf(function)] = foreignExtentContract{Signature: foreignExtentSignatureOf(function), Relations: append([]ForeignBufferExtentRelation(nil), relations...)}
	return nil
}

// foreignExtentTargetOf uses the selected declaration and ABI, never argument
// names or a callee's display spelling, as contract identity.
// Rules: rules/analysis/pitfall_analysis.md — "FFI pitfall analysis";
// rules/platform/ffi.md — §§2, 44.
func foreignExtentTargetOf(function Function) foreignExtentTarget {
	return foreignExtentTarget{Declaration: sourceTokenLocation(function.Token), Module: function.Module, Name: function.Name, ABI: function.ABI, LinkName: function.LinkName}
}

// foreignExtentSignatureOf detaches the full structural signature so changes to
// resolved types invalidate old foreign relationship metadata.
// Rules: rules/analysis/pitfall_analysis.md — "Canonical facts consumed by pitfall analysis";
// rules/compiler/compiler_analysis.md — §§9–10.
func foreignExtentSignatureOf(function Function) foreignExtentSignature {
	result := foreignExtentSignature{Return: semanticSnapshotType(function.ReturnType)}
	for _, parameter := range function.Parameters {
		result.Parameters = append(result.Parameters, semanticSnapshotType(parameter.Type))
		result.Ref = append(result.Ref, parameter.Ref)
		result.MutableRef = append(result.MutableRef, parameter.MutableRef)
		result.Consuming = append(result.Consuming, parameter.Consuming)
	}
	return result
}

// SetForeignBufferExtentContracts snapshots trusted producer input for the next
// analysis. Nil removes it; edits to the original store cannot change a running
// analysis, and call-site facts are always rebuilt by Analyze.
// Rules: rules/analysis/pitfall_analysis.md — "Canonical foreign extent relationships",
// "Incremental behavior"; rules/compiler/compiler_analysis.md — §§9–10.
func (a *Analyzer) SetForeignBufferExtentContracts(s *ForeignBufferExtentContractStore) {
	a.foreignExtentContracts = nil
	if s == nil {
		return
	}
	snapshot := &ForeignBufferExtentContractStore{contracts: map[foreignExtentTarget]foreignExtentContract{}}
	for target, contract := range s.contracts {
		signature := contract.Signature
		signature.Parameters = append([]Type(nil), signature.Parameters...)
		for i, typ := range signature.Parameters {
			signature.Parameters[i] = semanticSnapshotType(typ)
		}
		signature.Return = semanticSnapshotType(signature.Return)
		signature.Ref = append([]bool(nil), signature.Ref...)
		signature.MutableRef = append([]bool(nil), signature.MutableRef...)
		signature.Consuming = append([]bool(nil), signature.Consuming...)
		snapshot.contracts[target] = foreignExtentContract{Signature: signature, Relations: append([]ForeignBufferExtentRelation(nil), contract.Relations...)}
	}
	a.foreignExtentContracts = snapshot
}

// ResolvedForeignBufferExtent binds one canonical contract to the source-order
// actual argument positions. Tokens and display strings are detached evidence;
// optional origins are captured from resolved compiler-known members; missing
// origins remain unknown. Independent canonical quantity units and exact scalar
// storage stride may be known without proving the transfer is safe or intended.
// Rules: rules/analysis/pitfall_analysis.md — "Canonical foreign extent relationships".
type ResolvedForeignBufferExtent struct {
	Call, Declaration                   lexer.Token
	PointerArgument, ExtentArgument     int
	PointerSource, ExtentSource         lexer.Token
	PointerExpression, ExtentExpression string
	ExtentUnit                          ForeignExtentUnit
	AccessMode                          ForeignBufferAccess
	ContractSource                      lexer.Token
	PointerOrigin, ExtentOrigin         *Place
	// Independent quantity facts; zero stride means the layout is unknown.
	SuppliedExtentUnit  ForeignExtentUnit
	ElementStorageBytes int64
	ZeroExtent          bool
}

// ResolvedForeignBufferExtentsOf returns only contract-backed facts for the
// current analyzed call. An absent contract is unknown, never a NoFinding or a
// relationship inferred from names. Results cannot mutate analyzer facts.
// Rules: rules/analysis/pitfall_analysis.md — "FFI pitfall analysis", "Analysis states".
func (a *Analyzer) ResolvedForeignBufferExtentsOf(call *ast.CallExpression) ([]ResolvedForeignBufferExtent, bool) {
	if a == nil || call == nil {
		return nil, false
	}
	facts, ok := a.resolvedForeignBufferExtents[call]
	return cloneForeignBufferExtents(facts), ok
}

// recordForeignBufferExtents publishes selected foreign declaration contracts
// only during reachable final body analysis. It rejects stale signatures and
// unsupported argument mapping instead of guessing a binding relationship.
// Optional member origins are retained before leaving the current lexical scope.
// Rules: rules/analysis/pitfall_analysis.md — "FFI pitfall analysis", "Reachability",
// "Canonical foreign extent relationships"; rules/platform/ffi.md — §§2–3.
func (a *Analyzer) recordForeignBufferExtents(call *ast.CallExpression, resolved ResolvedCall) {
	if a.summaryPass || !a.callGraphPathReachable || resolved.Kind != ResolvedForeignCall || !resolved.Function.Extern || a.foreignExtentContracts == nil {
		return
	}
	contract, ok := a.foreignExtentContracts.contracts[foreignExtentTargetOf(resolved.Function)]
	if !ok || len(call.Arguments) != len(resolved.Function.Parameters) || !reflect.DeepEqual(contract.Signature, foreignExtentSignatureOf(resolved.Function)) {
		return
	}
	for _, parameter := range resolved.Function.Parameters {
		if parameter.Variadic {
			return
		}
	}
	var facts []ResolvedForeignBufferExtent
	for _, relation := range contract.Relations {
		pointer, extent := call.Arguments[relation.PointerArgument], call.Arguments[relation.ExtentArgument]
		if parameterUsageNodeIsNil(pointer) || parameterUsageNodeIsNil(extent) {
			return
		}
		facts = append(facts, ResolvedForeignBufferExtent{Call: call.Token, Declaration: resolved.Function.Token, PointerArgument: relation.PointerArgument, ExtentArgument: relation.ExtentArgument, PointerSource: expressionToken(pointer), ExtentSource: expressionToken(extent), PointerExpression: pointer.String(), ExtentExpression: extent.String(), ExtentUnit: relation.ExtentUnit, AccessMode: relation.AccessMode, ContractSource: relation.Source})
	}
	for i, relation := range contract.Relations {
		facts[i].PointerOrigin = a.foreignExtentMemberOrigin(call.Arguments[relation.PointerArgument], "Ptr")
		facts[i].ExtentOrigin = a.foreignExtentMemberOrigin(call.Arguments[relation.ExtentArgument], "Len", "SizeOf")
		a.recordForeignExtentQuantity(call.Arguments[relation.PointerArgument], call.Arguments[relation.ExtentArgument], &facts[i])
	}
	a.resolvedForeignBufferExtents[call] = facts
}
