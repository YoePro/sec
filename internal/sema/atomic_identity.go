package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
	"strings"
)

// AtomicTypeFacts is the compiler-owned public view of opaque atomic storage.
// Members come from the same catalog used by checking and completion.
// Rules: rules/concurrency/atomics.md — §§4,7,12–13,43–44.
type AtomicTypeFacts struct {
	Type          Type
	Element       Type
	Template      bool
	Members       []CompilerKnownMember
	Documentation string
}

// AtomicTypeFactsOf exposes only the canonical core identity, projecting a
// shared/mutable reference for presentation without exposing contained storage.
// Rules: rules/concurrency/atomics.md — §§4,7,12–13,31–32,43–44.
func AtomicTypeFactsOf(typ Type) (AtomicTypeFacts, bool) {
	typ = dereferenceType(typ)
	if !isAtomicFamilyIdentity(typ) {
		return AtomicTypeFacts{}, false
	}
	fact := AtomicTypeFacts{Type: typ, Documentation: "Compiler-known core atomic storage. Its representation is private; initialize with Atomic(value). Methods operate through a shared receiver. Concrete operation support depends on the CompilationPlan."}
	if len(typ.TypeArgs) == 0 && len(typ.GenericParameters) == 1 {
		fact.Template = true
		fact.Element = Type{Name: typ.GenericParameters[0], Kind: GenericType}
		view := typ
		view.TypeArgs = []Type{fact.Element}
		fact.Members = atomicMemberCatalog(view, fact.Element, true)
		for index, member := range fact.Members {
			fact.Members[index] = instantiateCompilerKnownPresentation(withSyntheticMetadata(classifyCompilerKnownMember(member)), view)
		}
	} else if isAtomicType(typ) && atomicElementTypeSupported(typ.TypeArgs[0]) {
		fact.Element = typ.TypeArgs[0]
		for _, member := range CompilerKnownMembersForType(typ, false) {
			if strings.HasPrefix(member.ID, "CKM-ATOMIC-") {
				fact.Members = append(fact.Members, member)
			}
		}
	} else {
		return AtomicTypeFacts{}, false
	}
	return fact, true
}

// atomicTypeUse withholds a concrete display if the same source position was
// resolved with conflicting generic substitutions in this analysis snapshot.
// Rules: rules/compiler/compiler_analysis.md — §§3,5–7; atomics.md — §43(3).
type atomicTypeUse struct {
	typ       Type
	ambiguous bool
}

// recordAtomicTypeUse retains resolved type/constructor facts for source hover.
// It never resolves types again from source spelling in the editor.
// Rules: rules/concurrency/atomics.md — §§4(6),7(8),43–44.
func (a *Analyzer) recordAtomicTypeUse(token lexer.Token, typ Type) {
	if !isAtomicType(typ) {
		return
	}
	if a.atomicTypeUses == nil {
		a.atomicTypeUses = map[sourceTokenKey]atomicTypeUse{}
	}
	key := sourceTokenLocation(token)
	if previous, exists := a.atomicTypeUses[key]; exists {
		if previous.ambiguous || !sameConcreteType(previous.typ, typ) {
			a.atomicTypeUses[key] = atomicTypeUse{ambiguous: true}
		}
		return
	}
	a.atomicTypeUses[key] = atomicTypeUse{typ: typ}
}

// recordAtomicTypeReference retains the referent's token for reference syntax,
// so hover on Atomic in ref Atomic[T] uses the resolved contained type.
// Rules: rules/concurrency/atomics.md — §§7(8),31,43(3).
func (a *Analyzer) recordAtomicTypeReference(ref *ast.TypeReference, typ Type) {
	if ref == nil {
		return
	}
	token := ref.Token
	if ref.Ref {
		token = ref.ReferentToken
	}
	a.recordAtomicTypeUse(token, dereferenceType(typ))
}

// AtomicTypeAt returns the concrete atomic type established at a type-use or
// constructor token, never a type guessed from its generic argument text.
// Rules: rules/concurrency/atomics.md — §§7(8),43(3); rules/tooling/lsp.md — Hover.
func (a *Analyzer) AtomicTypeAt(file string, line, column int) (Type, bool) {
	use, found := a.atomicTypeUses[sourceTokenKey{File: file, Line: line, Column: column}]
	return use.typ, found && !use.ambiguous
}

// inferAtomicConstructor initializes exactly one opaque nominal owner from T.
// Context supplies the contained type for literals, while ordinary nominal
// initialization, contracts and range checks still govern the source value.
// Rules: rules/concurrency/atomics.md — §§4,7–9,11,27–30;
// rules/types/types.md — Untyped literals versus typed values, Named types.
func (a *Analyzer) inferAtomicConstructor(expr *ast.CallExpression) (Type, expressionValue, bool) {
	display := expressionValue{Display: expr.String()}
	if len(expr.GenericArguments) != 0 {
		a.addErrorAtToken(expr.Token, "Atomic constructor infers its type argument from the initializer")
		return Type{Kind: InvalidType}, display, true
	}
	if !a.checkCompilerKnownCallArity(expr, "Atomic", 1, 1) {
		return Type{Kind: InvalidType}, display, true
	}
	var element Type
	if expected, known := a.expectedExpressionTypes[expr]; known && isAtomicType(expected) {
		element = expected.TypeArgs[0]
		actual, _ := a.inferExpressionWithExpected(expr.Arguments[0], element)
		if actual.Kind == InvalidType || !a.checkInitializerType(element, actual, expr.Arguments[0]) {
			return Type{Kind: InvalidType}, display, true
		}
		if a.checkDeclaredContractExpression(element, expr.Arguments[0]) {
			return Type{Kind: InvalidType}, display, true
		}
	} else {
		element, _ = a.inferExpression(expr.Arguments[0])
	}
	if element.Kind == InvalidType {
		return element, display, true
	}
	if !atomicElementTypeSupported(element) {
		a.addErrorAtToken(expressionToken(expr.Arguments[0]), "type %s is not supported by Atomic", typeDisplayName(element))
		return Type{Kind: InvalidType}, display, true
	}
	result := a.intrinsicGenericType("Atomic", element)
	if name, ok := expr.Callee.(*ast.Identifier); ok {
		a.recordAtomicTypeUse(name.Token, result)
	}
	return result, display, true
}
