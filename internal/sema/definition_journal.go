package sema

import "sec/internal/lexer"

// definitionJournal records, for one loop-body replay, the value every
// definition key had before the replay first wrote it, so the replay's
// navigation writes can be undone without copying the whole definition map
// at each loop header (tooling.lsp-responsiveness).
type definitionJournal map[sourceTokenKey]definitionJournalEntry

type definitionJournalEntry struct {
	tokens  []lexer.Token
	existed bool
}

// beginDefinitionJournal starts recording writes for a loop-body replay.
func (a *Analyzer) beginDefinitionJournal() {
	a.definitionJournals = append(a.definitionJournals, definitionJournal{})
}

// endDefinitionJournal restores every key the replay wrote. Writes of a
// nested replay were recorded in every active journal, so the outer one
// still restores them.
func (a *Analyzer) endDefinitionJournal() {
	last := len(a.definitionJournals) - 1
	journal := a.definitionJournals[last]
	a.definitionJournals = a.definitionJournals[:last]
	for key, entry := range journal {
		if entry.existed {
			a.definitionTokens[key] = entry.tokens
		} else {
			delete(a.definitionTokens, key)
		}
	}
}

// writeDefinitions stores the definitions of one use, journaling the
// previous value for every active replay.
func (a *Analyzer) writeDefinitions(key sourceTokenKey, tokens []lexer.Token) {
	if len(a.definitionJournals) > 0 {
		previous, existed := a.definitionTokens[key]
		for _, journal := range a.definitionJournals {
			if _, recorded := journal[key]; !recorded {
				journal[key] = definitionJournalEntry{tokens: previous, existed: existed}
			}
		}
	}
	a.definitionTokens[key] = tokens
}
