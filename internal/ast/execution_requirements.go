package ast

import "sec/internal/lexer"

// ExecutionRequirementClause retains the written callable contract, including
// invalid/repeated source for recovery. Normalization belongs to Sema.
// Rules: rules/foundations/grammar.md — §101A; cancellation.md — §24A.
type ExecutionRequirementClause struct {
	Token    lexer.Token
	Open     lexer.Token
	Close    lexer.Token
	Entries  []lexer.Token
	Commas   []lexer.Token
	Repeated []*ExecutionRequirementClause
	Invalid  bool
}

// TypeGrouping preserves attachment boundaries around returned callable types.
// Rules: rules/foundations/grammar.md — §101A(5).
type TypeGrouping struct{ Open, Close lexer.Token }
