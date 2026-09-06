package diagnostics

import "testing"

func TestRegistryIsValid(t *testing.T) {
	if err := Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestKnownDiagnosticSeverities(t *testing.T) {
	tests := map[string]Severity{
		LexerInvalidUTF8:             SeverityError,
		LexerUnexpectedByteOrderMark: SeverityError,
		LexerUnsupportedWhitespace:   SeverityError,
		LexerNonNFCIdentifier:        SeverityError,
		LexerIdentifierCharacter:     SeverityError,
		ParserSyntaxError:            SeverityError,
		MissingModuleDeclaration:     SeverityError,
		DuplicateModuleDeclaration:   SeverityError,
		ModuleDeclarationConflict:    SeverityError,
		DuplicateLocalVariable:       SeverityError,
		InterfaceInheritanceCycle:    SeverityError,
		IncompatibleUnitConversion:   SeverityError,
		IncompleteEnumSwitch:         SeverityWarning,
		DuplicateSwitchCase:          SeverityError,
		OperatorNonOrderable:         SeverityError,
		OperatorInvalidShiftCount:    SeverityError,
		OperatorShiftOverflow:        SeverityError,
		OperatorNonComparable:        SeverityError,
		OperatorStringRuntimeConcat:  SeverityError,
		OperatorInvalidMembership:    SeverityError,
		OperatorInvalidConcatOperand: SeverityError,
		OperatorIntegerOverflow:      SeverityError,
		OperatorDivisionByZero:       SeverityError,
		OperatorRemainderByZero:      SeverityError,
		RedundantAssociatedStatic:    SeverityInformation,
		LargeValueParameter:          SeverityInformation,
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

func TestRetiredDiagnosticIDsRemainReserved(t *testing.T) {
	definition, ok := Lookup(OperatorStringRuntimeConcat)
	if !ok {
		t.Fatal("retired diagnostic S1020 must remain registered")
	}
	if !definition.Retired {
		t.Fatal("S1020 must be marked retired")
	}

	replacement, ok := Lookup(OperatorInvalidConcatOperand)
	if !ok {
		t.Fatal("missing invalid concat operand diagnostic")
	}
	if replacement.Retired {
		t.Fatal("S1022 must be active")
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
