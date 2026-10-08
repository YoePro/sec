package sema

import (
	"sec/internal/lexer"
)

// contractSource retains the defining location through named inheritance.
// Rules: rules/types/contracts.md — Composition and Diagnostics.
func contractSource(contract Contract) lexer.Token {
	switch c := contract.(type) {
	case RangeContract:
		return c.Token
	case MembershipContract:
		return c.Token
	case MultipleOfContract:
		return c.Token
	case LengthContract:
		return c.Token
	case MarkerContract:
		return c.Token
	case RegexContract:
		return c.Token
	}
	return lexer.Token{}
}

// relateErrorsSince enriches an owning error without changing its identity,
// severity, primary location or a more specific existing related location.
// Rules: rules/tooling/diagnostics.md — §12 related locations;
// rules/types/contracts.md — Diagnostics; rules/types/default_values.md — Diagnostics.
func (a *Analyzer) relateErrorsSince(start int, token lexer.Token, label string) {
	if token.Line <= 0 || token.Column <= 0 {
		return
	}
	for i := start; i < len(a.errors); i++ {
		e := &a.errors[i]
		if e.PreviousLine > 0 || e.File == token.File && e.Line == token.Line && e.Column == token.Column {
			continue
		}
		e.PreviousFile, e.PreviousLine, e.PreviousColumn = token.File, token.Line, token.Column
		e.RelatedLabel = label
	}
}

// addContractError links a proven offending value to the precise inherited or
// local contract without discovering locations through source spelling.
// Rules: rules/types/contracts.md — Diagnostics.
func (a *Analyzer) addContractError(token lexer.Token, contract Contract, id string, help string, format string, args ...any) {
	start := len(a.errors)
	a.addErrorAtTokenWithMetadata(token, id, help, format, args...)
	a.relateErrorsSince(start, contractSource(contract), "contract declaration")
}

// relateMissingDefault links missing initialization to the declared field,
// invalid explicit default, or constrained type that prevents construction.
// Rules: rules/types/default_values.md — Diagnostics and Diagnostic examples.
func (a *Analyzer) relateMissingDefault(start int, typ Type, field lexer.Token) {
	if typ.InvalidExplicitDefault && typ.ExplicitDefaultToken.Line > 0 {
		a.relateErrorsSince(start, typ.ExplicitDefaultToken, "invalid explicit default")
	} else if _, _, ambiguous := ambiguousImplicitDefault(typ); ambiguous {
		a.relateErrorsSince(start, typ.DeclarationToken, "type requiring an explicit default")
	} else {
		a.relateErrorsSince(start, field, "field requiring initialization")
		a.relateErrorsSince(start, typ.DeclarationToken, "type without an implicit default")
	}
}

// relateConstrainedAssignment retains the source of the contract requiring try.
// Rules: rules/types/contracts.md — Initialization and assignment, Diagnostics.
func (a *Analyzer) relateConstrainedAssignment(start int, typ Type) {
	for _, c := range typ.Contracts {
		a.relateErrorsSince(start, contractSource(c), "contract requiring checked assignment")
	}
	// No contract source exists for synthetic/tooling-only types.
	a.relateErrorsSince(start, typ.DeclarationToken, "constrained type declaration")
}
