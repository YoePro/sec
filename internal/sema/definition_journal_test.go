package sema

import (
	"reflect"
	"testing"

	"sec/internal/lexer"
)

// A loop-body replay journals the definition keys it writes and restores
// them afterwards, including writes of a nested replay, without copying the
// whole definition map at each loop header.
func TestDefinitionJournalRestoresReplayWrites(t *testing.T) {
	a := NewAnalyzer()
	use := func(line int) sourceTokenKey { return sourceTokenKey{File: "f.sec", Line: line, Column: 1} }
	decl := func(line int) []lexer.Token { return []lexer.Token{{File: "f.sec", Line: line, Column: 5}} }
	a.definitionTokens = map[sourceTokenKey][]lexer.Token{}
	a.writeDefinitions(use(1), decl(10))
	before := map[sourceTokenKey][]lexer.Token{use(1): decl(10)}

	a.beginDefinitionJournal()
	a.writeDefinitions(use(1), decl(11))
	a.writeDefinitions(use(2), decl(12))
	a.beginDefinitionJournal()
	a.writeDefinitions(use(3), decl(13))
	a.writeDefinitions(use(1), decl(14))
	a.endDefinitionJournal()
	if !reflect.DeepEqual(a.definitionTokens[use(1)], decl(11)) || a.definitionTokens[use(3)] != nil {
		t.Fatalf("inner replay not undone: %v", a.definitionTokens)
	}
	a.endDefinitionJournal()
	if !reflect.DeepEqual(a.definitionTokens, before) || len(a.definitionJournals) != 0 {
		t.Fatalf("outer replay not undone: %v", a.definitionTokens)
	}
}
