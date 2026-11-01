package sema

import "sec/internal/ast"

// resolvedSourceTypeUse withholds a source site resolved differently by multiple
// specializations instead of arbitrarily displaying the last concrete type.
// Rules: rules/tooling/lsp.md — "One semantic source of truth", "Unknown facts";
// rules/declarations/generics.md — §30.
type resolvedSourceTypeUse struct {
	typ       Type
	ambiguous bool
}

// recordSourceTypeReference retains the successfully resolved type at its
// source name, including the referent name of a reference annotation. It does
// not resolve names or infer arguments a second time for tooling.
// Rules: rules/tooling/lsp.md — "Hover"; MD-120 correction §10(7).
func (a *Analyzer) recordSourceTypeReference(ref *ast.TypeReference, typ Type) {
	if ref == nil {
		return
	}
	token := ref.Token
	if ref.Ref {
		token = ref.ReferentToken
		typ = dereferenceType(typ)
	}
	if token.Line <= 0 {
		return
	}
	key := sourceTokenLocation(token)
	if previous, ok := a.resolvedSourceTypeUses[key]; ok {
		if previous.ambiguous || !sameConcreteType(previous.typ, typ) {
			a.resolvedSourceTypeUses[key] = resolvedSourceTypeUse{ambiguous: true}
			return
		}
	}
	a.resolvedSourceTypeUses[key] = resolvedSourceTypeUse{typ: typ}
}

// ResolvedSourceTypeAt exposes an immutable source annotation fact. Builtin
// error, constrained unknown E and concrete specializations remain distinct.
// Rules: rules/tooling/lsp.md — "Hover", "Unknown facts"; MD-120 §10(7).
func (a *Analyzer) ResolvedSourceTypeAt(file string, line, column int) (Type, bool) {
	if a == nil {
		return Type{}, false
	}
	use, ok := a.resolvedSourceTypeUses[sourceTokenKey{File: file, Line: line, Column: column}]
	if !ok || use.ambiguous {
		return Type{}, false
	}
	typ := use.typ
	// Early type annotations may precede completed interface requirements.
	// Reuse the exact canonical instance already built by Sema; this query
	// never instantiates a type or changes the analyzer's caches.
	if typ.Kind == InterfaceType {
		if len(typ.TypeArgs) > 0 {
			if resolved, found := a.genericTypeInstances[genericTypeInstanceKey(typ)]; found {
				typ = resolved
			}
		} else if declared, found := a.types[typ.Name]; found && declared.Kind == InterfaceType && declared.Module == typ.Module {
			typ = declared
		}
	}
	result := semanticSnapshotType(typ)
	result.GenericParameters = append([]string(nil), typ.GenericParameters...)
	result.GenericConstraints = append([]GenericConstraint(nil), typ.GenericConstraints...)
	for i := range result.GenericConstraints {
		result.GenericConstraints[i].Interface = semanticSnapshotType(result.GenericConstraints[i].Interface)
	}
	// General analysis snapshots intentionally omit declaration methods. Source
	// hover needs their signatures, detached without recursively expanding the
	// method surface of every nested interface parameter/result.
	result.InterfaceMethods = append([]Function(nil), typ.InterfaceMethods...)
	for i := range result.InterfaceMethods {
		method := &result.InterfaceMethods[i]
		method.ReturnType = semanticSnapshotType(method.ReturnType)
		method.Parameters = append([]FunctionParameter(nil), method.Parameters...)
		for j := range method.Parameters {
			method.Parameters[j].Type = semanticSnapshotType(method.Parameters[j].Type)
		}
	}
	return result, true
}
