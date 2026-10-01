package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// ResolvedStateTest is the frontend fact for one non-binding `is Variant` or
// `is empty` test. Binding names the tested local union binding when the
// subject is a plain identifier; StaticallyKnown and Value record a proven
// outcome, such as `is empty` on a definitely initialized binding.
//
// Rules:
//   - rules/declarations/unions.md — §8 "`is` tests for union state and active variant"
type ResolvedStateTest struct {
	SubjectType     Type
	Variant         string
	Empty           bool
	Binding         string
	MaybeEmpty      bool
	StaticallyKnown bool
	Value           bool
}

// ResolvedStateTestOf returns the fact recorded by completed semantic
// analysis without re-resolving the subject or variant.
func (a *Analyzer) ResolvedStateTestOf(expr *ast.StateTestExpression) (ResolvedStateTest, bool) {
	if a == nil || expr == nil {
		return ResolvedStateTest{}, false
	}
	fact, ok := a.resolvedStateTests[expr]
	return fact, ok
}

// inferStateTestExpression resolves `value is Variant` and `value is empty`.
// A local union binding whose initialization state may still be empty is
// inspected without a value read, because both tests observe initialization
// state; every other subject is read normally, so moved-from bindings remain
// ownership errors rather than an observable empty state.
//
// Rules:
//   - rules/declarations/unions.md — §7.1 "Purpose", §7.6 "Moved-from is not empty"
//   - rules/declarations/unions.md — §8.1 "Active variant test", §8.2 "Empty-state test", §8.4 "Impossible state tests"
//   - rules/declarations/unions.md — §12 "Result and Option"
//   - rules/control-flow/flowcontrol_if.md — §12 "State tests with `is`"
//   - rules/control-flow/flowcontrol_while.md — §8 "`is` state tests"
func (a *Analyzer) inferStateTestExpression(expr *ast.StateTestExpression) (Type, expressionValue) {
	invalid := Type{Kind: InvalidType}
	display := expressionValue{Display: expr.String()}
	fact := ResolvedStateTest{Empty: expr.Empty}

	identifier, isIdentifier := expr.Subject.(*ast.Identifier)
	if expr.Empty && !isIdentifier {
		a.addErrorAtToken(expressionToken(expr.Subject), "is empty requires a local union binding")
		return invalid, display
	}
	var subjectType Type
	if isIdentifier {
		fact.Binding = identifier.Value
		if assigned, tracked := a.assigned[identifier.Value]; tracked && !assigned {
			if symbol, ok := a.symbols[identifier.Value]; ok {
				a.bindDefinition(identifier.Token, symbol.Token)
				subjectType = symbol.Type
				fact.MaybeEmpty = true
				a.expressionTypes[expr.Subject] = subjectType
			}
		}
	}
	if !fact.MaybeEmpty {
		subjectType, _ = a.inferExpression(expr.Subject)
	}
	if subjectType.Kind == InvalidType {
		return invalid, display
	}
	fact.SubjectType = subjectType

	if expr.Empty {
		if subjectType.Kind != UnionType || subjectType.Name == "Option" {
			a.addErrorAtToken(expressionToken(expr.Subject), "is empty requires a local union binding, got %s", typeDisplayName(subjectType))
			return invalid, display
		}
		if !fact.MaybeEmpty {
			fact.StaticallyKnown, fact.Value = true, false
		}
		a.recordStateTest(expr, fact)
		return Type{Name: "bool", Kind: BoolType}, display
	}

	variants, ok := stateTestVariants(subjectType)
	if !ok {
		a.addErrorAtToken(expr.Variant.Token, "is %s requires a union value, got %s", expr.Variant.Value, typeDisplayName(subjectType))
		return invalid, display
	}
	if expr.Owner != nil && !a.stateTestOwnerMatches(expr.Owner.Value, subjectType) {
		a.addErrorAtToken(expr.Owner.Token, "is test qualifier %s does not match subject type %s", expr.Owner.Value, typeDisplayName(subjectType))
		return invalid, display
	}
	variant, found := variants[expr.Variant.Value]
	if !found {
		a.addErrorAtToken(expr.Variant.Token, "%s has no variant %s", typeDisplayName(subjectType), expr.Variant.Value)
		return invalid, display
	}
	if variant.Token.Type != "" {
		a.bindDefinition(expr.Variant.Token, variant.Token)
	}
	fact.Variant = expr.Variant.Value
	a.recordStateTest(expr, fact)
	return Type{Name: "bool", Kind: BoolType}, display
}

// recordStateTest publishes a state-test fact. A loop condition is analyzed
// again at the backedge, where the binding may already be initialized; the
// fact keeps the weakest knowledge so a possibly-empty observation is never
// replaced by a later proof that holds only on some iterations.
func (a *Analyzer) recordStateTest(expr *ast.StateTestExpression, fact ResolvedStateTest) {
	if previous, exists := a.resolvedStateTests[expr]; exists && previous.MaybeEmpty && !fact.MaybeEmpty {
		fact.MaybeEmpty = true
		fact.StaticallyKnown, fact.Value = false, false
	}
	a.resolvedStateTests[expr] = fact
}

// stateTestVariants returns the declared variant set an `is Variant` test may
// name: union variants, including Option, and the Result variants Ok and Err.
func stateTestVariants(typ Type) (map[string]UnionVariant, bool) {
	switch typ.Kind {
	case UnionType:
		variants := make(map[string]UnionVariant, len(typ.UnionVariants))
		for _, variant := range typ.UnionVariants {
			variants[variant.Name] = variant
		}
		return variants, true
	case ResultType:
		return map[string]UnionVariant{"Ok": {Name: "Ok"}, "Err": {Name: "Err"}}, true
	}
	return nil, false
}

func (a *Analyzer) stateTestOwnerMatches(owner string, subjectType Type) bool {
	if owner == subjectType.Name {
		return true
	}
	declared, ok := a.types[owner]
	return ok && declared.Name == subjectType.Name
}

// stateTestRefinement returns the local binding a state test proves
// initialized on the given branch: the true branch of `is Variant` and the
// false branch of `is empty`.
//
// Rules:
//   - rules/declarations/unions.md — §8.3 "Data-flow refinement"
func stateTestRefinement(fact ResolvedStateTest, trueBranch bool) (string, bool) {
	if fact.Binding == "" || !fact.MaybeEmpty {
		return "", false
	}
	if fact.Empty == trueBranch {
		return "", false
	}
	return fact.Binding, true
}

// diagnoseImpossibleStateTestBlock reports the first statement of a branch
// that a proven-false state test can never enter through the canonical
// unreachable-code diagnostic.
//
// Rules:
//   - rules/declarations/unions.md — §8.4 "Impossible state tests"
//   - rules/tooling/diagnostics.md — § 21 "Proven unreachable and dead code"
func (a *Analyzer) diagnoseImpossibleStateTestBlock(block *ast.BlockStatement, binding string) {
	if block == nil || len(block.Statements) == 0 {
		return
	}
	a.addErrorAtTokenWithMetadata(
		statementToken(block.Statements[0]),
		diagnostics.UnreachableStatement,
		"The binding "+binding+" is always initialized here, so the is empty test is never true. Remove the test.",
		"unreachable statement",
	)
}

// analyzeStateTestBranch analyzes one if branch with the initialization
// refinement proven by its state test applied on entry, restoring the incoming
// assignment state after the branch result is captured.
//
// Rules:
//   - rules/declarations/unions.md — §8.3 "Data-flow refinement"
func (a *Analyzer) analyzeStateTestBranch(block *ast.BlockStatement, fact ResolvedStateTest, trueBranch bool, reachable bool) branchAnalysis {
	binding, refined := stateTestRefinement(fact, trueBranch)
	if !refined {
		return a.analyzeBranchBlockWithCallGraphReachability(block, reachable)
	}
	previous := a.assigned
	a.assigned = copyAssigned(previous)
	a.assigned[binding] = true
	branch := a.analyzeBranchBlockWithCallGraphReachability(block, reachable)
	a.assigned = previous
	return branch
}

// inferMatchSubject resolves a match subject. A local union binding whose
// initialization state may still be empty is inspected without a value read
// and its name is returned so the match can require and refine the empty
// state; every other subject is read normally.
//
// Rules:
//   - rules/declarations/unions.md — §7.1 "Purpose", §10.1 "Empty pattern", §10.2 "Exhaustiveness with possible empty state"
func (a *Analyzer) inferMatchSubject(subject ast.Expression) (Type, string) {
	if identifier, ok := subject.(*ast.Identifier); ok {
		if assigned, tracked := a.assigned[identifier.Value]; tracked && !assigned {
			if symbol, ok := a.symbols[identifier.Value]; ok && symbol.Type.Kind == UnionType && symbol.Type.Name != "Option" {
				a.bindDefinition(identifier.Token, symbol.Token)
				a.expressionTypes[subject] = symbol.Type
				return symbol.Type, identifier.Value
			}
		}
	}
	subjectType, _ := a.inferExpression(subject)
	return subjectType, ""
}

// admitEmptyMatchArm validates one `empty` arm: it is reachable only when the
// subject may be empty, it may appear once unguarded, and it is unreachable
// after an unguarded catch-all.
//
// Rules:
//   - rules/declarations/unions.md — §10.1 "Empty pattern", §10.3 "Known initialized subjects", §10.4 "Catch-all"
func (a *Analyzer) admitEmptyMatchArm(expr *ast.MatchExpression, arm *ast.MatchArm, maybeEmptySubject string, catchAll bool, seenEmpty *bool) bool {
	if maybeEmptySubject == "" {
		a.addErrorAtToken(arm.Pattern.Token, "unreachable empty match arm; %s is always initialized here", expr.Subject.String())
		return false
	}
	if catchAll {
		a.addErrorAtToken(arm.Token, "unreachable match arm")
		return false
	}
	if arm.Guard != nil {
		return true
	}
	if *seenEmpty {
		a.addErrorAtToken(arm.Token, "duplicate empty match arm")
		return false
	}
	*seenEmpty = true
	return true
}
