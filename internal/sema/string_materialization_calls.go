package sema

import "sec/internal/ast"

// inferCompilerKnownStringConstructor validates the borrowed byte/rune
// sequence of a static string materializer and retains Result[StringError].
// No input ownership is transferred and no byte ToString decoder is implied.
// Rules: rules/corrections/md109-core-string-materialization-correction.md
// — §§1.2–1.7,2.1–2.6,8.1–8.4; rules/memory/borrowing.md — §15.2.
func (a *Analyzer) inferCompilerKnownStringConstructor(expr *ast.CallExpression, member CompilerKnownMember) (Type, expressionValue, bool) {
	if !a.checkCompilerKnownCallArity(expr, "string."+member.Name, 1, 1) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if move, explicit := explicitMoveArgument(expr.Arguments[0]); explicit {
		element := a.types["rune"]
		if member.Name == "FromByteArray" {
			element = a.types["byte"]
		}
		sequence := Type{Name: element.Name + "[]", Kind: SliceType, Element: &element}
		parameter := FunctionParameter{Name: "data", Type: Type{Name: "ref " + sequence.Name, Kind: ReferenceType, Element: &sequence}}
		function := Function{Name: "string." + member.Name}
		// Prefer the real trusted-core declaration when it is loaded; a
		// compiler-only analysis must not fabricate a source location.
		for _, declared := range a.functions[function.Name] {
			if len(declared.Parameters) == 1 && a.isTrustedCoreSourceToken(declared.Token) {
				parameter.Token = declared.Parameters[0].Token
				parameter.Name = declared.Parameters[0].Name
				function.Token = declared.Token
				break
			}
		}
		a.reportMoveIntoBorrowedParameter(move, parameter, function, expr)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	argumentType, _ := a.inferExpression(expr.Arguments[0])
	sequence := dereferenceType(argumentType)
	want := "rune"
	if member.Name == "FromByteArray" {
		want = "byte"
	}
	if (sequence.Kind != ArrayType && sequence.Kind != SliceType) || sequence.Element == nil || sequence.Element.Name != want {
		a.addErrorAtToken(expressionToken(expr.Arguments[0]), "string.%s requires a %s array or slice, got %s", member.Name, want, typeDisplayName(argumentType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
}
