package sema

import (
	"fmt"
	"sort"

	"sec/internal/ast"
)

// CompileTimeDependencyKind categorizes what a semantic CTE outcome depends
// on (compile_time_evaluation.md §§ 58, 60(2)–(3)).
type CompileTimeDependencyKind string

const (
	// CTEDependsOnDeclaration: an executed function or getter declaration.
	CTEDependsOnDeclaration CompileTimeDependencyKind = "DeclarationSemantics"
	// CTEDependsOnImmutableValue: an immutable compile-time-established value
	// the evaluation read, such as an immutable static.
	CTEDependsOnImmutableValue CompileTimeDependencyKind = "ImmutableCompileTimeValue"
	// CTEDependsOnGenericInstance: a concrete generic instance executed.
	CTEDependsOnGenericInstance CompileTimeDependencyKind = "GenericConcreteInstance"
	// CTEDependsOnTypeFact: a nominal type whose declaration, contract or
	// variants the evaluation consulted.
	CTEDependsOnTypeFact CompileTimeDependencyKind = "TypeSemanticFact"
	// CTEDependsOnLayoutFact: a layout fact such as the size of a type.
	CTEDependsOnLayoutFact CompileTimeDependencyKind = "LayoutFact"
	// CTEDependsOnCompilationPlan: a target fact of the CompilationPlan, such
	// as the platform scalar width of int, uint and float.
	CTEDependsOnCompilationPlan CompileTimeDependencyKind = "CompilationPlanFact"
	// CTEDependsOnImportedModule: semantic artifacts of another module whose
	// declarations or values the evaluation used.
	CTEDependsOnImportedModule CompileTimeDependencyKind = "ImportedSemanticArtifact"
)

// CompileTimeDependency is one conservative dependency of a CTE outcome.
type CompileTimeDependency struct {
	Kind     CompileTimeDependencyKind
	Identity string
}

// depend records a dependency of the executing request. Dependencies are
// collected per call and merged into the caller when the call returns, so a
// memoized call result carries the dependencies of its execution.
func (x *cteExecutor) depend(kind CompileTimeDependencyKind, identity string) {
	if identity == "" {
		return
	}
	if len(x.dependencies) == 0 {
		x.dependencies = []map[CompileTimeDependency]bool{{}}
	}
	x.dependencies[len(x.dependencies)-1][CompileTimeDependency{Kind: kind, Identity: identity}] = true
}

// dependOnModule records an imported module when a declaration or value of
// another module than the request's is used.
func (x *cteExecutor) dependOnModule(module string) {
	if module != "" && module != x.rootModule {
		x.depend(CTEDependsOnImportedModule, module)
	}
}

// dependOnType records the type facts and CompilationPlan facts a value of
// typ consults: a nominal type's declaration, and the platform scalar width
// for int, uint and float, which the CompilationPlan selects. Type arguments
// and elements are followed.
func (x *cteExecutor) dependOnType(typ Type) {
	switch {
	case typ.Kind == "" || typ.Kind == InvalidType:
		return
	case typ.Named && !typ.Intrinsic && typ.Module != "":
		x.depend(CTEDependsOnTypeFact, canonicalTypeIdentity(typ))
		x.dependOnModule(typ.Module)
	case !typ.Named && (typ.Name == "int" || typ.Name == "uint" || typ.Name == "float"):
		// The platform width of int, uint and float is a layout fact of the
		// plan; its layout fingerprint identifies it (platform_model.md § 23).
		width := "default"
		if bits := x.a.ScalarPlanWidthBits(); bits != 0 {
			width = fmt.Sprint(bits)
		}
		identity := "platform scalar width " + width
		if layoutFingerprint := x.a.PlanFingerprints().Layout; layoutFingerprint != "" {
			identity += " (layout " + layoutFingerprint[:16] + ")"
		}
		x.depend(CTEDependsOnCompilationPlan, identity)
	}
	for _, argument := range typ.TypeArgs {
		x.dependOnType(argument)
	}
	if typ.Element != nil {
		x.dependOnType(*typ.Element)
	}
}

// dependOnLayout records a layout fact the expression consults: a
// compiler-known SizeOf of a type or value.
func (x *cteExecutor) dependOnLayout(expr ast.Expression) {
	member, ok := expr.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return
	}
	known, ok := x.a.compilerKnownMemberFacts[sourceTokenLocation(member.Property.Token)]
	if !ok || (known.ID != "CKM-SIZEOF-TYPE" && known.ID != "CKM-SIZEOF-VALUE") {
		return
	}
	subject, ok := x.a.ResolvedTypeOf(member.Object)
	if !ok {
		subject = Type{Name: member.Object.String()}
	}
	x.depend(CTEDependsOnLayoutFact, "SizeOf "+canonicalTypeIdentity(subject))
}

// pushDependencies starts collecting a call's dependencies.
func (x *cteExecutor) pushDependencies() {
	if len(x.dependencies) == 0 {
		x.dependencies = []map[CompileTimeDependency]bool{{}}
	}
	x.dependencies = append(x.dependencies, map[CompileTimeDependency]bool{})
}

// popDependencies merges a finished call's dependencies into its caller and
// returns them in canonical order.
func (x *cteExecutor) popDependencies() []CompileTimeDependency {
	top := x.dependencies[len(x.dependencies)-1]
	x.dependencies = x.dependencies[:len(x.dependencies)-1]
	for dependency := range top {
		x.dependencies[len(x.dependencies)-1][dependency] = true
	}
	return sortedDependencies(top)
}

// mergeDependencies adds a memoized call's recorded dependencies.
func (x *cteExecutor) mergeDependencies(dependencies []CompileTimeDependency) {
	for _, dependency := range dependencies {
		x.depend(dependency.Kind, dependency.Identity)
	}
}

func sortedDependencies(set map[CompileTimeDependency]bool) []CompileTimeDependency {
	out := make([]CompileTimeDependency, 0, len(set))
	for dependency := range set {
		out = append(out, dependency)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Identity < out[j].Identity
	})
	return out
}

// CompileTimeDependencies returns the conservative dependencies of the
// required evaluation rooted at expr, for successful and failed outcomes
// alike: executed declarations and generic instances, immutable values read,
// type facts, CompilationPlan facts and imported modules. A change to any of
// them invalidates the outcome; nothing outside the set can change it.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 58, 60(2)–(3), 61(2).
func (a *Analyzer) CompileTimeDependencies(expr ast.Expression) ([]CompileTimeDependency, bool) {
	if a == nil || expr == nil {
		return nil, false
	}
	result, ok := a.cteResults[expr]
	if !ok {
		return nil, false
	}
	return append([]CompileTimeDependency(nil), result.Dependencies...), true
}
