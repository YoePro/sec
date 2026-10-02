package diagnostics

import (
	"fmt"
	"sort"
)

type Severity string

const (
	SeverityError       Severity = "error"
	SeverityWarning     Severity = "warning"
	SeverityInformation Severity = "information"
)

type Definition struct {
	ID              string
	Name            string
	Family          string
	DefaultSeverity Severity
	Mandatory       bool
	Retired         bool
}

const (
	LexerInvalidUTF8                    = "L1001"
	LexerUnexpectedByteOrderMark        = "L1002"
	LexerUnsupportedWhitespace          = "L1003"
	LexerNonNFCIdentifier               = "L1004"
	LexerIdentifierCharacter            = "L1005"
	LexerUnknownEscape                  = "L1006"
	LexerMalformedEscape                = "L1007"
	LexerInvalidUnicodeEscape           = "L1008"
	LexerCharacterLiteralLength         = "L1009"
	LexerMalformedBaseLiteral           = "L1010"
	LexerInvalidBaseDigit               = "L1011"
	LexerInvalidDigitSeparator          = "L1012"
	LexerInvalidNumericSuffix           = "L1013"
	LexerMissingExponentDigits          = "L1014"
	LexerUnterminatedBlockComment       = "L1015"
	LexerUnterminatedOrdinaryString     = "L1016"
	LexerUnterminatedRawString          = "L1017"
	LexerUnterminatedCharacterLiteral   = "L1018"
	LexerUnterminatedInterpolatedString = "L1019"
	LexerInvalidSourceCharacter         = "L1020"
	ParserSyntaxError                   = "P2001"
	ParserMissingToken                  = "P2002"
	ParserUnexpectedToken               = "P2003"
	ParserUnterminatedDelimiter         = "P2004"
	ParserInvalidDeclaration            = "P2005"
	ParserInvalidStatement              = "P2006"
	ParserInvalidExpression             = "P2007"
	ParserInvalidTypeReference          = "P2008"
	ParserInvalidPattern                = "P2009"
	ParserMissingSeparator              = "P2010"
	ParserMisplacedKeyword              = "P2011"
	ParserReservedSyntax                = "P2012"
	ParserInvalidAssignmentExpr         = "P2013"
	ParserChainedComparison             = "P2014"
	ParserRecoveryLimit                 = "P2015"
	ParserUnexpectedEndOfFile           = "P2016"
	ParserInvalidBlockMember            = "P2017"
	ParserCompatibilitySyntax           = "P2018"
	ParserUnimplementedFunction         = "P2019"
	ParserInvalidTestDeclaration        = "P2020"
	ParserMalformedForeignQualification = "P2021"
	ParserUnsupportedForeignForm        = "P2022"
	ParserLegacyAssignedType            = "P2023"
	ParserMultipleUnderlyingTypes       = "P2024"
	ParserLegacyInlineContract          = "P2025"
	ParserPrefixSequenceType            = "P2026"
	ParserFutureStructDeclaration       = "P2027"
	ParserLegacyEnumColonInitializer    = "P2028"
	ParserMultipleTypeDeclarationNames  = "P2029"
	MissingModuleDeclaration            = "S1001"
	DuplicateModuleDeclaration          = "S1002"
	ModuleDeclarationConflict           = "S1003"
	DuplicateLocalVariable              = "S1004"
	UnhandledMustUseResult              = "S1005"
	NonDiscardableValue                 = "S1006"
	ImplicitMoveDisallowed              = "S1007"
	InvalidExplicitDefault              = "S1008"
	NoDefaultValue                      = "S1009"
	MissingNonDefaultableField          = "S1010"
	InvalidMembershipValue              = "S1011"
	InterfaceInheritanceCycle           = "S1012"
	IncompatibleUnitConversion          = "S1013"
	IncompleteEnumSwitch                = "S1014"
	DuplicateSwitchCase                 = "S1015"
	OperatorNonOrderable                = "S1016"
	OperatorInvalidShiftCount           = "S1017"
	OperatorShiftOverflow               = "S1018"
	OperatorNonComparable               = "S1019"
	OperatorStringRuntimeConcat         = "S1020"
	OperatorInvalidMembership           = "S1021"
	OperatorInvalidConcatOperand        = "S1022"
	OperatorIntegerOverflow             = "S1023"
	OperatorDivisionByZero              = "S1024"
	OperatorRemainderByZero             = "S1025"
	RedundantAssociatedStatic           = "S1026"
	InvalidGenericParameterName         = "S1027"
	ReservedDeclarationName             = "S1028"
	OperatorInvalidInterpolationValue   = "S1029"
	TestDeclarationOutsideTestFile      = "S1030"
	EmptyTestName                       = "S1031"
	DuplicateTestIdentity               = "S1032"
	TestReturnValue                     = "S1033"
	TestingOutsideTestContext           = "S1034"
	InvalidTestingExpectArguments       = "S1035"
	InvalidTestingRequireArguments      = "S1036"
	InvalidTestingLogArguments          = "S1037"
	InvalidNominalTypeName              = "S1038"
	GenericParameterShadowsType         = "S1039"
	ParameterShadowsType                = "S1040"
	UnresolvedGenericExtern             = "S1041"
	InvalidTestingExpectEqualArguments  = "S1042"
	InvalidTestingRequireEqualArguments = "S1043"
	InvalidTestingTerminationArguments  = "S1044"
	UnreachableTryHandler               = "S1045"
	UnknownAttribute                    = "S1046"
	ImmutableRequiresInitializer        = "S1047"
	UnattachedAttribute                 = "S1048"
	ForbiddenTrySuccessHandler          = "S1049"
	StorageSiteContract                 = "S1050"
	UnresolvedImport                    = "S1051"
	UnionPayloadMoveStorage             = "S1052"
	DuplicateContractMembershipValue    = "S1053"
	EmptyContractMembership             = "S1054"
	RecursiveStructLayout               = "S1055"
	SwitchPatternBinding                = "S1056"
	LocalShadowsDeclaration             = "S1057"
	RegexContractUnavailable            = "S1058"
	DefaultViolatesContract             = "S1059"
	DefaultNotRepresentable             = "S1060"
	AmbiguousImplicitDefault            = "S1061"
	TypeNoDefaultValue                  = "S1062"
	InvalidDefaultedField               = "S1063"
	InapplicableContract                = "S1064"
	UnsatisfiableContractSet            = "S1065"
	InvalidContractArgument             = "S1066"
	IncompatibleMembershipValue         = "S1067"
	ValueViolatesContract               = "S1068"
	ConstrainedAssignmentRequiresTry    = "S1069"
	AuthoritativeMemberReplacement      = "S1070"
	RecursiveUnionLayout                = "S1071"
	ArenaUnsizedAllocationType          = "S1072"
	ArenaAllocationMissingDefault       = "S1073"
	ArenaNonTrivialDestructionType      = "S1074"
	StructuralMutationDuringIteration   = "S1075"
	ForeignUnknownCFundamentalType      = "S1076"
	ForeignCABIModelUnavailable         = "S1077"
	ForeignUnresolvedCBindingType       = "S1078"
	IllegalForeignType                  = "S1079"
	NullOutsideUnsafe                   = "S1080"
	NullWithoutRawPointerContext        = "S1081"
	NullEquality                        = "S1082"
	NullTestRequiresRawPointer          = "S1083"
	InvalidCustomFreeDeclaration        = "S1084"
	CustomFreePartialMove               = "S1085"
	DeferInsideFree                     = "S1086"
	FreeConsumesSelf                    = "S1087"
	UseAfterMove                        = "S1088"
	ConditionallyUnavailableUse         = "S1089"
	PartiallyUnavailableUse             = "S1090"
	HeterogeneousTryErrorBinding        = "S1091"
	NoPanicViolation                    = "S1092"
	TryPropagationIncompatible          = "S1093"
	TryResidualUnpropagatable           = "S1094"
	InvalidTryHandlerPattern            = "S1095"
	TryAssignmentWithoutFallibleTarget  = "S1096"
	UnreachableStatement                = "S3001"
	UseAfterDiscard                     = "S4001"
	LargeValueParameter                 = "A2001"
)

var registry = map[string]Definition{
	InvalidGenericParameterName:         {ID: InvalidGenericParameterName, Name: "names.invalid-generic-type-parameter", Family: "names", DefaultSeverity: SeverityError, Mandatory: true},
	InvalidNominalTypeName:              {ID: InvalidNominalTypeName, Name: "names.invalid-nominal-type", Family: "names", DefaultSeverity: SeverityError, Mandatory: true},
	GenericParameterShadowsType:         {ID: GenericParameterShadowsType, Name: "names.generic-parameter-shadows-type", Family: "names", DefaultSeverity: SeverityError, Mandatory: true},
	ParameterShadowsType:                {ID: ParameterShadowsType, Name: "names.parameter-shadows-type", Family: "names", DefaultSeverity: SeverityError, Mandatory: true},
	UnresolvedGenericExtern:             {ID: UnresolvedGenericExtern, Name: "ffi.unresolved-generic-extern", Family: "ffi", DefaultSeverity: SeverityError, Mandatory: true},
	UnknownAttribute:                    {ID: UnknownAttribute, Name: "attribute.unknown", Family: "attribute", DefaultSeverity: SeverityError, Mandatory: true},
	ImmutableRequiresInitializer:        {ID: ImmutableRequiresInitializer, Name: "variables.immutable-requires-initializer", Family: "variables", DefaultSeverity: SeverityError, Mandatory: true},
	UnattachedAttribute:                 {ID: UnattachedAttribute, Name: "attribute.unattached", Family: "attribute", DefaultSeverity: SeverityError, Mandatory: true},
	ForbiddenTrySuccessHandler:          {ID: ForbiddenTrySuccessHandler, Name: "error-handling.forbidden-try-success-handler", Family: "error-handling", DefaultSeverity: SeverityError, Mandatory: true},
	StorageSiteContract:                 {ID: StorageSiteContract, Name: "types.storage-site-contract", Family: "types", DefaultSeverity: SeverityError, Mandatory: true},
	UnresolvedImport:                    {ID: UnresolvedImport, Name: "modules.unresolved-import", Family: "modules", DefaultSeverity: SeverityError, Mandatory: true},
	UnionPayloadMoveStorage:             {ID: UnionPayloadMoveStorage, Name: "ownership.union-payload-move-storage", Family: "ownership", DefaultSeverity: SeverityError, Mandatory: true},
	DuplicateContractMembershipValue:    {ID: DuplicateContractMembershipValue, Name: "types.duplicate-in-contract-value", Family: "types", DefaultSeverity: SeverityError, Mandatory: true},
	EmptyContractMembership:             {ID: EmptyContractMembership, Name: "types.empty-in-contract", Family: "types", DefaultSeverity: SeverityError, Mandatory: true},
	RecursiveStructLayout:               {ID: RecursiveStructLayout, Name: "struct.recursive-by-value-layout", Family: "struct", DefaultSeverity: SeverityError, Mandatory: true},
	SwitchPatternBinding:                {ID: SwitchPatternBinding, Name: "switch.pattern-binding", Family: "flow-control", DefaultSeverity: SeverityError, Mandatory: true},
	LocalShadowsDeclaration:             {ID: LocalShadowsDeclaration, Name: "names.local-shadows-declaration", Family: "names", DefaultSeverity: SeverityError, Mandatory: true},
	RegexContractUnavailable:            {ID: RegexContractUnavailable, Name: "types.regex-contract-unavailable", Family: "types", DefaultSeverity: SeverityError, Mandatory: true},
	DefaultViolatesContract:             {ID: DefaultViolatesContract, Name: "types.default-violates-contract", Family: "types", DefaultSeverity: SeverityError, Mandatory: true},
	DefaultNotRepresentable:             {ID: DefaultNotRepresentable, Name: "types.default-not-representable", Family: "types", DefaultSeverity: SeverityError, Mandatory: true},
	AmbiguousImplicitDefault:            {ID: AmbiguousImplicitDefault, Name: "types.ambiguous-implicit-default", Family: "types", DefaultSeverity: SeverityError, Mandatory: true},
	TypeNoDefaultValue:                  {ID: TypeNoDefaultValue, Name: "types.no-default-value", Family: "types", DefaultSeverity: SeverityError, Mandatory: true},
	InvalidDefaultedField:               {ID: InvalidDefaultedField, Name: "struct.invalid-defaulted-field", Family: "struct", DefaultSeverity: SeverityError, Mandatory: true},
	InapplicableContract:                {ID: InapplicableContract, Name: "types.inapplicable-contract", Family: "types", DefaultSeverity: SeverityError, Mandatory: true},
	UnsatisfiableContractSet:            {ID: UnsatisfiableContractSet, Name: "types.unsatisfiable-contract-set", Family: "types", DefaultSeverity: SeverityError, Mandatory: true},
	InvalidContractArgument:             {ID: InvalidContractArgument, Name: "types.invalid-contract-argument", Family: "types", DefaultSeverity: SeverityError, Mandatory: true},
	IncompatibleMembershipValue:         {ID: IncompatibleMembershipValue, Name: "types.incompatible-in-contract-value", Family: "types", DefaultSeverity: SeverityError, Mandatory: true},
	ValueViolatesContract:               {ID: ValueViolatesContract, Name: "types.value-violates-contract", Family: "types", DefaultSeverity: SeverityError, Mandatory: true},
	ConstrainedAssignmentRequiresTry:    {ID: ConstrainedAssignmentRequiresTry, Name: "types.constrained-assignment-requires-try", Family: "types", DefaultSeverity: SeverityError, Mandatory: true},
	AuthoritativeMemberReplacement:      {ID: AuthoritativeMemberReplacement, Name: "members.authoritative-compiler-property", Family: "members", DefaultSeverity: SeverityError, Mandatory: true},
	RecursiveUnionLayout:                {ID: RecursiveUnionLayout, Name: "union.recursive-by-value-layout", Family: "union", DefaultSeverity: SeverityError, Mandatory: true},
	ArenaUnsizedAllocationType:          {ID: ArenaUnsizedAllocationType, Name: "arena.unsized-allocation-type", Family: "arena", DefaultSeverity: SeverityError, Mandatory: true},
	ArenaAllocationMissingDefault:       {ID: ArenaAllocationMissingDefault, Name: "arena.allocation-missing-default", Family: "arena", DefaultSeverity: SeverityError, Mandatory: true},
	ArenaNonTrivialDestructionType:      {ID: ArenaNonTrivialDestructionType, Name: "arena.non-trivially-destructible-allocation", Family: "arena", DefaultSeverity: SeverityError, Mandatory: true},
	StructuralMutationDuringIteration:   {ID: StructuralMutationDuringIteration, Name: "for.structural-mutation-during-iteration", Family: "flow-control", DefaultSeverity: SeverityError, Mandatory: true},
	ForeignUnknownCFundamentalType:      {ID: ForeignUnknownCFundamentalType, Name: "ffi.unknown-c-fundamental-type", Family: "ffi", DefaultSeverity: SeverityError, Mandatory: true},
	ForeignCABIModelUnavailable:         {ID: ForeignCABIModelUnavailable, Name: "ffi.c-abi-model-unavailable", Family: "ffi", DefaultSeverity: SeverityError, Mandatory: true},
	ForeignUnresolvedCBindingType:       {ID: ForeignUnresolvedCBindingType, Name: "ffi.unresolved-c-binding-type", Family: "ffi", DefaultSeverity: SeverityError, Mandatory: true},
	IllegalForeignType:                  {ID: IllegalForeignType, Name: "ffi.illegal-foreign-type", Family: "ffi", DefaultSeverity: SeverityError, Mandatory: true},
	NullOutsideUnsafe:                   {ID: NullOutsideUnsafe, Name: "ffi.null-outside-unsafe", Family: "ffi", DefaultSeverity: SeverityError, Mandatory: true},
	NullWithoutRawPointerContext:        {ID: NullWithoutRawPointerContext, Name: "ffi.null-without-raw-pointer-context", Family: "ffi", DefaultSeverity: SeverityError, Mandatory: true},
	NullEquality:                        {ID: NullEquality, Name: "ffi.null-equality", Family: "ffi", DefaultSeverity: SeverityError, Mandatory: true},
	NullTestRequiresRawPointer:          {ID: NullTestRequiresRawPointer, Name: "ffi.null-test-requires-raw-pointer", Family: "ffi", DefaultSeverity: SeverityError, Mandatory: true},
	InvalidCustomFreeDeclaration:        {ID: InvalidCustomFreeDeclaration, Name: "destruction.invalid-custom-free", Family: "destruction", DefaultSeverity: SeverityError, Mandatory: true},
	CustomFreePartialMove:               {ID: CustomFreePartialMove, Name: "ownership.custom-free-partial-move", Family: "ownership", DefaultSeverity: SeverityError, Mandatory: true},
	DeferInsideFree:                     {ID: DeferInsideFree, Name: "destruction.defer-inside-free", Family: "destruction", DefaultSeverity: SeverityError, Mandatory: true},
	FreeConsumesSelf:                    {ID: FreeConsumesSelf, Name: "destruction.free-consumes-self", Family: "destruction", DefaultSeverity: SeverityError, Mandatory: true},
	UseAfterMove:                        {ID: UseAfterMove, Name: "ownership.use-after-move", Family: "ownership", DefaultSeverity: SeverityError, Mandatory: true},
	ConditionallyUnavailableUse:         {ID: ConditionallyUnavailableUse, Name: "ownership.conditionally-unavailable-use", Family: "ownership", DefaultSeverity: SeverityError, Mandatory: true},
	PartiallyUnavailableUse:             {ID: PartiallyUnavailableUse, Name: "ownership.partially-unavailable-use", Family: "ownership", DefaultSeverity: SeverityError, Mandatory: true},
	HeterogeneousTryErrorBinding:        {ID: HeterogeneousTryErrorBinding, Name: "errors.heterogeneous-try-error-binding", Family: "errors", DefaultSeverity: SeverityError, Mandatory: true},
	NoPanicViolation:                    {ID: NoPanicViolation, Name: "panic.no-panic-violation", Family: "panic", DefaultSeverity: SeverityError, Mandatory: true},
	TryPropagationIncompatible:          {ID: TryPropagationIncompatible, Name: "errors.try-propagation-incompatible", Family: "errors", DefaultSeverity: SeverityError, Mandatory: true},
	TryResidualUnpropagatable:           {ID: TryResidualUnpropagatable, Name: "errors.try-residual-unpropagatable", Family: "errors", DefaultSeverity: SeverityError, Mandatory: true},
	InvalidTryHandlerPattern:            {ID: InvalidTryHandlerPattern, Name: "errors.invalid-try-handler-pattern", Family: "errors", DefaultSeverity: SeverityError, Mandatory: true},
	TryAssignmentWithoutFallibleTarget:  {ID: TryAssignmentWithoutFallibleTarget, Name: "errors.try-assignment-without-fallible-target", Family: "errors", DefaultSeverity: SeverityError, Mandatory: true},
	UseAfterDiscard:                     {ID: UseAfterDiscard, Name: "ownership.use-after-discard", Family: "ownership", DefaultSeverity: SeverityError, Mandatory: true},
	UnreachableStatement:                {ID: UnreachableStatement, Name: "control-flow.unreachable-statement", Family: "control-flow", DefaultSeverity: SeverityError, Mandatory: true},
	ReservedDeclarationName:             {ID: ReservedDeclarationName, Name: "names.reserved-declaration-name", Family: "names", DefaultSeverity: SeverityError, Mandatory: true},
	LexerUnknownEscape:                  {ID: LexerUnknownEscape, Name: "lexer.unknown-escape", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true},
	LexerMalformedEscape:                {ID: LexerMalformedEscape, Name: "lexer.malformed-escape", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true},
	LexerInvalidUnicodeEscape:           {ID: LexerInvalidUnicodeEscape, Name: "lexer.invalid-unicode-escape", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true},
	LexerCharacterLiteralLength:         {ID: LexerCharacterLiteralLength, Name: "lexer.character-literal-length", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true},
	LexerMalformedBaseLiteral:           {ID: LexerMalformedBaseLiteral, Name: "lexer.malformed-base-literal", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true},
	LexerInvalidBaseDigit:               {ID: LexerInvalidBaseDigit, Name: "lexer.invalid-base-digit", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true},
	LexerInvalidDigitSeparator:          {ID: LexerInvalidDigitSeparator, Name: "lexer.invalid-digit-separator", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true},
	LexerInvalidNumericSuffix:           {ID: LexerInvalidNumericSuffix, Name: "lexer.invalid-numeric-suffix", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true},
	LexerMissingExponentDigits:          {ID: LexerMissingExponentDigits, Name: "lexer.missing-exponent-digits", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true},
	LexerUnterminatedBlockComment:       {ID: LexerUnterminatedBlockComment, Name: "lexer.unterminated-block-comment", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true},
	LexerUnterminatedOrdinaryString:     {ID: LexerUnterminatedOrdinaryString, Name: "lexer.unterminated-ordinary-string", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true},
	LexerUnterminatedRawString:          {ID: LexerUnterminatedRawString, Name: "lexer.unterminated-raw-string", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true},
	LexerUnterminatedCharacterLiteral:   {ID: LexerUnterminatedCharacterLiteral, Name: "lexer.unterminated-character-literal", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true},
	LexerUnterminatedInterpolatedString: {ID: LexerUnterminatedInterpolatedString, Name: "lexer.unterminated-interpolated-string", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true},
	LexerInvalidSourceCharacter:         {ID: LexerInvalidSourceCharacter, Name: "lexer.invalid-source-character", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true},
	LexerNonNFCIdentifier: {
		ID: LexerNonNFCIdentifier, Name: "lexer.non-nfc-identifier", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true,
	},
	LexerIdentifierCharacter: {
		ID: LexerIdentifierCharacter, Name: "lexer.invalid-identifier-character", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true,
	},
	LexerInvalidUTF8: {
		ID: LexerInvalidUTF8, Name: "lexer.invalid-utf8", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true,
	},
	LexerUnexpectedByteOrderMark: {
		ID: LexerUnexpectedByteOrderMark, Name: "lexer.unexpected-byte-order-mark", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true,
	},
	LexerUnsupportedWhitespace: {
		ID: LexerUnsupportedWhitespace, Name: "lexer.unsupported-unicode-whitespace", Family: "lexer", DefaultSeverity: SeverityError, Mandatory: true,
	},
	ParserSyntaxError: {
		ID:              ParserSyntaxError,
		Name:            "parser.syntax-error",
		Family:          "parser",
		DefaultSeverity: SeverityError,
		Mandatory:       true,
	},
	ParserMissingToken:          parserDefinition(ParserMissingToken, "parser.missing-token"),
	ParserUnexpectedToken:       parserDefinition(ParserUnexpectedToken, "parser.unexpected-token"),
	ParserUnterminatedDelimiter: parserDefinition(ParserUnterminatedDelimiter, "parser.unterminated-delimiter"),
	ParserInvalidDeclaration:    parserDefinition(ParserInvalidDeclaration, "parser.invalid-declaration"),
	ParserInvalidStatement:      parserDefinition(ParserInvalidStatement, "parser.invalid-statement"),
	ParserInvalidExpression:     parserDefinition(ParserInvalidExpression, "parser.invalid-expression"),
	ParserInvalidTypeReference:  parserDefinition(ParserInvalidTypeReference, "parser.invalid-type-reference"),
	ParserInvalidPattern:        parserDefinition(ParserInvalidPattern, "parser.invalid-pattern"),
	ParserMissingSeparator:      parserDefinition(ParserMissingSeparator, "parser.missing-separator"),
	ParserMisplacedKeyword:      parserDefinition(ParserMisplacedKeyword, "parser.misplaced-keyword"),
	ParserReservedSyntax:        parserDefinition(ParserReservedSyntax, "parser.reserved-syntax"),
	ParserInvalidAssignmentExpr: parserDefinition(
		ParserInvalidAssignmentExpr,
		"parser.invalid-assignment-expression",
	),
	ParserChainedComparison:             parserDefinition(ParserChainedComparison, "parser.chained-comparison"),
	ParserRecoveryLimit:                 parserDefinition(ParserRecoveryLimit, "parser.recovery-limit"),
	ParserUnexpectedEndOfFile:           parserDefinition(ParserUnexpectedEndOfFile, "parser.unexpected-end-of-file"),
	ParserInvalidBlockMember:            parserDefinition(ParserInvalidBlockMember, "parser.invalid-block-member"),
	ParserUnimplementedFunction:         parserDefinition(ParserUnimplementedFunction, "parser.unimplemented-function"),
	ParserInvalidTestDeclaration:        parserDefinition(ParserInvalidTestDeclaration, "parser.invalid-test-declaration"),
	ParserMalformedForeignQualification: parserDefinition(ParserMalformedForeignQualification, "parser.malformed-foreign-qualification"),
	ParserUnsupportedForeignForm:        parserDefinition(ParserUnsupportedForeignForm, "parser.unsupported-foreign-form"),
	ParserLegacyAssignedType:            parserDefinition(ParserLegacyAssignedType, "parser.legacy-assigned-type"),
	ParserMultipleUnderlyingTypes:       parserDefinition(ParserMultipleUnderlyingTypes, "parser.multiple-underlying-types"),
	ParserLegacyInlineContract:          parserDefinition(ParserLegacyInlineContract, "parser.legacy-inline-contract"),
	ParserPrefixSequenceType:            parserDefinition(ParserPrefixSequenceType, "parser.prefix-sequence-type"),
	ParserFutureStructDeclaration:       parserDefinition(ParserFutureStructDeclaration, "parser.future-struct-declaration"),
	ParserLegacyEnumColonInitializer:    parserDefinition(ParserLegacyEnumColonInitializer, "parser.legacy-enum-colon-initializer"),
	ParserMultipleTypeDeclarationNames:  parserDefinition(ParserMultipleTypeDeclarationNames, "parser.multiple-type-declaration-names"),
	ParserCompatibilitySyntax: {
		ID:              ParserCompatibilitySyntax,
		Name:            "parser.compatibility-syntax",
		Family:          "parser",
		DefaultSeverity: SeverityWarning,
		Mandatory:       false,
	},
	MissingModuleDeclaration: {
		ID:              MissingModuleDeclaration,
		Name:            "modules.missing-module-declaration",
		Family:          "modules",
		DefaultSeverity: SeverityError,
		Mandatory:       true,
	},
	DuplicateModuleDeclaration: {
		ID:              DuplicateModuleDeclaration,
		Name:            "modules.duplicate-module-declaration",
		Family:          "modules",
		DefaultSeverity: SeverityError,
		Mandatory:       true,
	},
	ModuleDeclarationConflict: {
		ID:              ModuleDeclarationConflict,
		Name:            "names.module-declaration-conflict",
		Family:          "names",
		DefaultSeverity: SeverityError,
		Mandatory:       true,
	},
	DuplicateLocalVariable: {
		ID:              DuplicateLocalVariable,
		Name:            "names.duplicate-local-variable",
		Family:          "names",
		DefaultSeverity: SeverityError,
		Mandatory:       true,
	},
	UnhandledMustUseResult: {
		ID:              UnhandledMustUseResult,
		Name:            "ownership.unhandled-must-use-result",
		Family:          "ownership",
		DefaultSeverity: SeverityError,
		Mandatory:       true,
	},
	NonDiscardableValue: {
		ID:              NonDiscardableValue,
		Name:            "ownership.non-discardable-value",
		Family:          "ownership",
		DefaultSeverity: SeverityError,
		Mandatory:       true,
	},
	ImplicitMoveDisallowed: {
		ID:              ImplicitMoveDisallowed,
		Name:            "ownership.implicit-move-disallowed",
		Family:          "ownership",
		DefaultSeverity: SeverityError,
		Mandatory:       true,
	},
	InvalidExplicitDefault: {
		ID: InvalidExplicitDefault, Name: "types.invalid-explicit-default", Family: "types", DefaultSeverity: SeverityError, Mandatory: true,
	},
	NoDefaultValue: {
		ID: NoDefaultValue, Name: "variables.nondefaultable-requires-initializer", Family: "variables", DefaultSeverity: SeverityError, Mandatory: true,
	},
	MissingNonDefaultableField: {
		ID: MissingNonDefaultableField, Name: "struct.missing-nondefaultable-field", Family: "struct", DefaultSeverity: SeverityError, Mandatory: true,
	},
	InvalidMembershipValue: {
		ID: InvalidMembershipValue, Name: "types.in-list-value-violates-contract", Family: "types", DefaultSeverity: SeverityError, Mandatory: true,
	},
	InterfaceInheritanceCycle: {
		ID: InterfaceInheritanceCycle, Name: "interfaces.inheritance-cycle", Family: "interfaces", DefaultSeverity: SeverityError, Mandatory: true,
	},
	IncompatibleUnitConversion: {
		ID: IncompatibleUnitConversion, Name: "units.incompatible-conversion-dimensions", Family: "units", DefaultSeverity: SeverityError, Mandatory: true,
	},
	IncompleteEnumSwitch: {
		ID: IncompleteEnumSwitch, Name: "switch.incomplete-enum-coverage", Family: "flow-control", DefaultSeverity: SeverityWarning, Mandatory: false,
	},
	DuplicateSwitchCase: {
		ID: DuplicateSwitchCase, Name: "switch.duplicate-constant-case", Family: "flow-control", DefaultSeverity: SeverityError, Mandatory: true,
	},
	OperatorNonOrderable: {
		ID: OperatorNonOrderable, Name: "operator.non-orderable-operands", Family: "operators", DefaultSeverity: SeverityError, Mandatory: true,
	},
	OperatorInvalidShiftCount: {
		ID: OperatorInvalidShiftCount, Name: "operator.invalid-shift-count", Family: "operators", DefaultSeverity: SeverityError, Mandatory: true,
	},
	OperatorShiftOverflow: {
		ID: OperatorShiftOverflow, Name: "operator.signed-left-shift-overflow", Family: "operators", DefaultSeverity: SeverityError, Mandatory: true,
	},
	OperatorNonComparable: {
		ID: OperatorNonComparable, Name: "operator.non-comparable-operands", Family: "operators", DefaultSeverity: SeverityError, Mandatory: true,
	},
	OperatorStringRuntimeConcat: {
		ID: OperatorStringRuntimeConcat, Name: "operator.string-runtime-concat", Family: "operators", DefaultSeverity: SeverityError, Mandatory: true, Retired: true,
	},
	OperatorInvalidMembership: {
		ID: OperatorInvalidMembership, Name: "operator.invalid-membership", Family: "operators", DefaultSeverity: SeverityError, Mandatory: true,
	},
	OperatorInvalidConcatOperand: {
		ID: OperatorInvalidConcatOperand, Name: "operator.invalid-concat-operand", Family: "operators", DefaultSeverity: SeverityError, Mandatory: true,
	},
	OperatorInvalidInterpolationValue: {
		ID: OperatorInvalidInterpolationValue, Name: "operator.invalid-interpolation-value", Family: "operators", DefaultSeverity: SeverityError, Mandatory: true,
	},
	OperatorIntegerOverflow: {
		ID: OperatorIntegerOverflow, Name: "operator.constant-integer-overflow", Family: "operators", DefaultSeverity: SeverityWarning, Mandatory: false,
	},
	OperatorDivisionByZero: {
		ID: OperatorDivisionByZero, Name: "operator.constant-division-by-zero", Family: "operators", DefaultSeverity: SeverityError, Mandatory: true,
	},
	OperatorRemainderByZero: {
		ID: OperatorRemainderByZero, Name: "operator.constant-remainder-by-zero", Family: "operators", DefaultSeverity: SeverityError, Mandatory: true,
	},
	TestDeclarationOutsideTestFile: {
		ID: TestDeclarationOutsideTestFile, Name: "testing.declaration-outside-test-file", Family: "testing", DefaultSeverity: SeverityError, Mandatory: true,
	},
	EmptyTestName: {
		ID: EmptyTestName, Name: "testing.empty-test-name", Family: "testing", DefaultSeverity: SeverityError, Mandatory: true,
	},
	DuplicateTestIdentity: {
		ID: DuplicateTestIdentity, Name: "testing.duplicate-test-identity", Family: "testing", DefaultSeverity: SeverityError, Mandatory: true,
	},
	TestReturnValue: {
		ID: TestReturnValue, Name: "testing.return-value", Family: "testing", DefaultSeverity: SeverityError, Mandatory: true,
	},
	TestingOutsideTestContext: {
		ID: TestingOutsideTestContext, Name: "testing.outside-test-context", Family: "testing", DefaultSeverity: SeverityError, Mandatory: true,
	},
	InvalidTestingExpectArguments: {
		ID: InvalidTestingExpectArguments, Name: "testing.invalid-expect-arguments", Family: "testing", DefaultSeverity: SeverityError, Mandatory: true,
	},
	InvalidTestingRequireArguments: {
		ID: InvalidTestingRequireArguments, Name: "testing.invalid-require-arguments", Family: "testing", DefaultSeverity: SeverityError, Mandatory: true,
	},
	InvalidTestingLogArguments: {
		ID: InvalidTestingLogArguments, Name: "testing.invalid-log-arguments", Family: "testing", DefaultSeverity: SeverityError, Mandatory: true,
	},
	InvalidTestingExpectEqualArguments: {
		ID: InvalidTestingExpectEqualArguments, Name: "testing.invalid-expect-equal-arguments", Family: "testing", DefaultSeverity: SeverityError, Mandatory: true,
	},
	InvalidTestingRequireEqualArguments: {
		ID: InvalidTestingRequireEqualArguments, Name: "testing.invalid-require-equal-arguments", Family: "testing", DefaultSeverity: SeverityError, Mandatory: true,
	},
	InvalidTestingTerminationArguments: {
		ID: InvalidTestingTerminationArguments, Name: "testing.invalid-termination-arguments", Family: "testing", DefaultSeverity: SeverityError, Mandatory: true,
	},
	UnreachableTryHandler: {
		ID: UnreachableTryHandler, Name: "error-handling.unreachable-try-handler", Family: "error-handling", DefaultSeverity: SeverityError, Mandatory: true,
	},
	RedundantAssociatedStatic: {
		ID:              RedundantAssociatedStatic,
		Name:            "associated-values.redundant-static",
		Family:          "declarations",
		DefaultSeverity: SeverityInformation,
		Mandatory:       false,
		Retired:         true,
	},
	LargeValueParameter: {
		ID:              LargeValueParameter,
		Name:            "performance.large-value-parameter",
		Family:          "performance",
		DefaultSeverity: SeverityInformation,
		Mandatory:       false,
	},
}

func parserDefinition(id string, name string) Definition {
	return Definition{ID: id, Name: name, Family: "parser", DefaultSeverity: SeverityError, Mandatory: true}
}

func Lookup(id string) (Definition, bool) {
	definition, ok := registry[id]
	return definition, ok
}

func DefaultSeverity(id string) Severity {
	if definition, ok := Lookup(id); ok {
		return definition.DefaultSeverity
	}
	return ""
}

func All() []Definition {
	definitions := make([]Definition, 0, len(registry))
	for _, definition := range registry {
		definitions = append(definitions, definition)
	}
	sort.Slice(definitions, func(i int, j int) bool {
		return definitions[i].ID < definitions[j].ID
	})
	return definitions
}

func Validate() error {
	names := map[string]string{}
	for id, definition := range registry {
		if definition.ID != id {
			return fmt.Errorf("diagnostic registry key %s does not match definition ID %s", id, definition.ID)
		}
		if definition.Name == "" {
			return fmt.Errorf("diagnostic %s is missing a symbolic name", id)
		}
		if previousID, exists := names[definition.Name]; exists {
			return fmt.Errorf("diagnostic name %s is used by both %s and %s", definition.Name, previousID, id)
		}
		names[definition.Name] = id
		switch definition.DefaultSeverity {
		case SeverityError, SeverityWarning, SeverityInformation:
		default:
			return fmt.Errorf("diagnostic %s has invalid severity %q", id, definition.DefaultSeverity)
		}
	}
	return nil
}
