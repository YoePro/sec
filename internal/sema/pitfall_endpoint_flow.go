package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

type pitfallEndpointFlow struct {
	normal, endpoint, stable bool
}

type pitfallEndpointScan struct {
	builder    *pitfallBuilder
	domain     *ast.RangeExpression
	binding    lexer.Token
	collection string
	length     lexer.Token
	loopBreaks []*[]pitfallEndpointFlow
}

// bodyMayChangeLength invalidates endpoint/live-Len identity across iterations,
// including a mutation followed by continue before the current access. The
// range bound was evaluated once; a later Len is not a fresh loop bound.
// Rules: rules/control-flow/flowcontrol_for.md — §§12, 39;
// rules/analysis/pitfall_analysis.md — "Canonical facts consumed by pitfall analysis", "Guards participate in pitfall reasoning".
func (s *pitfallEndpointScan) bodyMayChangeLength(block *ast.BlockStatement) bool {
	length := s.domain.End.(*ast.MemberExpression).Object
	for length != nil {
		if identity, ok := s.builder.expressionIdentity(length); ok && s.builder.blockAssignsIdentity(block, identity) {
			return true
		}
		member, ok := length.(*ast.MemberExpression)
		if !ok {
			break
		}
		length = member.Object
	}
	changed := false
	for _, statement := range block.Statements {
		visitStatementExpressions(statement, func(expression ast.Expression) {
			switch expression := expression.(type) {
			case *ast.CallExpression:
				changed = changed || s.callMayChangeLength(expression)
			case *ast.AwaitExpression, *ast.RuntimeCallExpression:
				changed = true
			case *ast.RefExpression:
				changed = changed || expression.Mutable
			}
		})
	}
	return changed
}

// callMayChangeLength consumes compiler-known structure contracts and resolved
// mutable call authority. Missing user mutation summaries cannot establish
// that the saved endpoint equals the live Len on a later iteration.
// Rules: rules/analysis/pitfall_analysis.md — "Canonical facts consumed by pitfall analysis";
// rules/control-flow/flowcontrol_for.md — §§12, 39.
func (s *pitfallEndpointScan) callMayChangeLength(call *ast.CallExpression) bool {
	if _, constant := s.builder.analyzer.integerConstantValue(call); constant {
		return false
	}
	if len(s.builder.mutationsInExpression(call, s.collection, false)) != 0 {
		return true
	}
	resolved, ok := s.builder.analyzer.ResolvedCallTarget(call)
	if !ok {
		return true
	}
	if resolved.Function.ReceiverMutable && resolved.Function.CompilerKnownID == "" {
		return true
	}
	for _, parameter := range resolved.Function.Parameters {
		if parameter.MutableRef || parameter.Type.Kind == ReferenceType && parameter.Type.ReferenceMutable {
			return true
		}
	}
	return false
}

// inspectInclusiveLengthLoop follows evaluation/control flow at the inclusive
// endpoint instead of treating a syntactically preceding guard as global proof.
// Rules: rules/analysis/pitfall_analysis.md — "Inclusive upper bound against
// collection length", "Reachability", "Guards participate in pitfall reasoning";
// rules/control-flow/flowcontrol_for.md — §§12, 23, 25–26, 30–31.
func (b *pitfallBuilder) inspectInclusiveLengthLoop(loop *ast.ForStatement) {
	domain, ok := loop.Iterable.(*ast.RangeExpression)
	if !ok || domain == nil || loop.Body == nil || domain.Exclusive || len(loop.Bindings) != 1 || loop.Bindings[0].Discard {
		return
	}
	collection, length, ok := b.lengthReceiver(domain.End)
	if !ok {
		return
	}
	scan := pitfallEndpointScan{builder: b, domain: domain, binding: loop.Bindings[0].Token, collection: collection, length: length}
	unitStep := loop.Step == nil || b.pitfallIntegerEquals(loop.Step, 1)
	scan.block(loop.Body, pitfallEndpointFlow{normal: true, endpoint: true, stable: unitStep && !scan.bodyMayChangeLength(loop.Body)})
}

// joinEndpointFlow retains every continuing path; endpoint/Len identity is
// usable only if it survives every endpoint path that reaches the join.
// Rules: rules/analysis/pitfall_analysis.md — "Reachability", "Guards participate in pitfall reasoning".
func joinEndpointFlow(left, right pitfallEndpointFlow) pitfallEndpointFlow {
	return pitfallEndpointFlow{
		normal: left.normal || right.normal, endpoint: left.endpoint || right.endpoint,
		stable: (!left.endpoint || left.stable) && (!right.endpoint || right.stable),
	}
}

// branch filters only proven boolean outcomes, retaining unknown paths.
// Rules: rules/analysis/pitfall_analysis.md — "Semantic recognition, not syntax matching", "Reachability".
func (s *pitfallEndpointScan) branch(flow pitfallEndpointFlow, condition ast.Expression, selected bool) pitfallEndpointFlow {
	if value, known := s.builder.analyzer.constantBooleanValue(condition); known && value != selected {
		flow.normal, flow.endpoint = false, false
	}
	if value, known := s.condition(condition, flow.stable); known && value != selected {
		flow.endpoint = false
	}
	return flow
}

// condition evaluates the bounded boolean proof domain at i == initial Len.
// Negation, conjunction/disjunction and reversed comparisons compose proofs;
// a changed collection or an unrelated resolved subject remains unknown.
// Rules: rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning";
// rules/foundations/operators.md — "Logical operators".
func (s *pitfallEndpointScan) condition(expression ast.Expression, stable bool) (bool, bool) {
	if value, known := s.builder.analyzer.constantBooleanValue(expression); known {
		return value, true
	}
	switch expression := expression.(type) {
	case *ast.PrefixExpression:
		if expression.Operator == "!" {
			value, known := s.condition(expression.Right, stable)
			return !value, known
		}
	case *ast.InfixExpression:
		if expression.Operator == "&&" || expression.Operator == "||" {
			left, leftKnown := s.condition(expression.Left, stable)
			right, rightKnown := s.condition(expression.Right, stable)
			if expression.Operator == "&&" {
				if leftKnown && !left || rightKnown && !right {
					return false, true
				}
				return left && right, leftKnown && rightKnown
			}
			if leftKnown && left || rightKnown && right {
				return true, true
			}
			return left || right, leftKnown && rightKnown
		}
		if stable && (s.bindingAndLength(expression.Left, expression.Right) || s.bindingAndLength(expression.Right, expression.Left)) {
			switch expression.Operator {
			case "==", "<=", ">=":
				return true, true
			case "!=", "<", ">":
				return false, true
			}
		}
	}
	return false, false
}

// bindingAndLength correlates the loop definition and compiler-known Len
// receiver using Sema identity, never spelling.
// Rule: rules/analysis/pitfall_analysis.md — "Semantic correlation, not spelling heuristics".
func (s *pitfallEndpointScan) bindingAndLength(index, length ast.Expression) bool {
	if !s.builder.expressionUsesBinding(index, s.binding) {
		return false
	}
	collection, _, ok := s.builder.lengthReceiver(length)
	return ok && collection == s.collection
}

// block walks only normal reachable statements, retaining endpoint exclusion
// for accesses on non-endpoint paths. Termination affects later statements.
// Rules: rules/analysis/pitfall_analysis.md — "Reachability";
// rules/control-flow/flowcontrol_for.md — §§30–31.
func (s *pitfallEndpointScan) block(block *ast.BlockStatement, flow pitfallEndpointFlow) pitfallEndpointFlow {
	if block == nil {
		return flow
	}
	for _, statement := range block.Statements {
		if !flow.normal {
			break
		}
		flow = s.statement(statement, flow)
	}
	return flow
}

// statement tracks branch-local exits and joins. Inner loop break/continue
// affect that loop only. While continuation requires a false condition or an
// exit from its own break frame; finite for loops retain their zero-iteration path.
// Sema-owned never-executed if paths are omitted. Unknown forms cannot prove
// an endpoint exit merely by containing a transfer statement.
// Rules: rules/analysis/pitfall_analysis.md — "Reachability", "Guards participate in pitfall reasoning";
// rules/control-flow/flowcontrol_if.md — §§19–20, 27;
// rules/control-flow/flowcontrol_for.md — §§30–31; rules/errors/panic.md — §§15–17.
func (s *pitfallEndpointScan) statement(statement ast.Statement, flow pitfallEndpointFlow) pitfallEndpointFlow {
	switch statement := statement.(type) {
	case *ast.IfStatement:
		flow = s.expression(statement.Condition, flow)
		left := s.branch(flow, statement.Condition, true)
		right := s.branch(flow, statement.Condition, false)
		if resolved, ok := s.builder.analyzer.ResolvedIfFlowOf(statement); ok {
			if resolved.TruePathExecution == ResolvedIfPathNever {
				left.normal, left.endpoint = false, false
			}
			if resolved.FalsePathExecution == ResolvedIfPathNever {
				right.normal, right.endpoint = false, false
			}
		}
		return joinEndpointFlow(s.block(statement.Consequence, left), s.block(statement.Alternative, right))
	case *ast.UnsafeStatement:
		return s.block(statement.Body, flow)
	case *ast.ForStatement:
		flow = s.expression(statement.Iterable, flow)
		flow = s.expression(statement.Step, flow)
		breaks := []pitfallEndpointFlow{}
		s.loopBreaks = append(s.loopBreaks, &breaks)
		body := s.block(statement.Body, flow)
		s.loopBreaks = s.loopBreaks[:len(s.loopBreaks)-1]
		flow.stable = flow.stable && body.stable
		return flow
	case *ast.WhileStatement:
		flow = s.expression(statement.Condition, flow)
		breaks := []pitfallEndpointFlow{}
		s.loopBreaks = append(s.loopBreaks, &breaks)
		body := s.block(statement.Body, s.branch(flow, statement.Condition, true))
		s.loopBreaks = s.loopBreaks[:len(s.loopBreaks)-1]
		flow.stable = flow.stable && body.stable
		continuation := s.branch(flow, statement.Condition, false)
		for _, exit := range breaks {
			continuation = joinEndpointFlow(continuation, exit)
		}
		return continuation
	case *ast.DeferStatement:
		// Deferred execution is not the current body continuation.
		return flow
	case *ast.AssertStatement:
		flow = s.expression(statement.Condition, flow)
		return s.branch(flow, statement.Condition, true)
	case *ast.ReturnStatement:
		flow = s.expression(statement.Value, flow)
		flow.normal, flow.endpoint = false, false
		return flow
	case *ast.BreakStatement:
		if len(s.loopBreaks) != 0 {
			frame := s.loopBreaks[len(s.loopBreaks)-1]
			*frame = append(*frame, flow)
		}
		flow.normal, flow.endpoint = false, false
		return flow
	case *ast.ContinueStatement, *ast.PanicStatement, *ast.UnreachableStatement:
		flow.normal, flow.endpoint = false, false
		return flow
	case *ast.LetStatement:
		return s.expression(statement.Value, flow)
	case *ast.LetGroupStatement:
		for _, item := range statement.Lets {
			flow = s.expression(item.Value, flow)
		}
		return flow
	case *ast.AssignmentStatement:
		flow = s.expression(statement.Target, flow)
		flow = s.expression(statement.Value, flow)
		if identity, ok := s.builder.expressionIdentity(statement.Target); ok && identity == s.collection {
			flow.stable = false
		}
		return flow
	case *ast.ExpressionStatement:
		return s.expression(statement.Expression, flow)
	case *ast.DiscardStatement:
		return s.expression(statement.Value, flow)
	default:
		// Unsupported control-flow domains remain conservative. They may
		// contain accesses, but their nested exits do not discharge this loop.
		visitStatementExpressions(statement, func(expression ast.Expression) {
			if index, ok := expression.(*ast.IndexExpression); ok {
				s.index(index, flow)
			}
			if len(s.builder.mutationsInExpression(expression, s.collection, false)) != 0 {
				flow.stable = false
			}
		})
		return flow
	}
}

// expression follows left-to-right evaluation and short-circuit RHS paths;
// callable bodies are separate execution domains. Writes invalidate the live
// Len relation before later comparisons can use it as the saved loop endpoint.
// Rules: rules/analysis/pitfall_analysis.md — "Reachability", "Guards participate in pitfall reasoning";
// rules/foundations/operators.md — "Logical operators"; rules/control-flow/flowcontrol_for.md — §§12, 39.
func (s *pitfallEndpointScan) expression(expression ast.Expression, flow pitfallEndpointFlow) pitfallEndpointFlow {
	if parameterUsageNodeIsNil(expression) || !flow.normal {
		return flow
	}
	switch expression := expression.(type) {
	case *ast.InfixExpression:
		flow = s.expression(expression.Left, flow)
		if expression.Operator == "&&" || expression.Operator == "||" {
			selected := expression.Operator == "&&"
			skipped := s.branch(flow, expression.Left, !selected)
			right := s.expression(expression.Right, s.branch(flow, expression.Left, selected))
			return joinEndpointFlow(skipped, right)
		}
		return s.expression(expression.Right, flow)
	case *ast.PrefixExpression:
		return s.expression(expression.Right, flow)
	case *ast.IndexExpression:
		flow = s.expression(expression.Left, flow)
		flow = s.expression(expression.Index, flow)
		s.index(expression, flow)
	case *ast.MemberExpression:
		return s.expression(expression.Object, flow)
	case *ast.CallExpression:
		flow = s.expression(expression.Callee, flow)
		for _, argument := range expression.Arguments {
			flow = s.expression(argument, flow)
		}
		if len(s.builder.mutationsInExpression(expression, s.collection, false)) != 0 {
			flow.stable = false
		}
		if s.callMayChangeLength(expression) {
			flow.stable = false
		}
	case *ast.RefExpression:
		flow = s.expression(expression.Value, flow)
		if expression.Mutable {
			flow.stable = false
		}
	case *ast.ConversionExpression:
		return s.expression(expression.Value, flow)
	case *ast.SliceExpression:
		flow = s.expression(expression.Left, flow)
		flow = s.expression(expression.Start, flow)
		return s.expression(expression.End, flow)
	case *ast.RangeExpression:
		flow = s.expression(expression.Start, flow)
		return s.expression(expression.End, flow)
	case *ast.ArrayLiteral:
		for _, item := range expression.Elements {
			flow = s.expression(item, flow)
		}
	case *ast.StructLiteral:
		for _, field := range expression.Fields {
			flow = s.expression(field.Value, flow)
		}
	case *ast.SpreadExpression:
		return s.expression(expression.Value, flow)
	case *ast.TryExpression:
		return s.expression(expression.Expression, flow)
	case *ast.OkExpression:
		flow = s.expression(expression.Value, flow)
		for _, argument := range expression.Arguments {
			flow = s.expression(argument, flow)
		}
	case *ast.ErrExpression:
		flow = s.expression(expression.Value, flow)
		for _, argument := range expression.Arguments {
			flow = s.expression(argument, flow)
		}
	case *ast.RuntimeCallExpression:
		for _, argument := range expression.Arguments {
			flow = s.expression(argument, flow)
		}
		flow.stable = false
	case *ast.LambdaExpression:
		return flow
	case *ast.SpawnExpression:
		return s.expression(expression.Value, flow)
	case *ast.AwaitExpression:
		flow = s.expression(expression.Value, flow)
		flow.stable = false
	}
	return flow
}

// index retains suppression evidence only for accesses unreachable at the
// endpoint, while preserving the ordinary bounds owner and heuristic edit.
// Rules: rules/analysis/pitfall_analysis.md — "Inclusive upper bound against collection length", "Analysis states".
func (s *pitfallEndpointScan) index(index *ast.IndexExpression, flow pitfallEndpointFlow) {
	collection, ok := s.builder.expressionIdentity(index.Left)
	if !flow.normal || !ok || collection != s.collection || !s.builder.expressionUsesBinding(index.Index, s.binding) {
		return
	}
	if flow.endpoint && !flow.stable {
		// A saved endpoint may no longer equal live Len, or an explicit
		// stride may skip it. No endpoint invalidity or exclusion is proven.
		if evaluation := s.builder.counts[PitfallInclusiveLengthIndex]; evaluation != nil {
			evaluation.Incomplete = true
			if evaluation.FindingCount == 0 && evaluation.SuppressedCount == 0 {
				evaluation.State = PitfallStateNotEvaluated
			}
		}
		return
	}
	finding := PitfallFinding{
		Rule: PitfallInclusiveLengthIndex, Family: PitfallBoundsAndRanges,
		Classification: PitfallProvenInvalid, Confidence: PitfallConfidenceProven,
		Subject: PitfallSubject{Expression: index.String(), Source: index.Token},
		EvidenceFor: []PitfallEvidence{
			{Strength: PitfallEvidenceProof, Fact: "the inclusive range reaches the collection Len", Source: s.domain.Token},
			{Strength: PitfallEvidenceProof, Fact: "the same loop binding indexes the same collection", Source: index.Token},
		},
		OwningRule: "bounds",
		Actions:    []PitfallSuggestedAction{{Kind: PitfallSuggestedEdit, Title: "use the canonical half-open range", Replacement: "..<", Source: s.domain.Token}},
	}
	if !flow.endpoint {
		evidence := PitfallEvidence{Strength: PitfallEvidenceSuppressing, Fact: "control flow excludes the inclusive endpoint before this indexed access", Source: s.length}
		finding.State = PitfallStateSuppressed
		finding.EvidenceAgainst = []PitfallEvidence{evidence}
		finding.Suppression = &PitfallSuppression{Reason: "the indexed access is unreachable when the loop binding equals Len", Evidence: []PitfallEvidence{evidence}}
	}
	s.builder.add(finding)
}
