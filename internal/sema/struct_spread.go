package sema

import "sec/internal/lexer"

// validateStructSpreadSource checks the complete shallow source before it can
// supply fields or suppress omitted-field defaults. Spread cannot reconstruct
// inaccessible representation, erase nominal identity or hide a move/clone.
// Rules: rules/declarations/spread.md — §§6.1,6.3–6.5;
// rules/declarations/struct.md — §9;
// rules/foundations/names_scopes_visibility.md — §12.
func (a *Analyzer) validateStructSpreadSource(target, source Type, token lexer.Token) bool {
	if !sameConcreteType(target, source) {
		a.addErrorAtToken(token, "cannot spread %s into %s; spread source must have type %s", typeDisplayName(source), typeDisplayName(target), typeDisplayName(target))
		return false
	}
	if !implicitlyCopyable(source) {
		a.addErrorAtToken(token, "cannot spread %s into %s; %s is not implicitly copyable", typeDisplayName(source), typeDisplayName(target), typeDisplayName(source))
		return false
	}
	accessible := true
	for _, field := range target.Fields {
		if !a.canAccessStructField(target, field.Name) {
			a.addErrorAtToken(token, "field %s on %s is not accessible from module %s", field.Name, typeDisplayName(target), moduleDisplayName(a.currentModule))
			accessible = false
		}
	}
	return accessible
}
