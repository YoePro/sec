package sema

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

const (
	rawPointerTypedHelp    = "Select a concrete compatible RawPtr[T] pointee before typed access or element arithmetic; RawPtr[void] carries no typed value representation."
	rawPointerArityHelp    = "Pass exactly the arguments required by this raw-pointer operation; changing unsafe authorization cannot fix its argument count."
	rawPointerArgumentHelp = "Use the operation's declared argument type: writes require the pointee type and Difference requires a pointer with the same concrete element type in a compatible allocation/address domain."
)

// reportRawPointerUnsafe explains missing authorization using the same canonical
// caller obligations as hover/signature help, without claiming them as proofs.
// Rules: rules/memory/raw_pointers.md §§5.2,6,34(1–4),35(5–6);
// rules/memory/unsafe.md §4(9–13).
func (a *Analyzer) reportRawPointerUnsafe(token lexer.Token, pointer Type, operation, format string, args ...any) {
	help := "This operation lacks unsafe authorization. After establishing its caller obligations, authorize this call with `unsafe pointer." + operation + "(...)` or an `unsafe { }` block. Unsafe does not make a compiler-proven invalid operation valid."
	if operation == "AsRef" || operation == "AsRefMut" {
		help = "Read pointer." + operation + " inside an unsafe { } block after establishing its caller obligations. As* conversions are properties without parentheses; call-form unsafe cannot authorize a property read. Unsafe does not make a compiler-proven invalid operation valid."
	}
	if member, ok := compilerKnownMember(pointer, operation, false); ok && len(member.CallerObligations) > 0 {
		help += " Caller obligations (not proven by unsafe): " + strings.Join(member.CallerObligations, " ")
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.RawPointerUnsafeRequired, help, format, args...)
}

// reportInvalidRawPointerOperation explains a failed type/argument requirement
// independently of the enclosing unsafe context, retaining the primary wording.
// Rules: rules/memory/raw_pointers.md §§3(4),11–14,18A,34(1–4).
func (a *Analyzer) reportInvalidRawPointerOperation(token lexer.Token, help, format string, args ...any) {
	help += " Adding unsafe cannot repair this invalid operation."
	a.addErrorAtTokenWithMetadata(token, diagnostics.RawPointerInvalidOperation, help, format, args...)
}

// checkRawPointerCallArity attaches the raw-operation explanation to errors
// emitted by the shared argument-count owner, without a second diagnostic.
// Rules: rules/memory/raw_pointers.md §§11–14,18A,34;
// rules/tooling/diagnostics.md §§2(6–8),3(4–6).
func (a *Analyzer) checkRawPointerCallArity(expr *ast.CallExpression, name string, minArgs, maxArgs int) bool {
	before := len(a.errors)
	if a.checkCompilerKnownCallArity(expr, name, minArgs, maxArgs) {
		return true
	}
	for i := before; i < len(a.errors); i++ {
		a.errors[i].ID = diagnostics.RawPointerInvalidOperation
		a.errors[i].Help = "Use the declared call signature of " + name + "; these methods have no call-site type parameters. " + rawPointerArityHelp + " Adding unsafe cannot repair this invalid operation."
	}
	return false
}

// reportRawPointerProvenInvalid retains the producer's proof location so the
// editor and CLI can distinguish contradicted validity from missing permission.
// Rules: rules/memory/raw_pointers.md §§7(8),18(5),34;
// rules/tooling/diagnostics.md §9(1–7).
func (a *Analyzer) reportRawPointerProvenInvalid(token, cause lexer.Token, id, label, help, format string, args ...any) {
	endLine, endColumn := token.EndPosition()
	causeEndLine, causeEndColumn := cause.EndPosition()
	a.appendError(Error{
		ID: id, Severity: diagnostics.SeverityError, Message: fmt.Sprintf(format, args...), Help: help,
		File: token.File, Line: token.Line, Column: token.Column, EndLine: endLine, EndColumn: endColumn,
		Related: []RelatedLocation{{File: cause.File, Line: cause.Line, Column: cause.Column, EndLine: causeEndLine, EndColumn: causeEndColumn, Label: label}},
	})
}
