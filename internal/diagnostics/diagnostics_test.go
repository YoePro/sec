package diagnostics

import "testing"

func TestRegistryIsValid(t *testing.T) {
	if err := Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestKnownDiagnosticSeverities(t *testing.T) {
	tests := map[string]Severity{
		ParserSyntaxError:          SeverityError,
		MissingModuleDeclaration:   SeverityError,
		DuplicateModuleDeclaration: SeverityError,
		ModuleDeclarationConflict:  SeverityError,
		DuplicateLocalVariable:     SeverityError,
		LargeValueParameter:        SeverityInformation,
	}

	for id, severity := range tests {
		definition, ok := Lookup(id)
		if !ok {
			t.Fatalf("missing diagnostic %s", id)
		}
		if definition.DefaultSeverity != severity {
			t.Fatalf("%s severity = %q, want %q", id, definition.DefaultSeverity, severity)
		}
	}
}

func TestParserRecoveryDiagnosticsAreRegistered(t *testing.T) {
	ids := []string{
		ParserSyntaxError,
		ParserMissingToken,
		ParserUnexpectedToken,
		ParserUnterminatedDelimiter,
		ParserInvalidDeclaration,
		ParserInvalidStatement,
		ParserInvalidExpression,
		ParserInvalidTypeReference,
		ParserInvalidPattern,
		ParserMissingSeparator,
		ParserMisplacedKeyword,
		ParserReservedSyntax,
		ParserInvalidAssignmentExpr,
		ParserChainedComparison,
		ParserRecoveryLimit,
		ParserUnexpectedEndOfFile,
		ParserInvalidBlockMember,
		ParserCompatibilitySyntax,
	}
	for _, id := range ids {
		definition, ok := Lookup(id)
		if !ok {
			t.Fatalf("parser diagnostic %s is not registered", id)
		}
		if id != ParserCompatibilitySyntax && definition.DefaultSeverity != SeverityError {
			t.Fatalf("parser diagnostic %s severity = %q, want error", id, definition.DefaultSeverity)
		}
	}
}
