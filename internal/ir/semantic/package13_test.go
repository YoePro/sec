package semantic

import (
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// rules/mlir/packages/sec-mlir-dialect_package13.md plus the later P17 action
// amendment: the first executable slice remains trivial, but its facts use the
// current action vocabulary and preserve source-order spread/default origins.
func TestPackage13BuildsStructDefinitionSpreadConstructionAndFieldRead(t *testing.T) {
	source := `module main
type Pair struct { Value: int, Enabled: bool, Limit: uint }
fn Build(base: Pair) int {
  let merged := Pair { base..., Enabled: true }
  let defaults := Pair { Value: 4 }
  return merged.Value
}`
	p := parser.New(lexer.NewWithFile(source, "package13.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}
	module, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", SourceFiles: []string{"package13.sec"}, MaxPackage: 13})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	if len(module.Structs) != 1 || len(module.Structs[0].Fields) != 3 || module.Structs[0].Fields[0].ID != 0 || module.Structs[0].Fields[2].Name != "Limit" {
		t.Fatalf("struct definition = %#v", module.Structs)
	}
	counts := map[OpKind]int{}
	for _, block := range module.Functions[0].Blocks {
		for _, op := range block.Operations {
			counts[op.Kind]++
		}
	}
	if counts[OpStructSpreadFields] != 1 || counts[OpStructConstruct] != 2 || counts[OpStructExtractField] != 1 {
		t.Fatalf("operation counts = %#v\n%s", counts, Format(module))
	}
}

func TestPackage13UsesCurrentStringViewTraitsRatherThanStaleOwnershipAssumptions(t *testing.T) {
	source := `module main
type Item struct { Text: string }
fn Copy(value: Item) Item { return Item { value... } }`
	p := parser.New(lexer.NewWithFile(source, "package13-nontrivial.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}
	// Current strings are still trivial views, so use the action classification
	// actually recorded by Sema rather than inventing an ownership rejection.
	if _, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 13}); err != nil {
		t.Fatalf("current trivial string-view struct should build: %v", err)
	}
}

// rules/mlir/lowering-versions/sec_mlir_lowering_v9.md sections 9-11 require
// defaulted trivial storage and leaf-to-root nested replacement with one root
// store.
func TestPackage13BuildsDefaultedMutableNestedStructReplacement(t *testing.T) {
	source := `module main
type Inner struct { Value: int }
type Outer struct { Inner: Inner, Enabled: bool }
fn Update() int {
  let mut item: Outer
  item.Inner.Value = 7
  return item.Inner.Value
}`
	p := parser.New(lexer.NewWithFile(source, "package13-mutable.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}
	module, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", SourceFiles: []string{"package13-mutable.sec"}, MaxPackage: 13})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	counts := map[OpKind]int{}
	for _, block := range module.Functions[0].Blocks {
		for _, operation := range block.Operations {
			counts[operation.Kind]++
		}
	}
	if counts[OpStorageDeclare] != 1 || counts[OpStorageInit] != 1 || counts[OpStorageStore] != 1 || counts[OpStructReplaceField] != 2 {
		t.Fatalf("operation counts = %#v\n%s", counts, Format(module))
	}
}

// rules/mlir/packages/sec-mlir-dialect_package13.md sections 66-68 enable the
// Package 12 whole-payload binding once canonical synthetic structs exist.
func TestPackage13BuildsSyntheticStructLikeUnionPayloadBinding(t *testing.T) {
	source := `module main
type Position union {
  Unknown
  Known { X: int128, Y: uint256 }
}
fn Read(position: Position, zero: int128) int128 {
  return match position {
    Unknown => zero
    Known(payload) => payload.X
  }
}`
	p := parser.New(lexer.NewWithFile(source, "package13-union-payload.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}
	module, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", SourceFiles: []string{"package13-union-payload.sec"}, MaxPackage: 13})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	if len(module.Structs) != 1 || module.Structs[0].SyntheticOrigin != StructSyntheticUnionPayload || module.Structs[0].Name != "Position.Known" {
		t.Fatalf("synthetic structs = %#v", module.Structs)
	}
	counts := map[OpKind]int{}
	for _, block := range module.Functions[0].Blocks {
		for _, operation := range block.Operations {
			counts[operation.Kind]++
		}
	}
	if counts[OpUnionUnwrapField] != 2 || counts[OpStructConstruct] != 1 || counts[OpStructExtractField] != 1 {
		t.Fatalf("operation counts = %#v\n%s", counts, Format(module))
	}
}

// rules/mlir/packages/sec-mlir-dialect_package13.md sections 8-10 require
// empty, concrete-generic, nested-identity, and active wide-field support.
func TestPackage13BuildsEmptyAndConcreteGenericWideStructs(t *testing.T) {
	source := `module main
type Empty struct {}
type Box[T] struct { Value: T }
fn Read(empty: Empty, box: Box[int256]) int256 { return box.Value }`
	p := parser.New(lexer.NewWithFile(source, "package13-types.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}
	module, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", SourceFiles: []string{"package13-types.sec"}, MaxPackage: 13})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	if len(module.Structs) != 2 {
		t.Fatalf("structs = %#v", module.Structs)
	}
	byName := map[string]StructDefinition{}
	for _, definition := range module.Structs {
		byName[definition.Name] = definition
	}
	if len(byName["Empty"].Fields) != 0 || len(byName["Box"].TypeArguments) != 1 || len(byName["Box"].Fields) != 1 {
		t.Fatalf("structs = %#v", module.Structs)
	}
}
