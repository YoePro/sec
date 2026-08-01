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
}

const (
	ParserSyntaxError          = "P2001"
	MissingModuleDeclaration   = "S1001"
	DuplicateModuleDeclaration = "S1002"
	ModuleDeclarationConflict  = "S1003"
	DuplicateLocalVariable     = "S1004"
	UnhandledMustUseResult     = "S1005"
	NonDiscardableValue        = "S1006"
	ImplicitMoveDisallowed     = "S1007"
	InvalidExplicitDefault     = "S1008"
	NoDefaultValue             = "S1009"
	MissingNonDefaultableField = "S1010"
	LargeValueParameter        = "A2001"
)

var registry = map[string]Definition{
	ParserSyntaxError: {
		ID:              ParserSyntaxError,
		Name:            "parser.syntax-error",
		Family:          "parser",
		DefaultSeverity: SeverityError,
		Mandatory:       true,
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
	LargeValueParameter: {
		ID:              LargeValueParameter,
		Name:            "performance.large-value-parameter",
		Family:          "performance",
		DefaultSeverity: SeverityInformation,
		Mandatory:       false,
	},
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
