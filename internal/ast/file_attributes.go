package ast

// FileTargetDirectives returns the source-backed file selections in source
// order. Canonical directives retain their written Attribute; compatibility
// directives retain their original form without synthesizing an attribute.
// Statement attributes are excluded. Deriving this view from Statements keeps
// it valid when compilation units are combined or source selection filters
// them, without a second attribute store that callers must synchronize.
//
// Rules:
//   - rules/foundations/attributes.md — §§ 14.1(2–3), 15(1–6), 16(1), 25(2–5).
func (p *Program) FileTargetDirectives() []*TargetDirective {
	if p == nil {
		return nil
	}
	var directives []*TargetDirective
	for _, statement := range p.Statements {
		if directive, ok := statement.(*TargetDirective); ok && directive != nil {
			directives = append(directives, directive)
		}
	}
	return directives
}
