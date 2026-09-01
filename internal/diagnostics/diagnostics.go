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
	LexerInvalidUTF8             = "L1001"
	LexerUnexpectedByteOrderMark = "L1002"
	LexerUnsupportedWhitespace   = "L1003"
	ParserSyntaxError            = "P2001"
	ParserMissingToken           = "P2002"
	ParserUnexpectedToken        = "P2003"
	ParserUnterminatedDelimiter  = "P2004"
	ParserInvalidDeclaration     = "P2005"
	ParserInvalidStatement       = "P2006"
	ParserInvalidExpression      = "P2007"
	ParserInvalidTypeReference   = "P2008"
	ParserInvalidPattern         = "P2009"
	ParserMissingSeparator       = "P2010"
	ParserMisplacedKeyword       = "P2011"
	ParserReservedSyntax         = "P2012"
	ParserInvalidAssignmentExpr  = "P2013"
	ParserChainedComparison      = "P2014"
	ParserRecoveryLimit          = "P2015"
	ParserUnexpectedEndOfFile    = "P2016"
	ParserInvalidBlockMember     = "P2017"
	ParserCompatibilitySyntax    = "P2018"
	MissingModuleDeclaration     = "S1001"
	DuplicateModuleDeclaration   = "S1002"
	ModuleDeclarationConflict    = "S1003"
	DuplicateLocalVariable       = "S1004"
	UnhandledMustUseResult       = "S1005"
	NonDiscardableValue          = "S1006"
	ImplicitMoveDisallowed       = "S1007"
	InvalidExplicitDefault       = "S1008"
	NoDefaultValue               = "S1009"
	MissingNonDefaultableField   = "S1010"
	InvalidMembershipValue       = "S1011"
	InterfaceInheritanceCycle    = "S1012"
	IncompatibleUnitConversion   = "S1013"
	IncompleteEnumSwitch         = "S1014"
	DuplicateSwitchCase          = "S1015"
	OperatorNonOrderable         = "S1016"
	OperatorInvalidShiftCount    = "S1017"
	OperatorShiftOverflow        = "S1018"
	OperatorNonComparable        = "S1019"
	OperatorStringRuntimeConcat  = "S1020"
	OperatorInvalidMembership    = "S1021"
	OperatorInvalidConcatOperand = "S1022"
	OperatorIntegerOverflow      = "S1023"
	OperatorDivisionByZero       = "S1024"
	OperatorRemainderByZero      = "S1025"
	RedundantAssociatedStatic    = "S1026"
	LargeValueParameter          = "A2001"
)

var registry = map[string]Definition{
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
	ParserChainedComparison:   parserDefinition(ParserChainedComparison, "parser.chained-comparison"),
	ParserRecoveryLimit:       parserDefinition(ParserRecoveryLimit, "parser.recovery-limit"),
	ParserUnexpectedEndOfFile: parserDefinition(ParserUnexpectedEndOfFile, "parser.unexpected-end-of-file"),
	ParserInvalidBlockMember:  parserDefinition(ParserInvalidBlockMember, "parser.invalid-block-member"),
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
	OperatorIntegerOverflow: {
		ID: OperatorIntegerOverflow, Name: "operator.constant-integer-overflow", Family: "operators", DefaultSeverity: SeverityError, Mandatory: true,
	},
	OperatorDivisionByZero: {
		ID: OperatorDivisionByZero, Name: "operator.constant-division-by-zero", Family: "operators", DefaultSeverity: SeverityError, Mandatory: true,
	},
	OperatorRemainderByZero: {
		ID: OperatorRemainderByZero, Name: "operator.constant-remainder-by-zero", Family: "operators", DefaultSeverity: SeverityError, Mandatory: true,
	},
	RedundantAssociatedStatic: {
		ID:              RedundantAssociatedStatic,
		Name:            "associated-values.redundant-static",
		Family:          "declarations",
		DefaultSeverity: SeverityInformation,
		Mandatory:       false,
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
