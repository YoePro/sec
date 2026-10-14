package sema

import (
	"math/big"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// guardGenericInlineSelfStorage avoids recursive construction and delegates
// module declarations to the dependency graph. Callable-local and associated
// declarations use the same owning reporter when they are outside that graph.
// Rules: rules/memory/layout.md — §25(1–5), §42(4);
// rules/declarations/impl.md — Associated declarations.
func (a *Analyzer) guardGenericInlineSelfStorage(owner string, declaration *ast.TypeDeclStatement, reference *ast.TypeReference, field string, token lexer.Token) bool {
	if len(declaration.GenericParameters) == 0 || !genericFieldHasInlineSelfStorage(owner, reference) {
		return false
	}
	if a.inFunctionBody || a.currentImplTarget != "" {
		node := &structLayoutNode{name: owner, declaration: declaration}
		display := layoutNodeDisplay(node)
		id := diagnostics.RecursiveStructLayout
		if declaration.Union {
			id = diagnostics.RecursiveUnionLayout
		}
		a.reportStructLayoutCycle(id, []structLayoutEdge{{owner: owner, target: owner, ownerDisplay: display, targetDisplay: display, field: field, token: token, ownerDeclaration: declaration.Name.Token, storageType: typeReferenceDisplayName(reference)}})
	}
	return true
}

// genericFieldHasInlineSelfStorage avoids materializing a recursive host Type
// while declaration templates are built. This is a containment guard, not a
// specialization-growth proof; the source layout graph owns its diagnostic.
// Rules: rules/memory/layout.md — §25(1–5);
// rules/compiler/monomorphization.md — §18.4–18.6;
// rules/corrections/applied/correction8-20260823.md — Valid finite recursion, items 9–11.
func genericFieldHasInlineSelfStorage(owner string, ref *ast.TypeReference) bool {
	if ref == nil || ref.Invalid || ref.Ref {
		return false
	}
	if ref.ElementType != nil {
		length := big.NewInt(ref.ArrayLength)
		if ref.ArrayLengthExpression != nil {
			if exact, ok := constantIntegerValue(ref.ArrayLengthExpression); ok {
				length = exact
			}
		}
		return !ref.Slice && length.Sign() > 0 && genericFieldHasInlineSelfStorage(owner, ref.ElementType)
	}
	return ref.Name == owner
}

// genericStructFieldHasDirectRecursiveStorage rechecks direct inline storage
// after substitution. Only fixed arrays embed their element by value;
// references, views, raw pointers and owning dynamic containers are boundaries.
// Rules: rules/declarations/unions.md — §18;
// rules/memory/layout.md — §13(7), §25(1–5);
// rules/collections/collections.md — §5.2 Layout, §6.1 Meaning.
func genericStructFieldHasDirectRecursiveStorage(owner Type, field Type) bool {
	if sameConcreteType(owner, field) {
		return true
	}
	switch field.Kind {
	case ArrayType:
		return arrayShapeOf(field) == ArrayShapeFixed && field.Element != nil && genericStructFieldHasDirectRecursiveStorage(owner, *field.Element)
	default:
		return false
	}
}
