package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// builtinThreadStorageType defines a non-generic noCopy storage owner. The
// private capacity field is metadata, not the target-managed backing itself.
// Rules: rules/concurrency/threads.md — §8(1–7).
func builtinThreadStorageType() Type {
	return Type{Name: "ThreadStorage", Module: "core", Kind: StructType,
		Named: true, Declared: true, Intrinsic: true, ExplicitlyNonCopyable: true,
		NoCopyPolicyOrigin: "ThreadStorage",
		Fields:             []StructField{{Name: "_capacity", Type: Type{Name: "uint", Kind: UintType}}}}
}

// registerThreadStorageDeclaration accepts the canonical noCopy non-generic
// capacity metadata struct from trusted core; ordinary source cannot replace it.
// Rules: rules/concurrency/threads.md — §8(1–2),(6–7).
func (a *Analyzer) registerThreadStorageDeclaration(stmt *ast.TypeDeclStatement) bool {
	if stmt.Name.Value != "ThreadStorage" || !a.isTrustedCoreBuiltinDeclaration("ThreadStorage", stmt.Name.Token) {
		return false
	}
	typ := builtinThreadStorageType()
	typ.Fields[0].Type = a.types["uint"]
	valid := stmt.StructType != nil && stmt.BaseType == nil && stmt.AssignedType == nil && stmt.RegisterType == nil && !stmt.Union && !stmt.ErrorType && stmt.Contract == nil && stmt.Default == nil && len(stmt.Implements) == 0 && len(stmt.GenericParameters) == 0 && len(stmt.Attributes) == 1 && hasAttribute(stmt.Attributes, "noCopy")
	if valid {
		fields := stmt.StructType.Fields
		valid = len(fields) == 1 && fields[0] != nil && fields[0].Name != nil && fields[0].Name.Value == "_capacity" && fields[0].Type != nil && typeReferenceDisplayName(fields[0].Type) == "uint" && fields[0].Contract == nil && len(fields[0].Tags) == 0
		if valid {
			typ.Fields[0].Token = fields[0].Name.Token
		}
	}
	if !valid {
		a.addErrorAtToken(stmt.Name.Token, "ThreadStorage requires its canonical non-generic @noCopy core struct with only _capacity: uint")
	}
	typ.DeclarationToken = stmt.Name.Token
	a.types["ThreadStorage"] = typ
	return true
}

// rejectThreadStorageNonStruct retains the canonical metadata declaration form.
// Rules: rules/concurrency/threads.md — §8(1–2).
func (a *Analyzer) rejectThreadStorageNonStruct(name string, token lexer.Token) bool {
	if name != "ThreadStorage" || !a.isTrustedCoreBuiltinDeclaration(name, token) {
		return false
	}
	a.addErrorAtToken(token, "ThreadStorage requires its canonical non-generic @noCopy core struct")
	a.invalidTypeDeclarations[sourceTokenLocation(token)] = true
	return true
}

// inferThreadStorageConstructor validates the compile-time uint capacity and
// records it separately from type identity. Physical backing needs target facts
// and is deliberately not fabricated as a normal source aggregate.
// Rules: rules/concurrency/threads.md — §8(1–7);
// rules/compiler/compile_time_evaluation.md — SemanticCompileTimeRequiredContext.
func (a *Analyzer) inferThreadStorageConstructor(expr *ast.CallExpression) (Type, expressionValue, bool) {
	display := expressionValue{Display: expr.String()}
	invalid := Type{Kind: InvalidType}
	if !a.checkCompilerKnownCallArity(expr, "ThreadStorage", 1, 1) {
		return invalid, display, true
	}
	argument := expr.Arguments[0]
	expected := a.types["uint"]
	actual, _ := a.inferExpressionWithExpected(argument, expected)
	if actual.Kind == InvalidType {
		return invalid, display, true
	}
	if !a.canInitialize(expected, actual, argument) {
		a.addErrorAtTokenWithMetadata(expressionToken(argument), diagnostics.ThreadStorageCapacity,
			"Pass a compile-time uint byte capacity, for example ThreadStorage(65536). ThreadStorage has no generic size parameter.",
			"ThreadStorage capacity must be uint, got %s", typeDisplayName(actual))
		return invalid, display, true
	}
	constant, outcome := a.semanticCompileTimeConstantVisiting(argument, map[string]bool{}, expected)
	if outcome != compileTimeEvaluated {
		a.reportCompileTimeRequirement(argument, outcome, "ThreadStorage capacity", diagnostics.ThreadStorageCapacity,
			"Use a literal or an immutable compile-time uint binding; ThreadStorage reserves target-managed storage without dynamic allocation.")
		return invalid, display, true
	}
	if constant.Integer == nil || constant.Integer.Sign() < 0 || expected.MaxInteger != nil && constant.Integer.Cmp(expected.MaxInteger) > 0 {
		a.addErrorAtTokenWithMetadata(expressionToken(argument), diagnostics.ThreadStorageCapacity,
			"Choose a non-negative compile-time capacity representable as uint on the selected target.",
			"ThreadStorage capacity is outside the target uint range")
		return invalid, display, true
	}
	if a.threadStorageCapacities == nil {
		a.threadStorageCapacities = map[*ast.CallExpression]string{}
	}
	a.threadStorageCapacities[expr] = constant.Integer.String()
	return a.types["ThreadStorage"], display, true
}

// ThreadStorageCapacityOf exposes detached compile-time capacity evidence for
// later target backing planning; capacities do not create distinct Sec types.
// Rules: rules/concurrency/threads.md — §8(1),(3–5).
func (a *Analyzer) ThreadStorageCapacityOf(expr *ast.CallExpression) (string, bool) {
	if a == nil {
		return "", false
	}
	value, ok := a.threadStorageCapacities[expr]
	return value, ok
}

// threadStorageMembers exposes immutable capacity through the shared compiler
// catalog, independently of target backing representation.
// Rules: rules/concurrency/threads.md — §8(1–7).
func threadStorageMembers(typ Type) []CompilerKnownMember {
	storage := dereferenceType(typ)
	if storage.Name != "ThreadStorage" || !storage.Intrinsic || storage.Module != "core" {
		return nil
	}
	return []CompilerKnownMember{{
		ID: "CKM-THREAD-STORAGE-CAPACITY", Name: "Capacity", Kind: CompilerKnownProperty,
		Result: builtinType("uint"), Signature: "property Capacity: uint",
		Documentation: "Read-only byte capacity fixed at construction. ThreadStorage is non-generic and @noCopy; backing storage is managed by the compiler and selected target without dynamic allocation.",
		Rule:          "rules/concurrency/threads.md — §8(1–7)", Receiver: "ThreadStorage",
		TargetRestriction: "backing layout and alignment depend on the selected CompilationPlan",
	}}
}
