package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/hardware"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/modules"
	"strings"
)

// HardwareModuleContext is supplied by the selected module resolver. Files are
// source locators only; map values, not file paths or short module names, supply
// identity. Imports maps a source file and actual import path to its selected
// canonical candidates. Zero/multiple candidates fail closed on use.
// Rules: MD-111 §§2,8(3),12; rules/projects/modules.md §2.4.
type HardwareModuleContext struct {
	ModuleNames        map[modules.ModuleIdentity]modules.ModuleName
	Files              map[string]modules.ModuleIdentity
	Imports            map[string]map[string][]modules.ModuleIdentity
	EstablishedRegions []hardware.Region
}

// SetHardwareModuleContext snapshots module and target/Device provenance before
// declaration collection. It does not select source files (MD-111 §§4(2),12).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func (a *Analyzer) SetHardwareModuleContext(context HardwareModuleContext) {
	clone := HardwareModuleContext{ModuleNames: map[modules.ModuleIdentity]modules.ModuleName{}, Files: map[string]modules.ModuleIdentity{}, Imports: map[string]map[string][]modules.ModuleIdentity{}}
	for id, name := range context.ModuleNames {
		clone.ModuleNames[id] = name
	}
	for file, id := range context.Files {
		clone.Files[file] = id
	}
	for file, imports := range context.Imports {
		clone.Imports[file] = map[string][]modules.ModuleIdentity{}
		for spelling, ids := range imports {
			clone.Imports[file][spelling] = append([]modules.ModuleIdentity(nil), ids...)
		}
	}
	clone.EstablishedRegions = (hardware.Facts{Regions: context.EstablishedRegions}).Clone().Regions
	a.hardwareContext = clone
	a.hardwareFacts = hardware.Facts{}
	a.hardwareFactsValid = false
}

// HardwareFacts returns an isolated, validated declaration snapshot. False means
// no successful hardware pass, never permission to fall back to guessed facts.
// Rules: MD-111 §§2,4(3),6,12; incremental_compilation.md §15.
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func (a *Analyzer) HardwareFacts() (hardware.Facts, bool) {
	return a.hardwareFacts.Clone(), a.hardwareFactsValid
}

// ResolveHardwareDeclarations is the declaration-only stage available before
// ordinary body analysis. It consumes actual imports and resolver-owned module
// identities and shares its schema/normalizer with later project consumers.
// It neither performs general CTE nor issues hardware operations.
// Rules: MD-111 §§2–8,12(1)3–4,13(2).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func (a *Analyzer) ResolveHardwareDeclarations(program *ast.Program) (hardware.Facts, []Error) {
	if program == nil {
		return hardware.Facts{}, nil
	}
	hasHardware := false
	for _, stmt := range program.Statements {
		if _, ok := stmt.(*ast.HardwareDeclaration); ok {
			hasHardware = true
			break
		}
	}
	if !hasHardware {
		return hardware.Facts{UintBits: a.scalarPlanWidthBits, Domains: append([]layout.AddressDomain(nil), a.addressDomains...)}, nil
	}
	issues := []hardware.Issue{}
	type importBinding struct {
		module      modules.ModuleIdentity
		file, alias string
		token       lexer.Token
	}
	bindings := []importBinding{}
	imports := map[string]map[string][]modules.ModuleIdentity{}
	names := map[modules.ModuleIdentity]modules.ModuleName{}
	for id, name := range a.hardwareContext.ModuleNames {
		names[id] = name
	}
	for _, stmt := range program.Statements {
		if m, ok := stmt.(*ast.ModuleStatement); ok && m != nil {
			id := a.hardwareContext.Files[m.Token.File]
			if previous, exists := names[id]; exists && previous != modules.ModuleName(m.Path) {
				issues = append(issues, hardware.Issue{Code: "context", Origin: hardware.Origin{Token: m.NameToken}, Message: "conflicting source ModuleName for one canonical hardware module", Help: "Use the same module declaration and resolver-owned module identity for all files of a selected module."})
			} else {
				names[id] = modules.ModuleName(m.Path)
			}
		}
	}
	ordinary := map[modules.ModuleIdentity]map[string]lexer.Token{}
	for _, stmt := range program.Statements {
		token := statementToken(stmt)
		module := a.hardwareContext.Files[token.File]
		if imp, ok := stmt.(*ast.ImportStatement); ok && imp != nil {
			if imports[token.File] == nil {
				imports[token.File] = map[string][]modules.ModuleIdentity{}
			}
			alias := imp.Alias
			if alias == "" {
				candidates := a.hardwareContext.Imports[token.File][imp.Path]
				if len(candidates) == 1 {
					alias = string(names[candidates[0]])
				}
			}
			imports[token.File][alias] = append(imports[token.File][alias], a.hardwareContext.Imports[token.File][imp.Path]...)
			at := imp.AliasToken
			if at.Line == 0 {
				at = imp.PathToken
			}
			bindings = append(bindings, importBinding{module: module, file: token.File, alias: alias, token: at})
		}
		for _, decl := range moduleDeclarationsFromStatement(stmt) {
			if decl.Kind == moduleDeclarationUnit {
				continue
			} // separate unit namespace
			if ordinary[module] == nil {
				ordinary[module] = map[string]lexer.Token{}
			}
			ordinary[module][decl.Name] = decl.Token
		}
	}
	declarations := []hardware.Declaration{}
	for _, stmt := range program.Statements {
		decl, ok := stmt.(*ast.HardwareDeclaration)
		if !ok || decl == nil {
			continue
		}
		if decl.Invalid || decl.Name == nil {
			issues = append(issues, hardware.Issue{Code: "member", Origin: hardware.Origin{Token: decl.Token}, Message: "malformed hardware declaration has no valid metadata facts", Help: "Correct the declaration syntax before resolving hardware facts."})
			continue
		}
		module := a.hardwareContext.Files[decl.Token.File]
		d := hardware.Declaration{Identity: hardware.Identity{Module: module, Category: hardware.Category(decl.Category), Name: decl.Name.Value}, Origin: hardware.Origin{Token: decl.Name.Token}, Scope: hardware.Scope{Module: module, File: decl.Token.File, Imports: imports[decl.Token.File]}}
		if prev, conflict := ordinary[module][decl.Name.Value]; conflict {
			issues = append(issues, hardware.Issue{Code: "member", Origin: d.Origin, Related: hardware.Origin{Token: prev}, Message: "hardware declaration conflicts with ordinary declaration " + decl.Name.Value, Help: "Use a distinct declaration name in this canonical module."})
		}
		for _, member := range decl.Members {
			if member == nil || member.Name == nil {
				continue
			}
			d.Properties = append(d.Properties, hardware.Property{Name: member.Name.Value, Origin: hardware.Origin{Token: member.Name.Token}, Value: hardwareMetadataValue(member.Value)})
		}
		declarations = append(declarations, d)
	}
	// Hardware names join the ordinary module namespace; actual file-local
	// import aliases cannot hide a visible local hardware declaration (§8(3)).
	for _, binding := range bindings {
		for _, decl := range declarations {
			if binding.module == decl.Identity.Module && binding.alias == decl.Identity.Name && (!strings.HasPrefix(decl.Identity.Name, "__") || binding.file == decl.Scope.File) {
				issues = append(issues, hardware.Issue{Code: "symbol", Origin: hardware.Origin{Token: binding.token}, Related: decl.Origin, Message: "import qualifier conflicts with visible hardware declaration " + binding.alias, Help: "Use a distinct explicit import alias in this source file."})
			}
		}
	}
	facts, normalized := hardware.Normalize(hardware.TargetFacts{UintBits: a.scalarPlanWidthBits, Domains: a.addressDomains, EstablishedRegions: a.hardwareContext.EstablishedRegions}, declarations)
	issues = append(issues, normalized...)
	errors := make([]Error, 0, len(issues))
	for _, issue := range issues {
		errors = append(errors, hardwareDiagnostic(issue))
		if !issue.Warning {
			facts = hardware.Facts{}
		}
	}
	return facts, errors
}

// hardwareMetadataValue retains restricted AST values without runtime symbol
// lookup or evaluation. Rules: MD-111 §§3(3)–(6),8(1).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func hardwareMetadataValue(v *ast.HardwareValue) hardware.Value {
	if v == nil {
		return hardware.Value{}
	}
	result := hardware.Value{Kind: string(v.Kind), Text: v.Text, Bool: v.Bool, Origin: hardware.Origin{Token: v.Token}}
	for _, part := range v.Path {
		result.Path = append(result.Path, part.Value)
	}
	for _, element := range v.Elements {
		result.Elements = append(result.Elements, hardwareMetadataValue(element))
	}
	return result
}

// analyzeHardwareDeclarations publishes only a successful source-stage snapshot
// and forwards mandatory errors/configurable warnings (MD-111 §§12,13(2)).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func (a *Analyzer) analyzeHardwareDeclarations(program *ast.Program) {
	facts, errors := a.ResolveHardwareDeclarations(program)
	valid := true
	for _, err := range errors {
		if err.Severity == diagnostics.SeverityWarning {
			a.appendWarning(err)
		} else {
			a.appendError(err)
			valid = false
		}
	}
	a.hardwareFacts = facts
	a.hardwareFactsValid = valid
}

// hardwareDiagnostic gives each schema failure a stable identity and preserves
// source/related spans. Rules: MD-111 §13(2); diagnostics.md §24(4i).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func hardwareDiagnostic(issue hardware.Issue) Error {
	ids := map[string]string{"type": diagnostics.HardwareMemberType, "member": diagnostics.HardwareMemberSchema, "enum": diagnostics.HardwareEnumCase, "domain": diagnostics.HardwareUnknownDomain, "range": diagnostics.HardwareAddressExtent, "symbol": diagnostics.HardwareSymbolResolution, "capability": diagnostics.HardwareRequiredCapability, "conflict": diagnostics.HardwareRegionConflict, "unsupported": diagnostics.HardwareSchemaUnavailable, "context": diagnostics.HardwareContextUnavailable, "overlap": diagnostics.HardwareRegionOverlap, "reserved": diagnostics.ReservedDeclarationName}
	id := ids[issue.Code]
	token := issue.Origin.Token
	endLine, endColumn := token.EndPosition()
	related := issue.Related.Token
	return Error{ID: id, Severity: diagnostics.DefaultSeverity(id), Message: issue.Message, Help: issue.Help, File: token.File, Line: token.Line, Column: token.Column, EndLine: endLine, EndColumn: endColumn, PreviousFile: related.File, PreviousLine: related.Line, PreviousColumn: related.Column, RelatedLabel: "related hardware declaration or property"}
}
