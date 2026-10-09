package readiness

import (
	"sec/internal/ast"
	astwalk "sec/internal/ast/walk"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// RejectStringLengthContracts prevents unsupported backends from erasing new
// rune/byte validation. Loader-owned unused core roots are not output consumers;
// imported and inherited used type facts are checked recursively.
// Rules: rules/types/contracts.md — Core rule, Composition, Conversion failure layers.
func RejectStringLengthContracts(program *ast.Program, a *sema.Analyzer, backend string) error {
	output := *program
	output.Statements = nil
	for _, statement := range program.Statements {
		if program.SourceProvenance[wideToken(statement).File] != ast.SourceCore {
			output.Statements = append(output.Statements, statement)
		}
	}
	var types map[string]sema.Type
	if a != nil {
		types = a.Types()
	}
	return astwalk.Inspect(&output, func(node any) error {
		token := wideToken(node)
		if marker, ok := node.(*ast.MarkerContract); ok {
			if marker.Name == "minLen" || marker.Name == "maxLen" || marker.Name == "exactLen" || marker.Name == "notEmpty" || marker.Name == "minByteLen" || marker.Name == "maxByteLen" || marker.Name == "exactByteLen" {
				return stringContractFailure(token, backend)
			}
		}
		var typ sema.Type
		if expression, ok := node.(ast.Expression); ok && a != nil {
			typ, _ = a.ResolvedTypeOf(expression)
		} else if reference, ok := node.(*ast.TypeReference); ok {
			typ = types[reference.Name]
		}
		if token, found := stringContractToken(&typ, map[*sema.Type]bool{}); found {
			return stringContractFailure(token, backend)
		}
		return nil
	})
}

// stringContractToken retains nested canonical source facts without recovering
// semantic identity from a backend representation or a spelling substring.
// Rules: rules/types/contracts.md — Composition and Diagnostics.
func stringContractToken(typ *sema.Type, seen map[*sema.Type]bool) (lexer.Token, bool) {
	if typ == nil || seen[typ] {
		return lexer.Token{}, false
	}
	seen[typ] = true
	if typ.Kind == sema.StringType {
		for _, contract := range typ.Contracts {
			switch c := contract.(type) {
			case sema.LengthContract:
				return c.Token, true
			case sema.MarkerContract:
				if c.Name == "notEmpty" {
					return c.Token, true
				}
			}
		}
	}
	children := []*sema.Type{typ.Element, typ.FunctionReturnType}
	for i := range typ.TypeArgs {
		children = append(children, &typ.TypeArgs[i])
	}
	for i := range typ.Fields {
		children = append(children, &typ.Fields[i].Type)
	}
	for i := range typ.FunctionParameterTypes {
		children = append(children, &typ.FunctionParameterTypes[i])
	}
	for i := range typ.UnionVariants {
		variant := &typ.UnionVariants[i]
		children = append(children, variant.Payload)
		for j := range variant.PayloadFields {
			children = append(children, &variant.PayloadFields[j].Type)
		}
	}
	for _, child := range children {
		if token, found := stringContractToken(child, seen); found {
			return token, true
		}
	}
	return lexer.Token{}, false
}

// stringContractFailure exposes an implementation capability limitation, never
// a claimed source violation or a partial output module.
// Rules: rules/compiler/compiler_pipeline.md — §110(1).
func stringContractFailure(token lexer.Token, backend string) error {
	return &semantic.UnsupportedFeatureError{Feature: backend + " string contract runtime validation", Location: semantic.Location{File: token.File, Line: token.Line, Column: token.Column}}
}
