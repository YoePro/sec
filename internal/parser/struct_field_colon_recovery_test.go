package parser

import (
	"testing"

	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// Rules:
//   - rules/compiler/parser_recovery.md — "Struct declaration recovery"
//   - rules/compiler/parser_recovery.md — "Missing field colon"
func TestMissingStructFieldColonRecordsVirtualRepair(t *testing.T) {
	p := New(lexer.New("type Record struct { Value int, Next: int, }"))
	result := p.Parse()
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].ID != compilerdiagnostics.ParserMissingToken {
		t.Fatalf("missing colon did not produce one structured missing-token diagnostic: %+v", result.Diagnostics)
	}

	found := false
	for _, event := range result.Recovery {
		if event.Kind == RecoveryInsertMissingToken && len(event.Expected) == 1 && event.Expected[0] == lexer.COLON {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing colon did not record a virtual token repair: %+v", result.Recovery)
	}
}
