package sema

import (
	"reflect"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Non-lexical borrow liveness for plain reference holders (MD-061 § 4).
//
// A borrow lives until its holder's last required use on every path, not
// until the end of the enclosing block. After each statement the analyzer
// ends the borrows of a local `ref T` / `ref mut T` holder that can no longer
// be used. The rule is conservative: a holder stays live when it is named in
// any later statement of the current or an enclosing block, anywhere in an
// enclosing loop it was declared outside of (the backedge), inside any defer
// body or lambda of the function, or when another live local carries the same
// reference origin without its own borrow record (a derived reference the
// analysis cannot see). Views, structs and other aggregates holding
// references keep lexical borrows.
//
// Rules:
//   - rules/memory/borrowing.md — § 8.1 "Compiler-derived liveness"
//   - rules/corrections/applied/md015-md061-duration-reborrow-correction-20261015.md — § 4

type statementFrame struct {
	block *ast.BlockStatement
	index int
	depth int
}

var (
	lexerTokenType     = reflect.TypeOf(lexer.Token{})
	identifierNodeType = reflect.TypeOf(&ast.Identifier{})
	deferNodeType      = reflect.TypeOf(&ast.DeferStatement{})
	lambdaNodeType     = reflect.TypeOf(&ast.LambdaExpression{})
	memberNodeType     = reflect.TypeOf(&ast.MemberExpression{})
	fieldInitNodeType  = reflect.TypeOf(&ast.StructLiteralField{})
)

// identifierUse is the set of identifier names an AST subtree mentions, and
// the names it mentions inside a defer body or lambda.
type identifierUse struct {
	names    map[string]bool
	deferred map[string]bool
}

func (a *Analyzer) identifierUseOf(node any) identifierUse {
	if node == nil {
		return identifierUse{}
	}
	if a.identifierUses == nil {
		a.identifierUses = map[any]identifierUse{}
	}
	if use, ok := a.identifierUses[node]; ok {
		return use
	}
	use := identifierUse{names: map[string]bool{}, deferred: map[string]bool{}}
	visited := map[uintptr]bool{}
	var walk func(value reflect.Value, deferred bool)
	walk = func(value reflect.Value, deferred bool) {
		switch value.Kind() {
		case reflect.Interface:
			if !value.IsNil() {
				walk(value.Elem(), deferred)
			}
		case reflect.Pointer:
			if value.IsNil() {
				return
			}
			if visited[value.Pointer()] {
				return
			}
			visited[value.Pointer()] = true
			switch value.Type() {
			case identifierNodeType:
				name := value.Interface().(*ast.Identifier).Value
				use.names[name] = true
				if deferred {
					use.deferred[name] = true
				}
				return
			case memberNodeType:
				// A member name (`inner.count`) is not a use of a local
				// named `count`; only the object is.
				member := value.Interface().(*ast.MemberExpression)
				walk(reflect.ValueOf(member.Object), deferred)
				for _, argument := range member.OwnerGenericArguments {
					walk(reflect.ValueOf(argument), deferred)
				}
				return
			case fieldInitNodeType:
				// Likewise a struct literal field name, unless it is the
				// shorthand that also names the value.
				if field := value.Interface().(*ast.StructLiteralField); field.Value != nil {
					walk(reflect.ValueOf(field.Value), deferred)
					return
				}
			case deferNodeType, lambdaNodeType:
				deferred = true
			}
			walk(value.Elem(), deferred)
		case reflect.Struct:
			if value.Type() == lexerTokenType {
				return
			}
			for index := 0; index < value.NumField(); index++ {
				if value.Type().Field(index).IsExported() {
					walk(value.Field(index), deferred)
				}
			}
		case reflect.Slice, reflect.Array:
			for index := 0; index < value.Len(); index++ {
				walk(value.Index(index), deferred)
			}
		case reflect.Map:
			for _, key := range value.MapKeys() {
				walk(value.MapIndex(key), deferred)
			}
		}
	}
	walk(reflect.ValueOf(node), false)
	a.identifierUses[node] = use
	return use
}

// holderUsedLater reports whether name may still be used from the statement
// the innermost frame points at (inclusive) onward.
func (a *Analyzer) holderUsedLater(name string, declaredDepth int) bool {
	if len(a.statementFrames) == 0 {
		return true
	}
	if a.identifierUseOf(a.statementFrames[0].block).deferred[name] {
		return true
	}
	innermost := len(a.statementFrames) - 1
	for level := innermost; level >= 0; level-- {
		frame := a.statementFrames[level]
		if frame.index >= len(frame.block.Statements) {
			continue
		}
		from := frame.index + 1
		if level == innermost {
			from = frame.index
		} else {
			// frame.block.Statements[frame.index] encloses the deeper frames.
			// Only statement-level branching constructs separate the paths of
			// their blocks; a block nested inside an expression is followed by
			// the rest of that statement, which therefore counts in full. A
			// loop declared around the holder repeats all of itself.
			switch enclosing := frame.block.Statements[frame.index].(type) {
			case *ast.IfStatement, *ast.MatchStatement:
			case *ast.WhileStatement, *ast.ForStatement:
				if declaredDepth <= frame.depth && a.identifierUseOf(enclosing).names[name] {
					return true
				}
			default:
				if a.identifierUseOf(enclosing).names[name] {
					return true
				}
			}
		}
		for _, later := range frame.block.Statements[from:] {
			if a.identifierUseOf(later).names[name] {
				return true
			}
		}
	}
	return false
}

// releaseDeadReferenceHolders ends the borrows of plain reference holders
// that have no further required use.
func (a *Analyzer) releaseDeadReferenceHolders(next lexer.Token) {
	if a.summaryPass || len(a.borrows) == 0 {
		return
	}
	holders := map[string]bool{}
	for _, records := range a.borrows {
		for _, record := range records {
			if record.Holder != "" && !record.LoopCarried {
				holders[record.Holder] = true
			}
		}
	}
	for holder := range holders {
		symbol, ok := a.symbols[holder]
		if !ok || symbol.Type.Kind != ReferenceType || !symbol.Local || symbol.ImplicitMember {
			continue
		}
		if a.holderUsedLater(holder, symbol.ScopeDepth) || a.originSharedWithUntrackedHolder(holder, symbol) {
			continue
		}
		a.recordReferenceLifetimeEndBefore(holder, a.currentStatementToken, next)
		a.dropBorrowsHeldBy(holder)
	}
}

// originSharedWithUntrackedHolder reports another live local that carries the
// same reference origin as holder but has no borrow record of its own, so a
// derived reference would lose its protection if holder's borrow ended. The
// origin is the holder type's origin name or, for a reference returned from
// a call, the roots of its resolved origin Places (the callee's return-origin
// summary); a holder whose origin is unknown stays live.
func (a *Analyzer) originSharedWithUntrackedHolder(holder string, symbol Symbol) bool {
	origins := a.holderOriginRoots(holder, symbol)
	if len(origins) == 0 {
		return true
	}
	recorded := map[string]bool{}
	for _, records := range a.borrows {
		for _, record := range records {
			recorded[record.Holder] = true
		}
	}
	for name, other := range a.symbols {
		// The origin root itself (a reference parameter the holder borrows
		// through) is the source of the borrow, not a derived copy of it.
		if name == holder || origins[name] || !other.Local || !typeCarriesReferenceOrigin(other.Type) || recorded[name] {
			continue
		}
		shared := false
		for root := range a.holderOriginRoots(name, other) {
			shared = shared || origins[root]
		}
		if shared && a.holderUsedLater(name, other.ScopeDepth) {
			return true
		}
	}
	return false
}

// holderOriginRoots names the storage roots a reference holder may refer to.
func (a *Analyzer) holderOriginRoots(holder string, symbol Symbol) map[string]bool {
	roots := map[string]bool{}
	if symbol.Type.ReferenceOriginName != "" {
		roots[symbol.Type.ReferenceOriginName] = true
		return roots
	}
	origin, ok := a.localRefContainers[holder]
	if !ok || origin.Unknown || origin.Ambiguous {
		return nil
	}
	for _, place := range localOriginPlaces(origin) {
		if place.Root == "" || place.AmbiguousProvenance {
			return nil
		}
		roots[place.Root] = true
		for _, alternative := range place.AlternativeOrigins {
			if alternative.Root == "" {
				return nil
			}
			roots[alternative.Root] = true
		}
	}
	return roots
}
