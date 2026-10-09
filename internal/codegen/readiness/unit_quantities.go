package readiness

import (
	"sec/internal/ast"
	astwalk "sec/internal/ast/walk"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// RejectUnitQuantities prevents legacy lowering from erasing unresolved quantity
// semantics or conversion plans into numeric carriers. A unit declaration alone
// is not a runtime quantity. Analyzed imported aliases and nested aggregate types
// are checked through their resolved facts; raw APIs cannot invent such facts.
// Rules: rules/compiler/semantic_ir.md — §20(1–4);
// rules/corrections/applied/semantic-ir-units-correction-20260818.md — Erasure boundary.
func RejectUnitQuantities(program *ast.Program, analyzer *sema.Analyzer, backend string) error {
	var types map[string]sema.Type
	if analyzer != nil {
		types = analyzer.Types()
	}
	output := *program
	output.Statements = nil
	for _, statement := range program.Statements {
		if program.SourceProvenance[wideToken(statement).File] != ast.SourceCore {
			output.Statements = append(output.Statements, statement)
		}
	}
	return astwalk.Inspect(&output, func(node any) error {
		var token lexer.Token
		var typ sema.Type
		direct := false
		switch node := node.(type) {
		case *ast.TypeReference:
			token = node.Token
			direct = node.Unit != "" || node.UnitOnly
			typ = types[node.Name]
		case ast.Expression:
			if analyzer == nil {
				return nil
			}
			token = wideToken(node)
			typ, _ = analyzer.ResolvedTypeOf(node)
		default:
			return nil
		}
		if direct || quantityInType(typ, map[string]bool{}) {
			return &semantic.UnsupportedFeatureError{Feature: backend + " unit quantity semantics and conversion plans", Package: 3, Location: semantic.Location{File: token.File, Line: token.Line, Column: token.Column}}
		}
		return nil
	})
}

// quantityInType follows resolved quantity facts through nested nominal members
// without guessing units from familiar names or physical representation.
// Rules: rules/types/units.md — Structural expressions; semantic_ir.md — §20.
func quantityInType(typ sema.Type, seen map[string]bool) bool {
	if typ.Unit != "" || !typ.Dimension.IsZero() || typ.UnitSemantics.Identity != "" {
		return true
	}
	key := typ.Module + ":" + string(typ.Kind) + ":" + sema.TypeDisplayName(typ)
	if seen[key] {
		return false
	}
	seen[key] = true
	if typ.NamedBase != nil && quantityInType(*typ.NamedBase, seen) {
		return true
	}
	if typ.Element != nil && quantityInType(*typ.Element, seen) {
		return true
	}
	for _, arg := range typ.TypeArgs {
		if quantityInType(arg, seen) {
			return true
		}
	}
	for _, field := range typ.Fields {
		if quantityInType(field.Type, seen) {
			return true
		}
	}
	for _, variant := range typ.UnionVariants {
		if variant.Payload != nil && quantityInType(*variant.Payload, seen) {
			return true
		}
		for _, field := range variant.PayloadFields {
			if quantityInType(field.Type, seen) {
				return true
			}
		}
	}
	for _, parameter := range typ.FunctionParameterTypes {
		if quantityInType(parameter, seen) {
			return true
		}
	}
	return typ.FunctionReturnType != nil && quantityInType(*typ.FunctionReturnType, seen)
}
