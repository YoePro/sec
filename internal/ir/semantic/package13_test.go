package semantic

import (
	"errors"
	"strings"
	"testing"

	"sec/internal/ast"
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
	if len(module.Unions) != 1 || len(module.Unions[0].Variants) != 2 || module.Unions[0].Variants[1].SyntheticPayloadStruct != module.Structs[0].TypeID || string(module.Structs[0].SymbolID) != "main::Position#1$payload" {
		t.Fatalf("unstable union TypeID/variant payload identity: unions=%#v structs=%#v", module.Unions, module.Structs)
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
	assertEveryUnionFieldProjectionIsGuardDominated(t, module.Functions[0])
}

func TestPackage13RejectsNonTrivialStructLikeUnionPayload(t *testing.T) {
	source := `module main
type Resource union { Empty Full { Value: ref mut int } }
fn Inspect(resource: Resource) void {
  match resource {
    Empty => {}
    Full(payload) => { discard payload }
  }
}`
	p := parser.New(lexer.NewWithFile(source, "package13-nontrivial-payload.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}
	if _, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 13}); err == nil || !strings.Contains(err.Error(), "type ref mut int") {
		t.Fatalf("non-trivial payload error = %v", err)
	}
}

func assertEveryUnionFieldProjectionIsGuardDominated(t *testing.T, function *Function) {
	t.Helper()
	projections := 0
	for _, block := range function.Blocks {
		for _, operation := range block.Operations {
			if operation.Kind != OpUnionUnwrapField {
				continue
			}
			projections++
			predecessors := []*Block{}
			for _, candidate := range function.Blocks {
				if len(candidate.Operations) == 0 {
					continue
				}
				terminator := candidate.Operations[len(candidate.Operations)-1]
				for _, successor := range terminator.Successors {
					if successor.Block == block.ID {
						predecessors = append(predecessors, candidate)
					}
				}
			}
			if len(predecessors) != 1 {
				t.Fatalf("unwrap block ^%d predecessors = %d", block.ID, len(predecessors))
			}
			guardBlock := predecessors[0]
			if len(guardBlock.Operations) < 2 || guardBlock.Operations[len(guardBlock.Operations)-2].Kind != OpUnionIsVariant || guardBlock.Operations[len(guardBlock.Operations)-1].Kind != OpCondBranch || guardBlock.Operations[len(guardBlock.Operations)-1].Successors[0].Block != block.ID {
				t.Fatalf("unwrap in ^%d is not reached through the matching true guard: %#v", block.ID, guardBlock.Operations)
			}
		}
	}
	if projections == 0 {
		t.Fatal("expected guarded union field projections")
	}
}

// rules/declarations/struct.md section 4 and
// rules/mlir/semantic-ir/sec_semantic_ir_struct_v1.md sections 2-6 require
// generic field tags to survive into the canonical Semantic IR definition.
func TestPackage13BuildsEmptyAndConcreteGenericWideStructs(t *testing.T) {
	source := `module main
type Empty struct {}
type Wide struct { Signed128: int128, Signed256: int256, Unsigned128: uint128, Unsigned256: uint256, Exact: decimal128 }
type Box[T] struct { Value: T ` + "`wire:\"value\"`" + ` }
type Outer struct { Payload: Box[Wide] }
type Vehicle struct { Engine: Vehicle.Engine, Stored: int }
impl Vehicle {
  type Engine struct { MaxPower: int }
  property Power: int { get { return self.Stored } }
}
fn Read(empty: Empty, outer: Outer) int128 { return outer.Payload.Value.Signed128 }
fn Power(vehicle: Vehicle) int { return vehicle.Stored }`
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
	if len(module.Structs) != 6 {
		t.Fatalf("structs = %#v", module.Structs)
	}
	byName := map[string]*StructDefinition{}
	for _, definition := range module.Structs {
		definition := definition
		byName[definition.Name] = &definition
	}
	if len(byName["Empty"].Fields) != 0 || len(byName["Box"].TypeArguments) != 1 || len(byName["Box"].Fields) != 1 {
		t.Fatalf("structs = %#v", module.Structs)
	}
	if identity := string(byName["Box"].SymbolID); !strings.Contains(identity, "main::Box<") || !strings.Contains(identity, "main::Wide") {
		t.Fatalf("concrete generic identity = %q", identity)
	}
	if string(byName["Vehicle.Engine"].SymbolID) != "main::Vehicle.Engine" || byName["Vehicle"].Fields[0].Type != byName["Vehicle.Engine"].TypeID {
		t.Fatalf("nested identities: Vehicle=%#v Engine=%#v", byName["Vehicle"], byName["Vehicle.Engine"])
	}
	if len(byName["Vehicle"].Fields) != 2 {
		t.Fatalf("property leaked into stored fields: %#v", byName["Vehicle"].Fields)
	}
	wantWide := map[string]TypeKind{"int128": TypeInt, "int256": TypeInt, "uint128": TypeUint, "uint256": TypeUint, "decimal128": TypeDecimal128}
	for _, field := range byName["Wide"].Fields {
		typ, ok := module.Types.Lookup(field.Type)
		if !ok || wantWide[typ.Name] != typ.Kind {
			t.Fatalf("wide field %s = %#v", field.Name, typ)
		}
		delete(wantWide, typ.Name)
	}
	if len(wantWide) != 0 {
		t.Fatalf("missing wide field types: %#v", wantWide)
	}
	if tags := byName["Box"].Fields[0].Tags; len(tags) != 1 || tags[0] != (StructTag{Key: "wire", Value: "value"}) {
		t.Fatalf("Box field tags = %#v", tags)
	}
	if byName["Outer"].LayoutRef != "" {
		t.Fatalf("unexpected physical layout authority: %q", byName["Outer"].LayoutRef)
	}
	byName["Outer"].LayoutRef = "layout::outer"
	for index := range module.Structs {
		if module.Structs[index].Name == "Outer" {
			module.Structs[index].LayoutRef = byName["Outer"].LayoutRef
		}
	}
	if err := Verify(module); err != nil {
		t.Fatalf("optional LayoutRef rejected: %v", err)
	}
}

// Package 13 sections 54 and 74 require malformed declaration-order metadata
// to be rejected before lowering can observe it.
func TestPackage13StructDefinitionVerifierRejectsFieldIdentityErrors(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*StructDefinition)
	}{
		{"duplicate field ID", func(definition *StructDefinition) { definition.Fields[1].ID = definition.Fields[0].ID }},
		{"duplicate field name", func(definition *StructDefinition) { definition.Fields[1].Name = definition.Fields[0].Name }},
		{"non-contiguous field ID", func(definition *StructDefinition) { definition.Fields[1].ID = 3 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			module, err := analyzedModule(t, `module main
type Pair struct { First: int, Second: int }
fn Read(value: Pair) int { return value.First }`, 13)
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(&module.Structs[0])
			if err := Verify(module); err == nil || !strings.Contains(err.Error(), "invalid struct field") {
				t.Fatalf("Verify() = %v", err)
			}
		})
	}
}

// Package 13 sections 29-31 and 75-77 require source expressions to execute in
// source order even though the final aggregate operands are declaration ordered.
func TestPackage13PreservesStructSourceEvaluationOrderAndEvaluatesSpreadOnce(t *testing.T) {
	module, err := analyzedModule(t, `module main
type Pair struct { First: int, Second: int }
fn Number(value: int) int { return value }
fn PairValue() Pair { return Pair { First: 10, Second: 20 } }
fn Build() Pair {
  return Pair { Second: Number(2), PairValue()..., First: Number(1) }
}`, 13)
	if err != nil {
		t.Fatal(err)
	}
	function := functionNamed(t, module, "Build")
	var sequence []string
	for _, operation := range function.Blocks[0].Operations {
		switch operation.Kind {
		case OpDirectCall:
			sequence = append(sequence, "call:"+string(operation.Callee))
		case OpStructSpreadFields:
			sequence = append(sequence, "spread")
		case OpStructConstruct:
			sequence = append(sequence, "construct")
		}
	}
	if len(sequence) != 5 || !strings.Contains(sequence[0], "Number") || !strings.Contains(sequence[1], "PairValue") || sequence[2] != "spread" || !strings.Contains(sequence[3], "Number") || sequence[4] != "construct" {
		t.Fatalf("evaluation sequence = %#v\n%s", sequence, Format(module))
	}
	spreads := 0
	for _, operation := range function.Blocks[0].Operations {
		if operation.Kind == OpStructSpreadFields {
			spreads++
		}
	}
	if spreads != 1 {
		t.Fatalf("spread evaluations = %d", spreads)
	}
}

// Package 13 sections 33-34 record the later P17 vocabulary, while the P13
// executable subset remains construct-direct and copy-trivial only.
func TestPackage13AcceptsOnlyTrivialStructConstructionActions(t *testing.T) {
	module, err := analyzedModule(t, `module main
type Pair struct { First: int, Second: int }
fn Build(value: int) Pair { return Pair { First: 1, Second: value } }`, 13)
	if err != nil {
		t.Fatal(err)
	}
	construct := firstOperation(t, functionNamed(t, module, "Build"), OpStructConstruct)
	if len(construct.StructActions) != 2 || construct.StructActions[0] != StructActionConstructDirect || construct.StructActions[1] != StructActionCopyTrivial {
		t.Fatalf("construction actions = %#v", construct.StructActions)
	}
	for _, action := range []StructFieldAction{StructActionMove, StructActionCopySemanticInfallible} {
		original := construct.StructActions[0]
		construct.StructActions[0] = action
		if err := Verify(module); err == nil || !strings.Contains(err.Error(), "struct.construct field") {
			t.Fatalf("action %q Verify() = %v", action, err)
		}
		construct.StructActions[0] = original
	}
	for _, action := range []sema.ResolvedStructFieldAction{sema.StructFieldMove, sema.StructFieldCopySemanticInfallible} {
		mapped, ok := semanticStructAction(action)
		if !ok || mapped == StructActionConstructDirect || mapped == StructActionCopyTrivial {
			t.Fatalf("current action vocabulary lost %q: %q, %t", action, mapped, ok)
		}
	}
	for _, action := range []sema.ResolvedStructFieldAction{sema.StructFieldMove, sema.StructFieldCopySemanticInfallible} {
		if err := package13BuildLiteralWithForcedAction(t, action); err == nil || !strings.Contains(err.Error(), "non-trivial struct field action "+string(action)) {
			t.Fatalf("forced action %q error = %v", action, err)
		}
	}
}

// Package 13 section 39 permits stored-field reads from every ordinary SSA
// source shape without collapsing property access into a field operation.
func TestPackage13BuildsScalarWideNestedParameterAndReturnFieldReads(t *testing.T) {
	module, err := analyzedModule(t, `module main
type Inner struct { Value: int }
type Data struct { Scalar: int, Wide: int128, Nested: Inner }
fn Identity(value: Data) Data { return value }
fn Scalar(value: Data) int { return value.Scalar }
fn Wide(value: Data) int128 { return Identity(value).Wide }
fn Nested(value: Data) int { return value.Nested.Value }`, 13)
	if err != nil {
		t.Fatal(err)
	}
	if got := operationCount(functionNamed(t, module, "Scalar"), OpStructExtractField); got != 1 {
		t.Fatalf("scalar extracts = %d", got)
	}
	if got := operationCount(functionNamed(t, module, "Wide"), OpStructExtractField); got != 1 {
		t.Fatalf("return-value wide extracts = %d", got)
	}
	if got := operationCount(functionNamed(t, module, "Nested"), OpStructExtractField); got != 2 {
		t.Fatalf("nested extracts = %d", got)
	}
}

// Package 13 sections 44-48 require both initializer forms and a transactional
// leaf-to-root nested replacement after the complete RHS evaluation.
func TestPackage13BuildsExplicitAndDefaultedMutableStructReplacementInCommitOrder(t *testing.T) {
	module, err := analyzedModule(t, `module main
type Inner struct { Value: int }
type Outer struct { Inner: Inner, Enabled: bool }
fn Next() int { return 7 }
fn Update() int {
  let mut defaulted: Outer
  let mut item := Outer { Inner: Inner { Value: 1 }, Enabled: true }
  item.Inner.Value = Next()
  return item.Inner.Value
}`, 13)
	if err != nil {
		t.Fatal(err)
	}
	function := functionNamed(t, module, "Update")
	if operationCount(function, OpStorageDeclare) != 2 || operationCount(function, OpStorageInit) != 2 || operationCount(function, OpStorageStore) != 1 || operationCount(function, OpStructReplaceField) != 2 {
		t.Fatalf("unexpected mutable operation matrix:\n%s", Format(module))
	}
	callIndex, loadIndex, storeIndex := -1, -1, -1
	var replacements []*Operation
	for index := range function.Blocks[0].Operations {
		operation := &function.Blocks[0].Operations[index]
		switch operation.Kind {
		case OpDirectCall:
			if strings.Contains(string(operation.Callee), "Next") {
				callIndex = index
			}
		case OpStorageLoad:
			if loadIndex < 0 {
				loadIndex = index
			}
		case OpStructReplaceField:
			replacements = append(replacements, operation)
		case OpStorageStore:
			if storeIndex < 0 {
				storeIndex = index
			}
		}
	}
	if callIndex < 0 || loadIndex <= callIndex || len(replacements) != 2 || storeIndex <= loadIndex {
		t.Fatalf("commit order call=%d load=%d replacements=%d store=%d\n%s", callIndex, loadIndex, len(replacements), storeIndex, Format(module))
	}
	firstResult, _ := module.Types.Lookup(replacements[0].Results[0].Type)
	secondResult, _ := module.Types.Lookup(replacements[1].Results[0].Type)
	if firstResult.Name != "Inner" || secondResult.Name != "Outer" {
		t.Fatalf("replacement order = %s then %s", firstResult.Name, secondResult.Name)
	}
}

// package13BuildLiteralWithForcedAction drives the actual P13 literal builder
// with a valid compiler-owned plan whose action has been replaced by a later
// P17 action. This proves the executable boundary independently of verifier
// rejection for malformed already-built IR.
func package13BuildLiteralWithForcedAction(t *testing.T, action sema.ResolvedStructFieldAction) error {
	t.Helper()
	source := `module main
type Pair struct { First: int, Second: int }
fn Build() Pair { return Pair { First: 1, Second: 2 } }`
	p := parser.New(lexer.NewWithFile(source, "package13-forced-action.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}
	module, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 13})
	if err != nil {
		t.Fatal(err)
	}
	declaration := parsed.Program.Statements[2].(*ast.FunctionDeclaration)
	literal := declaration.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.StructLiteral)
	plan, ok := analyzer.ResolvedStructLiteralPlanOf(literal)
	if !ok {
		t.Fatal("missing struct literal plan")
	}
	plan.FinalFields[0].Action = action
	definition := module.Structs[0]
	owner := &builder{module: module, analyzer: analyzer, maxPackage: 13, definedStructs: map[TypeID]bool{definition.TypeID: true}}
	function := &Function{Name: "forced-action"}
	block := &Block{ID: 999}
	function.Blocks = append(function.Blocks, block)
	functionBuilder := &functionBuilder{owner: owner, fn: function, current: function.Blocks[0], nextValue: 1000, bindings: map[sema.BindingID]binding{}}
	_, err = functionBuilder.buildStructLiteral(literal, plan, definition.TypeID)
	return err
}

func functionNamed(t *testing.T, module *Module, name string) *Function {
	t.Helper()
	for _, function := range module.Functions {
		if function.Name == name {
			return function
		}
	}
	t.Fatalf("missing function %s", name)
	return nil
}

func firstOperation(t *testing.T, function *Function, kind OpKind) *Operation {
	t.Helper()
	for blockIndex := range function.Blocks {
		for operationIndex := range function.Blocks[blockIndex].Operations {
			operation := &function.Blocks[blockIndex].Operations[operationIndex]
			if operation.Kind == kind {
				return operation
			}
		}
	}
	t.Fatalf("missing operation %s", kind)
	return nil
}

func operationCount(function *Function, kind OpKind) int {
	count := 0
	for _, block := range function.Blocks {
		for _, operation := range block.Operations {
			if operation.Kind == kind {
				count++
			}
		}
	}
	return count
}

// rules/mlir/packages/sec-mlir-dialect_package13.md section 83, reconciled
// with the later ownership, copy/move, borrowing, and FFI rulebooks, requires
// every non-trivial struct path to stop at an explicit frontend or Semantic IR
// boundary. A failed build must never expose the module assembled before the
// unsupported operation was encountered.
func TestPackage13RejectsUnsupportedStructValuePathsWithoutPartialIR(t *testing.T) {
	type rejectionStage string
	const (
		parserStage rejectionStage = "parser"
		semaStage   rejectionStage = "sema"
		buildStage  rejectionStage = "semantic-ir"
	)
	tests := []struct {
		name       string
		source     string
		stage      rejectionStage
		wantDetail string
	}{
		{
			name: "move-only source field transfer",
			source: `module main
@noCopy
type Session struct { Value: int }
type Holder struct { Session: Session }
fn Move(value: Session) Holder { return Holder { Session: <-value } }`,
			stage: buildStage, wantDetail: "non-scalar parameter type Session",
		},
		{
			name: "ordinary move-only field read",
			source: `module main
@noCopy
type Session struct { Value: int }
type Holder struct { Session: Session }
fn Read() Session {
  let value := Holder { Session: Session { Value: 1 } }
  return value.Session
}`,
			stage: buildStage, wantDetail: "non-trivial struct field read",
		},
		{
			name: "borrowed reference field construction",
			source: `module main
type View struct { Value: ref int }
fn Build(value: ref int) View { return View { Value: value } }`,
			stage: buildStage, wantDetail: "type ref int",
		},
		{
			name: "mutable reference field construction",
			source: `module main
type View struct { Value: ref mut int }
fn Build(value: ref mut int) View { return View { Value: <-value } }`,
			stage: buildStage, wantDetail: "type ref mut int",
		},
		{
			name: "resource-owning dynamic array field",
			source: `module main
type Buffer struct { Values: int[] }
fn Keep(value: Buffer) Buffer { return value }`,
			stage: buildStage, wantDetail: "type int[]",
		},
		{
			name: "shared field borrow",
			source: `module main
type Pair struct { First: int, Second: int }
fn Borrow(value: Pair) void { let field := ref value.First }`,
			stage: buildStage, wantDetail: "type ref int",
		},
		{
			name: "mutable field borrow",
			source: `module main
type Pair struct { First: int, Second: int }
fn Borrow(value: Pair) void {
  let mut local := value
  let field := ref mut local.First
}`,
			stage: buildStage, wantDetail: "type ref mut int",
		},
		{
			name: "partial move",
			source: `module main
@noCopy
type Session struct { Value: int }
type Pair struct { First: Session, Second: Session }
fn Take() void {
  let value := Pair { First: Session { Value: 1 }, Second: Session { Value: 2 } }
  let first :<- value.First
  discard first
}`,
			stage: buildStage, wantDetail: "non-trivial struct field read",
		},
		{
			name: "non-trivial field replacement",
			source: `module main
@noCopy
type Session struct { Value: int }
type Holder struct { Session: Session }
fn Replace() void {
  let mut local := Holder { Session: Session { Value: 1 } }
  let replacement := Session { Value: 2 }
  local.Session <- replacement
}`,
			stage: buildStage, wantDetail: "non-trivial mutable local storage",
		},
		{
			name: "property read",
			source: `module main
type Point struct { X: int }
impl Point { property Value: int { get { return self.X } } }
fn Read(value: Point) int { return value.Value }`,
			stage: buildStage, wantDetail: "member expression",
		},
		{
			name: "property write",
			source: `module main
type Point struct { X: int }
impl Point { property Value: int { get { return self.X } set next { self.X = next } } }
fn Write() void {
  let mut value := Point { X: 0 }
  value.Value = 1
}`,
			stage: buildStage, wantDetail: "non-trivial or unresolved struct field assignment",
		},
		{
			name: "struct equality",
			source: `module main
type Point struct { X: int }
fn Equal(left: Point, right: Point) bool { return left == right }`,
			stage: buildStage, wantDetail: "unresolved or unsupported operator",
		},
		{
			name: "method receiver",
			source: `module main
type Point struct { X: int }
impl Point { fn Value() int { return self.X } }
fn Read(value: Point) int { return value.Value() }`,
			stage: buildStage, wantDetail: "method call",
		},
		{
			name: "custom free lifecycle",
			source: `module main
type Resource struct { Handle: int }
impl Resource { free { discard self.Handle } }`,
			stage: semaStage, wantDetail: "free operations are reserved for destruction",
		},
		{
			name: "foreign struct ABI syntax",
			source: `module main
extern "C" type Point struct { X: C::int, Y: C::int }`,
			stage: parserStage, wantDetail: "expected",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(test.source, "package13-boundary.sec"))
			parsed := p.Parse()
			if test.stage == parserStage {
				if !parsed.HasErrors || !strings.Contains(strings.Join(p.Errors(), "\n"), test.wantDetail) {
					t.Fatalf("parser errors = %v", p.Errors())
				}
				return
			}
			if parsed.HasErrors {
				t.Fatalf("unexpected parser errors: %v", p.Errors())
			}
			analyzer := sema.NewAnalyzer()
			semaErrors := analyzer.Analyze(parsed.Program)
			if test.stage == semaStage {
				if len(semaErrors) == 0 || !strings.Contains(semaErrors[0].Message, test.wantDetail) {
					t.Fatalf("sema errors = %v", semaErrors)
				}
				return
			}
			if len(semaErrors) != 0 {
				t.Fatalf("unexpected sema errors: %v", semaErrors)
			}
			module, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", SourceFiles: []string{"package13-boundary.sec"}, MaxPackage: 13})
			if module != nil {
				t.Fatalf("unsupported build exposed partial IR:\n%s", Format(module))
			}
			var unsupported *UnsupportedFeatureError
			if !errors.As(err, &unsupported) || unsupported.Package != 13 || !strings.Contains(unsupported.Feature, test.wantDetail) {
				t.Fatalf("build error = %#v, want Package 13 UnsupportedFeatureError containing %q", err, test.wantDetail)
			}
		})
	}
}

// Package 13 sections 84-86 keep successful structs entirely semantic. This
// test rejects the placeholder and physical-lowering vocabulary explicitly and
// also proves that a pure construction/spread/read path creates no storage.
func TestPackage13SuccessfulStructPathHasNoPlaceholderOrPhysicalLowering(t *testing.T) {
	module, err := analyzedModule(t, `module main
type Pair struct { First: int128, Second: uint256 }
fn Build(base: Pair) int128 {
  let value := Pair { base..., First: 7 }
  return value.First
}`, 13)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	counts := map[OpKind]int{}
	for _, function := range module.Functions {
		for _, block := range function.Blocks {
			for _, operation := range block.Operations {
				counts[operation.Kind]++
			}
		}
	}
	if counts[OpStructConstruct] != 1 || counts[OpStructSpreadFields] != 1 || counts[OpStructExtractField] != 1 {
		t.Fatalf("successful path did not exercise construct/spread/read: %#v\n%s", counts, Format(module))
	}
	text := strings.ToLower(Format(module))
	for _, forbidden := range []string{"undef", "poison", "llvm.", "physical-offset", "field-offset", "alloca", "aggregate.insert", "aggregate.extract"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("successful P13 IR contains forbidden %q:\n%s", forbidden, text)
		}
	}
	for _, function := range module.Functions {
		if len(function.Storages) != 0 {
			t.Fatalf("pure struct construction allocated hidden storage: %#v", function.Storages)
		}
	}
	for _, definition := range module.Structs {
		if definition.LayoutRef != "" {
			t.Fatalf("P13 invented physical layout authority: %#v", definition)
		}
	}
}
