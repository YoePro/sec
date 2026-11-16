package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// Sema publishes the cleanup registration of a generic-dependent value with a
// symbolic destruction requirement. A specialization resolves it against its
// concrete substitution: a concrete no-op destruction drops the registration
// (it is then an ordinary trivial value), every other one is tracked with the
// concrete type's destruction plans (importDestructionPlans).
// Rules: rules/compiler/monomorphization.md — §§ 11.2, 11.4, 12.3.

// concreteRegistration reports whether a registration still needs tracking
// in the function being built.
func (b *builder) concreteRegistration(registration sema.ResolvedOwnedCleanupRegistration) bool {
	return b.concreteObligation(registration.Place.Type)
}

// concreteObligation reports whether a value of the registered Place type
// still has a destruction obligation after substitution.
func (b *builder) concreteObligation(typ sema.Type) bool {
	if len(b.substitution) == 0 {
		return true
	}
	return !b.concreteTriviallyDestructible(typ)
}

func (fb *functionBuilder) ownedCleanupInitializationOf(stmt *ast.LetStatement) (sema.ResolvedOwnedCleanupRegistration, bool) {
	registration, ok := fb.owner.analyzer.ResolvedOwnedCleanupInitializationOf(stmt)
	return registration, ok && fb.owner.concreteRegistration(registration)
}

func (fb *functionBuilder) ownedCleanupParameterOf(binding sema.BindingID) (sema.ResolvedOwnedCleanupRegistration, bool) {
	registration, ok := fb.owner.analyzer.ResolvedOwnedCleanupParameterOf(binding)
	return registration, ok && fb.owner.concreteRegistration(registration)
}

func (fb *functionBuilder) ownedCleanupOptionBindingOf(stmt *ast.IfStatement) (sema.ResolvedOwnedCleanupRegistration, bool) {
	registration, ok := fb.owner.analyzer.ResolvedOwnedCleanupOptionBindingOf(stmt)
	return registration, ok && fb.owner.concreteRegistration(registration)
}

func (fb *functionBuilder) ownedCleanupMatchBindingOf(arm *ast.MatchArm) (sema.ResolvedOwnedCleanupRegistration, bool) {
	registration, ok := fb.owner.analyzer.ResolvedOwnedCleanupMatchBindingOf(arm)
	return registration, ok && fb.owner.concreteRegistration(registration)
}

func (fb *functionBuilder) ownedCleanupHandlerBindingOf(handler *ast.TryHandler) (sema.ResolvedOwnedCleanupRegistration, bool) {
	registration, ok := fb.owner.analyzer.ResolvedOwnedCleanupHandlerBindingOf(handler)
	return registration, ok && fb.owner.concreteRegistration(registration)
}

func (fb *functionBuilder) ownedCleanupAssignmentOf(stmt *ast.AssignmentStatement) (sema.ResolvedOwnedCleanupAssignment, bool) {
	assignment, ok := fb.owner.analyzer.ResolvedOwnedCleanupAssignmentOf(stmt)
	return assignment, ok && fb.owner.concreteRegistration(assignment.Registration)
}

// concreteDestructionPlans resolves a registration's destruction plans for
// the type being built: in a specialization the template's symbolic plan set
// is replaced by the substituted type's concrete plans.
func (b *builder) concreteDestructionPlans(set sema.DestructionPlanSet, typ sema.Type) (sema.DestructionPlanSet, sema.Type) {
	if len(b.substitution) == 0 {
		return set, typ
	}
	typ = b.substituteTemplateType(typ, b.substitution)
	return sema.ResolveDestructionPlans(typ), typ
}

// matchPlanOf is Sema's match plan with each template binding whose
// copyability was unproven resolved for the concrete specialization: a copy of
// a trivially copyable payload, otherwise the move template validity required.
// Rules: rules/compiler/monomorphization.md — § 11.2; rules/memory/copy_move.md — §§ 3–5.
func (b *builder) matchPlanOf(expr *ast.MatchExpression) (sema.ResolvedMatchPlan, bool) {
	plan, ok := b.analyzer.ResolvedMatchPlanOf(expr)
	if !ok || len(b.substitution) == 0 {
		return plan, ok
	}
	arms := append([]sema.ResolvedMatchArm(nil), plan.Arms...)
	for index := range arms {
		if arms[index].BindingAction != sema.MatchBindingConditional {
			continue
		}
		if b.concreteTriviallyCopyable(arms[index].BindingType) {
			arms[index].BindingAction = sema.MatchBindingCopyTrivial
		} else {
			arms[index].BindingAction = sema.MatchBindingMove
		}
	}
	plan.Arms = arms
	return plan, ok
}

// triviallyDestructible, triviallyCopyable and copyClassificationOf classify a
// type taken from Sema's facts. Inside a specialization those facts describe
// the template, so the type is substituted first; elsewhere they are Sema's
// classification unchanged.
// Rules: rules/compiler/monomorphization.md — §§ 11.2, 11.4.
func (b *builder) triviallyDestructible(typ sema.Type) bool {
	return sema.TriviallyDestructible(b.concreteType(typ))
}

func (b *builder) triviallyCopyable(typ sema.Type) bool {
	return sema.TriviallyCopyable(b.concreteType(typ))
}

func (b *builder) copyClassificationOf(typ sema.Type) sema.CopyClassification {
	return sema.CopyClassificationOf(b.concreteType(typ))
}

func (b *builder) concreteType(typ sema.Type) sema.Type {
	if len(b.substitution) == 0 {
		return typ
	}
	return b.substituteTemplateType(typ, b.substitution)
}

// referenceUseOf is Sema's reference use with its type-dependent read and
// replacement classification resolved for the concrete specialization.
func (b *builder) referenceUseOf(expr ast.Expression) (sema.ResolvedReferenceUse, bool) {
	use, ok := b.analyzer.ResolvedReferenceUseOf(expr)
	if !ok || len(b.substitution) == 0 {
		return use, ok
	}
	use.CopyTrivialRead = use.Kind == sema.ReferenceUseRead && b.copyClassificationOf(use.Place.Type) == sema.CopyTrivial
	use.TrivialReplacement = use.Kind == sema.ReferenceUseWrite && b.triviallyDestructible(use.Place.Type)
	return use, ok
}
