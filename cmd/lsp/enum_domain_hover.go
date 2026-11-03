package main

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// enumDomainHoverDetails presents the compiler-owned closed/open classifier.
// String-enum details retain their existing value/alias/default presentation.
// Rules: rules/declarations/enums.md — §§7–10,12,18;
// rules/tooling/lsp.md — "Hover", "Unknown facts".
func enumDomainHoverDetails(typ sema.Type) string {
	if typ.Kind != sema.EnumType {
		return ""
	}
	if typ.Underlying == "string" {
		return stringEnumHoverDetails(typ)
	}
	domain, known := sema.EnumDomainOf(typ)
	if !known {
		return ""
	}
	owner := sema.TypeDisplayName(typ)
	lines := []string{"Underlying type: `" + domain.Underlying + "`"}
	if domain.Open {
		lines = append(lines, "Domain: open bit-backed enum. Undeclared bit patterns are valid runtime values.",
			"Match must cover the complete bit domain or include an unknown-value fallback (`_`).",
			"Integer conversion: `"+owner+"(value)` is checked unless the compiler proves the value fits; failure `EnumValueError.OutOfRange`. An in-range unnamed value does not fail.")
	} else {
		lines = append(lines, "Domain: closed (declared underlying value classes only). Aliases share a value class.",
			"Match must cover every declared value class or include `_`.",
			"Runtime integer conversion: `try "+owner+"(value)` unless the compiler proves membership; result `Result["+owner+", EnumValueError]`, failures `EnumValueError.OutOfRange` and `EnumValueError.UndeclaredValue`.")
	}
	lines = append(lines, "Conversion to an integer is infallible only when the compiler proves representability for the selected target; otherwise it returns `Result[T, ConversionError]` and requires explicit handling.")
	return "\n\n" + strings.Join(lines, "\n\n")
}

// enumConversionHover presents accepted conversions at the actual callee token,
// preserving the compiler's Result channel and exactness basis. It never judges
// safety from the named cases of an open enum or from target spelling.
// Rules: rules/declarations/enums.md — §§12,18;
// rules/types/types.md — "Exact integer conversion proof";
// rules/tooling/lsp.md — "Hover", "Unknown facts".
func enumConversionHover(text string, program *ast.Program, analyzer *sema.Analyzer, hovered lexer.Token) (hoverResult, bool) {
	for _, expression := range astExpressionsInProgram(program) {
		call, ok := expression.(*ast.CallExpression)
		if !ok || !expressionOwnsTypeToken(call, hovered) {
			continue
		}
		info, known := analyzer.EnumConversionInfoOf(call)
		if !known {
			continue
		}
		contents := fmt.Sprintf("```sec\n%s → %s\nresult: %s\n```", lspTypeName(info.Source), lspTypeName(info.Target), lspTypeName(info.Result))
		if info.Checked {
			contents += "\n\nChecked conversion: handle the Result explicitly with `try` or another Result handler."
		} else {
			contents += "\n\nInfallible conversion: no failure handling is required."
		}
		if info.Proof.Proven() {
			basis := "the resolved source domain"
			switch info.Proof.Basis {
			case sema.IntegerConversionConstant:
				basis = "the known constant value"
			case sema.IntegerConversionClosedEnumDomain:
				basis = "the declared value classes of the closed enum"
			case sema.IntegerConversionOpenEnumDomain:
				basis = "the complete bit domain, including unnamed patterns"
			}
			contents += "\n\nExact conversion proved from " + basis
			if info.Proof.SourceMin != nil && info.Proof.SourceMax != nil {
				contents += "; source bounds `" + info.Proof.SourceMin.String() + "` through `" + info.Proof.SourceMax.String() + "` (inclusive)"
			}
			if info.Proof.PlanPointerWidthBits != 0 {
				contents += fmt.Sprintf("; selected plan width %d bits", info.Proof.PlanPointerWidthBits)
			}
			contents += "."
		}
		contents += enumDomainHoverDetails(info.Enum)
		return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: tokenRange(text, hovered)}, true
	}
	return hoverResult{}, false
}
