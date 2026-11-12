package sema

import (
	"fmt"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// UnsafeOperationKind is the canonical identity of a class of operations
// that require an unsafe context (unsafe.md § 47).
type UnsafeOperationKind string

const (
	UnsafeCallableCall        UnsafeOperationKind = "unsafe-call"
	UnsafeRawPointerAccess    UnsafeOperationKind = "raw-pointer-access"
	UnsafeRawPointerArith     UnsafeOperationKind = "raw-pointer-arithmetic"
	UnsafeRawToSafeReference  UnsafeOperationKind = "raw-to-safe-reference"
	UnsafeAddressOf           UnsafeOperationKind = "address-of"
	UnsafeRawAddressConvert   UnsafeOperationKind = "raw-address-conversion"
	UnsafeNullSentinel        UnsafeOperationKind = "null-sentinel"
	UnsafeInlineAssemblyBlock UnsafeOperationKind = "inline-assembly"
)

// UnsafeOperation is one registry entry: what the operation is, the
// obligations the unsafe context accepts, the diagnostic a missing context
// produces, whether call-form `unsafe f(...)` can authorize it, a safe
// alternative where one exists and the owning rule.
type UnsafeOperation struct {
	Kind            UnsafeOperationKind
	Explanation     string
	Obligations     []string
	Diagnostic      string
	CallForm        bool
	SafeAlternative string
	Rule            string
}

// unsafeOperationRegistry is the one shared source of unsafe operation
// definitions consumed by Sema diagnostics and tooling (unsafe.md § 47(1)–(4)).
var unsafeOperationRegistry = map[UnsafeOperationKind]UnsafeOperation{
	UnsafeCallableCall: {Kind: UnsafeCallableCall, Explanation: "calls an unsafe function, method or unsafe extern declaration whose caller obligations the compiler cannot prove",
		Obligations: []string{"every caller obligation the callee documents (unsafe.md § 8(2))"}, Diagnostic: diagnostics.UnsafeCallRequiresContext, CallForm: true,
		SafeAlternative: "call a safe wrapper that establishes the obligations", Rule: "rules/memory/unsafe.md — §§ 6, 8, 11"},
	UnsafeRawPointerAccess: {Kind: UnsafeRawPointerAccess, Explanation: "reads or writes storage through a raw pointer",
		Obligations: []string{"the pointer is non-null, aligned and in a live, valid range", "the pointee is initialized for a read and no conflicting access exists"},
		Diagnostic:  diagnostics.RawPointerUnsafeRequired, CallForm: true, SafeAlternative: "use a safe reference obtained from the owner", Rule: "rules/memory/unsafe.md — §§ 19–21; rules/memory/raw_pointers.md — §§ 11–14"},
	UnsafeRawPointerArith: {Kind: UnsafeRawPointerArith, Explanation: "performs raw-pointer arithmetic",
		Obligations: []string{"the result stays within the same allocation or one past its end", "the byte distance fits the target's pointer domain"},
		Diagnostic:  diagnostics.RawPointerUnsafeRequired, CallForm: true, SafeAlternative: "index a slice or array instead", Rule: "rules/memory/unsafe.md — § 20; rules/memory/raw_pointers.md — § 13"},
	UnsafeRawToSafeReference: {Kind: UnsafeRawToSafeReference, Explanation: "asserts that a raw pointer is a valid safe reference for the reference's whole live range",
		Obligations: []string{"the pointee is live, initialized and correctly aligned for the whole live range", "no conflicting mutable or shared authority exists"},
		Diagnostic:  diagnostics.RawPointerUnsafeRequired, SafeAlternative: "keep the safe reference you derived the pointer from", Rule: "rules/memory/unsafe.md — § 22; rules/memory/raw_pointers.md — § 18"},
	UnsafeAddressOf: {Kind: UnsafeAddressOf, Explanation: "forms a raw address of an addressable Place",
		Obligations: []string{"the pointer is used only while the Place is live and its access contract holds"},
		Diagnostic:  diagnostics.RawPointerUnsafeRequired, SafeAlternative: "borrow the Place with ref or ref mut", Rule: "rules/memory/raw_pointers.md — § 17"},
	UnsafeRawAddressConvert: {Kind: UnsafeRawAddressConvert, Explanation: "converts between a raw pointer and an integer address",
		Obligations: []string{"the target pointer width, address space and representation admit the conversion"},
		Diagnostic:  diagnostics.RawPointerUnsafeRequired, SafeAlternative: "keep the value as a safe reference or typed handle", Rule: "rules/memory/unsafe.md — § 20; rules/memory/raw_pointers.md — § 15"},
	UnsafeNullSentinel: {Kind: UnsafeNullSentinel, Explanation: "uses the raw-pointer null sentinel or null test",
		Obligations: []string{"null is confined to raw pointers at foreign boundaries"},
		Diagnostic:  diagnostics.NullOutsideUnsafe, SafeAlternative: "use Option and None for an absent value", Rule: "rules/memory/unsafe.md — § 19(1a); rules/platform/ffi.md — § 11"},
	UnsafeInlineAssemblyBlock: {Kind: UnsafeInlineAssemblyBlock, Explanation: "executes inline assembly whose effects the compiler cannot verify",
		Obligations: []string{"the assembly respects the declared operands, clobbers and target contract"},
		Diagnostic:  diagnostics.UnsafeOperationRequiresContext, SafeAlternative: "use a compiler-known operation", Rule: "rules/memory/unsafe.md — § 34; rules/platform/inline_assembly.md"},
}

// UnsafeOperations lists the registry in kind order for tooling.
func UnsafeOperations() []UnsafeOperation {
	out := make([]UnsafeOperation, 0, len(unsafeOperationRegistry))
	for _, operation := range unsafeOperationRegistry {
		out = append(out, operation)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out
}

// UnsafeOperationSite is one occurrence of an unsafe operation: its
// registry kind, the concrete operation, its location, whether an unsafe
// context authorized it and the obligations accepted there.
type UnsafeOperationSite struct {
	Kind        UnsafeOperationKind
	Operation   string
	Token       lexer.Token
	Authorized  bool
	Obligations []string
}

// unsafeContext is one active `unsafe { }` block.
type unsafeContext struct {
	statement *ast.UnsafeStatement
	uses      int
	nested    bool
	// errors is the error count when the block started; a block whose
	// contents are in error gets no advisory, since the corrected code may
	// need the context.
	errors int
}

// authorizeUnsafe records an unsafe operation site and reports whether an
// unsafe context authorizes it: an enclosing `unsafe { }` block, or for a
// call-form operation the call's own `unsafe` prefix. Every unsafe
// requirement goes through it, so classification is not distributed as
// ad-hoc checks (unsafe.md § 47(3)).
func (a *Analyzer) authorizeUnsafe(kind UnsafeOperationKind, operation string, token lexer.Token, call *ast.CallExpression, obligations []string) bool {
	registry := unsafeOperationRegistry[kind]
	prefixed := registry.CallForm && call != nil && call.HasUnsafePrefix()
	authorized := a.inUnsafe || prefixed
	if len(obligations) == 0 {
		obligations = registry.Obligations
	}
	key := sourceTokenLocation(token)
	if a.unsafeSiteIndex == nil {
		a.unsafeSiteIndex = map[sourceTokenKey]int{}
	}
	site := UnsafeOperationSite{Kind: kind, Operation: operation, Token: token, Authorized: authorized, Obligations: append([]string(nil), obligations...)}
	if index, seen := a.unsafeSiteIndex[key]; seen {
		a.unsafeSites[index] = site
	} else {
		a.unsafeSiteIndex[key] = len(a.unsafeSites)
		a.unsafeSites = append(a.unsafeSites, site)
	}
	switch {
	case a.inUnsafe && len(a.unsafeContexts) > 0:
		a.unsafeContexts[len(a.unsafeContexts)-1].uses++
	case prefixed:
		if a.usedUnsafePrefixes == nil {
			a.usedUnsafePrefixes = map[*ast.CallExpression]bool{}
		}
		a.usedUnsafePrefixes[call] = true
	}
	return authorized
}

// missingUnsafeHelp explains a missing context from the registry: the
// obligations the context would accept and that unsafe never repairs a
// proven-invalid operation (unsafe.md § 50(3)–(4); § 52(4)).
func missingUnsafeHelp(kind UnsafeOperationKind, obligations []string) string {
	registry := unsafeOperationRegistry[kind]
	if len(obligations) == 0 {
		obligations = registry.Obligations
	}
	where := "an `unsafe { }` block"
	if registry.CallForm {
		where = "call-form `unsafe f(...)` or an `unsafe { }` block"
	}
	help := "This operation " + registry.Explanation + ". Authorize it with " + where + " after establishing its caller obligations: " + strings.Join(obligations, "; ") + ". Unsafe accepts these obligations; it does not make a compiler-proven invalid operation valid."
	if registry.SafeAlternative != "" {
		help += " Safe alternative: " + registry.SafeAlternative + "."
	}
	return help
}

// enterUnsafeContext and leaveUnsafeContext bracket an `unsafe { }` block.
// A block that authorizes no unsafe operation is unnecessary; one nested in
// another unsafe context is redundant. Both are informational
// (unsafe.md §§ 5, 50(5)).
func (a *Analyzer) enterUnsafeContext(statement *ast.UnsafeStatement) {
	a.unsafeContexts = append(a.unsafeContexts, unsafeContext{statement: statement, nested: a.inUnsafe, errors: len(a.errors)})
}

func (a *Analyzer) leaveUnsafeContext() {
	context := a.unsafeContexts[len(a.unsafeContexts)-1]
	a.unsafeContexts = a.unsafeContexts[:len(a.unsafeContexts)-1]
	// Operations inside a nested block are authorized by the enclosing one
	// too, so the enclosing block is not empty.
	if len(a.unsafeContexts) > 0 {
		a.unsafeContexts[len(a.unsafeContexts)-1].uses += context.uses
	}
	if a.summaryPass || context.statement == nil || len(a.errors) > context.errors {
		return
	}
	switch {
	case context.nested:
		a.addUnsafeAdvisory(context.statement.Token, diagnostics.UnsafeRedundant,
			"The enclosing unsafe context already authorizes these operations; remove this inner unsafe.",
			"unsafe block is redundant inside another unsafe context")
	case context.uses == 0:
		a.addUnsafeAdvisory(context.statement.Token, diagnostics.UnsafeUnnecessary,
			"The block contains no operation that requires unsafe; remove unsafe so the code stays reviewable.",
			"unsafe block contains no unsafe operation")
	}
}

// noteUnsafePrefix remembers a call-form unsafe prefix so an unused one is
// reported once the analysis knows every authorized operation.
func (a *Analyzer) noteUnsafePrefix(call *ast.CallExpression) {
	if call == nil || !call.HasUnsafePrefix() || a.summaryPass {
		return
	}
	if a.unsafePrefixes == nil {
		a.unsafePrefixes = map[*ast.CallExpression]bool{}
	}
	if _, seen := a.unsafePrefixes[call]; !seen {
		a.unsafePrefixes[call] = a.inUnsafe
	}
}

// reportUnusedUnsafePrefixes reports call-form unsafe that authorized
// nothing, or that an enclosing block already covered.
func (a *Analyzer) reportUnusedUnsafePrefixes() {
	calls := make([]*ast.CallExpression, 0, len(a.unsafePrefixes))
	for call := range a.unsafePrefixes {
		calls = append(calls, call)
	}
	sort.Slice(calls, func(i, j int) bool { return tokenBefore(calls[i].Unsafe, calls[j].Unsafe) })
	errorLines := map[string]bool{}
	for _, err := range a.errors {
		errorLines[fmt.Sprintf("%s:%d", err.File, err.Line)] = true
	}
	for _, call := range calls {
		// A call in error may need the prefix once corrected.
		if errorLines[fmt.Sprintf("%s:%d", call.Unsafe.File, call.Unsafe.Line)] {
			continue
		}
		switch {
		case a.unsafePrefixes[call]:
			a.addUnsafeAdvisory(call.Unsafe, diagnostics.UnsafeRedundant,
				"The enclosing unsafe block already authorizes this call; remove the call-form unsafe.",
				"call-form unsafe is redundant inside an unsafe block")
		case !a.usedUnsafePrefixes[call]:
			a.addUnsafeAdvisory(call.Unsafe, diagnostics.UnsafeUnnecessary,
				"This call needs no unsafe; remove the prefix so the code stays reviewable.",
				"call-form unsafe authorizes no unsafe operation")
		}
	}
}

// addUnsafeAdvisory records an informational unsafe advisory. Advisories
// never affect validity, so they are kept apart from the analysis errors
// and published by tooling as Information (unsafe.md § 50(5)).
func (a *Analyzer) addUnsafeAdvisory(token lexer.Token, id, help, format string, args ...any) {
	endLine, endColumn := token.EndPosition()
	advisory := Error{ID: id, Severity: diagnostics.SeverityInformation, Help: help, Message: fmt.Sprintf(format, args...),
		File: token.File, Line: token.Line, Column: token.Column, EndLine: endLine, EndColumn: endColumn}
	for _, existing := range a.unsafeAdvisories {
		if existing.ID == advisory.ID && existing.File == advisory.File && existing.Line == advisory.Line && existing.Column == advisory.Column {
			return
		}
	}
	a.unsafeAdvisories = append(a.unsafeAdvisories, advisory)
}

// UnsafeAdvisories are this analysis's unnecessary and redundant unsafe
// contexts, in source order.
func (a *Analyzer) UnsafeAdvisories() []Error {
	if a == nil {
		return nil
	}
	out := append([]Error(nil), a.unsafeAdvisories...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Column < out[j].Column
	})
	return out
}

// UnsafeOperationSites lists this analysis's unsafe operation sites in
// source order, for tooling (unsafe.md § 52(1), (5)).
func (a *Analyzer) UnsafeOperationSites() []UnsafeOperationSite {
	if a == nil {
		return nil
	}
	sites := append([]UnsafeOperationSite(nil), a.unsafeSites...)
	sort.Slice(sites, func(i, j int) bool { return tokenBefore(sites[i].Token, sites[j].Token) })
	return sites
}

// UnsafeOperationAt returns the unsafe operation site at a source position
// with its registry entry.
func (a *Analyzer) UnsafeOperationAt(file string, line, column int) (UnsafeOperationSite, UnsafeOperation, bool) {
	if a == nil {
		return UnsafeOperationSite{}, UnsafeOperation{}, false
	}
	for _, site := range a.unsafeSites {
		token := site.Token
		end := token.Column + len([]rune(token.Lexeme))
		if token.File == file && token.Line == line && column >= token.Column && column < end {
			return site, unsafeOperationRegistry[site.Kind], true
		}
	}
	return UnsafeOperationSite{}, UnsafeOperation{}, false
}
