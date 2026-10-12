package sema

import "sec/internal/ast"

// Instance-method receiver facts (P15-47). An instance method receives its
// implicit `self` as a shared or mutable borrow of the caller's receiver
// Place (impl.md § 8, semantic_ir.md § 60(3)). Sema publishes the callee-side
// reference facts of that receiver, which identifiers of a method body denote
// the receiver itself or one of its stored fields, and, at a call site, the
// implicit borrow or reference pass that produces the receiver argument.
//
// Rules:
//   - rules/declarations/impl.md — § 8 "self"
//   - rules/compiler/semantic_ir.md — §§ 60(3), 61(4)
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 59-60

// ImplicitReceiverUse is one identifier of a method body that denotes the
// receiver: `self` itself, or a stored field of the receiver named without
// `self.` (Field holds its member plan).
type ImplicitReceiverUse struct {
	Self  bool
	Field ResolvedStructMemberPlan
}

// recordReceiverFacts publishes the callee-side reference facts of the
// implicit receiver of a non-static method.
func (a *Analyzer) recordReceiverFacts(function Function) {
	if a.summaryPass || function.Static || function.ImplTarget == "" {
		return
	}
	place, ok := a.resolvePlace(&ast.Identifier{Token: function.Token, Value: "self"})
	if !ok {
		return
	}
	place.RootKind = PlaceRootReceiver
	if a.receiverFacts == nil {
		a.receiverFacts = map[sourceTokenKey]ReferenceFacts{}
	}
	a.receiverFacts[sourceTokenLocation(function.Token)] = a.referenceFactsForPlace(place, function.ReceiverMutable, function.Token)
}

// ReceiverReferenceFactsOf returns the callee-side reference facts of the
// implicit receiver of an instance method.
func (a *Analyzer) ReceiverReferenceFactsOf(function Function) (ReferenceFacts, bool) {
	if a == nil {
		return ReferenceFacts{}, false
	}
	facts, ok := a.receiverFacts[sourceTokenLocation(function.Token)]
	return cloneReferenceFacts(facts), ok
}

// recordImplicitReceiverUse records that an identifier read in a method body
// denotes the implicit receiver or one of its stored fields.
func (a *Analyzer) recordImplicitReceiverUse(expr *ast.Identifier, symbol Symbol) {
	if a.summaryPass || a.currentImplTarget == "" || symbol.Local || symbol.Parameter {
		return
	}
	target, ok := a.types[a.currentImplTarget]
	if !ok {
		return
	}
	if a.implicitReceiverUses == nil {
		a.implicitReceiverUses = map[*ast.Identifier]ImplicitReceiverUse{}
	}
	if expr.Value == "self" && !symbol.ImplicitMember {
		a.implicitReceiverUses[expr] = ImplicitReceiverUse{Self: true}
		return
	}
	if !symbol.ImplicitMember {
		return
	}
	for fieldID, field := range target.Fields {
		if field.Name == expr.Value {
			a.implicitReceiverUses[expr] = ImplicitReceiverUse{Field: ResolvedStructMemberPlan{
				Kind: MemberStoredField, OwnerType: target, MemberType: field.Type,
				FieldID: uint32(fieldID), FieldName: field.Name, Tags: cloneStructTags(field.Tags),
				Action: resolvedStructCopyAction(field.Type),
			}}
			return
		}
	}
}

// ResolvedImplicitReceiverOf reports whether an identifier of a method body
// denotes the implicit receiver or one of its stored fields.
func (a *Analyzer) ResolvedImplicitReceiverOf(expr *ast.Identifier) (ImplicitReceiverUse, bool) {
	if a == nil || expr == nil {
		return ImplicitReceiverUse{}, false
	}
	use, ok := a.implicitReceiverUses[expr]
	return use, ok
}

// recordReceiverReferencePlan records how the receiver argument of an
// instance-method call is produced: a reference holder is passed (shared copy
// or reborrow) and an owned receiver Place is implicitly borrowed with the
// method's receiver capability.
func (a *Analyzer) recordReceiverReferencePlan(callee Function, receiver ast.Expression) {
	if a.summaryPass || !functionUsesReceiver(callee) || callee.ReceiverConsuming || receiver == nil {
		return
	}
	if identifier, ok := receiver.(*ast.Identifier); ok {
		if _, isSelf := a.implicitReceiverUses[identifier]; isSelf {
			return
		}
	}
	place, ok := a.resolvePlace(receiver)
	if !ok {
		return
	}
	if receiverType, typed := a.expressionTypes[receiver]; typed && typeCarriesReferenceOrigin(receiverType) {
		a.recordReferenceUse(receiver, ReferenceUsePass, place, callee.ReceiverMutable, expressionToken(receiver))
		return
	}
	a.recordReferencePlan(receiver, place, callee.ReceiverMutable, true, expressionToken(receiver))
}
