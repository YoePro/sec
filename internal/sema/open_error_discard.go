package sema

import (
	"sort"
	"strings"
)

// openErrorObligations returns the error-marked types of the analyzed
// program that are not discardable. The open error root may carry any of
// them after widening, so an error value is discardable only when this list
// is empty. The list is computed once per analysis, after declarations.
//
// Rules:
//   - rules/control-flow/discard.md — "Recursive discardability": Result is discardable when every possible active payload is
//   - rules/errors/errorhandling.md — §2.1 "Error assignability", §17 "Err(_) and explicit error acknowledgement"
func (a *Analyzer) openErrorObligations() []Type {
	if a.openErrorObligationsReady {
		return a.openErrorObligationList
	}
	names := make([]string, 0, len(a.types))
	for name, typ := range a.types {
		if typ.ErrorAssignable && typ.Kind != ErrorRootType && typ.Kind != InvalidType && !isDiscardableType(typ) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	list := make([]Type, 0, len(names))
	for _, name := range names {
		list = append(list, a.types[name])
	}
	a.openErrorObligationList = list
	a.openErrorObligationsReady = true
	return list
}

// isDiscardable extends recursive discardability to the open error root using
// the program's declared error types as the compiler-owned set of possible
// concrete payloads.
func (a *Analyzer) isDiscardable(typ Type) bool {
	if !isDiscardableType(typ) {
		return false
	}
	return !typeMentionsErrorRoot(typ, map[string]bool{}) || len(a.openErrorObligations()) == 0
}

// nonDiscardableSubject names the type a discard diagnostic reports; for the
// open error root it also names the declared error types it may carry.
func (a *Analyzer) nonDiscardableSubject(typ Type) string {
	if !isDiscardableType(typ) || !typeMentionsErrorRoot(typ, map[string]bool{}) {
		return typeDisplayName(typ)
	}
	names := []string{}
	for _, obligation := range a.openErrorObligations() {
		names = append(names, typeDisplayName(obligation))
	}
	return typeDisplayName(typ) + ", which may carry " + strings.Join(names, ", ") + ","
}

func typeMentionsErrorRoot(typ Type, visiting map[string]bool) bool {
	typ = dereferenceType(typ)
	if typ.Kind == ErrorRootType {
		return true
	}
	if key := typeObligationKey(typ); key != "" {
		if visiting[key] {
			return false
		}
		visiting[key] = true
		defer delete(visiting, key)
	}
	for _, argument := range typ.TypeArgs {
		if typeMentionsErrorRoot(argument, visiting) {
			return true
		}
	}
	if typ.Element != nil && typeMentionsErrorRoot(*typ.Element, visiting) {
		return true
	}
	for _, field := range typ.Fields {
		if typeMentionsErrorRoot(field.Type, visiting) {
			return true
		}
	}
	for _, variant := range typ.UnionVariants {
		if variant.Payload != nil && typeMentionsErrorRoot(*variant.Payload, visiting) {
			return true
		}
		for _, field := range variant.PayloadFields {
			if typeMentionsErrorRoot(field.Type, visiting) {
				return true
			}
		}
	}
	return false
}
