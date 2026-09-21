package diagnostics

import "testing"

func TestRegistryIsValid(t *testing.T) {
	if err := Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestKnownDiagnosticSeverities(t *testing.T) {
	tests := map[string]Severity{
		LexerInvalidUTF8:                    SeverityError,
		LexerUnexpectedByteOrderMark:        SeverityError,
		LexerUnsupportedWhitespace:          SeverityError,
		LexerNonNFCIdentifier:               SeverityError,
		LexerIdentifierCharacter:            SeverityError,
		LexerMalformedBaseLiteral:           SeverityError,
		LexerInvalidBaseDigit:               SeverityError,
		LexerInvalidDigitSeparator:          SeverityError,
		LexerInvalidNumericSuffix:           SeverityError,
		LexerMissingExponentDigits:          SeverityError,
		LexerUnterminatedBlockComment:       SeverityError,
		LexerUnterminatedOrdinaryString:     SeverityError,
		LexerUnterminatedRawString:          SeverityError,
		LexerUnterminatedCharacterLiteral:   SeverityError,
		LexerUnterminatedInterpolatedString: SeverityError,
		LexerInvalidSourceCharacter:         SeverityError,
		ParserSyntaxError:                   SeverityError,
		MissingModuleDeclaration:            SeverityError,
		DuplicateModuleDeclaration:          SeverityError,
		ModuleDeclarationConflict:           SeverityError,
		DuplicateLocalVariable:              SeverityError,
		ReservedDeclarationName:             SeverityError,
		InvalidNominalTypeName:              SeverityError,
		GenericParameterShadowsType:         SeverityError,
		ParameterShadowsType:                SeverityError,
		UnresolvedGenericExtern:             SeverityError,
		UnreachableStatement:                SeverityError,
		InterfaceInheritanceCycle:           SeverityError,
		IncompatibleUnitConversion:          SeverityError,
		IncompleteEnumSwitch:                SeverityWarning,
		DuplicateSwitchCase:                 SeverityError,
		OperatorNonOrderable:                SeverityError,
		OperatorInvalidShiftCount:           SeverityError,
		OperatorShiftOverflow:               SeverityError,
		OperatorNonComparable:               SeverityError,
		OperatorStringRuntimeConcat:         SeverityError,
		OperatorInvalidMembership:           SeverityError,
		OperatorInvalidConcatOperand:        SeverityError,
		OperatorInvalidInterpolationValue:   SeverityError,
		OperatorIntegerOverflow:             SeverityError,
		OperatorDivisionByZero:              SeverityError,
		OperatorRemainderByZero:             SeverityError,
		RedundantAssociatedStatic:           SeverityInformation,
		TestDeclarationOutsideTestFile:      SeverityError,
		EmptyTestName:                       SeverityError,
		DuplicateTestIdentity:               SeverityError,
		TestReturnValue:                     SeverityError,
		TestingOutsideTestContext:           SeverityError,
		InvalidTestingExpectArguments:       SeverityError,
		InvalidTestingRequireArguments:      SeverityError,
		InvalidTestingLogArguments:          SeverityError,
		InvalidTestingExpectEqualArguments:  SeverityError,
		InvalidTestingRequireEqualArguments: SeverityError,
		InvalidTestingTerminationArguments:  SeverityError,
		UnreachableTryHandler:               SeverityError,
		LargeValueParameter:                 SeverityInformation,
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
		ParserInvalidTestDeclaration,
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

func TestUnknownAttributeDiagnosticIsRegistered(t *testing.T) {
	definition, ok := Lookup(UnknownAttribute)
	if !ok {
		t.Fatal("missing unknown attribute diagnostic")
	}
	if definition.Name != "attribute.unknown" || definition.Family != "attribute" || !definition.Mandatory || definition.DefaultSeverity != SeverityError {
		t.Fatalf("unknown attribute diagnostic = %+v", definition)
	}
}

func TestImmutableRequiresInitializerDiagnosticIsRegistered(t *testing.T) {
	definition, ok := Lookup(ImmutableRequiresInitializer)
	if !ok {
		t.Fatal("missing immutable initializer diagnostic")
	}
	if definition.Name != "variables.immutable-requires-initializer" || definition.Family != "variables" || !definition.Mandatory || definition.DefaultSeverity != SeverityError {
		t.Fatalf("immutable initializer diagnostic = %+v", definition)
	}
}

func TestUnattachedAttributeDiagnosticIsRegistered(t *testing.T) {
	definition, ok := Lookup(UnattachedAttribute)
	if !ok {
		t.Fatal("missing unattached attribute diagnostic")
	}
	if definition.Name != "attribute.unattached" || definition.Family != "attribute" || !definition.Mandatory || definition.DefaultSeverity != SeverityError {
		t.Fatalf("unattached attribute diagnostic = %+v", definition)
	}
}

func TestForbiddenTrySuccessHandlerDiagnosticIsRegistered(t *testing.T) {
	definition, ok := Lookup(ForbiddenTrySuccessHandler)
	if !ok {
		t.Fatal("missing forbidden try success handler diagnostic")
	}
	if definition.Name != "error-handling.forbidden-try-success-handler" || definition.Family != "error-handling" || !definition.Mandatory || definition.DefaultSeverity != SeverityError {
		t.Fatalf("forbidden try success handler diagnostic = %+v", definition)
	}
}
