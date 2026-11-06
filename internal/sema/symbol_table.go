package sema

import "iter"

// symbolTable is the analyzer's lexical symbol environment as a chain of
// layers. Entering a scope (a block body, branch, loop body, function or test
// body, match arm, try handler, lambda or loop-header replay) creates a child
// layer instead of copying every visible symbol; leaving the scope restores the
// parent, discarding the child with all its writes and removals. The observable
// behavior is that of the former per-scope map copy, at a cost proportional to
// what the scope itself defines or changes.
//
// Invariant: a layer is not written while a child of it is in use. Every scope
// boundary saves the parent, works in the child and restores the parent, so
// writes always land in the innermost live layer. Symbol values are shared
// between layers; their reference-typed fields (callable target sets, string
// rune-count evidence) are replaced rather than mutated in place, and public
// snapshots still detach them with cloneSymbol.
//
// Rules:
//   - rules/tooling/lsp.md — "Responsiveness model"
//   - rules/compiler/compiler_analysis.md — immutable analysis results
type symbolTable struct {
	parent *symbolTable
	local  map[string]Symbol
	// hidden names are removed in this layer although a parent defines them.
	hidden map[string]struct{}
	depth  int
}

// maxSymbolTableDepth bounds the lookup chain; a deeper child flattens its
// parent first.
const maxSymbolTableDepth = 48

func newSymbolTable() *symbolTable {
	return &symbolTable{local: map[string]Symbol{}}
}

// child opens a scope layer over t.
func (t *symbolTable) child() *symbolTable {
	if t == nil {
		return newSymbolTable()
	}
	if t.depth >= maxSymbolTableDepth {
		t = t.flatten()
	}
	return &symbolTable{parent: t, local: map[string]Symbol{}, depth: t.depth + 1}
}

// flatten returns an independent single-layer table with the same contents.
func (t *symbolTable) flatten() *symbolTable {
	out := &symbolTable{local: make(map[string]Symbol, t.len())}
	for name, symbol := range t.all() {
		out.local[name] = symbol
	}
	return out
}

func (t *symbolTable) lookup(name string) (Symbol, bool) {
	for layer := t; layer != nil; layer = layer.parent {
		if symbol, ok := layer.local[name]; ok {
			return symbol, true
		}
		if _, removed := layer.hidden[name]; removed {
			return Symbol{}, false
		}
	}
	return Symbol{}, false
}

func (t *symbolTable) get(name string) Symbol {
	symbol, _ := t.lookup(name)
	return symbol
}

func (t *symbolTable) has(name string) bool {
	_, ok := t.lookup(name)
	return ok
}

func (t *symbolTable) set(name string, symbol Symbol) {
	t.local[name] = symbol
	if t.hidden != nil {
		delete(t.hidden, name)
	}
}

func (t *symbolTable) del(name string) {
	delete(t.local, name)
	if t.parent != nil && t.parent.has(name) {
		if t.hidden == nil {
			t.hidden = map[string]struct{}{}
		}
		t.hidden[name] = struct{}{}
	}
}

// all yields every visible symbol once, innermost definition first. Like
// ranging over a map, the order is unspecified.
func (t *symbolTable) all() iter.Seq2[string, Symbol] {
	return func(yield func(string, Symbol) bool) {
		// Like a nil map, a nil table is empty.
		if t == nil {
			return
		}
		if t.parent == nil {
			for name, symbol := range t.local {
				if !yield(name, symbol) {
					return
				}
			}
			return
		}
		// Only the names of the upper layers need tracking; the bottom layer,
		// usually the module-level declarations, is checked against them.
		seen := map[string]struct{}{}
		for layer := t; layer != nil; layer = layer.parent {
			bottom := layer.parent == nil
			for name, symbol := range layer.local {
				if _, done := seen[name]; done {
					continue
				}
				if !bottom {
					seen[name] = struct{}{}
				}
				if !yield(name, symbol) {
					return
				}
			}
			for name := range layer.hidden {
				seen[name] = struct{}{}
			}
		}
	}
}

func (t *symbolTable) len() int {
	count := 0
	for range t.all() {
		count++
	}
	return count
}

// symbolTableOf builds a single-layer table from a map (tests, fixtures).
func symbolTableOf(symbols map[string]Symbol) *symbolTable {
	table := newSymbolTable()
	for name, symbol := range symbols {
		table.local[name] = symbol
	}
	return table
}
