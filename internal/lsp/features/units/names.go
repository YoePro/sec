package units

import (
	"sec/internal/ast"
	astwalk "sec/internal/ast/walk"
)

// RewriteNames resolves module qualifiers in structural unit factors, including
// annotations in signatures, fields and conversions. Source text and tokens
// remain intact for diagnostics and edits; Sema resolves the rewritten names.
// Rules: rules/projects/modules.md — §§7–9; rules/types/units.md —
// Structural unit expressions, Unit identity and named types;
// rules/tooling/lsp.md — Unit actions and Snapshots.
func RewriteNames(program *ast.Program, resolve func(string) string) {
	_ = astwalk.Inspect(program, func(node any) error {
		if unit, ok := node.(*ast.UnitExpression); ok && unit.Kind == ast.UnitExpressionName {
			unit.Name = resolve(unit.Name)
		}
		return nil
	})
}
