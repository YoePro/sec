package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// recordTypePackIndexBorrow distinguishes a selected reference parameter's
// call-bounded borrow from a by-value read. Candidate overloads never call it;
// ordinary Place/authority validation still checks the selected borrow.
// Rules: rules/declarations/functions.md §§17–18,38A(5)(2),(5);
// rules/memory/borrowing.md §15.2–15.4.
func (a *Analyzer) recordTypePackIndexBorrow(argument ast.Expression) {
	index, ok := argument.(*ast.IndexExpression)
	if !ok {
		return
	}
	if _, owned := a.typePackIndexReaders[index]; !owned {
		return
	}
	if a.typePackBorrowedIndexes == nil {
		a.typePackBorrowedIndexes = map[*ast.IndexExpression]bool{}
	}
	a.typePackBorrowedIndexes[index] = true
}

// typePackValueRead returns the earliest true by-value read of one position
// in a template. Borrowed positions require no copy, and copying a different
// position does not require this position to be copyable.
// Rules: rules/declarations/functions.md §38A(5)(2),(5),(10)(1–2).
func (a *Analyzer) typePackValueRead(function Function, position int) (lexer.Token, bool) {
	var first lexer.Token
	found := false
	owner := sourceTokenLocation(function.Token)
	for index, reader := range a.typePackIndexReaders {
		if reader != owner || a.typePackIndexes[index] != position || a.typePackBorrowedIndexes[index] {
			continue
		}
		token := expressionToken(index.Index)
		if !found || token.ByteStart < first.ByteStart {
			first, found = token, true
		}
	}
	return first, found
}
