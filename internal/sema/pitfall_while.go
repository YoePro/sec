package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

type capacityWhileBound struct {
	index, collection string
	token             lexer.Token
	inclusive         bool
}

type capacityWhileContext struct {
	bounds     []capacityWhileBound
	equalities map[*ast.IndexExpression]lexer.Token
	accesses   map[*ast.IndexExpression]pitfallGuardAccess
}

// capacityWhileBounds normalizes true conjuncts, reversed comparisons and
// negation without interpreting names as collection contracts. Unknown or
// disjunctive bounds, getters and side-effecting conditions yield no relation.
// Rules: rules/analysis/pitfall_analysis.md — "Capacity used as live length",
// "Canonical facts consumed by pitfall analysis", "False-positive control".
func (b *pitfallBuilder) capacityWhileBounds(condition ast.Expression, truth bool) []capacityWhileBound {
	if parameterUsageNodeIsNil(condition) {
		return nil
	}
	if prefix, ok := condition.(*ast.PrefixExpression); ok && prefix.Operator == "!" {
		return b.capacityWhileBounds(prefix.Right, !truth)
	}
	comparison, ok := condition.(*ast.InfixExpression)
	if !ok {
		return nil
	}
	if comparison.Operator == "&&" && truth || comparison.Operator == "||" && !truth {
		return append(b.capacityWhileBounds(comparison.Left, truth), b.capacityWhileBounds(comparison.Right, truth)...)
	}
	operator := comparison.Operator
	if !truth {
		operator = map[string]string{"<": ">=", "<=": ">", ">": "<=", ">=": "<"}[operator]
	}
	index, capacity := comparison.Left, comparison.Right
	if operator == ">" || operator == ">=" {
		index, capacity = capacity, index
		operator = map[string]string{">": "<", ">=": "<="}[operator]
	}
	if operator != "<" && operator != "<=" || !b.pitfallPureOperand(index) {
		return nil
	}
	identity, indexOK := b.expressionIdentity(index)
	collection, token, capacityOK := b.capacityReceiver(capacity)
	member, memberOK := capacity.(*ast.MemberExpression)
	if !indexOK || !capacityOK || !memberOK || !b.pitfallPureOperand(member.Object) {
		return nil
	}
	return []capacityWhileBound{{index: identity, collection: collection, token: token, inclusive: operator == "<="}}
}

// walkWhileStatement preserves mandatory reachability and optional loop proof
// scopes. Canonical guard flow tracks each capacity relation up to an actual
// access; condition equality is re-established on each iteration. Raw witness
// tags use their exact Capacity token and never become live-length guards.
// Rules: rules/analysis/pitfall_analysis.md — "Reachability", "Capacity used
// as live length", "Guards participate in pitfall reasoning";
// rules/control-flow/flowcontrol_while.md — while condition evaluation.
func (b *pitfallBuilder) walkWhileStatement(loop *ast.WhileStatement) {
	b.walkExpression(loop.Condition)
	if flow, ok := b.analyzer.ResolvedWhileFlowOf(loop); ok && flow.ConditionKnown && !flow.ConditionValue {
		return
	}
	outerWhiles, outerEqualities := b.activeCapacityWhiles, b.activeCapacityEqualities
	defer func() { b.activeCapacityWhiles, b.activeCapacityEqualities = outerWhiles, outerEqualities }()
	guards := b.conditionIndexGuards(loop.Condition)
	if !b.boundsOnly {
		bounds := b.capacityWhileBounds(loop.Condition, true)
		context := capacityWhileContext{}
		seed := append(b.survivingIndexGuards(b.activeIndexGuards, loop), guards...)
		for _, bound := range bounds {
			witness := pitfallIndexGuard{indexIdentity: bound.index, collectionIdentity: bound.collection, source: bound.token}
			if b.guardSurvivesStatement(witness, &ast.ExpressionStatement{Expression: loop.Condition}) {
				duplicate := false
				for n, previous := range context.bounds {
					if previous.index == bound.index && previous.collection == bound.collection {
						duplicate = true
						if previous.inclusive && !bound.inclusive {
							context.bounds[n] = bound
						}
					}
				}
				if !duplicate {
					context.bounds = append(context.bounds, bound)
				}
			}
		}
		if len(context.bounds) != 0 {
			context.accesses = map[*ast.IndexExpression]pitfallGuardAccess{}
			for _, bound := range context.bounds {
				seed = append(seed, pitfallIndexGuard{indexIdentity: bound.index, collectionIdentity: bound.collection, source: bound.token})
			}
			conditional, protections, equalities := b.capacityWhileConditionalProofs(loop.Body)
			context.equalities = equalities
			for _, access := range b.guardedAccesses(loop.Body, seed, nil) {
				protected := protections[access.access]
				_, equality := equalities[access.access]
				if !conditional[access.access] || len(protected) != 0 || equality {
					access.guards = append(append([]pitfallIndexGuard(nil), access.guards...), protected...)
					context.accesses[access.access] = access
				}
			}
			b.activeCapacityWhiles = append(append([]capacityWhileContext(nil), outerWhiles...), context)
		}
		b.activeCapacityEqualities = mergeCapacityEqualities(outerEqualities, b.capacityBranchProof(loop.Condition, true))
	}
	b.withLoopIndexGuards(loop, guards, func() { b.walkBlock(loop.Body) })
}

// inspectWhileCapacityIndex reports only accesses whose canonical index and
// collection still match a live capacity condition. Length guards and strict
// capacity/length equality supply explicit suppression; inclusive capacity
// remains unsafe at its endpoint even when Len equals Capacity. Intent advice
// never asserts mandatory invalidity or certifies an automatic source edit.
// Rules: rules/analysis/pitfall_analysis.md — "Capacity used as live length",
// "Suppressing evidence", "Fix safety", "Diagnostic ownership and coalescing".
func (b *pitfallBuilder) inspectWhileCapacityIndex(index *ast.IndexExpression) {
	if len(b.activeCapacityWhiles) == 0 || !b.pitfallPureOperand(index.Left) || !b.pitfallPureOperand(index.Index) {
		return
	}
	for n := len(b.activeCapacityWhiles) - 1; n >= 0; n-- {
		context := b.activeCapacityWhiles[n]
		access, known := context.accesses[index]
		if !known {
			continue
		}
		for _, bound := range context.bounds {
			if access.indexIdentity != bound.index || access.collectionIdentity != bound.collection {
				continue
			}
			capacityLive, lengthLive := false, false
			var proof lexer.Token
			for _, guard := range access.guards {
				if guard.indexIdentity != bound.index || guard.collectionIdentity != bound.collection {
					continue
				}
				capacityWitness := false
				for _, candidate := range context.bounds {
					if sameSourceToken(candidate.token, guard.source) {
						capacityWitness = true
					}
				}
				if sameSourceToken(bound.token, guard.source) {
					capacityLive = true
				}
				if !capacityWitness {
					lengthLive, proof = true, guard.source
				}
			}
			if !capacityLive {
				continue
			}
			finding := PitfallFinding{
				Rule: PitfallCapacityAsLength, Family: PitfallBoundsAndRanges,
				Classification: PitfallLikelyMistake, Confidence: PitfallConfidenceHigh,
				Subject: PitfallSubject{Expression: index.String(), Source: index.Token}, OwningRule: "bounds",
				EvidenceFor: []PitfallEvidence{
					{Strength: PitfallEvidenceProof, Fact: "the while condition bounds the index by the same collection's Capacity", Source: bound.token},
					{Strength: PitfallEvidenceStrong, Fact: "Capacity may exceed live Len; the capacity bound does not establish a live-element index", Source: index.Token},
				},
				Actions: []PitfallSuggestedAction{{Kind: PitfallSuggestedEdit, Title: "bound the while traversal by live Len", Replacement: "Len", Source: bound.token}},
			}
			if bound.inclusive {
				finding.Actions[0].Title += " with a strict < comparison"
				finding.Actions[0].Replacement = ""
			}
			reason := "a strict live-length guard protects this access"
			if !lengthLive && !bound.inclusive {
				proof, lengthLive = context.equalities[index]
				if !lengthLive {
					proof, lengthLive = b.activeCapacityEqualities[bound.collection]
				}
				reason = "Len == Capacity is proven at this strict capacity-bounded access"
			}
			if lengthLive {
				evidence := PitfallEvidence{Strength: PitfallEvidenceSuppressing, Fact: reason, Source: proof}
				finding.State, finding.EvidenceAgainst = PitfallStateSuppressed, []PitfallEvidence{evidence}
				finding.Suppression = &PitfallSuppression{Reason: reason, Evidence: []PitfallEvidence{evidence}}
			}
			b.add(finding)
			return
		}
	}
}
