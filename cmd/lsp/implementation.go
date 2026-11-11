package main

import (
	"fmt"
	"sort"

	"sec/internal/lexer"
)

// implementationsForSource answers textDocument/implementation: from an
// interface or interface member (at its declaration or any use resolving to
// it), the explicitly conforming types or their implementing members, from
// Sema's recorded conformance facts only. Other symbols have no
// implementation result.
// Rules: rules/tooling/lsp.md — § 14(1); rules/declarations/interfaces.md — §§ 4–6.
func implementationsForSource(uri string, text string, pos position, overlays ...sourceOverlay) (locations []location) {
	defer func() {
		if recover() != nil {
			locations = nil
		}
	}()
	program := parseProgramForLSP(uri, text)
	if program == nil {
		return nil
	}
	overlay := firstSourceOverlay(overlays)
	_, analyzer, release := cachedLSPAnalysis(uri, text, program, overlay)
	defer release()
	use, ok := sourceTokenAtPosition(uri, text, pos)
	if !ok {
		return nil
	}
	targets := append([]lexer.Token{use}, analyzer.DefinitionsAt(use.File, use.Line, use.Column)...)
	seen := map[string]bool{}
	for _, target := range targets {
		for _, implementation := range analyzer.InterfaceImplementationsAt(target.File, target.Line, target.Column) {
			implementationURI := uri
			if implementation.File != "" {
				implementationURI = uriFromPath(implementation.File)
			}
			rng := definitionTokenRange(sourceTextForToken(uri, text, overlay, implementation), implementation)
			key := fmt.Sprintf("%s:%d:%d", implementationURI, rng.Start.Line, rng.Start.Character)
			if seen[key] {
				continue
			}
			seen[key] = true
			locations = append(locations, location{URI: implementationURI, Range: rng})
		}
	}
	sort.SliceStable(locations, func(i, j int) bool {
		if locations[i].URI != locations[j].URI {
			return locations[i].URI < locations[j].URI
		}
		if locations[i].Range.Start.Line != locations[j].Range.Start.Line {
			return locations[i].Range.Start.Line < locations[j].Range.Start.Line
		}
		return locations[i].Range.Start.Character < locations[j].Range.Start.Character
	})
	return locations
}
