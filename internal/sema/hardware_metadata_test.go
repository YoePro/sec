package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/hardware"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/modules"
	"sec/internal/parser"
)

func hardwareProgram(t *testing.T, names ...string) *ast.Program {
	t.Helper()
	result := &ast.Program{}
	for _, name := range names {
		file := "../../testdata/hardware/metadata/" + name
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), file))
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatal(p.Errors())
		}
		result.Statements = append(result.Statements, program.Statements...)
	}
	return result
}
func hardwareFile(name string) string { return "../../testdata/hardware/metadata/" + name }
func hardwareAnalyzer(bits uint16, depth AnalysisDepth) *Analyzer {
	return NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: bits, AddressDomains: []layout.AddressDomain{{Identity: "cpu-physical", AddressBits: 128, UnitBits: 8, Linear: true}}}, depth)
}
func hardwareHasID(errors []Error, id string) bool {
	for _, err := range errors {
		if err.ID == id {
			return true
		}
	}
	return false
}

// TestHardwareMetadataAnalysis exercises the real parser-to-Sema pass at every
// mandatory analysis depth and both CPU widths (MD-111 §§3–8,13(4)–(5)).
func TestHardwareMetadataAnalysis(t *testing.T) {
	for _, bits := range []uint16{32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			analyzer := hardwareAnalyzer(bits, depth)
			module := modules.ModuleIdentity{ImportRootIdentity: "application", CanonicalImportPath: "board/hardware"}
			analyzer.SetHardwareModuleContext(HardwareModuleContext{Files: map[string]modules.ModuleIdentity{hardwareFile("valid.sec"): module}})
			if errors := analyzer.Analyze(hardwareProgram(t, "valid.sec")); len(errors) > 0 {
				t.Fatalf("uint%d/%s: %v", bits, depth, errors)
			}
			facts, ok := analyzer.HardwareFacts()
			if !ok || len(facts.Regions) != 1 || len(facts.Resources) != 1 || len(facts.Endpoints) != 2 {
				t.Fatalf("facts %+v", facts)
			}
			if facts.Endpoints[0].Identity.Module != module || facts.Endpoints[0].Resource != facts.Endpoints[1].Resource || facts.Endpoints[0].Address.String() != "1073742336" {
				t.Fatal("lost canonical module/resource/address facts")
			}
			if _, exists := analyzer.types["HardwareRegionKind"]; exists {
				t.Fatal("metadata enum became a runtime type")
			}
			if _, exists := analyzer.symbols["OutputState"]; exists {
				t.Fatal("metadata resource became a runtime value")
			}
			before, _ := analyzer.HardwareFacts()
			facts.Endpoints[0].Address.SetInt64(1)
			facts.Endpoints[0].Properties["size"].Integer.SetInt64(100)
			facts.Regions[0].Properties["domain"] = hardware.ResolvedValue{}
			after, _ := analyzer.HardwareFacts()
			if !reflect.DeepEqual(before, after) {
				t.Fatal("caller mutated analyzer hardware snapshot")
			}
			analyzer.Analyze(&ast.Program{})
			empty, ok := analyzer.HardwareFacts()
			if !ok || len(empty.Endpoints) > 0 {
				t.Fatal("facts survived analyzer reuse")
			}
		}
	}
}

// TestHardwareMetadataImports uses actual file-local imports; canonical owner
// identities come only from resolver inputs (MD-111 §§2,8(3),12).
func TestHardwareMetadataImports(t *testing.T) {
	owner := modules.ModuleIdentity{ImportRootIdentity: "dependency-a", CanonicalImportPath: "drivers/gpio"}
	client := modules.ModuleIdentity{ImportRootIdentity: "application", CanonicalImportPath: "app"}
	context := HardwareModuleContext{Files: map[string]modules.ModuleIdentity{hardwareFile("driver.sec"): owner, hardwareFile("importer.sec"): client}, Imports: map[string]map[string][]modules.ModuleIdentity{hardwareFile("importer.sec"): {"drivers/gpio": {owner}}}}
	analyzer := hardwareAnalyzer(32, AnalysisStandard)
	analyzer.SetHardwareModuleContext(context)
	// Mutating caller-owned module selection must not affect installed context.
	context.Files[hardwareFile("driver.sec")] = client
	context.Imports[hardwareFile("importer.sec")]["drivers/gpio"][0] = client
	program := hardwareProgram(t, "importer.sec", "driver.sec")
	facts, errors := analyzer.ResolveHardwareDeclarations(program)
	if len(errors) > 0 || len(facts.Endpoints) != 1 {
		t.Fatal(errors)
	}
	reference := facts.Endpoints[0].Properties["region"]
	if reference.Reference.Module != owner || reference.Reference.Name != "PeripheralSpace" || reference.Origin.Token.File != hardwareFile("importer.sec") || facts.Regions[0].Origin.Token.File != hardwareFile("driver.sec") {
		t.Fatal("lost cross-module identity/provenance")
	}
	// An omitted alias uses the selected source ModuleName, not the last path
	// segment (driver, rather than gpio).
	for _, stmt := range program.Statements {
		switch s := stmt.(type) {
		case *ast.ImportStatement:
			s.Alias = ""
		case *ast.HardwareDeclaration:
			for _, member := range s.Members {
				if member.Value.Kind == ast.HardwareValuePath && len(member.Value.Path) == 2 && member.Value.Path[0].Value == "board" {
					member.Value.Path[0].Value = "driver"
				}
			}
		}
	}
	if _, errors = analyzer.ResolveHardwareDeclarations(program); len(errors) > 0 {
		t.Fatalf("source module qualifier: %v", errors)
	}
	for _, stmt := range program.Statements {
		if d, ok := stmt.(*ast.HardwareDeclaration); ok && d.Category == "endpoint" {
			for _, member := range d.Members {
				if member.Name.Value == "resource" {
					member.Value.Path[1].Value = "_Internal"
				}
			}
		}
	}
	if _, errors = analyzer.ResolveHardwareDeclarations(program); !hardwareHasID(errors, diagnostics.HardwareSymbolResolution) {
		t.Fatal(errors)
	}
	for _, err := range errors {
		if err.ID == diagnostics.HardwareSymbolResolution && (err.PreviousFile != hardwareFile("driver.sec") || err.Line == 0 || err.Help == "") {
			t.Fatal("missing inaccessible-symbol provenance")
		}
	}
	// A canonical import path ambiguous across roots is never selected by order.
	context.Files[hardwareFile("driver.sec")] = owner
	context.Imports[hardwareFile("importer.sec")]["drivers/gpio"] = []modules.ModuleIdentity{owner, {ImportRootIdentity: "dependency-b", CanonicalImportPath: "drivers/gpio"}}
	analyzer.SetHardwareModuleContext(context)
	if _, errors = analyzer.ResolveHardwareDeclarations(hardwareProgram(t, "driver.sec", "importer.sec")); !hardwareHasID(errors, diagnostics.HardwareSymbolResolution) {
		t.Fatal(errors)
	}
}

// TestHardwareMetadataDiagnostics verifies stable mandatory IDs and invalid fact
// suppression at the Sema boundary (MD-111 §13(2),(5); diagnostics.md §24(4i)).
func TestHardwareMetadataDiagnostics(t *testing.T) {
	owner := modules.ModuleIdentity{ImportRootIdentity: "project", CanonicalImportPath: "hardware"}
	for _, test := range []struct{ file, id string }{{"type_invalid.sec", diagnostics.HardwareMemberType}, {"namespace_invalid.sec", diagnostics.HardwareMemberSchema}} {
		analyzer := hardwareAnalyzer(64, AnalysisStandard)
		analyzer.SetHardwareModuleContext(HardwareModuleContext{Files: map[string]modules.ModuleIdentity{hardwareFile(test.file): owner}})
		errors := analyzer.Analyze(hardwareProgram(t, test.file))
		if !hardwareHasID(errors, test.id) {
			t.Fatalf("%s: %v", test.file, errors)
		}
		facts, ok := analyzer.HardwareFacts()
		if ok || len(facts.Resources) > 0 {
			t.Fatal("invalid metadata published")
		}
		for _, err := range errors {
			if err.ID == test.id && (err.File != hardwareFile(test.file) || err.Line == 0 || err.Column == 0 || err.EndColumn <= err.Column || err.Help == "") {
				t.Fatalf("missing exact diagnostic metadata: %+v", err)
			}
		}
	}
	analyzer := hardwareAnalyzer(32, AnalysisDeep)
	if _, errors := analyzer.ResolveHardwareDeclarations(hardwareProgram(t, "valid.sec")); !hardwareHasID(errors, diagnostics.HardwareContextUnavailable) {
		t.Fatal("canonical module identity guessed", errors)
	}
	analyzer = NewAnalyzer()
	analyzer.SetHardwareModuleContext(HardwareModuleContext{Files: map[string]modules.ModuleIdentity{hardwareFile("valid.sec"): owner}})
	if _, errors := analyzer.ResolveHardwareDeclarations(hardwareProgram(t, "valid.sec")); !hardwareHasID(errors, diagnostics.HardwareContextUnavailable) || !hardwareHasID(errors, diagnostics.HardwareUnknownDomain) {
		t.Fatal("target facts guessed", errors)
	}
}

// Wide source addresses and the full-domain extent remain exact through the
// lexer, restricted AST and Sema; CPU uint never limits them (MD-111 §3(3)–(4)).
func TestHardwareMetadataWideSourceIntegers(t *testing.T) {
	module := modules.ModuleIdentity{ImportRootIdentity: "project", CanonicalImportPath: "wide"}
	for _, bits := range []uint16{32, 64} {
		for _, file := range []string{"wide.sec", "full_domain.sec"} {
			a := hardwareAnalyzer(bits, AnalysisStandard)
			a.SetHardwareModuleContext(HardwareModuleContext{Files: map[string]modules.ModuleIdentity{hardwareFile(file): module}})
			if errors := a.Analyze(hardwareProgram(t, file)); len(errors) > 0 {
				t.Fatal(errors)
			}
			facts, ok := a.HardwareFacts()
			if !ok {
				t.Fatal("no wide source facts")
			}
			if file == "wide.sec" && facts.Endpoints[0].Address.String() != "18446744073709551616" {
				t.Fatal("wide address narrowed")
			}
			if file == "full_domain.sec" && facts.Regions[0].Size.String() != "340282366920938463463374607431768211456" {
				t.Fatal("domain cardinality narrowed")
			}
			// Installing a new module/Device context invalidates the previous snapshot.
			a.SetHardwareModuleContext(HardwareModuleContext{})
			if _, valid := a.HardwareFacts(); valid {
				t.Fatal("stale facts remained valid after context change")
			}
		}
	}
}

// Region warnings obey project diagnostic policy without fabricating binding
// aliases or corrupting valid metadata (MD-111 §§10(6),13(2); diagnostics.md §18).
func TestHardwareMetadataRegionWarningPolicy(t *testing.T) {
	owner := modules.ModuleIdentity{ImportRootIdentity: "project", CanonicalImportPath: "hardware"}
	for _, policy := range []diagnostics.RulePolicy{nil, {diagnostics.HardwareRegionOverlap: "off"}, {diagnostics.HardwareRegionOverlap: "error"}} {
		a := hardwareAnalyzer(32, AnalysisStandard)
		a.SetHardwareModuleContext(HardwareModuleContext{Files: map[string]modules.ModuleIdentity{hardwareFile("overlap.sec"): owner}})
		a.SetDiagnosticRules(policy)
		errors := a.Analyze(hardwareProgram(t, "overlap.sec"))
		warnings := a.Warnings()
		switch policy[diagnostics.HardwareRegionOverlap] {
		case "off":
			if len(errors)+len(warnings) != 0 {
				t.Fatal(errors, warnings)
			}
		case "error":
			if !hardwareHasID(errors, diagnostics.HardwareRegionOverlap) || len(warnings) > 0 {
				t.Fatal(errors, warnings)
			}
		default:
			if len(errors) > 0 || len(warnings) != 1 || warnings[0].ID != diagnostics.HardwareRegionOverlap || warnings[0].PreviousLine != 2 {
				t.Fatal(errors, warnings)
			}
		}
		facts, valid := a.HardwareFacts()
		if !valid || len(facts.Regions) != 2 {
			t.Fatal("advisory changed hardware facts")
		}
	}
}

// A newly introduced hardware name participates in the source qualifier
// namespace, including import aliases (modules.md §11; MD-111 §8(3)).
func TestHardwareMetadataQualifierConflict(t *testing.T) {
	file := hardwareFile("qualifier_invalid.sec")
	owner := modules.ModuleIdentity{ImportRootIdentity: "project", CanonicalImportPath: "hardware"}
	driver := modules.ModuleIdentity{ImportRootIdentity: "dependency", CanonicalImportPath: "drivers/gpio"}
	a := hardwareAnalyzer(32, AnalysisStandard)
	a.SetHardwareModuleContext(HardwareModuleContext{Files: map[string]modules.ModuleIdentity{file: owner}, Imports: map[string]map[string][]modules.ModuleIdentity{file: {"drivers/gpio": {driver}}}})
	facts, errors := a.ResolveHardwareDeclarations(hardwareProgram(t, "qualifier_invalid.sec"))
	if !hardwareHasID(errors, diagnostics.HardwareSymbolResolution) || len(facts.Resources) > 0 {
		t.Fatal(errors)
	}
	for _, err := range errors {
		if err.ID == diagnostics.HardwareSymbolResolution && (err.Line != 2 || err.PreviousLine != 3) {
			t.Fatal("lost qualifier/declaration provenance", err)
		}
	}
}
