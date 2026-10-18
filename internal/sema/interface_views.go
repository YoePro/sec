package sema

import (
	"fmt"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// borrowedInterfaceView reports whether a call argument of type actual can be
// passed to a borrowed interface view parameter of type expected (`ref I` or
// `ref mut I` with I an interface, including a fully instantiated generic
// interface such as Iterator[int]). The argument's concrete type must have a
// valid explicit conformance to I, possibly through interface inheritance, or
// already be a view of an interface that includes I; a matching set of member
// names is never sufficient. A shared reference never becomes an exclusive
// view. An owned argument is borrowed at the call site by the ordinary
// implicit call-site borrow, whose mutability and lifetime checks still apply.
//
// Rules:
//   - rules/declarations/interfaces.md — § 4, § 5, § 7.1 "Borrowed interface views"
//   - rules/corrections/applied/md057-interface-iterator-parameters-correction-20261017.md — §§ 1–2, 7.1–7.2
func (a *Analyzer) borrowedInterfaceView(expected, actual Type) bool {
	if expected.Kind != ReferenceType || expected.Element == nil || expected.Element.Kind != InterfaceType {
		return false
	}
	if actual.Kind == ReferenceType && expected.ReferenceMutable && !actual.ReferenceMutable {
		return false
	}
	concrete := dereferenceType(actual)
	if concrete.Kind == ReferenceType || concrete.Kind == InvalidType {
		return false
	}
	return a.hasValidExplicitInterfaceConformance(concrete, *expected.Element)
}

// rejectInterfaceByValueParameter reports a parameter of the compiler-known
// interface Iterator[T] written by value: MD-057 does not make Iterator[T] a
// by-value type and forbids treating it as an owned value or an implicit
// anonymous generic parameter. Other interfaces written by value keep their
// existing treatment; their owned-value semantics are outside MD-057 (MD-107).
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — § 37.1
//   - rules/declarations/interfaces.md — § 7.1
//   - rules/corrections/applied/md057-interface-iterator-parameters-correction-20261017.md — §§ 5.3, 5.4, 7.5, 8.9
func (a *Analyzer) rejectInterfaceByValueParameter(name string, typ Type, token lexer.Token) bool {
	if !isIteratorInterface(typ) {
		return false
	}
	display := typeDisplayName(typ)
	a.addErrorAtTokenWithMetadata(token, diagnostics.InterfaceByValueParameter,
		fmt.Sprintf("Borrow the iterator as `%s: ref mut %s`, or take a concrete iterator through a named generic parameter `[I: %s](%s: I)`.", name, display, display, name),
		"parameter %q cannot have interface type %s by value; Iterator[T] is not a by-value type", name, display)
	return true
}
