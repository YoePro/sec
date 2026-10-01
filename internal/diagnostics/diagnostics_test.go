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
		UnresolvedImport:                    SeverityError,
		UnionPayloadMoveStorage:             SeverityError,
		DuplicateContractMembershipValue:    SeverityError,
		EmptyContractMembership:             SeverityError,
		RecursiveStructLayout:               SeverityError,
		SwitchPatternBinding:                SeverityError,
		LocalShadowsDeclaration:             SeverityError,
		UseAfterDiscard:                     SeverityError,
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
		OperatorIntegerOverflow:             SeverityWarning,
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

func TestStorageSiteContractDiagnosticIsRegistered(t *testing.T) {
	definition, ok := Lookup(StorageSiteContract)
	if !ok {
		t.Fatal("missing storage-site contract diagnostic")
	}
	if definition.Name != "types.storage-site-contract" || definition.Family != "types" || !definition.Mandatory || definition.DefaultSeverity != SeverityError {
		t.Fatalf("storage-site contract diagnostic = %+v", definition)
	}
}

func TestDuplicateContractMembershipValueDiagnosticIsRegistered(t *testing.T) {
	definition, ok := Lookup(DuplicateContractMembershipValue)
	if !ok {
		t.Fatal("missing duplicate contract membership value diagnostic")
	}
	if definition.Name != "types.duplicate-in-contract-value" || definition.Family != "types" || !definition.Mandatory || definition.DefaultSeverity != SeverityError {
		t.Fatalf("duplicate contract membership value diagnostic = %+v", definition)
	}
}

func TestEmptyContractMembershipDiagnosticIsRegistered(t *testing.T) {
	definition, ok := Lookup(EmptyContractMembership)
	if !ok {
		t.Fatal("missing empty contract membership diagnostic")
	}
	if definition.Name != "types.empty-in-contract" || definition.Family != "types" || !definition.Mandatory || definition.DefaultSeverity != SeverityError {
		t.Fatalf("empty contract membership diagnostic = %+v", definition)
	}
}

func TestRecursiveStructLayoutDiagnosticIsRegistered(t *testing.T) {
	definition, ok := Lookup(RecursiveStructLayout)
	if !ok {
		t.Fatal("missing recursive struct layout diagnostic")
	}
	if definition.Name != "struct.recursive-by-value-layout" || definition.Family != "struct" || !definition.Mandatory || definition.DefaultSeverity != SeverityError {
		t.Fatalf("recursive struct layout diagnostic = %+v", definition)
	}
}

func TestSwitchPatternBindingDiagnosticIsRegistered(t *testing.T) {
	definition, ok := Lookup(SwitchPatternBinding)
	if !ok {
		t.Fatal("missing switch pattern binding diagnostic")
	}
	if definition.Name != "switch.pattern-binding" || definition.Family != "flow-control" || !definition.Mandatory || definition.DefaultSeverity != SeverityError {
		t.Fatalf("switch pattern binding diagnostic = %+v", definition)
	}
}

func TestLocalShadowsDeclarationDiagnosticIsRegistered(t *testing.T) {
	definition, ok := Lookup(LocalShadowsDeclaration)
	if !ok {
		t.Fatal("missing local shadowing diagnostic")
	}
	if definition.Name != "names.local-shadows-declaration" || definition.Family != "names" || !definition.Mandatory || definition.DefaultSeverity != SeverityError {
		t.Fatalf("local shadowing diagnostic = %+v", definition)
	}
}
