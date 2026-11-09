package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// analyzeAsmStatement enforces the unsafe source boundary before operand checks.
// Rules: rules/platform/inline_assembly.md — unsafe boundary and §§22–27.
func (a *Analyzer) analyzeAsmStatement(stmt *ast.AsmStatement) {
	if !a.inUnsafe {
		a.addErrorAtToken(stmt.Token, "asm is only allowed inside unsafe")
		return
	}
	if stmt.Block != nil {
		a.analyzeAsmBlock(stmt)
		return
	}
	if stmt.Template == nil {
		a.addErrorAtToken(stmt.Token, "asm statement requires string template")
	}
}

// unsafeAsmReturns recognizes the existing single-block output return form.
// Rules: rules/platform/inline_assembly.md — outputs and control-flow boundary.
func unsafeAsmReturns(stmt *ast.UnsafeStatement) bool {
	if stmt == nil || stmt.Body == nil || len(stmt.Body.Statements) != 1 {
		return false
	}
	asmStmt, ok := stmt.Body.Statements[0].(*ast.AsmStatement)
	return ok && asmStmt.Block != nil && len(asmStmt.Block.Outputs) > 0
}

// analyzeAsmBlock validates source operands before exposing named machine outputs.
// Width/constraint and physical target validation remain separate obligations.
// Rules: rules/platform/inline_assembly.md — §§22,26–27, outputs.
func (a *Analyzer) analyzeAsmBlock(stmt *ast.AsmStatement) {
	if stmt.Block.Template == nil {
		a.addErrorAtToken(stmt.Token, "asm block requires string template")
	}
	for _, input := range stmt.Block.Inputs {
		if input.Value == nil {
			a.addErrorAtToken(stmt.Token, "asm input %s missing value", input.Register)
			continue
		}
		typ, _ := a.inferExpression(input.Value)
		a.validateAsmRegisterType(typ, expressionToken(input.Value), "input", input.Register)
	}
	if len(stmt.Block.Outputs) == 0 {
		a.addErrorAtToken(stmt.Token, "asm block requires at least one output")
	}
	for _, output := range stmt.Block.Outputs {
		outputType := a.currentFunctionReturn
		if outputType.Kind == InvalidType || outputType.Kind == VoidType {
			outputType = Type{Name: "int64", Kind: IntType}
		}
		a.validateAsmRegisterType(outputType, output.Token, "output", output.Register)
		if output.Name == "" {
			continue
		}
		if a.defineSymbol(output.Name, outputType, false, output.Token) {
			a.assigned[output.Name] = true
		}
	}
}

// validateAsmRegisterType rejects semantic/high-level values at the direct
// machine operand boundary without guessing widths, ABI or native constraints.
// Rules: rules/platform/inline_assembly.md — §§22,26–27.
func (a *Analyzer) validateAsmRegisterType(typ Type, token lexer.Token, direction, register string) bool {
	if typ.Kind == InvalidType {
		return false
	}
	primitive := typ.Kind == IntType || typ.Kind == UintType || typ.Kind == FloatType || typ.Kind == RawPtrType
	if primitive && !typ.Named && !typ.Declared && len(typ.Contracts) == 0 && typ.Unit == "" {
		return true
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.AsmInvalidRegisterOperandType,
		"Explicitly extract or convert the required representation to an unconstrained primitive machine value or RawPtr before the asm boundary. unsafe does not waive operand type rules.",
		"asm %s %s cannot use %s as a direct register operand", direction, register, typeDisplayName(typ))
	return false
}
