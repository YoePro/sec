package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// checkUnionPayloadFields resolves named fields and defaults and rejects
// invalid payload ownership before the outer construction can commit.
// Rules: rules/declarations/unions.md — payload construction;
// rules/memory/copy_move.md — §§10.4,12; rules/types/default_values.md — "Diagnostics".
func (a *Analyzer) checkUnionPayloadFields(unionType Type, variant UnionVariant, expr *ast.StructLiteral, token lexer.Token) bool {
	valid := true
	expected := map[string]StructField{}
	for _, field := range variant.PayloadFields {
		expected[field.Name] = field
	}

	seen := map[string]lexer.Token{}
	for _, field := range expr.Fields {
		if field != nil && field.Spread {
			a.addErrorAtToken(field.Token, "spread is not supported in union payload literals")
			valid = false
			continue
		}
		if field == nil || field.Name == nil {
			valid = false
			continue
		}
		name := field.Name.Value
		if _, exists := seen[name]; exists {
			a.addErrorAtToken(field.Name.Token, "duplicate payload field %s for %s.%s", name, typeDisplayName(unionType), variant.Name)
			valid = false
			continue
		}
		seen[name] = field.Name.Token

		expectedField, ok := expected[name]
		if !ok {
			a.addErrorAtToken(field.Name.Token, "unknown payload field %s for %s.%s", name, typeDisplayName(unionType), variant.Name)
			valid = false
			continue
		}

		valueType, _ := a.inferOwningConstructionValue(field.Value, expectedField.Type)
		if valueType.Kind != InvalidType && !a.canInitialize(expectedField.Type, valueType, field.Value) {
			a.addErrorAtToken(expressionToken(field.Value), "payload field %s for %s.%s must be %s, got %s", name, typeDisplayName(unionType), variant.Name, typeDisplayName(expectedField.Type), typeDisplayName(valueType))
			valid = false
			continue
		}
		if valueType.Kind == InvalidType {
			valid = false
		}
		if valueType.Kind != InvalidType {
			// rules/declarations/unions.md; correction17.md: union payload
			// fields use the ordinary destination representability check.
			if a.checkCompileTimeContractExpression(expectedField.Type, field.Value) {
				valid = false
			}
		}
		if valueType.Kind != InvalidType {
			if !a.validateOwningConstructionSource(field.Value) {
				valid = false
			}
		}
	}

	for _, field := range variant.PayloadFields {
		if _, supplied := seen[field.Name]; supplied {
			continue
		}
		resolution := DefaultValueOf(field.Type)
		value := defaultExpression(resolution, field.Type, token)
		if value == nil {
			start := len(a.errors)
			// rules/types/default_values.md, "Diagnostics": types.no-default-value
			// unless the cause is an ambiguous nearest-to-zero tie.
			if id, help, reason, ambiguous := noDefaultDiagnostic(field.Type); ambiguous {
				a.addErrorAtTokenWithMetadata(token, id, "initialize the payload field explicitly or "+help, "missing payload field %s for %s.%s because %s", field.Name, typeDisplayName(unionType), variant.Name, reason)
			} else {
				a.addErrorAtTokenWithMetadata(token, diagnostics.TypeNoDefaultValue, "initialize the payload field explicitly", "missing payload field %s for %s.%s because %s has no default value", field.Name, typeDisplayName(unionType), variant.Name, typeDisplayName(field.Type))
			}
			a.relateMissingDefault(start, field.Type, field.Token)
			valid = false
			continue
		}
		expr.Fields = append(expr.Fields, &ast.StructLiteralField{Token: token, Name: &ast.Identifier{Token: field.Token, Value: field.Name}, Value: value})
	}
	return valid
}

// resolveResultValueInitializer checks contextual Ok/Err payloads as owning
// construction, preserving explicit transfers and rejecting implicit ones.
// Rules: rules/errors/errorhandling.md — §5 "Ok and Err";
// rules/memory/copy_move.md — §§9.2,10.3(1–4),12.1(2–3).
func (a *Analyzer) resolveResultValueInitializer(resultType Type, expr ast.Expression) (Type, bool) {
	if len(resultType.TypeArgs) != 2 {
		return Type{Kind: InvalidType}, false
	}
	var payload ast.Expression
	var expected Type
	var name string
	switch expr := expr.(type) {
	case *ast.OkExpression:
		payload, expected, name = expr.Value, resultType.TypeArgs[0], "Ok"
	case *ast.ErrExpression:
		payload, expected, name = expr.Value, resultType.TypeArgs[1], "Err"
	default:
		return Type{}, false
	}
	if payload == nil {
		if name == "Ok" && expected.Kind == VoidType {
			return resultType, true
		}
		a.addErrorAtToken(expressionToken(expr), "%s requires a payload of type %s", name, typeDisplayName(expected))
		return Type{Kind: InvalidType}, true
	}
	valueType, _ := a.inferOwningConstructionValue(payload, expected)
	if valueType.Kind == InvalidType {
		return valueType, true
	}
	if !a.canInitialize(expected, valueType, payload) {
		a.addErrorAtToken(expressionToken(payload), "cannot initialize %s with %s(%s)", typeDisplayName(resultType), name, typeDisplayName(valueType))
		return Type{Kind: InvalidType}, true
	}
	if !a.validateOwningConstructionSource(payload) {
		return Type{Kind: InvalidType}, true
	}
	return resultType, true
}
