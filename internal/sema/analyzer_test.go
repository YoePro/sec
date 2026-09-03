package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func TestAnalyzeSimpleLetInitializers(t *testing.T) {
	input := `
module main

let a: int := -5
let b: uint := 5
let c: uint := -5
let d: bool := "test"
let e: uuid := 1
`

	errors := analyzeSource(t, input)

	expected := []string{
		"value -5 overflows uint at 6:16",
		"cannot initialize bool with string at 7:16",
		"unknown type uuid at 8:8",
	}

	assertSemaErrors(t, errors, expected)
}

func TestLifecycleInitAndNewSemantics(t *testing.T) {
	errors := analyzeSource(t, `
module main

type BuildError enum error {
    Invalid,
}

type Point struct {
    x: int,
}

impl Point {
    init(x: int) {
        self.x = x
    }
}

type Resource struct {
    value: int,
}

impl Resource {
    init(value: int) BuildError {
        self.value = value
    }
}

type Defaults struct {
    value: int,
}

impl Defaults {
    init(value: int) {
        self.value = value
    }
}

fn PointAt(x: int) Point {
    return new Point(x)
}

fn Open(value: int) Result[Resource, BuildError] {
    return Ok(try new Resource(value))
}

fn DefaultValue() Defaults {
    return new Defaults()
}
`)
	assertSemaErrors(t, errors, nil)

	errors = analyzeSource(t, `
module main
type BuildError enum error { Invalid }
type Resource struct { value: int, }
impl Resource {
    init(value: int) BuildError { self.value = value }
}
fn Invalid() void {
    let resource := new Resource(1)
}
`)
	if len(errors) == 0 || !strings.Contains(errors[0].Message, "selects a fallible init") {
		t.Fatalf("missing unhandled construction diagnostic: %+v", errors)
	}
}

func TestInitOverloadIdentityIgnoresConstructionErrorType(t *testing.T) {
	errors := analyzeSource(t, `
module main
type FirstError enum error { Failed }
type SecondError enum error { Failed }
type Resource struct { value: int, }
impl Resource {
    init(value: int) FirstError { self.value = value }
}

impl extends Resource {
    init(value: int) SecondError { self.value = value }
}
`)
	if len(errors) == 0 || !strings.Contains(errors[0].Message, "duplicate init signature init(int) for Resource") {
		t.Fatalf("missing duplicate init signature diagnostic: %+v", errors)
	}
}

func TestOrdinaryImplMustBelongToDefiningModule(t *testing.T) {
	program := parser.New(lexer.NewWithFile(`
module graphics
type Image struct { width: int, }
`, "graphics.sec")).ParseProgram()
	application := parser.New(lexer.NewWithFile(`
module application
impl Image {
    fn Width() int { return self.width }
}
`, "application.sec")).ParseProgram()
	program.Statements = append(program.Statements, application.Statements...)
	errors := NewAnalyzer().Analyze(program)
	if len(errors) == 0 || !strings.Contains(errors[0].Message, "must be declared in defining module graphics") {
		t.Fatalf("missing defining-module impl diagnostic: %+v", errors)
	}
}

func TestRuneArrayToStringMaterializesText(t *testing.T) {
	errors := analyzeSource(t, `
module main

fn pair() string {
	let runes: rune[2] := ['A', 'B']
	return runes.ToString()
}

fn sliceText() string {
	let runes: rune[3] := ['a', 'b', 'c']
	let view := ref runes[..]
	return view.ToString()
}

fn directSliceText() string {
	let runes: rune[3] := ['a', 'b', 'c']
	return runes[1..<3].ToString()
}
`)

	assertSemaErrors(t, errors, nil)
}

func TestImplMethodMutabilityPropagatesThroughLetInitializer(t *testing.T) {
	errors := analyzeSource(t, `
module main

type Reader struct {
	position: int,
}

impl Reader {
	fn advance() int {
		self.position += 1
		return self.position
	}

	fn readBody() int {
		let position := self.advance()
		return position
	}
}

fn main() int {
	let mut reader := Reader{ position: 0 }
	return reader.readBody()
}
`)

	assertSemaErrors(t, errors, nil)
}

func TestCallGraphRecordsDirectAndStaticMethodCalls(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `
module main

fn helper(value: int) int {
	return value
}

type Counter struct {
	value: int,
}

impl Counter {
	fn read() int {
		return self.value
	}
}

fn use(counter: Counter) int {
	return helper(counter.read())
}
`)
	assertSemaErrors(t, errors, nil)

	graph := analyzer.CallGraph()
	var useID CallableID
	for _, node := range graph.Nodes() {
		if node.Name == "use" {
			useID = node.ID
		}
	}
	if useID == "" {
		t.Fatal("call graph does not contain use")
	}

	sites := graph.Outgoing(useID)
	if len(sites) != 2 {
		t.Fatalf("outgoing calls = %+v, want helper and Counter.read", sites)
	}
	dispatches := map[CallDispatchKind]bool{}
	targets := map[string]bool{}
	for _, site := range sites {
		dispatches[site.Dispatch] = true
		if len(site.Targets) != 1 {
			t.Fatalf("site targets = %+v, want one closed target", site.Targets)
		}
		target, ok := graph.Node(site.Targets[0])
		if !ok {
			t.Fatalf("missing target node %q", site.Targets[0])
		}
		targets[target.Name] = true
		if site.Execution != CallExecutionSynchronous || site.Source.Line == 0 {
			t.Fatalf("incomplete call-site metadata: %+v", site)
		}
	}
	if !dispatches[CallDispatchDirect] || !dispatches[CallDispatchStaticMethod] {
		t.Fatalf("dispatches = %+v, want direct and static method", dispatches)
	}
	if !targets["helper"] || !targets["Counter.read"] {
		t.Fatalf("targets = %+v, want helper and Counter.read", targets)
	}
}

func TestCallGraphIncomingGroupsCallSitesByTarget(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `
module main

fn helper() int {
	return 1
}

fn first() int {
	return helper()
}

fn second() int {
	return helper() + helper()
}
`)
	assertSemaErrors(t, errors, nil)

	graph := analyzer.CallGraph()
	var helperID CallableID
	for _, node := range graph.Nodes() {
		if node.Name == "helper" {
			helperID = node.ID
		}
	}
	if helperID == "" {
		t.Fatal("call graph does not contain helper")
	}
	if sites := graph.Incoming(helperID); len(sites) != 3 {
		t.Fatalf("incoming calls = %+v, want three call sites", sites)
	}
}

func TestCallGraphProgramRootReachabilityPrunesLiteralFalseBranch(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `
module main

fn dead() void {}

fn live() void {}

fn main() void {
	if false {
		dead()
	}
	live()
}
`)
	assertSemaErrors(t, errors, nil)

	graph := analyzer.CallGraph()
	roots := graph.Roots()
	if len(roots) != 1 || roots[0].Kind != CallRootProgramEntry {
		t.Fatalf("roots = %+v, want main program-entry root", roots)
	}
	mainID := callGraphNodeIDByName(t, graph, "main")
	liveID := callGraphNodeIDByName(t, graph, "live")
	deadID := callGraphNodeIDByName(t, graph, "dead")
	reachable := map[CallableID]bool{}
	for _, node := range graph.ReachableFrom(roots[0].ID) {
		reachable[node.ID] = true
	}
	if !reachable[mainID] || !reachable[liveID] || reachable[deadID] {
		t.Fatalf("reachable = %+v, want main/live but not dead", reachable)
	}
	if roots := graph.RootsReaching(deadID); len(roots) != 0 {
		t.Fatalf("dead roots = %+v, want none", roots)
	}
	if sites := graph.Outgoing(mainID); len(sites) != 1 || sites[0].Targets[0] != liveID {
		t.Fatalf("main outgoing = %+v, want only live", sites)
	}
}

func TestCallGraphDetectsSameStackRecursionComponents(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `
module main

fn alpha() void {
	beta()
}

fn beta() void {
	alpha()
}

fn solo() void {
	solo()
}

fn main() void {
	alpha()
}
`)
	assertSemaErrors(t, errors, nil)

	graph := analyzer.CallGraph()
	alphaID := callGraphNodeIDByName(t, graph, "alpha")
	betaID := callGraphNodeIDByName(t, graph, "beta")
	soloID := callGraphNodeIDByName(t, graph, "solo")
	mainID := callGraphNodeIDByName(t, graph, "main")
	component := graph.SameStackSCC(alphaID)
	if len(component) != 2 {
		t.Fatalf("alpha SCC = %+v, want alpha and beta", component)
	}
	if !graph.IsSameStackRecursive(alphaID) || !graph.IsSameStackRecursive(betaID) || !graph.IsSameStackRecursive(soloID) {
		t.Fatalf("expected alpha, beta, and solo to be recursive")
	}
	if graph.IsSameStackRecursive(mainID) {
		t.Fatal("main must not be recursive")
	}
	if roots := graph.RootsReaching(soloID); len(roots) != 0 {
		t.Fatalf("uninvoked solo roots = %+v, want none", roots)
	}
}

func TestCallGraphSeparatesSpawnExecutionAndCreatesDerivedRoots(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `
module main

fn Build() int {
	return 1
}

fn TaskWork(value: int) void {}

fn ThreadWork() void {}

fn main() void {
	let taskHandle := spawn task TaskWork(Build())
	let threadHandle := spawn thread ThreadWork()
	detach taskHandle
	detach threadHandle
}
`)
	assertSemaErrors(t, errors, nil)

	graph := analyzer.CallGraph()
	mainID := callGraphNodeIDByName(t, graph, "main")
	taskID := callGraphNodeIDByName(t, graph, "TaskWork")
	threadID := callGraphNodeIDByName(t, graph, "ThreadWork")
	buildID := callGraphNodeIDByName(t, graph, "Build")

	executions := map[CallableID]CallExecutionRelation{}
	for _, site := range graph.Outgoing(mainID) {
		if len(site.Targets) == 1 {
			executions[site.Targets[0]] = site.Execution
		}
	}
	if executions[taskID] != CallExecutionSpawnTask || executions[threadID] != CallExecutionSpawnThread || executions[buildID] != CallExecutionSynchronous {
		t.Fatalf("execution relations = %+v", executions)
	}

	roots := graph.Roots()
	rootKinds := map[CallRootKind]int{}
	for _, root := range roots {
		rootKinds[root.Kind]++
	}
	if rootKinds[CallRootProgramEntry] != 1 || rootKinds[CallRootTaskEntry] != 1 || rootKinds[CallRootThreadEntry] != 1 {
		t.Fatalf("root kinds = %+v, want program, task, and thread entry", rootKinds)
	}
	if roots := graph.RootsReaching(taskID); len(roots) != 2 {
		t.Fatalf("TaskWork roots = %+v, want program and task entry", roots)
	}
	if roots := graph.RootsReaching(buildID); len(roots) != 1 || roots[0].Kind != CallRootProgramEntry {
		t.Fatalf("Build roots = %+v, want only program entry", roots)
	}
	if graph.IsSameStackRecursive(mainID) || len(graph.SameStackSCC(mainID)) != 1 {
		t.Fatal("spawn targets must not join main's same-stack SCC")
	}
}

func TestCallGraphSpawnCycleIsNotSameStackRecursion(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `
module main

fn A() void {
	let child := spawn B()
	detach child
}

fn B() void {
	let child := spawn A()
	detach child
}

fn main() void {
	let child := spawn A()
	detach child
}
`)
	assertSemaErrors(t, errors, nil)

	graph := analyzer.CallGraph()
	aID := callGraphNodeIDByName(t, graph, "A")
	bID := callGraphNodeIDByName(t, graph, "B")
	if graph.IsSameStackRecursive(aID) || graph.IsSameStackRecursive(bID) {
		t.Fatal("task-spawn cycle must not be reported as same-stack recursion")
	}
	for _, id := range []CallableID{aID, bID} {
		for _, site := range graph.Outgoing(id) {
			if site.Execution != CallExecutionSpawnTask {
				t.Fatalf("spawn-cycle site = %+v, want spawn-task", site)
			}
		}
	}
}

func TestCallGraphDoesNotRecordUnsupportedProcessSpawn(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `
module main

fn Work() void {}

fn main() void {
	let child := spawn process Work()
}
`)
	if len(errors) == 0 || !strings.Contains(errors[0].Message, "spawn process is not implemented yet") {
		t.Fatalf("errors = %+v, want unsupported process-spawn diagnostic", errors)
	}
	graph := analyzer.CallGraph()
	mainID := callGraphNodeIDByName(t, graph, "main")
	if sites := graph.Outgoing(mainID); len(sites) != 0 {
		t.Fatalf("unsupported process spawn produced graph sites: %+v", sites)
	}
}

func callGraphNodeIDByName(t *testing.T, graph *CallGraph, name string) CallableID {
	t.Helper()
	for _, node := range graph.Nodes() {
		if node.Name == name {
			return node.ID
		}
	}
	t.Fatalf("call graph does not contain %s", name)
	return ""
}

func TestModuleDeclarationIsRequired(t *testing.T) {
	errors := analyzeSourceRaw(t, `
let a := 1
`)

	expected := []string{
		"missing module declaration",
	}

	assertSemaErrors(t, errors, expected)

	if len(errors) != 1 || errors[0].ID != diagnostics.MissingModuleDeclaration {
		t.Fatalf("wrong diagnostic ID. got=%q want=%q", errors[0].ID, diagnostics.MissingModuleDeclaration)
	}
}

func TestDuplicateModuleDeclaration(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

module main
`)

	expected := []string{
		"duplicate module declaration main at 4:1, previous declaration at 2:1",
	}

	assertSemaErrors(t, errors, expected)

	if len(errors) != 1 || errors[0].ID != diagnostics.DuplicateModuleDeclaration {
		t.Fatalf("wrong diagnostic ID. got=%q want=%q", errors[0].ID, diagnostics.DuplicateModuleDeclaration)
	}
}

func TestModuleDeclarationNamespaceConflicts(t *testing.T) {
	input := `
module main

type User struct {
	id: int,
}

fn User() int {
	return 1
}

fn Format(value: int) string {
	return "int"
}

fn Format(value: string) string {
	return value
}

fn Decode() int {
	return 1
}

type Decode int

type Packet int
type Packet string

type Storage int
let Storage: int := 1

fn Limit() int {
	return 1
}
let Limit: int := 2
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"function User conflicts with type User declared here at 8:4, previous declaration at 4:6",
		"type Decode conflicts with function Decode declared here at 24:6, previous declaration at 20:4",
		"duplicate declaration Packet in module main at 27:6, previous declaration at 26:6",
		"variable Storage conflicts with type Storage declared here at 30:5, previous declaration at 29:6",
		"variable Limit conflicts with function Limit declared here at 35:5, previous declaration at 32:4",
	}
	assertSemaErrors(t, errors, expected)

	for _, err := range errors {
		if err.ID != diagnostics.ModuleDeclarationConflict {
			t.Fatalf("wrong diagnostic ID for %q. got=%q want=%q", err.Message, err.ID, diagnostics.ModuleDeclarationConflict)
		}
	}
}

func TestAnalyzeIgnoresTypedNilTopLevelStatements(t *testing.T) {
	program := &ast.Program{
		Statements: []ast.Statement{
			&ast.ModuleStatement{
				Token: lexer.Token{Type: lexer.MODULE, Lexeme: "module", Line: 1, Column: 1},
				Path:  "main",
			},
			(*ast.ModuleStatement)(nil),
			(*ast.TypeDeclStatement)(nil),
			(*ast.UnitDeclStatement)(nil),
			(*ast.EnumDeclaration)(nil),
			(*ast.InterfaceDeclaration)(nil),
			(*ast.FunctionDeclaration)(nil),
			(*ast.StructStatement)(nil),
			(*ast.LetStatement)(nil),
			(*ast.LetGroupStatement)(nil),
			(*ast.ImportStatement)(nil),
			(*ast.AssignmentStatement)(nil),
			(*ast.ExpressionStatement)(nil),
			(*ast.ReturnStatement)(nil),
			(*ast.IfStatement)(nil),
			(*ast.ForStatement)(nil),
			(*ast.WhileStatement)(nil),
			(*ast.SwitchStatement)(nil),
			(*ast.SelectStatement)(nil),
			(*ast.UnsafeStatement)(nil),
			(*ast.MatchStatement)(nil),
		},
	}

	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) > 0 {
		t.Fatalf("Analyze returned errors for typed nil statements: %v", errors)
	}
}

func TestUnderscoreVisibilityAcrossModules(t *testing.T) {
	input := `
module y

type _SharedInt int
type __PrivateInt int

fn _shared() int {
	return 1
}

fn __private() int {
	return 2
}

module z

fn UseShared() int {
	let value: _SharedInt := 1
	return _shared() + value
}

fn UsePrivateFunction() int {
	return __private()
}

fn UsePrivateType() int {
	let value: __PrivateInt := 1
	return 0
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"type _SharedInt is not accessible from module z at 18:13",
		"function _shared is not accessible from module z at 19:9",
		"function __private is not accessible from module z at 23:9",
		"type __PrivateInt is not accessible from module z at 27:13",
	}

	assertSemaErrors(t, errors, expected)
}

func TestDoubleUnderscoreIsVisibleInExactModule(t *testing.T) {
	input := `
module y

type __PrivateInt int

fn __private() int {
	return 2
}

fn UsePrivate() int {
	let value: __PrivateInt := 1
	return __private() + value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestExecutableCodeIsNotAllowedAtModuleScope(t *testing.T) {
	input := `
module main

let mut i := 0
i += 1
return i
`

	errors := analyzeSource(t, input)

	expected := []string{
		"assignment is not allowed at module scope at 5:1",
		"return is not allowed at module scope at 6:1",
	}

	assertSemaErrors(t, errors, expected)
}

func TestImmutableTypedLetWithoutInitializer(t *testing.T) {
	input := `
module main

fn Test() void {
	let a: int
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"immutable variable a requires initializer at 5:6",
	}
	assertSemaErrors(t, errors, expected)
}

func TestLetInitializerTypeMismatches(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "int from bool",
			input:    "let i: int := true",
			expected: "cannot initialize int with bool at 1:15",
		},
		{
			name:     "bool from int",
			input:    "let b: bool := 1",
			expected: "cannot initialize bool with int at 1:16",
		},
		{
			name:     "bool from string",
			input:    `let b: bool := "hello"`,
			expected: "cannot initialize bool with string at 1:16",
		},
		{
			name:     "string from int",
			input:    "let s: string := 42",
			expected: "cannot initialize string with int at 1:18",
		},
		{
			name:     "int from float",
			input:    "let i: int := 3.14",
			expected: "cannot initialize int with decimal at 1:15",
		},
		{
			name:     "float from bool",
			input:    "let f: float := true",
			expected: "cannot initialize float with bool at 1:17",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := analyzeSource(t, tt.input)
			assertSemaErrors(t, errors, []string{tt.expected})
		})
	}
}

func TestDecimalInitializersAndAssignments(t *testing.T) {
	input := `
let pi: decimal := 3.141592
let neg: decimal := -0.5
let mut p: decimal := 1
fn Test() void {
	p += .1
	p += 0.1
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestDecimalDoesNotImplicitlyAcceptFloatVariables(t *testing.T) {
	input := `
let f: float64 := 3.14
let d: decimal := f
let mut p: decimal := 1
fn Test() void {
	p += f
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot initialize decimal with float64 at 3:19",
		"cannot add float64 to decimal at 6:7",
	}

	assertSemaErrors(t, errors, expected)
}

func TestDecimalLiteralInfersDecimalByDefault(t *testing.T) {
	input := `
let x := 3.14
let f: float64 := 3.14
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	x := analyzer.symbols["x"]
	if x.Type.Name != "decimal" {
		t.Fatalf("wrong inferred type. got=%q want=%q", x.Type.Name, "decimal")
	}

	f := analyzer.symbols["f"]
	if f.Type.Name != "float64" {
		t.Fatalf("wrong explicit type. got=%q want=%q", f.Type.Name, "float64")
	}
}

func TestScientificExponentLiteralInference(t *testing.T) {
	input := `
let exact := 1.25e-3
let exactUpper := .5E+4
let floating := 1e3g
let decimal := 1e3m
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	expected := map[string]string{
		"exact":      "decimal",
		"exactUpper": "decimal",
		"floating":   "float",
		"decimal":    "decimal",
	}
	for name, want := range expected {
		if got := analyzer.symbols[name].Type.Name; got != want {
			t.Fatalf("%s inferred as %q, want %q", name, got, want)
		}
	}
}

func TestNumericLiteralSuffixesAndBases(t *testing.T) {
	input := `
let i := 10i
let u := 10u
let f := 10g
let d := 10m
let hf := 1.5g
let hd := 1.5m
let b := 0b1000
let o := 0o10
let x := 0x8u
let h := 0x8m
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	expected := map[string]string{
		"i":  "int",
		"u":  "uint",
		"f":  "float",
		"d":  "decimal",
		"hf": "float",
		"hd": "decimal",
		"b":  "int",
		"o":  "int",
		"x":  "uint",
		"h":  "decimal",
	}
	for name, want := range expected {
		if got := analyzer.symbols[name].Type.Name; got != want {
			t.Fatalf("%s inferred as %q, want %q", name, got, want)
		}
	}
}

func TestDecimalLiteralValueUsesLexeme(t *testing.T) {
	tests := []struct {
		input string
		want  DecimalValue
	}{
		{input: "3.14", want: DecimalValue{Int64: 314, Scale: 2}},
		{input: ".1", want: DecimalValue{Int64: 1, Scale: 1}},
		{input: "-0.5", want: DecimalValue{Int64: -5, Scale: 1}},
		{input: "100", want: DecimalValue{Int64: 100, Scale: 0}},
		{input: "1.25e-3", want: DecimalValue{Int64: 125, Scale: 5}},
		{input: "1.5e2", want: DecimalValue{Int64: 150, Scale: 0}},
		{input: ".5E+4", want: DecimalValue{Int64: 5000, Scale: 0}},
		{input: "1_234.5_678", want: DecimalValue{Int64: 12345678, Scale: 4}},
		{input: "1.25e1_0", want: DecimalValue{Int64: 12500000000, Scale: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			expr := parseExpressionSource(t, tt.input)
			got, ok := decimalLiteralValue(expr)
			if !ok {
				t.Fatalf("expected decimal literal value for %q", tt.input)
			}
			if got != tt.want {
				t.Fatalf("wrong decimal value. got=%+v want=%+v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeAssignmentsAndRedeclarations(t *testing.T) {
	input := `
module main

let mut a: uint := 5
let u: uint := 6
fn Test() void {
	a = u - 6
	let u: uint := 7
	u = 1
	missing = 1
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"variable \"u\" already declared at 8:6, previous declaration at 5:5",
		"cannot assign to immutable variable u at 9:2",
		"undefined variable missing at 10:2",
	}

	assertSemaErrors(t, errors, expected)
}

func TestAnalyzeLetGroups(t *testing.T) {
	input := `
int mut: a, b, c
float: f := 5.4, pi := 3.14
let x := 9, s := "hello", ok := true
let mut ma := 9, ms := "hello", mok := false
fn Test() void {
	a = x
	ma = 10
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	for _, name := range []string{"a", "b", "c", "ma", "ms", "mok"} {
		if !analyzer.symbols[name].Mutable {
			t.Fatalf("%s should be mutable", name)
		}
	}

	for _, name := range []string{"f", "pi", "x", "s", "ok"} {
		if analyzer.symbols[name].Mutable {
			t.Fatalf("%s should be immutable", name)
		}
	}

	if analyzer.symbols["f"].Type.Name != "float" {
		t.Fatalf("wrong f type. got=%q want=float", analyzer.symbols["f"].Type.Name)
	}
	if analyzer.symbols["pi"].Type.Name != "float" {
		t.Fatalf("wrong pi type. got=%q want=float", analyzer.symbols["pi"].Type.Name)
	}
}

func TestAnalyzeTypedDeclarationGroup(t *testing.T) {
	input := `
type TokenType string

TokenType (
	ILLEGAL := "ILLEGAL",
	EOF := "EOF",
	IDENT := "IDENT",
)

fn Test() TokenType {
	return IDENT
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	for _, name := range []string{"ILLEGAL", "EOF", "IDENT"} {
		symbol := analyzer.symbols[name]
		if symbol.Mutable {
			t.Fatalf("%s should be immutable", name)
		}
		if symbol.Type.Name != "TokenType" {
			t.Fatalf("%s type = %q, want TokenType", name, symbol.Type.Name)
		}
	}
}

func TestNamedIntegerRangeChecksConstantExpressions(t *testing.T) {
	input := `
type Percent int range 0..100

let mut p: Percent := 50
fn Test() void {
	try p = 100 { Err(error) => { discard error } }
	try p = 101 { Err(error) => { discard error } }
	try p = 10 * 10 { Err(error) => { discard error } }
	try p = 50 + 51 { Err(error) => { discard error } }
	try p = 50 { Err(error) => { discard error } }
	try p += 20 { Err(error) => { discard error } }
	try p += 60 { Err(error) => { discard error } }
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"value 101 violates range contract Percent 0..100 at 7:10",
		"value 101 violates range contract Percent 0..100 at 9:13",
		"value 130 violates range contract Percent 0..100 at 12:11",
	}

	assertSemaErrors(t, errors, expected)
}

func TestContractedVariableAssignmentRequiresTry(t *testing.T) {
	input := `
type Percent int range 0..100

let mut p: Percent := 50
fn Test() void {
	p = 60
	p += 1
	try p = 70 { Err(error) => { discard error } }
	try p += 1 { Err(error) => { discard error } }
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"assigning variable p requires try because Percent has contracts at 6:2",
		"assigning variable p requires try because Percent has contracts at 7:2",
	}

	assertSemaErrors(t, errors, expected)
}

func TestNamedTypeRegistryRangeContractAndInitialization(t *testing.T) {
	input := `
type Percent int range 0..100

let p1: Percent := 90
let p2: Percent := 101
let x := 90
let p3: Percent := x
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)

	expected := []string{
		"value 101 violates range contract Percent 0..100 at 5:20",
		"cannot initialize Percent with int at 7:20",
	}

	assertSemaErrors(t, errors, expected)

	percent := analyzer.types["Percent"]
	if !percent.Named {
		t.Fatal("Percent should be registered as named type")
	}
	if percent.Underlying != "int" {
		t.Fatalf("wrong underlying type. got=%q want=%q", percent.Underlying, "int")
	}
}

func TestContractTypeRequiresExplicitConversionFromVariable(t *testing.T) {
	input := `
type Percent int range 0..100

let _a: int := 90
let _tooMuch: int := 101
let mut precent: Percent := 0
fn Test() void {
	try precent += 50 { Err(error) => { discard error } }
	try precent += _a { Err(error) => { discard error } }
	try precent = Percent(_a) { Err(error) => { discard error } }
	try precent = Percent(_tooMuch) { Err(error) => { discard error } }
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot add int to Percent at 9:17",
		"value 101 violates range contract Percent 0..100 at 11:16",
	}

	assertSemaErrors(t, errors, expected)
}

func TestNamedRangeTypeStoresContract(t *testing.T) {
	input := `
module main

type Percent int range 0..100
`

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	analyzer := NewAnalyzer()
	errors := analyzer.Analyze(program)
	assertSemaErrors(t, errors, nil)

	percent := analyzer.types["Percent"]
	if !percent.Named {
		t.Fatal("Percent should be named")
	}
	if percent.Underlying != "int" {
		t.Fatalf("wrong underlying type. got=%q want=%q", percent.Underlying, "int")
	}
	if len(percent.Contracts) != 1 {
		t.Fatalf("wrong contract count. got=%d want=1", len(percent.Contracts))
	}

	contract, ok := percent.Contracts[0].(RangeContract)
	if !ok {
		t.Fatalf("contract is not RangeContract. got=%T", percent.Contracts[0])
	}

	if contract.Min.String() != "0" || contract.Max.String() != "100" {
		t.Fatalf("wrong range contract. got=%s..%s want=0..100", contract.Min, contract.Max)
	}
}

func TestVariableContractFormsOnNamedTypes(t *testing.T) {
	input := `
type PageSize int range 10..100 multipleOf 10
type OddNumber int odd
type EvenNumber int even
type Role string in ["admin", "user", "guest"]
type Tags string[] notEmpty unique
type Measurement float finite
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	if len(analyzer.types["PageSize"].Contracts) != 2 {
		t.Fatalf("PageSize should have two contracts, got %d", len(analyzer.types["PageSize"].Contracts))
	}
	if len(analyzer.types["OddNumber"].Contracts) != 1 {
		t.Fatalf("OddNumber should have one contract, got %d", len(analyzer.types["OddNumber"].Contracts))
	}
	if len(analyzer.types["EvenNumber"].Contracts) != 1 {
		t.Fatalf("EvenNumber should have one contract, got %d", len(analyzer.types["EvenNumber"].Contracts))
	}
	if len(analyzer.types["Role"].Contracts) != 1 {
		t.Fatalf("Role should have one contract, got %d", len(analyzer.types["Role"].Contracts))
	}
	if len(analyzer.types["Tags"].Contracts) != 2 {
		t.Fatalf("Tags should have two contracts, got %d", len(analyzer.types["Tags"].Contracts))
	}
	if len(analyzer.types["Measurement"].Contracts) != 1 {
		t.Fatalf("Measurement should have one contract, got %d", len(analyzer.types["Measurement"].Contracts))
	}
}

func TestVariableContractApplicabilityErrors(t *testing.T) {
	input := `
type BadUniqueString string unique
type BadUniqueInt int unique
type BadNotEmptyInt int notEmpty
type BadFiniteInt int finite
type BadMultipleString string multipleOf 2
type BadRangeString string range 1..10
type BadOddString string odd
type BadEvenFloat float even
`

	errors := analyzeSource(t, input)
	expected := []string{
		"unique contract does not apply to string at 2:29",
		"unique contract does not apply to int at 3:23",
		"notEmpty contract does not apply to int at 4:25",
		"finite contract does not apply to int at 5:23",
		"multipleOf contract does not apply to string at 6:31",
		"range contract does not apply to string at 7:28",
		"odd contract does not apply to string at 8:26",
		"even contract does not apply to float at 9:25",
	}
	assertSemaErrors(t, errors, expected)
}

func TestIntegerMarkerContractsCheckConstantInitializers(t *testing.T) {
	input := `
type PageSize int range 10..100 multipleOf 10
type OddNumber int odd
type EvenNumber int even

let ok: PageSize := 20
let bad: PageSize := 25
let oddBad: OddNumber := 10
let evenBad: EvenNumber := 11
`

	errors := analyzeSource(t, input)
	expected := []string{
		"value 25 violates multipleOf contract PageSize 10 at 7:22",
		"value 10 violates odd contract OddNumber at 8:26",
		"value 11 violates even contract EvenNumber at 9:28",
	}
	assertSemaErrors(t, errors, expected)
}

func TestIntegerContractSetConsistency(t *testing.T) {
	input := `
type OddEven int odd even
type OddTens int range 10..100 multipleOf 10 odd
type NoEven int range 1..<2 even
type NoMultiple int range 11..19 multipleOf 10
type ValidOddMultiple int range 9..21 multipleOf 3 odd
type ValidEvenMultiple int range 10..20 multipleOf 10 even
`

	errors := analyzeSource(t, input)
	expected := []string{
		"contracts odd and even cannot be combined at 2:22",
		"contracts multipleOf 10 and odd cannot be combined because every multiple is even at 3:46",
		"contracts cannot be satisfied together for NoEven at 4:29",
		"contracts cannot be satisfied together for NoMultiple at 5:34",
	}
	assertSemaErrors(t, errors, expected)
}

// rules/types/contracts.md; correction3.md combines every same-family integer
// contract using range intersection and divisor LCM.
func TestCombinedIntegerContractSetConsistency(t *testing.T) {
	errors := analyzeSource(t, `
type Disjoint int range 1..4 range 8..12
type NoLCM int range 1..10 multipleOf 4 multipleOf 6
type ValidLCM int range 1..20 multipleOf 4 multipleOf 6
`)
	if len(errors) != 2 || !errorsContainMessage(errors, "contracts cannot be satisfied together for Disjoint") || !errorsContainMessage(errors, "contracts cannot be satisfied together for NoLCM") {
		t.Fatalf("combined contract errors = %v", errors)
	}
}

// rules/types/contracts.md and rules/types/default_values.md; correction4.md
// revalidates inherited membership against contracts introduced by derivation.
func TestInheritedMembershipIsRevalidated(t *testing.T) {
	errors := analyzeSource(t, `
type Choices int in [1, 2]
type Invalid Choices even
type Compatible Choices range 1..3
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "membership value 1 violates another contract on Invalid") {
		t.Fatalf("inherited membership errors = %v", errors)
	}
}

func TestVariableLevelContractRequiresTryOnLaterAssignment(t *testing.T) {
	input := `
let mut percentage: int range 0..100 := 50

fn Test() void {
	percentage = 60
	try percentage = 70 {
		Err(error) => {
			discard error
		}
	}
}
`

	errors := analyzeSource(t, input)
	expected := []string{
		"assigning variable percentage requires try because int has contracts at 5:2",
	}
	assertSemaErrors(t, errors, expected)
}

func TestVariableLevelUniqueRejectsScalarStorage(t *testing.T) {
	input := `
let mut bad: string unique := "tag"
let mut values: int[3] unique := [1, 2, 3]
`

	errors := analyzeSource(t, input)
	expected := []string{
		"unique contract does not apply to string at 2:21",
	}
	assertSemaErrors(t, errors, expected)
}

func TestCollectionShapedTypesResolve(t *testing.T) {
	input := `
module main

fn Use(
    values: list[int, 8],
    lookup: map[string, int, 16],
    flags: set[string],
    position: vector[float64, 3],
    transform: matrix[float32, 4, 4],
    image: tensor[float32, 3, 224, 224],
    view: tensor_view[float32, 3],
    shape: Shape[3],
	strides: Strides[3],
	layout: TensorLayout[3],
	space: MemorySpace,
) void {
    return
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	fn := analyzer.functions["Use"][0]
	expected := []string{
		"list[int, 8]",
		"map[string, int, 16]",
		"set[string]",
		"vector[float64, 3]",
		"matrix[float32, 4, 4]",
		"tensor[float32, 3, 224, 224]",
		"tensor_view[float32, 3]",
		"Shape[3]",
		"Strides[3]",
		"TensorLayout[3]",
		"MemorySpace",
	}
	for i, want := range expected {
		if got := typeDisplayName(fn.Parameters[i].Type); got != want {
			t.Fatalf("parameter %d type. got=%q want=%q", i, got, want)
		}
	}
}

func TestCollectionShapedTypeValidation(t *testing.T) {
	input := `
module main

fn Invalid(
    zeroList: list[int, 0],
    badVector: vector[int],
    negativeMatrix: matrix[int, -1, 4],
    badShape: Shape[int],
	missingStridesRank: Strides,
	tooManyLayoutRanks: TensorLayout[2, 3],
	genericMemorySpace: MemorySpace[int],
) void {
    return
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"list capacity must be greater than zero at 5:15",
		"vector requires 1 compile-time integer arguments, got 0 at 6:16",
		"matrix arguments must be non-negative at 7:21",
		"Shape argument must be a compile-time integer at 8:21",
		"Strides requires 1 compile-time integer arguments, got 0 at 9:22",
		"TensorLayout requires 1 compile-time integer arguments, got 2 at 10:22",
		"MemorySpace is not generic at 11:22",
	})
}

func TestNamedUnitTypeRegistryStoresUnit(t *testing.T) {
	input := `
unit SEK currency
unit m physical
unit s physical
type Money decimal<SEK>
type Speed decimal<m/s>
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	money := analyzer.types["Money"]
	if !money.Named {
		t.Fatal("Money should be named")
	}
	if money.Underlying != "decimal" {
		t.Fatalf("wrong underlying type. got=%q want=%q", money.Underlying, "decimal")
	}
	if money.Unit != "SEK" {
		t.Fatalf("wrong unit. got=%q want=%q", money.Unit, "SEK")
	}

	speed := analyzer.types["Speed"]
	if speed.Unit != "m/s" {
		t.Fatalf("wrong unit. got=%q want=%q", speed.Unit, "m/s")
	}
}

func TestUnitDeclarationRegistersSemanticUnit(t *testing.T) {
	input := `
module main

unit Hertz physical
unit Packet uint other
type Frequency decimal<Hertz>

let hz: Hertz := 10
let f: Frequency := 10
let p: Packet := 3u
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	hertz := analyzer.types["Hertz"]
	if !hertz.Named || hertz.Kind != DecimalType || hertz.Unit != "Hertz" {
		t.Fatalf("wrong Hertz type: %+v", hertz)
	}
	if !hertz.Dimension.Equal(analyzer.types["Frequency"].Dimension) {
		t.Fatalf("Hertz declaration and use should share one unit dimension. got=%+v want=%+v", hertz.Dimension, analyzer.types["Frequency"].Dimension)
	}

	if unit := analyzer.units["Hertz"]; unit.Category != PhysicalUnit {
		t.Fatalf("Hertz should be a physical unit. got=%q", unit.Category)
	}

	packet := analyzer.types["Packet"]
	if !packet.Named || packet.Kind != UintType || packet.Unit != "Packet" {
		t.Fatalf("wrong Packet type: %+v", packet)
	}
	if unit := analyzer.units["Packet"]; unit.Category != OtherUnit {
		t.Fatalf("Packet should be an other unit. got=%q", unit.Category)
	}
	if packet.Dimension.Base["Packet"] != 1 {
		t.Fatalf("Packet should keep semantic base dimension. got=%+v", packet.Dimension)
	}
}

func TestUnitMetadataAllowsDimensionlessInlineComment(t *testing.T) {
	input := `
unit dB decimal physical

impl dB {
	dimension: [] // Dimensionless ratio
	scale: 1 // Identity scale
	system: SI // Standard unit system
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	decibel := analyzer.types["dB"]
	if !decibel.Dimension.IsZero() {
		t.Fatalf("dB should be dimensionless. got=%+v", decibel.Dimension)
	}
}

func TestUnitDefaultNumericMetadataAndUnitOnlyType(t *testing.T) {
	input := `
unit s physical
unit hp float physical

impl hp {
	LongName: "Horsepower"
	Symbol: "hp"
	BaseUnit: false
	Status: deprecated
	Dimension: [mass^1, length^2, time^-3]
	Scale: 735.49875g
	System: Imperial
}

let seconds: <s> := 5
let fast: <s> := 5g
let power: <hp> := 10g
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	secondType := analyzer.symbols["seconds"].Type
	if secondType.Name != "decimal" || secondType.Unit != "s" {
		t.Fatalf("wrong seconds type: %+v", secondType)
	}

	fastType := analyzer.symbols["fast"].Type
	if fastType.Name != "decimal" || fastType.Unit != "s" {
		t.Fatalf("wrong fast type: %+v", fastType)
	}

	powerType := analyzer.symbols["power"].Type
	if powerType.Name != "float" || powerType.Unit != "hp" {
		t.Fatalf("wrong power type: %+v", powerType)
	}

	horsepower := analyzer.units["hp"]
	if horsepower.DefaultNumeric != "float" || horsepower.Category != PhysicalUnit {
		t.Fatalf("wrong hp declaration metadata: %+v", horsepower)
	}
	if horsepower.LongName != "Horsepower" || horsepower.Symbol != "hp" || horsepower.IsBaseUnit {
		t.Fatalf("wrong hp descriptive metadata: %+v", horsepower)
	}
	if horsepower.Status != StatusDeprecated || horsepower.System != "Imperial" || horsepower.Scale != "735.49875g" {
		t.Fatalf("wrong hp semantic metadata: %+v", horsepower)
	}

	warnings := analyzer.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("wrong warning count. got=%d warnings=%v", len(warnings), warnings)
	}
	if warnings[0].Error() != "unit hp is deprecated at 17:12" {
		t.Fatalf("wrong warning. got=%q", warnings[0].Error())
	}
}

func TestUnitDeclarationRejectsNonNumericStorage(t *testing.T) {
	input := `
module main

unit Bad string
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"unit Bad default representation must be a plain compiler-known numeric scalar, got string at 4:10",
	}

	assertSemaErrors(t, errors, expected)
}

func TestUnitDeclarationRejectsNamedNumericDefaultCarrier(t *testing.T) {
	input := `
module main

type Count int
unit Item Count other
`
	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"unit Item default representation must be a plain compiler-known numeric scalar, got Count at 5:11",
	})
}

func TestRevisionTwoUnitCategoryDefaults(t *testing.T) {
	input := `
unit Percent ratio
unit Octet uint information
unit EUR currency
unit Packet
`
	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)
	if unit := analyzer.units["Percent"]; unit.Category != RatioUnit || !unit.Dimension.IsZero() {
		t.Fatalf("ratio default=%+v", unit)
	}
	if unit := analyzer.units["Octet"]; unit.Category != InformationUnit || unit.Dimension.Base["information"] != 1 {
		t.Fatalf("information default=%+v", unit)
	}
	if unit := analyzer.units["EUR"]; unit.Category != CurrencyUnit || unit.Dimension.Base["EUR"] != 1 {
		t.Fatalf("currency default=%+v", unit)
	}
	if unit := analyzer.units["Packet"]; unit.Category != OtherUnit || unit.Dimension.Base["Packet"] != 1 {
		t.Fatalf("other default=%+v", unit)
	}
	if unit := analyzer.units["Percent"]; !unit.DimensionEstablished {
		t.Fatalf("ratio dimension should be established: %+v", unit)
	}
}

// rules/types/units.md; correction6.md removes the hidden spelling-based unit
// catalog and preserves unresolved physical dimensions explicitly.
func TestUnitsRequireDeclarationsAndPhysicalDimensionMetadata(t *testing.T) {
	errors := analyzeSource(t, `type Distance decimal<m>`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "unknown unit m") {
		t.Fatalf("undeclared unit errors = %v", errors)
	}
	analyzer, errors := analyzeSourceWithAnalyzer(t, `unit Temperature physical`)
	assertSemaErrors(t, errors, nil)
	unit := analyzer.units["Temperature"]
	if unit.DimensionEstablished || !unit.Dimension.IsZero() {
		t.Fatalf("physical unit dimension = %+v, want unresolved", unit)
	}
}

func TestUnitNamesRejectCompilerKnownCollisions(t *testing.T) {
	input := `
module main

unit bit information
unit byte information
unit list other
`
	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"unit name bit is compiler-known and cannot be declared at 4:6",
		"unit name byte is compiler-known and cannot be declared at 5:6",
		"unit name list is compiler-known and cannot be declared at 6:6",
	})
}

func TestStructuralUnitExpressionsAndCarrierDefaults(t *testing.T) {
	input := `
unit widget other
unit tick other

let speed: <widget/tick> := 10
let acceleration: <widget/tick^2> := 2
let grouped: float<(widget/tick)^2> := 3g
let identity: <widget/widget> := 1
`
	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)
	if speed := analyzer.symbols["speed"].Type; speed.Kind != DecimalType || speed.Dimension.Base["widget"] != 1 || speed.Dimension.Base["tick"] != -1 {
		t.Fatalf("speed type=%+v", speed)
	}
	if acceleration := analyzer.symbols["acceleration"].Type; acceleration.Dimension.Base["tick"] != -2 {
		t.Fatalf("acceleration type=%+v", acceleration)
	}
	if grouped := analyzer.symbols["grouped"].Type; grouped.Kind != FloatType || grouped.Dimension.Base["widget"] != 2 || grouped.Dimension.Base["tick"] != -2 {
		t.Fatalf("grouped type=%+v", grouped)
	}
	if identity := analyzer.symbols["identity"].Type; identity.Kind != DecimalType || !identity.Dimension.IsZero() {
		t.Fatalf("identity type=%+v", identity)
	}
}

func TestUnitMetadataKindAndTransforms(t *testing.T) {
	input := `
unit Celsius physical
impl Celsius {
	Kind: temperature
	Scale: 1
	Transform: affine
	Offset: 273.15
	Origin: absolute_zero
}

unit Decibel ratio
impl Decibel {
	Kind: ratio
	Transform: logarithmic
	LogBase: 10
	LogFactor: 10
	Reference: 1
}
`
	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)
	celsius := analyzer.units["Celsius"]
	if celsius.Kind != "temperature" || celsius.Transform != AffineUnitTransform || celsius.Offset != "273.15" || celsius.Origin != "absolute_zero" {
		t.Fatalf("Celsius metadata=%+v", celsius)
	}
	decibel := analyzer.units["Decibel"]
	if decibel.Kind != "ratio" || decibel.Transform != LogarithmicUnitTransform || decibel.LogBase != "10" || decibel.LogFactor != "10" || decibel.Reference != "1" {
		t.Fatalf("Decibel metadata=%+v", decibel)
	}
}

func TestUnitMetadataRejectsIncompleteTransforms(t *testing.T) {
	input := `
module main

unit BadAffine physical
impl BadAffine {
	Transform: affine
}
unit BadLog ratio
impl BadLog {
	Transform: logarithmic
	LogBase: 10
}
`
	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"affine unit BadAffine requires Scale, Offset, and Origin metadata at 5:6",
		"logarithmic unit BadLog requires LogBase, LogFactor, and Reference metadata at 9:6",
	})
}

func TestUnitConversionRequiresCompatibleDimensions(t *testing.T) {
	input := `
module main

unit Distance decimal physical
impl Distance {
	dimension: [length^1]
	scale: 1
	system: SI

	fn Distance(value: Duration) Distance {
		return 0
	}
}

unit Duration decimal physical
impl Duration {
	dimension: [time^1]
	scale: 1
	system: SI
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	expected := []string{
		"unit conversion Distance from Duration requires compatible dimensions at 10:21",
	}
	assertSemaErrors(t, errors, expected)
	if errors[0].ID != diagnostics.IncompatibleUnitConversion {
		t.Fatalf("wrong diagnostic ID. got=%q want=%q", errors[0].ID, diagnostics.IncompatibleUnitConversion)
	}
	if errors[0].Help == "" {
		t.Fatal("expected unit conversion diagnostic help")
	}
	if _, exists := analyzer.functions["Distance"]; exists {
		t.Fatal("incompatible unit conversion must not be registered as a constructor")
	}
}

func TestIntegerBackedUnitConversionUsesDimensionValidation(t *testing.T) {
	input := `
module main

unit Packet uint other
impl Packet {
	dimension: [packet^1]
	scale: 1
	system: Domain
}

unit Batch uint other
impl Batch {
	dimension: [packet^1]
	scale: 10
	system: Domain

	fn Batch(value: Packet) Batch {
		return 0
	}
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)
	if len(analyzer.functions["Batch"]) != 1 {
		t.Fatalf("integer-backed unit conversion was not registered: %+v", analyzer.functions["Batch"])
	}
}

func TestUnitsValidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/units_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestUnitsAdvancedValidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/units_advanced_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestInterfaceValidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/interface_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestInterfaceInvalidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/interface_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	if len(errors) == 0 {
		t.Fatal("expected interface_invalid.sec to produce semantic errors")
	}
}

func TestFunctionsValidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/functions_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestFunctionsInvalidSemanticPrefixFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/functions_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	source := semanticPrefixBeforeMarker(t, string(input), "// Parser recovery cases")
	errors := analyzeSourceRaw(t, source)
	if len(errors) == 0 {
		t.Fatal("expected functions_invalid.sec semantic prefix to produce errors")
	}
}

func TestLambdaValidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/lambda_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestLambdaInvalidSemanticPrefixFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/lambda_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	source := semanticPrefixBeforeMarker(t, string(input), "// Parser and explicitly postponed syntax")
	errors := analyzeSourceRaw(t, source)
	if len(errors) == 0 {
		t.Fatal("expected lambda_invalid.sec semantic prefix to produce errors")
	}
}

func TestLambdaEscapingSpecFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/lambda_escaping_spec.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	expected := []string{
		"escaping captured lambda is not supported yet at 11:28",
		"escaping captured lambda is not supported yet at 17:38",
		"escaping captured lambda is not supported yet at 24:38",
	}
	assertSemaErrors(t, errors, expected)
}

func TestUnitsInvalidSemanticFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/units_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	const marker = "// Invalid direct unit declaration syntax and parser recovery"
	source := string(input)
	idx := strings.Index(source, marker)
	if idx < 0 {
		t.Fatalf("missing parser recovery marker %q", marker)
	}

	errors := analyzeSourceRaw(t, source[:idx])
	if len(errors) == 0 {
		t.Fatal("expected semantic unit errors")
	}
}

func semanticPrefixBeforeMarker(t *testing.T, source string, marker string) string {
	t.Helper()
	idx := strings.Index(source, marker)
	if idx < 0 {
		t.Fatalf("missing marker %q", marker)
	}
	return source[:idx]
}

func TestRegisterDeclarationAndAddressedInstance(t *testing.T) {
	input := `
module main

unit rpm uint physical

type MotorProtocol register[8] {
	Speed: bit[4]<rpm>,
	Enabled: bit,
	_: bit[3],
}

@address(0x40021000)
let mut motorProtocol: MotorProtocol

fn StartMotor(speed: rpm) void {
	motorProtocol.Speed = speed
	motorProtocol.Enabled = true
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	register := analyzer.types["MotorProtocol"]
	if register.Kind != RegisterType || register.RegisterWidth != 8 {
		t.Fatalf("wrong register type: %+v", register)
	}
	if len(register.RegisterFields) != 3 || register.RegisterFields[0].Name != "Speed" || register.RegisterFields[0].Width != 4 {
		t.Fatalf("wrong register fields: %+v", register.RegisterFields)
	}
	symbol := analyzer.symbols["motorProtocol"]
	if !symbol.Addressed || !symbol.Volatile || symbol.Address != "0x40021000" {
		t.Fatalf("wrong addressed symbol: %+v", symbol)
	}
}

func TestRegisterAllocationOrderProducesDeterministicBitOffsets(t *testing.T) {
	input := `
module main

type Low register[8] {
	First: bit[2],
	Second: bit[3],
	_: bit[3],
}

type High register[8] msb-first big-endian {
	First: bit[2],
	Second: bit[3],
	_: bit[3],
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	low := analyzer.types["Low"]
	if low.RegisterAllocationOrder != "lsb-first" || low.RegisterByteOrder != "" {
		t.Fatalf("wrong Low layout metadata: %+v", low)
	}
	if low.RegisterFields[0].BitOffset != 0 || low.RegisterFields[1].BitOffset != 2 || low.RegisterFields[2].BitOffset != 5 {
		t.Fatalf("wrong lsb-first offsets: %+v", low.RegisterFields)
	}

	high := analyzer.types["High"]
	if high.RegisterAllocationOrder != "msb-first" || high.RegisterByteOrder != "big-endian" {
		t.Fatalf("wrong High layout metadata: %+v", high)
	}
	if high.RegisterFields[0].BitOffset != 6 || high.RegisterFields[1].BitOffset != 3 || high.RegisterFields[2].BitOffset != 0 {
		t.Fatalf("wrong msb-first offsets: %+v", high.RegisterFields)
	}
}

// rules/declarations/registers.md, sections 3, 10, and 11.6. Nested nominal
// fields are source-order independent and retain their own access contracts.
func TestNestedRegisterFieldsAndCompileTimeWidths(t *testing.T) {
	input := `
module main

let QUARTER := 1 << 2

type StatusWord register[QUARTER * 4] {
	Flags: Flags,
	Mode: bit[QUARTER],
	_: bit[QUARTER * 2],
}

type Flags register[QUARTER] msb-first {
	Ready: bit read-only,
	Command: bit write-only,
	_: bit[2],
}

type Locked register[QUARTER] {
	Flags: Flags read-only,
}

fn Update(status: ref mut StatusWord, locked: ref mut Locked, replacement: Flags) void {
	status.Flags.Ready = true
	status.Flags = replacement
	locked.Flags.Command = true
}
`
	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, []string{
		"register field Ready is read-only and cannot be written at 23:15",
		"cannot write whole nested register field Flags because contained field Flags.Ready has read-only semantics at 24:9",
		"cannot write through nested register field Flags with read-only semantics at 25:15",
	})
	status := analyzer.types["StatusWord"]
	if status.RegisterWidth != 16 || len(status.RegisterFields) != 3 {
		t.Fatalf("StatusWord = %+v", status)
	}
	flags := status.RegisterFields[0]
	if flags.Width != 4 || flags.Type.Name != "Flags" || flags.Type.Kind != RegisterType || len(flags.Type.RegisterFields) != 3 {
		t.Fatalf("nested Flags field = %+v", flags)
	}
	if ready := flags.Type.RegisterFields[0]; ready.Access != RegisterReadOnly || ready.BitOffset != 3 {
		t.Fatalf("nested Ready field = %+v", ready)
	}
}

func TestNestedRegisterRejectsRecursiveLayout(t *testing.T) {
	input := `
module main

type First register[8] { Next: Second }
type Second register[8] { Next: First }
`
	errors := analyzeSourceRaw(t, input)
	found := false
	for _, diagnostic := range errors {
		if strings.Contains(diagnostic.Message, "recursive or infinitely sized nested layout") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("errors = %v; want recursive nested-register diagnostic", errors)
	}
}

func TestRegisterWidthsRejectRuntimeAndNonPositiveExpressions(t *testing.T) {
	input := `
module main

fn RuntimeWidth() int { return 8 }
type Dynamic register[RuntimeWidth()] { Value: bit[8] }
type Zero register[2 - 2] { Value: bit }
`
	errors := analyzeSourceRaw(t, input)
	wanted := []string{
		"register Dynamic width must be a compile-time integer",
		"register Zero width must be positive",
	}
	for _, fragment := range wanted {
		found := false
		for _, diagnostic := range errors {
			if strings.Contains(diagnostic.Message, fragment) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("errors = %v; missing %q", errors, fragment)
		}
	}
}

// rules/declarations/registers.md, section 12. Integer conversion is direct
// only for constants or complete source domains proven to fit.
func TestCheckedIntegerToRegisterConversion(t *testing.T) {
	input := `
module main

type Packet12 register[12] { Value: bit[12] }

fn Exact(raw: uint8) Packet12 { return Packet12(raw) }
fn Checked(raw: uint16) Result[Packet12, ArithmeticError] { return Ok(try Packet12(raw)) }
fn Handled(raw: uint16) Result[Packet12, ArithmeticError] { return Ok(try Packet12(raw)) }

let valid := Packet12(0xFFF)
let invalid := Packet12(0x1000)
`
	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"integer value 4096 does not fit register Packet12 width 12 at 11:25",
	})
}

func TestRegisterFieldAccessFactsAndDirectValidation(t *testing.T) {
	input := `
module main

type Device register[6] {
	Control: bit read-write,
	Ready: bit read-only,
	Command: bit write-only,
	Pending: bit write-one-clear,
	Fault: bit write-zero-clear,
	Event: bit clear-on-read,
}

fn Valid(device: ref mut Device) void {
	let control := device.Control
	let ready := device.Ready
	let event := device.Event
	device.Control = false
	device.Command = true
	device.Pending = true
	device.Fault = false
	discard control
	discard ready
	discard event
}

fn Invalid(device: ref mut Device) void {
	let command := device.Command
	device.Ready = false
	device.Command += true
	device.Pending += true
	device.Event += true
	discard command
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	wantAccess := []RegisterFieldAccess{
		RegisterReadWrite,
		RegisterReadOnly,
		RegisterWriteOnly,
		RegisterWriteOneClear,
		RegisterWriteZeroClear,
		RegisterClearOnRead,
	}
	for index, access := range wantAccess {
		if got := analyzer.types["Device"].RegisterFields[index].Access; got != access {
			t.Fatalf("field %d access = %q, want %q", index, got, access)
		}
	}
	messages := make([]string, 0, len(errors))
	for _, err := range errors {
		messages = append(messages, err.Message)
	}
	wantMessages := []string{
		"register field Device.Command is write-only and cannot be read",
		"register field Ready is read-only and cannot be written",
		"write-only register field Command cannot be used with += because compound assignment reads the field",
		"register field Pending with write-one-clear semantics cannot use compound assignment",
		"register field Event with clear-on-read semantics cannot use compound assignment",
	}
	for _, want := range wantMessages {
		found := false
		for _, message := range messages {
			if strings.Contains(message, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing %q in errors: %v", want, errors)
		}
	}
}

func TestErrorEnumMarkerRootTypeAndWidening(t *testing.T) {
	input := `
module main

enum SensorError error { Failed }
enum Plain { Failed }

fn SensorFailure() Result[void, SensorError] {
	return Err(SensorError.Failed)
}

fn Widen() Result[void, error] {
	try SensorFailure()
	return Err(SensorError.Failed)
}

fn RejectPlain() Result[void, error] {
	return Err(Plain.Failed)
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	if got := analyzer.types["error"]; got.Kind != ErrorRootType || !got.Intrinsic {
		t.Fatalf("compiler-known error root = %#v", got)
	}
	if got := analyzer.types["SensorError"]; !got.ErrorAssignable || got.Kind != EnumType {
		t.Fatalf("marked error enum = %#v", got)
	}
	if got := analyzer.types["Plain"]; got.ErrorAssignable {
		t.Fatalf("ordinary enum became error-assignable: %#v", got)
	}
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "must return Err(error), got Err(Plain)") {
		t.Fatalf("error-root diagnostics = %v", errors)
	}
}

func TestErrorUnionMarkerAndResultErrorChannelConstraint(t *testing.T) {
	input := `
module main

type DetailedError union error {
    Open { Path: string, Code: int }
    Read(string)
}
type PlainFailure union { Failed }
enum PlainEnum { Failed }

fn Precise(value: DetailedError) Result[void, DetailedError] {
    return Err(value)
}

fn Widen(value: DetailedError) Result[void, error] {
    return Err(value)
}

fn InvalidUnion() Result[int, PlainFailure] {
    return Ok(1)
}

fn InvalidEnum() Result[int, PlainEnum] {
    return Ok(1)
}

fn InvalidBuiltin() Result[int, string] {
    return Ok(1)
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	if got := analyzer.types["DetailedError"]; got.Kind != UnionType || !got.ErrorAssignable || len(got.UnionVariants) != 2 {
		t.Fatalf("marked payload error union = %#v", got)
	}
	if got := analyzer.types["PlainFailure"]; got.ErrorAssignable {
		t.Fatalf("ordinary union became error-assignable: %#v", got)
	}
	assertSemaErrors(t, errors, []string{
		"Result error type PlainFailure is not an error type; use error or declare PlainFailure with the error marker at 19:31",
		"Result error type PlainEnum is not an error type; use error or declare PlainEnum with the error marker at 23:30",
		"Result error type string is not an error type; use error or declare string with the error marker at 27:33",
	})
}

func TestStructLiteralOptionFieldsAcceptContextualNone(t *testing.T) {
	input := `
module main

type State struct {
	First: Option[int],
	Second: Option[string],
}

fn Build() State {
	return State {
		First: None,
		Second: None(),
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestMemberAssignmentsAcceptQualifiedContextualOptionNone(t *testing.T) {
	input := `
module main

type State struct {
	Text: Option[string],
	Count: Option[int],
}

impl State {
	init() {
		self.Text = Option.None
		self.Count = Option.None
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestOptionEnumPayloadCanBeSelectedWithBindingGuard(t *testing.T) {
	input := `
module main

enum Mode {
	Strict,
	Lax,
}

fn Render(mode: Option[Mode]) int {
	return match mode {
		Option.Some(value) where value == Mode.Strict => 1
		Option.Some(value) where value == Mode.Lax => 2
		Option.None => 0
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestOptionEnumPayloadGuardsRemainNonExhaustiveWhenValueClassIsMissing(t *testing.T) {
	input := `
module main

enum Mode {
	Strict,
	Lax,
}

fn Render(mode: Option[Mode]) int {
	return match mode {
		Option.Some(value) where value == Mode.Strict => 1
		Option.None => 0
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"non-exhaustive match for Option[Mode]: missing Some at 10:9",
	})
}

func TestCanonicalReferenceTypeCallArgumentsRemainAvailable(t *testing.T) {
	input := `
module main

type Device struct { Count: uint }

fn Touch(device: ref mut Device) void {
	device.Count += 1
}

fn Twice(device: ref mut Device) void {
	Touch(device)
	Touch(device)
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

// rules/declarations/registers.md, Raw bit-field domains; correction13.md.
func TestWideRegisterFieldUsesExactArbitraryPrecisionRange(t *testing.T) {
	input := `
module main

type Wide65 register[65] {
	Value: bit[65],
}

type Wide128 register[128] {
	Value: bit[128],
}

@address(0x1000)
let mut wide65: Wide65
@address(0x2000)
let mut wide128: Wide128

fn Update() void {
	wide65.Value = 18446744073709551616
	wide65.Value = 36893488147419103231
	wide65.Value = 36893488147419103232
	wide128.Value = 1267650600228229401496703205376
	wide128.Value = 340282366920938463463374607431768211455
	wide128.Value = 340282366920938463463374607431768211456
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, []string{
		"value 36893488147419103232 overflows bit[65] at 20:17",
		"value 340282366920938463463374607431768211456 overflows bit[128] at 23:18",
	})

	field65 := analyzer.types["Wide65"].RegisterFields[0].Type
	if field65.Name != "bit[65]" || field65.BitWidth != 65 || field65.MaxInteger.String() != "36893488147419103231" {
		t.Fatalf("bit[65] field type = %+v", field65)
	}
	field128 := analyzer.types["Wide128"].RegisterFields[0].Type
	if field128.Name != "bit[128]" || field128.BitWidth != 128 || field128.MaxInteger.String() != "340282366920938463463374607431768211455" {
		t.Fatalf("bit[128] field type = %+v", field128)
	}
}

func TestRegisterValidationErrors(t *testing.T) {
	input := `
module main

unit rpm uint physical

type InvalidProtocol register[8] {
	Speed: bit[4],
	Enabled: bit,
	Mode: bit[2],
}

type BadField register[8] {
	Value: bit[0],
	_: bit[8],
}

type MotorStatus register[8] {
	Running: bit,
	Fault: bit,
	_: bit[6],
}

type MotorProtocol register[8] {
	Speed: bit[4]<rpm>,
	Enabled: bit,
	_: bit[3],
}

@address(0x40021000)
let motorStatus: MotorStatus

@address(0x40021001)
let mut motorProtocol: MotorProtocol

fn Test() void {
	let reserved := motorStatus._
	motorStatus.Running = true
	motorProtocol.Speed = 19
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"register InvalidProtocol declares 8 bits but its fields occupy 7 bits at 6:22",
		"register field BadField.Value width must be positive at 13:13",
		"reserved register field _ cannot be accessed at 36:30",
		"cannot assign to field Running on read-only addressed register motorStatus at 37:14",
		"value 19 overflows rpm at 38:24",
	}

	assertSemaErrors(t, errors, expected)
}

func TestRegisterBitBackedEnumImplSelfAndBitAssignments(t *testing.T) {
	input := `
module main

unit ticks uint physical

enum ClockSource bit[2] {
	APB_CLK: 0b00
	PLL_CLK: 0b10
}

type TimerConfig register[32] {
	ClkSrc: ClockSource
	En: bit
	_: bit[29]
}

type TimerValue register[32] {
	Ticks: bit[32]<ticks>
}

impl TimerConfig {
	fn Start(source: ClockSource) void {
		self.ClkSrc = source
		self.En = 1
	}

	property IsRunning: bool {
		get {
			return self.En
		}
	}
}

@address(0x3FF5F000)
let mut config: TimerConfig

@address(0x3FF5F004)
let value: TimerValue

fn Test() void {
	config.Start(ClockSource.PLL_CLK)
	let active := config.IsRunning
	let count: ticks := value.Ticks
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestRegisterValidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/register_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestRegister7TryAndMatchFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/register7_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestRegister8FrontendFeaturesExceptKnownProgramError(t *testing.T) {
	input, err := os.ReadFile("../../testdata/register8_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "unknown member AnyHigh on Tmp4719HighLimitStatus") {
		t.Fatalf("register8 frontend errors = %v; want only the documented Some(status) program error", errors)
	}
}

func TestExplicitCharRuneIntegerConversions(t *testing.T) {
	input := `
fn Scalars(value: char, scalar: rune) void {
	let encoded: byte := byte(value)
	let code: int := int(value)
	let runeCode: int := int(scalar)
	let restored: char := char(code | 0x20)
	let widened: rune := rune(value)
}
`
	if errors := analyzeSource(t, input); len(errors) != 0 {
		t.Fatalf("explicit scalar conversions produced errors: %v", errors)
	}
}

func TestStringIterationUsesCanonicalRuneImplProperties(t *testing.T) {
	input := `
module core

impl rune {
	property Utf8Length: uint {
		get { return 1u }
	}
	property IsWhitespace: bool {
		get { return false }
	}
}

fn Inspect(text: string) uint {
	let mut width: uint := 0u
	for character in text {
		if character.IsWhitespace {
			continue
		}
		width += character.Utf8Length
	}
	return width
}
`
	const sourceFile = "sec/core/scalar_iteration.sec"
	l := lexer.NewWithFile(input, sourceFile)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	program.SourceProvenance = map[string]ast.SourceProvenance{sourceFile: ast.SourceCore}
	if errors := NewAnalyzer().Analyze(program); len(errors) != 0 {
		t.Fatalf("string iteration lost rune impl properties: %v", errors)
	}
}

func TestRegisterInvalidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/register_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	if len(errors) == 0 {
		t.Fatal("expected register_invalid.sec to produce semantic errors")
	}
}

func TestUnitAliasTypesAreNominal(t *testing.T) {
	input := `
unit SEK currency
unit m physical
unit s physical
type Money decimal<SEK>
type Speed decimal<m/s>

let mut mo: Money := 5.90
let mut sp: Speed := 50
fn Test() void {
	mo += sp
}
`

	errors := analyzeSource(t, input)

	if len(errors) != 1 || !strings.Contains(errors[0].Message, "cannot add Speed to Money") {
		t.Fatalf("nominal unit errors = %v", errors)
	}
}

func TestUnitDivisionKeepsAnonymousStructuralType(t *testing.T) {
	input := `
unit m physical
unit s physical
type Meter decimal<m>
type Second decimal<s>
type Speed decimal<m/s>

let d: Meter := 100
let t: Second := 9.58
let v := d / t
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	v := analyzer.symbols["v"]
	if v.Type.Name == "Speed" || v.Type.UnitSemantics.Identity != StructuralUnitIdentity || v.Type.Unit != "m/s" {
		t.Fatalf("division invented a named result: %+v", v.Type)
	}
}

func TestUnitMultiplicationKeepsAnonymousStructuralType(t *testing.T) {
	input := `
unit m physical
unit s physical
type Meter decimal<m>
type Second decimal<s>
type Speed decimal<m/s>

let v: Speed := 10
let t: Second := 2
let d := v * t
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	d := analyzer.symbols["d"]
	if d.Type.Name == "Meter" || d.Type.UnitSemantics.Identity != StructuralUnitIdentity {
		t.Fatalf("multiplication invented a named result: %+v", d.Type)
	}
}

func TestUnitTypeMultipliedByIntKeepsUnitType(t *testing.T) {
	input := `
unit SEK currency
type Money decimal<SEK>

let m: Money := 10
let doubled := m * 2
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	doubled := analyzer.symbols["doubled"]
	if doubled.Type.Name != "Money" {
		t.Fatalf("wrong inferred type. got=%q want=%q", doubled.Type.Name, "Money")
	}
}

func TestUnitsV2KindBlocksDimensionOnlyAddition(t *testing.T) {
	input := `
unit Hz physical
unit rpm physical
impl Hz {
 Dimension: [time^-1]
 Kind: frequency
 Scale: 1
}
impl rpm {
 Dimension: [time^-1]
 Kind: rotational_frequency
 Scale: 1 / 60
}
fn Invalid(left: Hz, right: rpm) Hz { return left + right }
`
	errors := analyzeSource(t, input)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "incompatible unit dimension or Kind") {
		t.Fatalf("Kind mismatch errors = %v", errors)
	}
}

func TestUnitsV2StructuralResultDoesNotInventNamedUnit(t *testing.T) {
	input := `
unit m physical
unit s physical
unit Speed physical
impl m {
 Dimension: [length^1]
 Kind: length
 Scale: 1
}
impl s {
 Dimension: [time^1]
 Kind: duration
 Scale: 1
}
impl Speed {
 Dimension: [length^1, time^-1]
 Kind: speed
 Scale: 1
}
fn Derive(distance: m, duration: s) decimal<m/s> { return distance / duration }
`
	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)
	derived := analyzer.functions["Derive"][0].ReturnType
	if derived.UnitSemantics.Identity != StructuralUnitIdentity || derived.Unit != "m/s" || derived.Name == "Speed" {
		t.Fatalf("derived unit lost structural identity: %+v", derived)
	}
}

func TestUnitsV2PointAndLogarithmicArithmetic(t *testing.T) {
	input := `
unit C physical
unit DeltaC physical
unit dB ratio
impl C {
 Dimension: [temperature^1]
 Kind: temperature
 Scale: 1
 Transform: affine
 Offset: 27315 / 100
 Origin: absolute_zero
}
impl DeltaC {
 Dimension: [temperature^1]
 Kind: temperature
 Scale: 1
}
impl dB {
 Dimension: []
 Kind: ratio
 Transform: logarithmic
 LogBase: 10
 LogFactor: 10
 Reference: 1
}
fn Shift(point: C, delta: DeltaC) C { return point + delta }
fn Difference(left: C, right: C) void { let difference := left - right }
fn Bad(left: C, right: C) C { return left + right }
fn BadLog(left: dB, right: dB) dB { return left + right }
`
	errors := analyzeSource(t, input)
	if len(errors) != 2 || !strings.Contains(errors[0].Message, "two unit points") || !strings.Contains(errors[1].Message, "logarithmic") {
		t.Fatalf("point/logarithmic errors = %v", errors)
	}
}

func TestUnitsV2CurrencyDerivationWarnsAndFactorConversionValidates(t *testing.T) {
	input := `
unit EUR currency
unit SEK currency
fn Convert(value: SEK, factor: decimal<EUR/SEK>) EUR { return EUR(value, factor) }
fn Rate(value: EUR, seconds: decimal) decimal<EUR> { return value / seconds }
`
	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)
	if warnings := analyzer.Warnings(); len(warnings) == 0 || !strings.Contains(warnings[0].Message, "contains currency") {
		t.Fatalf("currency warnings = %v", warnings)
	}
}

func TestUnitsV2RejectsNonPositiveOrInexactMetadata(t *testing.T) {
	input := `
unit Zero physical
unit Dynamic physical
impl Zero {
 Dimension: [length^1]
 Scale: 0
}
impl Dynamic {
 Dimension: [length^1]
 Scale: runtime_value
}
`
	errors := analyzeSource(t, input)
	if len(errors) != 2 || !strings.Contains(errors[0].Message, "invalid scale metadata") || !strings.Contains(errors[1].Message, "invalid scale metadata") {
		t.Fatalf("scale metadata errors = %v", errors)
	}
}

func TestUnitsV2RejectsLossyImplicitFixedConversion(t *testing.T) {
	input := `
unit Whole physical
unit Third physical
impl Whole {
 Dimension: [length^1]
 Kind: length
 Scale: 1
}
impl Third {
 Dimension: [length^1]
 Kind: length
 Scale: 1 / 3
}
fn Add(left: Whole, right: Third) Whole { return left + right }
`
	errors := analyzeSource(t, input)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "without loss") {
		t.Fatalf("lossy implicit conversion errors = %v", errors)
	}
}

func TestUnitsV2FactorConversionRejectsWrongAlgebra(t *testing.T) {
	input := `
unit EUR currency
unit SEK currency
unit s physical
fn Invalid(value: SEK, factor: decimal<s>) EUR { return EUR(value, factor) }
`
	errors := analyzeSource(t, input)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "conversion factor") {
		t.Fatalf("factor algebra errors = %v", errors)
	}
}

func TestMoneyTimesMoneyIsStructuralAndWarns(t *testing.T) {
	input := `
unit SEK currency
type Money decimal<SEK>

let a: Money := 10
let b: Money := 2
let invalid := a * b
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)
	if analyzer.symbols["invalid"].Type.UnitSemantics.Identity != StructuralUnitIdentity || len(analyzer.Warnings()) == 0 {
		t.Fatalf("currency product did not remain structural with a warning: type=%+v warnings=%v", analyzer.symbols["invalid"].Type, analyzer.Warnings())
	}
}

func TestSameUnitDivisionInfersDecimal(t *testing.T) {
	input := `
unit SEK currency
type Money decimal<SEK>

let a: Money := 10
let b: Money := 2
let ratio := a / b
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	ratio := analyzer.symbols["ratio"]
	if ratio.Type.Name != "decimal" {
		t.Fatalf("wrong inferred type. got=%q want=%q", ratio.Type.Name, "decimal")
	}
}

func TestImplicitLetDefinesImmutableSymbol(t *testing.T) {
	input := `
let mut a := 10
let b := 9
fn Test() void {
	b = a
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot assign to immutable variable b at 5:2",
	}

	assertSemaErrors(t, errors, expected)
}

func TestBuiltinTypes(t *testing.T) {
	analyzer := NewAnalyzer()

	if anyType := analyzer.types["any"]; anyType.Kind != AnyType || !anyType.Intrinsic {
		t.Fatalf("any must be a compiler-known intrinsic top type: %+v", anyType)
	}
	if decimal := analyzer.types["decimal"]; decimal.Kind != DecimalType {
		t.Fatalf("decimal has wrong type kind: %q", decimal.Kind)
	}

	for _, name := range []string{"date", "datetime", "duration", "time"} {
		typ, exists := analyzer.types[name]
		if !exists || !typ.Intrinsic || typ.Name != name {
			t.Errorf("%s must be a compiler-known intrinsic type: %+v", name, typ)
		}
		if IsDefaultable(typ) {
			t.Errorf("%s must not acquire a default before temporal rules define one", name)
		}
		if canInitialize(typ, analyzer.types["int"], &ast.IntegerLiteral{Token: lexer.Token{Lexeme: "1"}}) ||
			canInitialize(typ, analyzer.types["string"], &ast.StringLiteral{Token: lexer.Token{Lexeme: `"value"`}}) {
			t.Errorf("%s must not accept an implicit numeric or string conversion", name)
		}
	}

	for _, name := range []string{"bytes"} {
		if _, exists := analyzer.types[name]; exists {
			t.Errorf("%s must not be a builtin type", name)
		}
	}
}

func TestAnyAcceptsValuesWithoutImplicitNarrowing(t *testing.T) {
	input := `
module main

type User struct {
	name: string,
}

fn BoxInteger(value: int) any {
	return value
}

fn BoxUser(value: User) any {
	return value
}

fn Preserve(value: any) any {
	return value
}
`

	assertSemaErrors(t, analyzeSource(t, input), nil)

	invalid := `
module main

fn Narrow(value: any) int {
	return value
}

fn Inspect(value: any) void {
	value.Unknown
}
`
	errors := analyzeSource(t, invalid)
	if len(errors) != 2 {
		t.Fatalf("errors = %#v, want implicit-narrowing and member-access diagnostics", errors)
	}
	if !strings.Contains(errors[0].Message, "must return int, got any") {
		t.Fatalf("first error = %q", errors[0].Message)
	}
	if !strings.Contains(errors[1].Message, "unknown member Unknown on any") {
		t.Fatalf("second error = %q", errors[1].Message)
	}
}

func TestCompilerKnownTypeNamesCannotBeRedeclared(t *testing.T) {
	input := `
module main

type int int

enum Result {
	Replacement,
}

interface any {
}

interface Iterator[T] {
}

unit string decimal physical

fn Preserve(value: any) int {
	return 1
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	wants := []string{
		"type name int is compiler-known and cannot be redeclared",
		"type name Result is compiler-known and cannot be redeclared",
		"type name any is compiler-known and cannot be redeclared",
		"type name Iterator is compiler-known and cannot be redeclared",
		"unit name string is compiler-known and cannot be declared",
	}
	if len(errors) != len(wants) {
		t.Fatalf("errors = %#v, want %d diagnostics", errors, len(wants))
	}
	for _, want := range wants {
		found := false
		for _, diagnostic := range errors {
			if strings.Contains(diagnostic.Message, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("errors = %#v, missing %q", errors, want)
		}
	}
	if analyzer.types["int"].Kind != IntType || analyzer.types["Result"].Kind != ResultType || analyzer.types["any"].Kind != AnyType || analyzer.types["string"].Kind != StringType {
		t.Fatalf("compiler-known types were replaced: int=%+v Result=%+v any=%+v string=%+v", analyzer.types["int"], analyzer.types["Result"], analyzer.types["any"], analyzer.types["string"])
	}
}

func TestVoidTypeUseRestrictions(t *testing.T) {
	input := `
module main

type Invalid struct {
	field: void,
}

interface InvalidInterface {
	fn Apply(value: void) void
	property Value: void {
		get
	}
}

impl Invalid {
	property Value: void {
		get { return }
	}
}

fn InvalidParameter(value: void) void {
}

fn InvalidStorage() void {
	let mut local: void
}

fn InvalidTypeArguments(
	optional: Option[void],
	badError: Result[int, void],
	sequence: void[2],
	slice: ref void[],
	function: fn(void) void,
) void {
}

fn Allowed(opaque: RawPtr[void], result: Result[void, ContractError]) void {
}
`

	errors := analyzeSource(t, input)
	wants := []string{
		"stored field Invalid.field cannot have type void",
		"interface method parameter \"value\" cannot have type void",
		"interface property InvalidInterface.Value cannot have type void",
		"property Invalid.Value cannot have type void",
		"parameter \"value\" cannot have type void",
		"variable local cannot have type void",
		"Option does not permit void as a type argument",
		"Result does not permit void as a type argument",
		"sequence element type cannot be void",
		"slice element type cannot be void",
		"function type parameter cannot have type void",
	}
	if len(errors) != len(wants) {
		t.Fatalf("errors = %#v, want %d diagnostics", errors, len(wants))
	}
	for _, want := range wants {
		found := false
		for _, diagnostic := range errors {
			if strings.Contains(diagnostic.Message, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("errors = %#v, missing %q", errors, want)
		}
	}
}

func TestCallableCapabilitiesParticipateInSemanticTypeIdentity(t *testing.T) {
	program := parser.New(lexer.New(`
module main

type Callables struct {
    shared: fn(int) int,
    mutable: mut fn(int) int,
    consuming: -> fn(int) int,
}
`)).ParseProgram()
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("sema errors: %v", errors)
	}
	fields := analyzer.Types()["Callables"].Fields
	wantCapabilities := []CallableCapability{CallableShared, CallableMutable, CallableConsuming}
	wantNames := []string{"fn(int) int", "mut fn(int) int", "-> fn(int) int"}
	for index := range wantCapabilities {
		if got := normalizedCallableCapability(fields[index].Type.FunctionCapability); got != wantCapabilities[index] {
			t.Fatalf("field %d capability = %q, want %q", index, got, wantCapabilities[index])
		}
		if got := typeDisplayName(fields[index].Type); got != wantNames[index] {
			t.Fatalf("field %d display = %q, want %q", index, got, wantNames[index])
		}
	}
	if sameFunctionType(fields[0].Type, fields[1].Type) || sameFunctionType(fields[1].Type, fields[2].Type) {
		t.Fatal("distinct callable capabilities collapsed to one semantic type identity")
	}
	if canonicalTypeIdentity(fields[0].Type) == canonicalTypeIdentity(fields[1].Type) || canonicalTypeIdentity(fields[1].Type) == canonicalTypeIdentity(fields[2].Type) {
		t.Fatal("distinct callable capabilities collapsed to one canonical type identity")
	}
}

func TestTemporalBuiltinTypesResolveWithoutImport(t *testing.T) {
	input := `
module main

fn PreserveDate(value: date) date {
	return value
}

fn PreserveTime(value: time) time {
	return value
}

fn PreserveDateTime(value: datetime) datetime {
	return value
}

fn PreserveDuration(value: duration) duration {
	return value
}
`

	assertSemaErrors(t, analyzeSource(t, input), nil)
}

func TestRedeclarationDoesNotReplaceExistingSymbol(t *testing.T) {
	input := `
let mut a: uint := 5
let a: int := 1
fn Test() void {
	a = -1
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"variable \"a\" already declared at 3:5, previous declaration at 2:9",
		"value -1 overflows uint at 5:6",
	}

	assertSemaErrors(t, errors, expected)
}

func TestIntegerLiteralRanges(t *testing.T) {
	input := `
let a: int8 := 200
let b: uint8 := 255
let c: uint8 := 256
let d: uint := -1
let e: int8 := -128
let f: int8 := -129
`

	errors := analyzeSource(t, input)

	expected := []string{
		"value 200 overflows int8 at 2:16",
		"value 256 overflows uint8 at 4:17",
		"value -1 overflows uint at 5:16",
		"value -129 overflows int8 at 7:16",
	}

	assertSemaErrors(t, errors, expected)
}

func TestCharAndRuneNumericSuffixes(t *testing.T) {
	input := `
module main

let marker: rune := '$'

fn Matches(ch: rune, letter: char) bool {
	switch ch {
	case '$':
		return letter == 65t
	default:
		return ch == '$' && ch == '\n' && ch == 0r && letter == 65t
}
}
`

	assertSemaErrors(t, analyzeSource(t, input), nil)
}

func TestCharacterLiteralShapesToRuneFunctionParameter(t *testing.T) {
	input := `
module main

fn AcceptRune(value: rune) bool {
	return value == 'A'
}

fn Test() bool {
	return AcceptRune('A')
}
`

	assertSemaErrors(t, analyzeSource(t, input), nil)
}

func TestInferIncompleteExpressionDoesNotPanic(t *testing.T) {
	analyzer := NewAnalyzer()
	analyzer.Analyze(&ast.Program{})
	expr := &ast.InfixExpression{
		Token:    lexer.Token{Lexeme: "==", Line: 1, Column: 3},
		Left:     &ast.IntegerLiteral{Token: lexer.Token{Lexeme: "1", Line: 1, Column: 1}},
		Operator: "==",
		Right:    nil,
	}

	typ, _ := analyzer.inferExpression(expr)
	if typ.Kind != InvalidType {
		t.Fatalf("wrong inferred type. got=%s want=%s", typ.Kind, InvalidType)
	}
}

func TestSecCharacterLiteralEscapes(t *testing.T) {
	input := `
module main

fn Test(ch: rune) bool {
	return ch == '\n' || ch == '\0' || ch == '\'' || ch == '\x41' || ch == '\u{03A9}'
}
`

	assertSemaErrors(t, analyzeSource(t, input), nil)
}

func TestCharAndRuneNumericSuffixesRequireUnicodeScalarsAndExactTypes(t *testing.T) {
	input := `
module main

let high: rune := 1114112r
let surrogate: char := 55296t
let wrong: char := 65r

fn Values(items: int[65r]) void {
}

fn Different(letter: char, codepoint: rune) bool {
	return letter == codepoint
}
`

	errors := analyzeSource(t, input)
	expected := []string{
		"array length must be a compile-time integer at 8:22",
		"value 1114112r is not a valid Unicode scalar value at 4:19",
		"value 55296t is not a valid Unicode scalar value at 5:24",
		"cannot initialize char with rune at 6:20",
		"cannot compare char and rune at 12:16",
	}
	assertSemaErrors(t, errors, expected)
}

func TestCompilerKnownLenReturnsIntForStringsAndArrays(t *testing.T) {
	input := `
module main

fn Count(text: string, slice: ref rune[]) int {
	let owned := [0r, 1r]
	return len(text) + len(slice) + len(owned)
}
`

	assertSemaErrors(t, analyzeSource(t, input), nil)
}

func TestCompilerKnownLenRejectsUnsupportedArgumentsAndRedeclaration(t *testing.T) {
	input := `
module main

fn len(value: bool) int {
	return 0
}

fn Test() int {
	let none := len()
	let bad := len(true)
	let explicit := len[rune]([0r])
	return 0
}
`

	errors := analyzeSource(t, input)
	expected := []string{
		"function len is compiler-known and cannot be declared at 4:4",
		"len expects 1 argument, got 0 at 9:14",
		"len requires a compiler-known sequence or collection, got bool at 10:17",
		"len infers its element type from its argument at 11:18",
	}
	assertSemaErrors(t, errors, expected)
}

func TestCompilerKnownStringSliceIsRestrictedToCoreSource(t *testing.T) {
	input := `module core

fn Slice(value: string, start: uint, end: uint) string {
	return __StringSliceUnchecked(value, start, end)
}
`
	l := lexer.NewWithFile(input, "/tmp/project/sec/core/string.sec")
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	program.SourceProvenance = map[string]ast.SourceProvenance{"/tmp/project/sec/core/string.sec": ast.SourceCore}
	assertSemaErrors(t, NewAnalyzer().Analyze(program), nil)

	delete(program.SourceProvenance, "/tmp/project/sec/core/string.sec")
	forged := NewAnalyzer().Analyze(program)
	if len(forged) != 1 || !strings.Contains(forged[0].Message, "privileged core source") {
		t.Fatalf("forged core path errors = %v", forged)
	}

	errors := analyzeSource(t, input)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "compiler-internal operation available only to privileged core source") {
		t.Fatalf("ordinary source errors = %v", errors)
	}
}

func TestCompilerKnownStringSliceValidatesArguments(t *testing.T) {
	input := `module core

fn Slice(value: string) string {
	return __StringSliceUnchecked(value, true, 2u)
}
`
	l := lexer.NewWithFile(input, "sec/core/string.sec")
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	program.SourceProvenance = map[string]ast.SourceProvenance{"sec/core/string.sec": ast.SourceCore}
	errors := NewAnalyzer().Analyze(program)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "argument 2 to __StringSliceUnchecked must be uint, got bool") {
		t.Fatalf("errors = %v", errors)
	}
}

func TestTypeRangeBoundsFitBaseType(t *testing.T) {
	input := `
type Valid int8 range -128..127
type MaxOverflow int8 range 0..1000
type MinOverflow int8 range -129..100
type UintMinOverflow uint8 range -1..
type OpenRange uint8 range ..255
`

	errors := analyzeSource(t, input)

	expected := []string{
		"value 1000 overflows int8 at 3:32",
		"value -129 overflows int8 at 4:29",
		"value -1 overflows uint8 at 5:34",
	}

	assertSemaErrors(t, errors, expected)
}

func TestStructTypeDeclarationRegistersNamedType(t *testing.T) {
	input := `
type Meter decimal<m>

type Coordinate struct {
	x: Meter,
	y: Meter,
	z: Meter,
}

let mut c: Coordinate

unit m physical
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	coordinate := analyzer.types["Coordinate"]
	if !coordinate.Named {
		t.Fatal("Coordinate should be registered as named type")
	}
	if coordinate.Kind != StructType {
		t.Fatalf("wrong type kind. got=%q want=%q", coordinate.Kind, StructType)
	}
	if len(coordinate.Fields) != 3 {
		t.Fatalf("wrong field count. got=%d want=3", len(coordinate.Fields))
	}
}

func TestStructDuplicateFields(t *testing.T) {
	input := `
type Bad struct {
	x: int,
	x: int,
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		`duplicate field "x" in struct Bad at 4:2`,
	}

	assertSemaErrors(t, errors, expected)
}

func TestStructUnknownFieldType(t *testing.T) {
	input := `
type Bad struct {
	x: UnknownType,
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"unknown type UnknownType at 3:5",
	}

	assertSemaErrors(t, errors, expected)
}

func TestStructFieldTagsArePreserved(t *testing.T) {
	input := `
type User struct {
	ID: int ` + "`json:\"id\" db:\"user_id\"`" + `,
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	user := analyzer.types["User"]
	if len(user.Fields) != 1 {
		t.Fatalf("wrong field count. got=%d want=1", len(user.Fields))
	}
	if len(user.Fields[0].Tags) != 2 {
		t.Fatalf("wrong tag count. got=%d want=2", len(user.Fields[0].Tags))
	}
	if user.Fields[0].Tags[1].Key != "db" || user.Fields[0].Tags[1].Value != "user_id" {
		t.Fatalf("wrong db tag: %+v", user.Fields[0].Tags[1])
	}
}

func TestStructFieldRangeContract(t *testing.T) {
	input := `
type User struct {
	Active: bool,
	Name: string,
	Age: int range 0..130,
}

let ok := User{ Active: true, Name: "Ada", Age: 42 }
let bad := User{ Active: true, Name: "Ada", Age: 131 }
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)

	expected := []string{
		"value 131 violates range contract int 0..130 at 9:50",
	}

	assertSemaErrors(t, errors, expected)

	user := analyzer.types["User"]
	if len(user.Fields) != 3 {
		t.Fatalf("wrong field count. got=%d want=3", len(user.Fields))
	}
	if len(user.Fields[2].Type.Contracts) != 1 {
		t.Fatalf("Age should have one range contract, got %d", len(user.Fields[2].Type.Contracts))
	}
}

func TestImplTargetMustExist(t *testing.T) {
	input := `
impl MissingType {
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"unknown impl target MissingType at 2:6",
	}

	assertSemaErrors(t, errors, expected)
}

func TestCoreStringCanImplementBuiltinString(t *testing.T) {
	input := `module string

impl string {
	fn Len() uint {
		return self.len
	}
}
`

	l := lexer.NewWithFile(input, filepath.Join("sec", "core", "string.sec"))
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	analyzer := NewAnalyzer()
	// rules/library/core-library.md grants builtin implementation authority
	// through loader-owned provenance, never through the file name alone.
	program.SourceProvenance = map[string]ast.SourceProvenance{
		filepath.Join("sec", "core", "string.sec"): ast.SourceCore,
	}
	assertSemaErrors(t, analyzer.Analyze(program), nil)
	if len(analyzer.functions["string.Len"]) == 0 {
		t.Fatal("core string impl did not register string.Len")
	}
}

func TestTrustedCoreCanImplementTemporalBuiltins(t *testing.T) {
	for _, target := range []string{"date", "time", "datetime", "duration"} {
		t.Run(target, func(t *testing.T) {
			sourceFile := filepath.Join("sec", "core", target+".sec")
			input := "module core\n\nimpl " + target + " {\n\tfn Identity() " + target + " { return self }\n}\n"
			program := parser.New(lexer.NewWithFile(input, sourceFile)).ParseProgram()
			program.SourceProvenance = map[string]ast.SourceProvenance{sourceFile: ast.SourceCore}

			analyzer := NewAnalyzer()
			assertSemaErrors(t, analyzer.Analyze(program), nil)
			if len(analyzer.functions[target+".Identity"]) == 0 {
				t.Fatalf("trusted core impl did not register %s.Identity", target)
			}
		})
	}
}

func TestUserCodeCannotImplementTemporalBuiltin(t *testing.T) {
	input := `module app

impl duration {
}
`
	program := parser.New(lexer.NewWithFile(input, filepath.Join("app", "duration.sec"))).ParseProgram()
	assertSemaErrors(t, NewAnalyzer().Analyze(program), []string{
		"impl target duration is not a named type at app/duration.sec:3:6",
	})
}

func TestTrustedCoreCanImplementCompilerKnownCollectionAndShapedTypes(t *testing.T) {
	// rules/collections/collections.md sections 1-2 and
	// rules/collections/shaped-types.md section 34 allow ordinary core helpers
	// on these compiler-known identities without granting user monkey-patching.
	targets := []struct {
		name   string
		target string
	}{
		{name: "list", target: "list[T]"},
		{name: "map", target: "map[K, V]"},
		{name: "set", target: "set[T]"},
		{name: "vector", target: "vector[T]"},
		{name: "matrix", target: "matrix[T]"},
		{name: "tensor", target: "tensor[T]"},
		{name: "tensor_view", target: "tensor_view[T]"},
		{name: "Shape", target: "Shape"},
		{name: "Strides", target: "Strides"},
		{name: "TensorLayout", target: "TensorLayout"},
		{name: "MemorySpace", target: "MemorySpace"},
	}

	for _, test := range targets {
		t.Run(test.name, func(t *testing.T) {
			sourceFile := filepath.Join("sec", "core", test.name+".sec")
			input := "module core\n\nimpl " + test.target + " {\n\tfn CoreMarker() void {}\n}\n"
			program := parser.New(lexer.NewWithFile(input, sourceFile)).ParseProgram()
			program.SourceProvenance = map[string]ast.SourceProvenance{sourceFile: ast.SourceCore}

			analyzer := NewAnalyzer()
			assertSemaErrors(t, analyzer.Analyze(program), nil)
			if len(analyzer.functions[test.name+".CoreMarker"]) == 0 {
				t.Fatalf("trusted core impl did not register %s.CoreMarker", test.name)
			}
		})
	}
}

func TestUserCodeCannotImplementCompilerKnownCollection(t *testing.T) {
	input := `module app

impl list[T] {
}
`
	program := parser.New(lexer.NewWithFile(input, filepath.Join("app", "list.sec"))).ParseProgram()
	assertSemaErrors(t, NewAnalyzer().Analyze(program), []string{
		"impl target list is not a named type at app/list.sec:3:6",
	})
}

func TestCoreLookingPathCannotImplementBuiltinStringWithoutProvenance(t *testing.T) {
	input := `module core

impl string {
}
`

	const sourceFile = "/tmp/project/sec/core/string.sec"
	l := lexer.NewWithFile(input, sourceFile)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	expected := []string{
		"impl target string is not a named type at /tmp/project/sec/core/string.sec:3:6",
	}
	assertSemaErrors(t, NewAnalyzer().Analyze(program), expected)
}

func TestUserCodeCannotImplementBuiltinString(t *testing.T) {
	input := `module app

impl string {
}
`

	l := lexer.NewWithFile(input, filepath.Join("app", "string.sec"))
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	analyzer := NewAnalyzer()
	expected := []string{
		"impl target string is not a named type at app/string.sec:3:6",
	}
	assertSemaErrors(t, analyzer.Analyze(program), expected)
}

func TestInterfaceImplementationConformance(t *testing.T) {
	input := `
module main

interface Vehicle {
	fn Start() void
	fn Stop() void

	property IsRunning: bool {
		get
	}
}

type Car struct {
	running: bool,
}

impl Car implements Vehicle {
	property IsRunning: bool {
		get {
			return running
		}
	}

	fn Start() void {
		return
	}

	fn Stop() void {
		return
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestInterfaceMethodRejectsAdditionalGenericParameters(t *testing.T) {
	input := `
module main

interface Mapper[T] {
	fn Map[U](value: T) U
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"interface method Map cannot declare method-level generic parameters; use interface generic parameters instead at 5:9",
	})
}

func TestInterfaceConformancePreservesConsumingParameterContract(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

interface Sink {
	fn Send(-> value: int) void
}

type Correct struct {}
impl Correct implements Sink {
	fn Send(-> value: int) void {}
}

type Wrong struct {}
impl Wrong implements Sink {
	fn Send(value: int) void {}
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "type Wrong method Send does not match interface Sink") {
		t.Fatalf("wrong consuming interface errors: %v", errors)
	}
}

func TestInterfaceImplementationErrors(t *testing.T) {
	input := `
module main

interface Vehicle {
	fn Start() void
	fn Stop() void

	property IsRunning: bool {
		get
		set running
	}
}

interface Marker {
}

type NotInterface struct {
}

type Duplicate struct {
}

type BadTarget struct {
}

type MissingMembers struct {
}

impl Duplicate implements Marker, Marker {
}

impl BadTarget implements NotInterface {
}

impl MissingMembers implements Vehicle {
}

type WrongSignature struct {
	running: bool,
}

impl WrongSignature implements Vehicle {
	property IsRunning: bool {
		get {
			return running
		}
	}

	fn Start(extra: int) void {
		return
	}

	fn Stop() int {
		return 0
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"duplicate implemented interface Marker on Duplicate at 29:35",
		"implemented type NotInterface on BadTarget is not an interface at 32:27",
		"type MissingMembers implements Vehicle but is missing method Start at 5:5",
		"type MissingMembers implements Vehicle but is missing method Stop at 6:5",
		"type MissingMembers implements Vehicle but is missing property IsRunning at 8:11",
		"type WrongSignature method Start does not match interface Vehicle at 5:5",
		"type WrongSignature method Stop does not match interface Vehicle at 6:5",
		"type WrongSignature property IsRunning must provide set for interface Vehicle at 8:11",
	}
	assertSemaErrors(t, errors, expected)
}

func TestInterfaceInheritanceCycles(t *testing.T) {
	input := `module main

interface SelfCycle implements SelfCycle {
}

interface LeftCycle implements RightCycle {
}

interface RightCycle implements LeftCycle {
}

interface FirstCycle implements SecondCycle {
}

interface SecondCycle implements ThirdCycle {
}

interface ThirdCycle implements FirstCycle {
}

interface ReachesCycle implements FirstCycle {
	fn Required() void
}

type InvalidImplementation struct {
}

impl InvalidImplementation implements ReachesCycle {
}
`

	errors := analyzeSourceRaw(t, input)
	if len(errors) != 3 {
		t.Fatalf("errors = %+v, want three inheritance-cycle diagnostics", errors)
	}
	wantMessages := []string{
		"interface inheritance cycle: SelfCycle -> SelfCycle",
		"interface inheritance cycle: LeftCycle -> RightCycle -> LeftCycle",
		"interface inheritance cycle: FirstCycle -> SecondCycle -> ThirdCycle -> FirstCycle",
	}
	for index, err := range errors {
		if err.ID != diagnostics.InterfaceInheritanceCycle {
			t.Fatalf("error %d ID = %q, want %q", index, err.ID, diagnostics.InterfaceInheritanceCycle)
		}
		if err.Severity != diagnostics.SeverityError {
			t.Fatalf("error %d severity = %q, want error", index, err.Severity)
		}
		if err.Message != wantMessages[index] {
			t.Fatalf("error %d message = %q, want %q", index, err.Message, wantMessages[index])
		}
		if err.Help != "remove one implements relationship from the cycle" {
			t.Fatalf("error %d help = %q", index, err.Help)
		}
	}
}

func TestInterfaceReceiverCapabilitiesAtCallSites(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

interface Resource {
	fn Inspect() int
	mut fn Update() void
	-> fn Detach() int
}

fn Shared(value: ref Resource) void {
	discard value.Inspect()
	value.Update()
	value.Detach()
}

fn Mutable(value: ref mut Resource) void {
	discard value.Inspect()
	value.Update()
	value.Detach()
}

fn Owned(value: Resource) void {
	discard value.Detach()
	discard value
}
`)
	if len(errors) != 4 {
		t.Fatalf("errors = %v, want four receiver diagnostics", errors)
	}
	want := []string{
		"method Update requires mutable receiver",
		"consuming method Detach requires an owned receiver; ref Resource does not transfer ownership",
		"consuming method Detach requires an owned receiver; ref mut Resource does not transfer ownership",
		"use of moved value value",
	}
	for index, fragment := range want {
		if !strings.Contains(errors[index].Message, fragment) {
			t.Fatalf("error %d = %q, want fragment %q", index, errors[index].Message, fragment)
		}
	}
}

func TestInterfaceReceiverCapabilityConformance(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

interface SharedContract {
	fn Change() void
}

interface MutableContract {
	mut fn Observe() int
}

type MutableImplementation struct {
	value: int,
}

impl MutableImplementation implements SharedContract {
	fn Change() void {
		self.value = 1
	}
}

type SharedImplementation struct {
	value: int,
}

impl SharedImplementation implements MutableContract {
	fn Observe() int {
		return self.value
	}
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "method Change does not match interface SharedContract") {
		t.Fatalf("errors = %v", errors)
	}
}

func TestImplTargetsNamedKinds(t *testing.T) {
	input := `
type Vehicle struct {
}

enum FuelType {
	Petrol,
}

type ResultLike union {
	Ok,
	Err(int),
}

type Status register[1] {
	Ready: bit,
}

type Percent int range 0..100

impl Vehicle {
}

impl FuelType {
}

impl ResultLike {
}

impl Status {
}

impl Percent {
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestDuplicateImplBlocksAreInvalid(t *testing.T) {
	input := `
type Vehicle struct {
}

impl Vehicle {
}

impl Vehicle {
}
`

	errors := analyzeSource(t, input)
	expected := []string{
		"duplicate impl block for Vehicle; additional blocks must use impl extends Vehicle at 8:6, previous declaration at 5:6",
	}
	assertSemaErrors(t, errors, expected)
}

func TestExplicitImplExtensionsComposeMemberSurface(t *testing.T) {
	input := `
type Vehicle struct {
	speed: int,
}

impl Vehicle {
	fn Speed() int {
		return self.speed
	}
}

impl extends Vehicle {
	fn IsStopped() bool {
		return self.Speed() == 0
	}
}

impl extends Vehicle {
	fn Stop() void {
		self.speed = 0
	}
}

fn Test(vehicle: Vehicle) bool {
	return vehicle.IsStopped()
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)
	for _, name := range []string{"Vehicle.Speed", "Vehicle.IsStopped", "Vehicle.Stop"} {
		if len(analyzer.functions[name]) != 1 {
			t.Fatalf("missing composed impl method %s: %+v", name, analyzer.functions[name])
		}
	}
}

func TestImplExtensionRequiresPrimaryBlock(t *testing.T) {
	input := `
type Vehicle struct {
}

impl extends Vehicle {
	fn Stop() void {
	}
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, []string{
		"impl extends Vehicle requires a primary impl Vehicle block in the same module at 5:14",
	})
}

func TestImplExtensionMayLiveInAnotherFileOfSameModule(t *testing.T) {
	primary := parser.New(lexer.NewWithFile(`module fleet

type Vehicle struct {
}

impl Vehicle {
	fn Start() void {
	}
}
`, "vehicle.sec")).ParseProgram()
	extension := parser.New(lexer.NewWithFile(`module fleet

impl extends Vehicle {
	fn Stop() void {
	}
}
`, "vehicle_stop.sec")).ParseProgram()
	// Put the extension first to prove that neither file nor declaration order
	// determines whether the primary is found.
	program := &ast.Program{Statements: append(extension.Statements, primary.Statements...)}

	analyzer := NewAnalyzer()
	assertSemaErrors(t, analyzer.Analyze(program), nil)
	for _, name := range []string{"Vehicle.Start", "Vehicle.Stop"} {
		if len(analyzer.functions[name]) != 1 {
			t.Fatalf("missing cross-file impl method %s", name)
		}
	}
}

func TestImplExtensionCannotCrossModuleBoundary(t *testing.T) {
	primary := parser.New(lexer.NewWithFile(`module fleet

type Vehicle struct {
}

impl Vehicle {
}
`, "vehicle.sec")).ParseProgram()
	extension := parser.New(lexer.NewWithFile(`module workshop

impl extends Vehicle {
}
`, "workshop.sec")).ParseProgram()
	program := &ast.Program{Statements: append(primary.Statements, extension.Statements...)}

	errors := NewAnalyzer().Analyze(program)
	assertSemaErrors(t, errors, []string{
		"impl extension for Vehicle must be in module fleet at workshop.sec:3:14, previous declaration at vehicle.sec:6:6",
	})
}

func TestImplExtensionMemberConflictsAreCheckedAcrossBlocks(t *testing.T) {
	input := `
type Vehicle struct {
}

impl Vehicle {
	fn Status() int {
		return 0
	}
}

impl extends Vehicle {
	property Status: int {
		get {
			return 0
		}
	}
}

unit m physical
unit s physical
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, []string{
		"property Status conflicts with method Status in Vehicle at 12:11",
	})
}

func TestDuplicateNestedTypeAcrossImplBlocks(t *testing.T) {
	input := `
type Vehicle struct {
}

impl Vehicle {
	enum FuelType {
		petrol,
	}

	enum FuelType {
		diesel,
	}
}
`

	errors := analyzeSource(t, input)
	expected := []string{
		"duplicate nested type \"FuelType\" in impl Vehicle at 10:7",
	}
	assertSemaErrors(t, errors, expected)
}

func TestNestedEnumForwardReference(t *testing.T) {
	input := `
type Vehicle struct {
	fuel_type: Vehicle.FuelType,
}

impl Vehicle {
	enum FuelType {
		petrol,
		diesel,
		electric,
	}
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	typ := analyzer.types["Vehicle"]
	if len(typ.Fields) != 1 || typ.Fields[0].Type.Name != "Vehicle.FuelType" {
		t.Fatalf("wrong field type: %+v", typ.Fields)
	}
	enumType := analyzer.types["Vehicle.FuelType"]
	if enumType.Kind != EnumType || len(enumType.EnumValues) != 3 {
		t.Fatalf("wrong enum type: %+v", enumType)
	}
}

func TestNestedTypeRequiresQualifiedNameOutsideImpl(t *testing.T) {
	input := `
type Vehicle struct {
	fuel_type: FuelTypes,
}

impl Vehicle {
	enum FuelTypes {
		Petrol,
	}
}

let mut ft: FuelTypes := FuelTypes.Petrol
`

	errors := analyzeSource(t, input)

	expected := []string{
		"unknown type FuelTypes at 3:13",
		"unknown type FuelTypes at 12:13",
	}

	assertSemaErrors(t, errors, expected)
}

func TestNestedTypeShortNameInsideImpl(t *testing.T) {
	input := `
type Vehicle struct {
	fuel_type: Vehicle.FuelTypes,
}

impl Vehicle {
	enum FuelTypes {
		Petrol,
	}

	property FuelType: FuelTypes {
		get {
			return FuelTypes.Petrol
		}
	}
}

let mut ft: Vehicle.FuelTypes := Vehicle.FuelTypes.Petrol
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestNestedEnumDuplicateValue(t *testing.T) {
	input := `
type Vehicle struct {
}

impl Vehicle {
	enum FuelType {
		petrol,
		petrol,
	}
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		`duplicate enum value "petrol" in enum Vehicle.FuelType at 8:3, previous declaration at 7:3`,
	}

	assertSemaErrors(t, errors, expected)
}

func TestEnumValuesAndNamespaceConstants(t *testing.T) {
	input := `
enum Color {
	red,
	green,
	blue,
}

enum Status int {
	unknown = 0,
	active = 10,
	paused,
	disabled = 99,
}

let c: Color := Color.red
let s: Status := Status.paused
let explicitColor: Color := Color(1)
let explicitInt: int := int(Color.green)
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	color := analyzer.types["Color"]
	if color.Kind != EnumType || color.EnumConsts["green"].Value.String() != "1" {
		t.Fatalf("wrong Color enum: %+v", color)
	}

	status := analyzer.types["Status"]
	if status.Kind != EnumType || status.EnumConsts["paused"].Value.String() != "10" {
		t.Fatalf("wrong Status enum: %+v", status)
	}

	if analyzer.symbols["c"].Type.Name != "Color" {
		t.Fatalf("wrong c type: %+v", analyzer.symbols["c"])
	}
	if analyzer.symbols["explicitInt"].Type.Name != "int" {
		t.Fatalf("wrong explicitInt type: %+v", analyzer.symbols["explicitInt"])
	}
}

func TestBitBackedEnumRegisterField(t *testing.T) {
	input := `
enum ClockSource: bit[2] {
	Internal = 0b00,
	External = 0b01,
	Bypass = 0b10,
}

type ClockConfig register[32] {
	Source: ClockSource,
	Enabled: bit,
	_: bit[29],
}

@address(0x40000000)
let mut config: ClockConfig

fn Configure() void {
	config.Source = ClockSource.External
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	clockSource := analyzer.types["ClockSource"]
	if clockSource.Kind != EnumType || clockSource.BitWidth != 2 || clockSource.Underlying != "bit[2]" {
		t.Fatalf("wrong bit-backed enum type: %+v", clockSource)
	}
	if clockSource.EnumConsts["Bypass"].Value.String() != "2" {
		t.Fatalf("wrong bit-backed enum values: %+v", clockSource.EnumConsts)
	}

	config := analyzer.types["ClockConfig"]
	if config.Kind != RegisterType || len(config.RegisterFields) != 3 {
		t.Fatalf("wrong register type: %+v", config)
	}
	source := config.RegisterFields[0]
	if source.Width != 2 || source.Type.Name != "ClockSource" || source.Type.Kind != EnumType {
		t.Fatalf("wrong enum-backed register field: %+v", source)
	}
}

func TestImplMethodsPreserveDeclaringModule(t *testing.T) {
	input := `
module fmt

type Buffer struct {
    data: byte[4],
}

impl Buffer {
    fn Flush() void {
    }
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)
	methods := analyzer.functions["Buffer.Flush"]
	if len(methods) != 1 || methods[0].Module != "fmt" {
		t.Fatalf("impl method lost declaring module: %+v", methods)
	}
}

func TestBitBackedEnumRangeErrors(t *testing.T) {
	input := `
enum InvalidWidth: bit[0] {
	zero,
}

enum InvalidValue: bit[2] {
	tooLarge = 4,
}

enum ClockSource: bit[2] {
	Internal = 0,
	External = 1,
}

type ClockConfig register[2] {
	Source: ClockSource,
}

@address(0x40000000)
let mut config: ClockConfig

fn InvalidAssignments() void {
	config.Source = 5
	let converted := ClockSource(5)
}
`

	errors := analyzeSource(t, input)
	expected := []string{
		"enum InvalidWidth bit width must be between 1 and 256, got 0 at 2:6",
		"value 4 overflows bit[2] at 7:2",
		"value 5 does not fit in 2-bit enum ClockSource at 23:18",
		"value 5 does not fit in 2-bit enum ClockSource at 24:31",
	}
	assertSemaErrors(t, errors, expected)
}

func TestRegisterRejectsNonBitEnumField(t *testing.T) {
	input := `
enum PlainMode {
	Off,
	On,
}

enum ReservedMode: bit[1] {
	Off,
	On,
}

type InvalidFields register[2] {
	Mode: PlainMode,
	_: ReservedMode,
}
`

	errors := analyzeSource(t, input)
	expected := []string{
		"register field InvalidFields.Mode type must be bit, a bit-backed enum, or a register, got PlainMode at 13:8",
		"reserved register field _ must use bit or bit[N] at 14:5",
	}
	assertSemaErrors(t, errors, expected)
}

func TestBitEnumDynamicConversionRequiresTry(t *testing.T) {
	input := `
enum ClockSource: bit[2] {
	Internal,
	External,
}

fn Convert(value: int) ClockSource {
	return ClockSource(value)
}
`

	errors := analyzeSource(t, input)
	expected := []string{
		"function Convert must return ClockSource, got Result[ClockSource, EnumValueError] at 8:9",
	}
	assertSemaErrors(t, errors, expected)
}

func TestBitEnumFixtures(t *testing.T) {
	valid, err := os.ReadFile("../../testdata/bit_enum_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	assertSemaErrors(t, analyzeSourceRaw(t, string(valid)), nil)

	invalid, err := os.ReadFile("../../testdata/bit_enum_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if errors := analyzeSourceRaw(t, string(invalid)); len(errors) == 0 {
		t.Fatal("expected bit_enum_invalid.sec to produce semantic errors")
	}
}

func TestEnumImplicitIntegerAssignmentsAreInvalid(t *testing.T) {
	input := `
enum Color {
	red,
	green,
}

let c: Color := 1
let i: int := Color.red
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot initialize Color with int at 7:17",
		"cannot initialize int with Color at 8:20",
	}

	assertSemaErrors(t, errors, expected)
}

func TestEnumUnderlyingTypeErrors(t *testing.T) {
	input := `
enum BadUnknown Missing {
	a,
}

enum BadBool bool {
	a,
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"unknown type Missing at 2:17",
		"enum BadBool underlying type must be integer or string, got bool at 6:14",
	}

	assertSemaErrors(t, errors, expected)
}

// rules/declarations/enums.md revision 2.0: string-backed enums are closed
// nominal enums with explicit constant member values and value-class aliases.
func TestStringBackedEnumSemantics(t *testing.T) {
	input := `
module main

enum Program string {
	OneCare = "Zebra OneCare",
	LegacyOneCare = "Zebra OneCare",
	VIQ default = "Z1C+VIQ",
}

let mut Current: Program

fn Name(value: Program) string {
	return string(value)
}

fn Parse(value: string) Result[Program, EnumValueError] {
	return Ok(try Program(value))
}

fn Classify(value: Program) int {
	return match value {
		Program.OneCare => 1
		Program.VIQ => 2
	}
}

fn SwitchClassify(value: Program) int {
	switch value {
		case Program.OneCare:
			return 1
		case Program.VIQ:
			return 2
	}
}
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)

	program := analyzer.Types()["Program"]
	if program.Kind != EnumType || program.Underlying != "string" || program.EnumDefault != "VIQ" {
		t.Fatalf("wrong string enum type: %+v", program)
	}
	if current, ok := analyzer.symbols["Current"]; !ok || current.Type.Name != "Program" || !analyzer.assigned["Current"] {
		t.Fatalf("string enum default initialization was not synthesized: symbol=%+v assigned=%v", current, analyzer.assigned["Current"])
	}
	oneCare := program.EnumConsts["OneCare"]
	legacy := program.EnumConsts["LegacyOneCare"]
	viq := program.EnumConsts["VIQ"]
	if oneCare.StringValue == nil || legacy.StringValue == nil || viq.StringValue == nil ||
		*oneCare.StringValue != "Zebra OneCare" || *legacy.StringValue != "Zebra OneCare" || *viq.StringValue != "Z1C+VIQ" {
		t.Fatalf("wrong string enum constants: %+v", program.EnumConsts)
	}
	if first, _ := enumValueClassKey(oneCare); first == "" {
		t.Fatal("string enum member has no value-class key")
	} else if alias, _ := enumValueClassKey(legacy); first != alias {
		t.Fatalf("string aliases have different value classes: %q and %q", first, alias)
	}
}

func TestStringBackedEnumRejectsInvalidMembersAndConversion(t *testing.T) {
	input := `
enum Missing string {
	Value,
}

enum Wrong string {
	Value = 1,
}

enum Iota string {
	Value = iota,
}

enum Program string {
	OneCare = "Zebra OneCare",
}

let invalid := Program("Unknown")
`
	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, []string{
		"string-backed enum member Missing.Value requires an explicit initializer at 3:2",
		"string-backed enum value Wrong.Value initializer must be a compile-time string constant at 7:10",
		"iota is not available in string-backed enum Iota at 11:10",
		"string value \"Unknown\" is not a declared value of closed enum Program at 18:24",
	})
}

func TestEnumInitializerMustBeIntegerConstant(t *testing.T) {
	input := `
let x := 1

enum Bad {
	a = x,
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"enum value Bad.a initializer must be integer constant at 5:6",
	}

	assertSemaErrors(t, errors, expected)
}

func TestImplPropertyChecks(t *testing.T) {
	input := `
type Speed decimal<m/s>

type Vehicle struct {
	_speed: Speed,
}

impl Vehicle {
	property TopSpeed: Speed {
		get {
			return _speed
		}
	}

	property TopSpeed: Missing {
		set value {
			_speed = value
		}
	}
}

unit m physical
unit s physical
`

	errors := analyzeSource(t, input)

	expected := []string{
		`duplicate property "TopSpeed" in impl Vehicle at 15:11`,
	}

	assertSemaErrors(t, errors, expected)
}

func TestImplPropertyUnknownType(t *testing.T) {
	input := `
type Vehicle struct {
}

impl Vehicle {
	property Missing: UnknownType {
		get {
		}
	}
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"unknown type UnknownType at 6:20",
	}

	assertSemaErrors(t, errors, expected)
}

// rules/declarations/static.md, sections 6-12; properties.md, section 10.
// rules/declarations/static.md section 6: immutable let and static let share
// one associated member category, while bare mutable impl storage is invalid.
func TestImplAssociatedLetAndRedundantStaticInformation(t *testing.T) {
	input := `
module main

type Program string

impl Program {
	let OneCare := "Zebra OneCare"
	static let VIQ := "Z1C+VIQ"
	let mut Invalid := "shared"
}

fn Read() string {
	return Program.OneCare
}
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"mutable associated storage must be declared with static let mut at 9:2",
	})

	oneCare, ok := analyzer.symbols["Program.OneCare"]
	if !ok || oneCare.Mutable || oneCare.Storage != StorageOriginStatic || oneCare.Type.Kind != StringType {
		t.Fatalf("canonical associated let was not registered correctly: %+v", oneCare)
	}
	viq, ok := analyzer.symbols["Program.VIQ"]
	if !ok || viq.Mutable || viq.Storage != StorageOriginStatic || viq.Type.Kind != StringType {
		t.Fatalf("compatibility associated static let was not registered correctly: %+v", viq)
	}
	warnings := analyzer.Warnings()
	if len(warnings) != 1 || warnings[0].ID != diagnostics.RedundantAssociatedStatic || warnings[0].Severity != diagnostics.SeverityInformation ||
		warnings[0].Message != "static is redundant on immutable associated declaration VIQ" {
		t.Fatalf("wrong redundant-static information diagnostic: %+v", warnings)
	}
}

func TestStaticPropertyFrontendSemantics(t *testing.T) {
	valid := `
module main

interface CurrentSource {
    static property Current: int { get set value }
}

type Counter struct { value: int }

impl Counter implements CurrentSource {
    static let mut Storage: int := 1

    static property Current: int {
        get { return Counter.Storage }
        set value { Counter.Storage = value }
    }

    property Instance: int {
        get { return self.value }
    }
}

fn Read() int { return Counter.Current }
fn Update() void { Counter.Current = 2 }
`
	assertSemaErrors(t, analyzeSourceRaw(t, valid), nil)

	invalid := `
module main

type Counter struct { value: int }

impl Counter {
    static let Fixed: int := 1
    static property Current: int {
        get { return self.value }
    }
    property Instance: int { get { return self.value } }
}

let counter := Counter { value: 0 }
let a := counter.Current
let b := Counter.Instance
fn Update() void { Counter.Fixed = 2 }
`
	errors := analyzeSourceRaw(t, invalid)
	wants := []string{
		"undefined variable self",
		"static property Counter.Current must be accessed through type Counter",
		"instance property Counter.Instance requires a value receiver",
		"cannot assign to immutable static storage Counter.Fixed",
	}
	for _, want := range wants {
		found := false
		for _, err := range errors {
			if strings.Contains(err.Message, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing %q in errors: %v", want, errors)
		}
	}
}

// rules/declarations/static.md, sections 15-16.
func TestStaticInitializerValidationAndDependencyOrder(t *testing.T) {
	valid := `
module main

static let First: int := Second + 1
static let Second: int := 2
let result := First
`
	assertSemaErrors(t, analyzeSourceRaw(t, valid), nil)

	invalid := `
module main

fn LoadConfiguration() int { return 1 }

static let RuntimeValue: int := LoadConfiguration()
static let A: int := B
static let B: int := A

fn Cached() int {
    static let Value: int := LoadConfiguration()
    return Value
}

fn Captured(input: int) int {
    static let ValueFromInput: int := input
    return ValueFromInput
}
`
	errors := analyzeSourceRaw(t, invalid)
	wants := []string{
		"static initializer for RuntimeValue must be compile-time evaluable",
		"cyclic static initialization: A -> B -> A",
		"static initializer for Value must be compile-time evaluable",
		"static initializer for ValueFromInput must be compile-time evaluable",
	}
	for _, want := range wants {
		found := false
		for _, err := range errors {
			if strings.Contains(err.Message, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing %q in errors: %v", want, errors)
		}
	}
}

func TestStructLiteralAndFieldAccess(t *testing.T) {
	input := `
type Speed decimal<m/s>

type Vehicle struct {
	_speed: Speed,
}

let speed: Speed := 10
let vehicle := Vehicle{ _speed: speed }
let current := vehicle._speed

unit m physical
unit s physical
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	current := analyzer.symbols["current"]
	if current.Type.Name != "Speed" {
		t.Fatalf("wrong member type. got=%q want=Speed", current.Type.Name)
	}
}

func TestStructLiteralFieldErrors(t *testing.T) {
	input := `
type Speed decimal<m/s>

type Vehicle struct {
	_speed: Speed,
}

let bad := Vehicle{ missing: Speed(1), _speed: "fast" }

unit m physical
unit s physical
`

	errors := analyzeSource(t, input)

	expected := []string{
		`unknown field "missing" in struct Vehicle at 8:21`,
		"cannot initialize field _speed with string at 8:48",
	}

	assertSemaErrors(t, errors, expected)
}

func TestPropertyAccessAndFallibleAssignment(t *testing.T) {
	input := `
type Speed decimal<m/s>

type Vehicle struct {
	_speed: Speed,
}

impl Vehicle {
	property TopSpeed: Speed {
		get {
			return _speed
		}
		try set value {
			_speed = value
		}
	}
}

let speed: Speed := 10
let mut vehicle := Vehicle{ _speed: speed }
let current := vehicle.TopSpeed
fn Test() void {
	vehicle.TopSpeed = speed
}

unit m physical
unit s physical
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)

	expected := []string{
		"assigning fallible property TopSpeed requires try at 23:10",
	}

	assertSemaErrors(t, errors, expected)

	current := analyzer.symbols["current"]
	if current.Type.Name != "Speed" {
		t.Fatalf("wrong property type. got=%q want=Speed", current.Type.Name)
	}
}

// rules/declarations/properties.md, Read and compound access; correction12.md.
func TestWriteOnlyPropertyReadAndCompoundAssignmentAreRejected(t *testing.T) {
	input := `
module main

type Sink struct {
	_value: int,
}

impl Sink {
	property WriteOnly: int {
		set next {
			self._value = next
		}
	}

	property ReadOnly: int {
		get {
			return self._value
		}
	}

	fn ReadInside() int {
		return WriteOnly
	}

	fn UpdateInside() void {
		WriteOnly = 1
		WriteOnly += 1
		ReadOnly += 1
	}
}

let mut sink := Sink { _value: 0 }
let invalidRead := sink.WriteOnly

fn UpdateOutside() void {
	sink.WriteOnly = 1
	sink.WriteOnly += 1
	sink.ReadOnly += 1
}
`

	errors := analyzeSourceRaw(t, input)
	if len(errors) != 6 {
		t.Fatalf("errors = %v, want six accessor diagnostics", errors)
	}
	counts := map[string]int{}
	for _, err := range errors {
		switch {
		case strings.Contains(err.Message, "property WriteOnly has no getter and cannot be used with +="):
			counts["compound-no-getter"]++
		case strings.Contains(err.Message, "property WriteOnly has no getter"):
			counts["read-no-getter"]++
		case strings.Contains(err.Message, "property ReadOnly has no setter"):
			counts["write-no-setter"]++
		}
	}
	for _, expected := range []string{"compound-no-getter", "read-no-getter", "write-no-setter"} {
		if count := counts[expected]; count != 2 {
			t.Fatalf("diagnostic %q occurred %d times in %v, want 2", expected, count, errors)
		}
	}
}

func TestInterfacePropertySetterFallibilityConformance(t *testing.T) {
	input := `
enum ConfigError error {
	Invalid,
}

interface FallibleConfig {
	property Mode: int {
		try set mode
	}
}

interface PlainConfig {
	property Mode: int {
		set mode
	}
}

type Plain struct {}
impl Plain implements FallibleConfig {
	property Mode: int {
		set mode {}
	}
}

type Fallible struct {}
impl Fallible implements PlainConfig {
	property Mode: int {
		try set mode {
			return Err(ConfigError.Invalid)
		}
	}
}
`

	errors := analyzeSource(t, input)
	if len(errors) != 2 {
		t.Fatalf("wrong sema error count: got %d, want 2: %v", len(errors), errors)
	}
	want := []string{
		"type Plain property Mode must provide fallible setter for interface FallibleConfig, got infallible setter",
		"type Fallible property Mode must provide infallible setter for interface PlainConfig, got fallible setter",
	}
	for _, expected := range want {
		found := false
		for _, diagnostic := range errors {
			if strings.Contains(diagnostic.Error(), expected) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing sema error %q in %v", expected, errors)
		}
	}
}

func TestPropertiesValidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/properties_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestPropertiesInvalidSemanticFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/properties_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	const marker = "// Parser recovery: malformed property declarations"
	source := string(input)
	idx := strings.Index(source, marker)
	if idx < 0 {
		t.Fatalf("missing parser recovery marker %q", marker)
	}

	errors := analyzeSourceRaw(t, source[:idx])
	if len(errors) == 0 {
		t.Fatal("expected semantic property errors")
	}
}

func TestTryAssignmentHandlersUseImplicitOk(t *testing.T) {
	input := `
type Speed decimal<m/s>

enum IOError error {
	InvalidValue,
}

type Vehicle struct {
	_speed: Speed,
}

impl Vehicle {
	property TopSpeed: Speed {
		get {
			return _speed
		}
		try set value {
			return Err(IOError.InvalidValue)
		}
	}
}

fn Log(error: IOError) void {
	return
}

fn Test(car: Vehicle, current_speed: Speed) void {
	try car.TopSpeed = current_speed {
		Err(error) => Log(error)
	}
}

unit m physical
unit s physical
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestTryAssignmentAllowsExplicitOkHandler(t *testing.T) {
	input := `
type Speed decimal<m/s>

enum IOError error {
	InvalidValue,
}

type Vehicle struct {
	_speed: Speed,
}

impl Vehicle {
	property TopSpeed: Speed {
		get {
			return _speed
		}
		try set value {
			return Err(IOError.InvalidValue)
		}
	}
}

fn Log(error: IOError) void {
	return
}

fn Test(car: Vehicle, current_speed: Speed) void {
	try car.TopSpeed = current_speed {
		Ok(_) => {}
		Err(error) => Log(error)
	}
}

unit m physical
unit s physical
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestTryExpressionAllowsExplicitOkHandler(t *testing.T) {
	input := `
enum IOError error {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(10)
}

fn Test() int {
	let value := try Read() {
		Ok(v) => v
		Err(error) => 0
	}

	return value
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestPropertyAccessBeforeImplDeclaration(t *testing.T) {
	input := `
type Speed decimal<m/s>

type Vehicle struct {
	_speed: Speed,
}

let speed: Speed := 10
let vehicle := Vehicle{ _speed: speed }
let current := vehicle.TopSpeed

impl Vehicle {
	property TopSpeed: Speed {
		get {
			return _speed
		}
	}
}

unit m physical
unit s physical
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	current := analyzer.symbols["current"]
	if current.Type.Name != "Speed" {
		t.Fatalf("wrong property type. got=%q want=Speed", current.Type.Name)
	}
}

func TestPropertyBodyCanReferenceLaterPropertyInSameImpl(t *testing.T) {
	input := `
type Speed decimal<m/s>

type Vehicle struct {
	_speed: Speed,
}

impl Vehicle {
	property Current: Speed {
		get {
			return TopSpeed
		}
	}

	property TopSpeed: Speed {
		get {
			return _speed
		}
	}
}

unit m physical
unit s physical
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestPropertyGetterCanReturnSelf(t *testing.T) {
	input := `
type Counter struct {
	value: int,
}

impl Counter {
	property Whole: Counter {
		get {
			return self
		}
	}
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestPropertyBodyChecks(t *testing.T) {
	input := `
type Speed decimal<m/s>
type Money decimal<SEK>

type Vehicle struct {
	_speed: Speed,
}

impl Vehicle {
	property TopSpeed: Speed {
		get {
			return _speed
		}
	}

	property MoneyValue: Money {
		get {
			return Money(5.90)
		}
	}

	property BadGet: Speed {
		get {
			return Money(5.90)
		}
	}

	property MissingGet: Speed {
		get {
			_speed = _speed
		}
	}

	property UnknownGet: Speed {
		get {
			return missing
		}
	}

	property BadSet: Speed {
		set value {
			return Err(IOError.InvalidValue)
		}
	}
}

unit SEK currency
unit m physical
unit s physical
`

	errors := analyzeSource(t, input)

	expected := []string{
		"function BadGet.get must return Speed, got Money at 24:11",
		"cannot assign to immutable variable _speed at 30:4",
		"getter MissingGet must return Speed at 28:11",
		"undefined variable missing at 36:11",
		"non-fallible setter BadSet cannot return Err at 41:3",
		"Err can only be returned from Result-returning function at 42:11",
	}

	assertSemaErrors(t, errors, expected)
}

func TestPropertySetterBodyUsesFullSemanticAnalysis(t *testing.T) {
	input := `
module main

type Counter struct {
	value: int,
}

impl Counter {
	property Value: int {
		set next {
			if next {
				value = missing
			}
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"if condition must be bool, got int at 11:7",
		"undefined variable missing at 12:13",
	}
	assertSemaErrors(t, errors, expected)
}

func TestPropertyShortNameFallibleAssignmentRequiresTry(t *testing.T) {
	input := `
module main

enum PropertyError error {
	Rejected,
}

type Counter struct {
	value: int,
}

impl Counter {
	property Checked: int {
		get {
			return value
		}
		try set next {
			return Err(PropertyError.Rejected)
		}
	}

	property Mirror: int {
		set next {
			Checked = next
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"assigning fallible property Checked requires try at 24:4",
	}
	assertSemaErrors(t, errors, expected)
}

func TestPropertyShortNameTryAssignmentUsesPropertyErrorType(t *testing.T) {
	input := `
module main

enum PropertyError error {
	Rejected,
}

type Counter struct {
	value: int,
}

impl Counter {
	property Checked: int {
		get {
			return value
		}
		try set next {
			return Err(PropertyError.Rejected)
		}
	}

	property Mirror: int {
		set next {
			try Checked = next {
				Err(error) => {
					value = 0
				}
			}
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestImplMethodShortFieldWriteMakesSelfMutable(t *testing.T) {
	input := `
module main

type Counter struct {
	value: int,
}

impl Counter {
	fn Increment() void {
		value = value + 1
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestImplMethodShortFieldWriteAcceptsOwnedMutableParameterReceiver(t *testing.T) {
	input := `
module main

type Counter struct {
	value: int,
}

impl Counter {
	fn Increment() void {
		value = value + 1
	}
}

fn Use(counter: Counter) void {
	counter.Increment()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestDiscardedMutableSelfMethodCallInfersMutableReceiver(t *testing.T) {
	input := `
module main

type Counter struct {
	value: int,
}

impl Counter {
	fn Advance() int {
		self.value += 1
		return self.value
	}

	fn Skip() void {
		discard self.Advance()
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestReturnedMutableSelfMethodCallInfersMutableReceiver(t *testing.T) {
	input := `
module main

type Reader struct {
	position: int,
}

impl Reader {
	fn Equal() bool {
		return self.readTwo(1)
	}

	fn readTwo(kind: int) bool {
		self.position += 2
		return kind == 0
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestImplMethodLocalShadowsShortFieldAlias(t *testing.T) {
	input := `
module main

type Cursor struct {
	line: int,
}

impl Cursor {
	fn NextLine() int {
		let mut line := self.line
		line += 1
		return line
	}
}

fn Read(cursor: Cursor) int {
	return cursor.NextLine()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestInvalidInitializerDoesNotCauseUndefinedVariableCascade(t *testing.T) {
	input := `
module main

fn UseInvalidValue() void {
	let value := Missing()
	value
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"unknown function or type Missing at 5:15",
	}
	assertSemaErrors(t, errors, expected)
}

func TestFunctionDeclarations(t *testing.T) {
	input := `
fn add(a: int, b: int) int {
	return a + b
}

fn noop() void {
	return
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	add := analyzer.functions["add"][0]
	if add.ReturnType.Name != "int" || len(add.Parameters) != 2 {
		t.Fatalf("wrong add function: %+v", add)
	}

	noop := analyzer.functions["noop"][0]
	if noop.ReturnType.Name != "void" || len(noop.Parameters) != 0 {
		t.Fatalf("wrong noop function: %+v", noop)
	}
}

func TestOwnedParametersAreMutableReferenceBindingsAreNot(t *testing.T) {
	input := `
fn Normalize(value: int) int {
	value = 0
	return value
}

fn RebindShared(value: ref int) void {
	value = value
}

fn RebindMutable(value: ref mut int) void {
	value = value
}
`

	_, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot assign to immutable variable value at 8:2",
		"cannot assign to immutable variable value at 12:2",
	})
}

func TestConsumingParameterConsumesCopyableArgument(t *testing.T) {
	input := `
module main

fn Consume(-> value: int) void {}

fn Use() void {
	let value := 1
	Consume(<-value)
	discard value
}
`

	errors := analyzeSourceRaw(t, input)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "consumed by call") || !strings.Contains(errors[0].Message, "no longer available") {
		t.Fatalf("wrong consuming-call errors: %v", errors)
	}
}

func TestConsumingParameterRejectsReferencesAndOwnershipOnlyOverload(t *testing.T) {
	input := `
module main

fn Shared(-> value: ref int) void {}
fn Mutable(-> value: ref mut int) void {}

fn Process(value: int) void {}
fn Process(-> value: int) void {}
`

	errors := analyzeSourceRaw(t, input)
	if len(errors) != 3 {
		t.Fatalf("wrong consuming-parameter error count: got %d errors=%v", len(errors), errors)
	}
	want := []string{
		"consuming parameter cannot use ref type",
		"consuming parameter cannot use ref mut type",
		"function overloads cannot differ only by consuming parameter mode",
	}
	for i := range want {
		if !strings.Contains(errors[i].Message, want[i]) {
			t.Fatalf("error %d = %q, want %q", i, errors[i].Message, want[i])
		}
	}
}

func TestFunctionOverloads(t *testing.T) {
	input := `
fn Pick(a: int) int {
	return a
}

fn Pick(a: string) string {
	return a
}

let i := Pick(1)
let s := Pick("hello")
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, []string{
		"static initializer for i must be compile-time evaluable; runtime execution or invocation-local state is not allowed at 10:5",
		"static initializer for s must be compile-time evaluable; runtime execution or invocation-local state is not allowed at 11:5",
	})

	if len(analyzer.functions["Pick"]) != 2 {
		t.Fatalf("wrong overload count. got=%d want=2", len(analyzer.functions["Pick"]))
	}
	if analyzer.symbols["i"].Type.Name != "int" {
		t.Fatalf("wrong i type: %+v", analyzer.symbols["i"])
	}
	if analyzer.symbols["s"].Type.Name != "string" {
		t.Fatalf("wrong s type: %+v", analyzer.symbols["s"])
	}
}

func TestFunctionOverloadsIncludeUnits(t *testing.T) {
	input := `
unit V decimal physical
unit W decimal physical

impl V {
	dimension: [mass^1, length^2, time^-3, electric_current^-1]
	scale: 1
	system: SI
}

impl W {
	dimension: [mass^1, length^2, time^-3]
	scale: 1
	system: SI
}

fn Convert(value: decimal<V>) decimal<V> {
	return value
}

fn Convert(value: decimal<W>) decimal<W> {
	return value
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	if len(analyzer.functions["Convert"]) != 2 {
		t.Fatalf("wrong overload count. got=%d want=2", len(analyzer.functions["Convert"]))
	}
	if analyzer.functions["Convert"][0].Parameters[0].Type.Unit == analyzer.functions["Convert"][1].Parameters[0].Type.Unit {
		t.Fatalf("unit overloads collapsed: %+v", analyzer.functions["Convert"])
	}
}

func TestFunctionOverloadsIncludeNumericKindAndUnits(t *testing.T) {
	input := `
unit s decimal physical
unit Hz decimal physical

impl s {
	dimension: [time^1]
	scale: 1
	system: SI
}

impl Hz {
	dimension: [time^-1]
	scale: 1
	system: SI
}

fn Convert(value: decimal<s>) decimal<Hz> {
	return decimal<Hz>(1.0)
}

fn Convert(value: float<s>) float<Hz> {
	return float<Hz>(1.0g)
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	if len(analyzer.functions["Convert"]) != 2 {
		t.Fatalf("wrong overload count. got=%d want=2", len(analyzer.functions["Convert"]))
	}
}

func TestDuplicateFunctionSignature(t *testing.T) {
	input := `
fn Pick(a: int) int {
	return a
}

fn Pick(value: int) int {
	return value
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		`duplicate function "Pick" with same signature at 6:4, previous declaration at 2:4`,
	}

	assertSemaErrors(t, errors, expected)
}

func TestReturnTypeCannotDistinguishOverload(t *testing.T) {
	input := `
fn Pick(a: int) int {
	return a
}

fn Pick(value: int) string {
	return "value"
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		`duplicate function "Pick" with same signature at 6:4, previous declaration at 2:4`,
	}

	assertSemaErrors(t, errors, expected)
}

func TestAmbiguousFunctionOverload(t *testing.T) {
	input := `
type Percent int range 0..100
type Score int range 0..100

fn Pick(a: Percent) Percent {
	return a
}

fn Pick(a: Score) Score {
	return a
}

let p := Pick(10)
`

	errors := analyzeSource(t, input)

	expected := []string{
		"static initializer for p must be compile-time evaluable; runtime execution or invocation-local state is not allowed at 13:5",
		"ambiguous call to Pick at 13:10",
	}

	assertSemaErrors(t, errors, expected)
}

func TestOverloadExactMatchPreferredOverConversion(t *testing.T) {
	input := `
fn Print(value: int) int {
	return value
}

fn Print(value: int64) int64 {
	return value
}

let x := Print(10)
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, []string{
		"static initializer for x must be compile-time evaluable; runtime execution or invocation-local state is not allowed at 10:5",
	})

	if analyzer.symbols["x"].Type.Name != "int" {
		t.Fatalf("wrong x type. got=%q want=int", analyzer.symbols["x"].Type.Name)
	}
}

func TestOverloadNamedTypesRemainDistinct(t *testing.T) {
	input := `
type Percent int range 0..100

fn Set(value: int) int {
	return value
}

fn Set(value: Percent) Percent {
	return value
}

let p: Percent := 50
let selectedNamed := Set(p)
let selectedInt := Set(50)
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, []string{
		"static initializer for selectedNamed must be compile-time evaluable; runtime execution or invocation-local state is not allowed at 13:5",
		"static initializer for selectedInt must be compile-time evaluable; runtime execution or invocation-local state is not allowed at 14:5",
	})

	if analyzer.symbols["selectedNamed"].Type.Name != "Percent" {
		t.Fatalf("wrong selectedNamed type. got=%q want=Percent", analyzer.symbols["selectedNamed"].Type.Name)
	}
	if analyzer.symbols["selectedInt"].Type.Name != "int" {
		t.Fatalf("wrong selectedInt type. got=%q want=int", analyzer.symbols["selectedInt"].Type.Name)
	}
}

func TestFunctionCalls(t *testing.T) {
	input := `
fn Add(a: int, b: int) int {
	return a + b
}

let x := Add(1, 2)
let y: int := Add(3, 4)
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, []string{
		"static initializer for x must be compile-time evaluable; runtime execution or invocation-local state is not allowed at 6:5",
		"static initializer for y must be compile-time evaluable; runtime execution or invocation-local state is not allowed at 7:5",
	})

	if analyzer.symbols["x"].Type.Name != "int" {
		t.Fatalf("wrong x type: %+v", analyzer.symbols["x"])
	}
	if analyzer.symbols["y"].Type.Name != "int" {
		t.Fatalf("wrong y type: %+v", analyzer.symbols["y"])
	}
}

func TestFunctionCallTypeErrors(t *testing.T) {
	input := `
fn Add(a: int, b: int) int {
	return a + b
}

let v: float := Add(2, 4)
let w: int := Add(1.5, 1.5)
`

	errors := analyzeSource(t, input)

	expected := []string{
		"static initializer for v must be compile-time evaluable; runtime execution or invocation-local state is not allowed at 6:5",
		"static initializer for w must be compile-time evaluable; runtime execution or invocation-local state is not allowed at 7:5",
		"cannot initialize float with int at 6:17",
		"argument 1 to Add must be int, got decimal at 7:19",
		"argument 2 to Add must be int, got decimal at 7:24",
	}

	assertSemaErrors(t, errors, expected)
}

func TestFunctionCallWrongArgumentCount(t *testing.T) {
	input := `
fn Add(a: int, b: int) int {
	return a + b
}

let wrongCount := Add(1)
`

	errors := analyzeSource(t, input)

	expected := []string{
		"static initializer for wrongCount must be compile-time evaluable; runtime execution or invocation-local state is not allowed at 6:5",
		"function Add expects 2 arguments, got 1 at 6:19",
	}

	assertSemaErrors(t, errors, expected)
}

func TestCallSyntaxStillSupportsConversions(t *testing.T) {
	input := `
enum Color {
	red,
	green,
}

let explicitColor: Color := Color(1)
let explicitInt: int := int(Color.green)
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestExplicitNumericToBoolConversions(t *testing.T) {
	input := `
let a: bool := bool(0)
let b: bool := bool(1)
let c: bool := bool(-1)
let d: bool := bool(0.0)
let e: bool := bool(0.1)
let i8: int8 := -1
let u8: uint8 := 1
let f32: float32 := 0.0
let x: bool := bool(i8)
let y: bool := bool(u8)
let z: bool := bool(f32)

fn Test(value: int) void {
	if value {
	}
	return
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"if condition must be bool, got int at 15:5",
	}

	assertSemaErrors(t, errors, expected)
}

func TestFunctionDuplicateParameter(t *testing.T) {
	input := `
fn bad(a: int, a: int) int {
	return a
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		`duplicate parameter "a" at 2:16`,
	}

	assertSemaErrors(t, errors, expected)
}

func TestFunctionMustReturn(t *testing.T) {
	input := `
fn bad() int {
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"function bad must return int at 2:4",
	}

	assertSemaErrors(t, errors, expected)
}

func TestFunctionReturnTypeMismatch(t *testing.T) {
	input := `
fn bad() int {
	return true
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"function bad must return int, got bool at 3:9",
	}

	assertSemaErrors(t, errors, expected)
}

func TestFunctionBoolExpressions(t *testing.T) {
	input := `
fn IsPositive(a: int) bool {
	return a > 0
}

fn Logic(a: bool, b: bool) bool {
	return !a || (a && b)
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestLogicalOperatorsRequireBool(t *testing.T) {
	input := `
fn BadAnd(a: int) bool {
	return a && true
}

fn BadNot(a: int) bool {
	return !a
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"operator && requires bool operands at 3:11",
		"operator ! requires bool operand at 7:9",
	}

	assertSemaErrors(t, errors, expected)
}

func TestEqualityRequiresCompatibleTypes(t *testing.T) {
	input := `
fn ValidBoolEquality(value: bool, number: int) void {
	if value == bool(number) {
	}
	return
}

fn InvalidBoolEquality(value: bool, number: int) void {
	if value == number {
	}
	return
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot compare bool and int at 9:11",
	}

	assertSemaErrors(t, errors, expected)
}

func TestFunctionUnknownReturnTypeDoesNotCascade(t *testing.T) {
	input := `
fn UnknownReturn() UnknownType {
	return 0
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"unknown type UnknownType at 2:20",
	}

	assertSemaErrors(t, errors, expected)
}

func TestResultReturnExpressions(t *testing.T) {
	input := `
enum IOError error {
	InvalidValue,
}

fn OkResult() Result[int, IOError] {
	return Ok(1)
}

fn VoidOkResult() Result[void, IOError] {
	return Ok()
}

fn ErrResult() Result[int, IOError] {
	return Err(IOError.InvalidValue)
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestResultReturnExpressionErrors(t *testing.T) {
	input := `
enum IOError error {
	InvalidValue,
}

fn BadOk() Result[int, IOError] {
	return Ok(IOError.InvalidValue)
}

fn MissingOkValue() Result[int, IOError] {
	return Ok()
}

fn BadErr() Result[int, IOError] {
	return Err(1)
}

fn Plain() Result[int, IOError] {
	return 1
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"function BadOk must return Ok(int), got Ok(IOError) at 7:19",
		"function MissingOkValue must return Ok(int), got Ok() at 11:9",
		"function BadErr must return Err(IOError), got Err(int) at 15:13",
		"function Plain returning Result[int, IOError] must return Ok(...) or Err(...) at 19:9",
	}

	assertSemaErrors(t, errors, expected)
}

func TestTryResultExpression(t *testing.T) {
	input := `
module main

type Speed decimal<m/s>

enum IOError error {
	FileNotFound,
	AccessDenied,
	InvalidValue,
}

fn CalculateSpeed() Result[Speed, IOError] {
	return Ok(Speed(42.5))
}

fn FailCalculation() Result[Speed, IOError] {
	return Err(IOError.InvalidValue)
}

fn UseResult() Result[Speed, IOError] {
	let speed := try CalculateSpeed()

	return Ok(speed)
}

unit m physical
unit s physical
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestResultAndTryErrors(t *testing.T) {
	input := `
module main

type Speed decimal<m/s>

enum IOError error {
	FileNotFound,
	AccessDenied,
	InvalidValue,
}

fn WrongOkType() Result[Speed, IOError] {
	return Ok(IOError.InvalidValue)
}

fn WrongErrType() Result[Speed, IOError] {
	return Err(Speed(10))
}

fn PlainReturn() Result[Speed, IOError] {
	return Speed(10)
}

fn InvalidTry() Speed {
	let speed := try Speed(10)

	return speed
}

unit m physical
unit s physical
`

	errors := analyzeSource(t, input)

	expected := []string{
		"function WrongOkType must return Ok(Speed), got Ok(IOError) at 13:19",
		"function WrongErrType must return Err(IOError), got Err(Speed) at 17:13",
		"function PlainReturn returning Result[Speed, IOError] must return Ok(...) or Err(...) at 21:9",
		"try requires Result expression at 25:15",
	}

	assertSemaErrors(t, errors, expected)
}

func TestTryRequiresCompatibleFunctionResultContext(t *testing.T) {
	input := `
module main

type Speed decimal<m/s>

enum IOError error {
	InvalidValue,
}

enum ParseError error {
	InvalidNumber,
}

fn ReadSpeed() Result[Speed, IOError] {
	return Ok(Speed(10))
}

fn ParseSpeed() Result[Speed, ParseError] {
	return Err(ParseError.InvalidNumber)
}

fn WrongPropagation() Result[Speed, IOError] {
	let speed := try ParseSpeed()
	return Ok(speed)
}

fn CannotPropagate() Speed {
	return try ReadSpeed()
}

unit m physical
unit s physical
`

	errors := analyzeSource(t, input)

	expected := []string{
		"bodyless try propagates ParseError with return Err, but this function returns Result[Speed, IOError]; add a local try handler or map ParseError to IOError at 23:15",
		"bodyless try propagates IOError with return Err, but this function returns Speed; add a local try handler or change the function return type to Result[Speed, IOError] at 28:9",
	}

	assertSemaErrors(t, errors, expected)
}

func TestTryHandlersCanHandleLocally(t *testing.T) {
	input := `
module main

type Speed decimal<m/s>

enum IOError error {
	InvalidValue,
}

enum ParseError error {
	InvalidNumber,
}

fn ParseSpeed() Result[Speed, ParseError] {
	return Err(ParseError.InvalidNumber)
}

fn UseFallback() Speed {
	let speed := try ParseSpeed() {
		Err(error) => Speed(0)
	}
	return speed
}

fn ConvertError() Result[Speed, IOError] {
	let speed := try ParseSpeed() {
		Err(error) => return Err(IOError.InvalidValue)
	}
	return Ok(speed)
}

unit m physical
unit s physical
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestTryHandlersCanUseExplicitMatchWrapper(t *testing.T) {
	input := `
module main

type Speed decimal<m/s>

enum IOError error {
	InvalidValue,
}

fn ReadSpeed() Result[Speed, IOError] {
	return Err(IOError.InvalidValue)
}

fn UseFallback() Speed {
	let speed := try ReadSpeed() {
		match {
			Err(IOError.InvalidValue) => Speed(0)
			Err(error) => Speed(1)
		}
	}
	return speed
}

unit m physical
unit s physical
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestTryHandlerErrors(t *testing.T) {
	input := `
module main

type Speed decimal<m/s>
type Money decimal<SEK>

enum IOError error {
	InvalidValue,
	AccessDenied,
}

fn ReadSpeed() Result[Speed, IOError] {
	return Err(IOError.InvalidValue)
}

fn WrongFallback() Speed {
	let speed := try ReadSpeed() {
		Err(error) => Money(0)
	}
	return speed
}

fn MissingHandler() Speed {
	let speed := try ReadSpeed() {
		Err(IOError.InvalidValue) => Speed(0)
	}
	return speed
}

unit SEK currency
unit m physical
unit s physical
`

	errors := analyzeSource(t, input)

	expected := []string{
		"try handler must produce Speed, got Money at 18:17",
		"non-exhaustive try handlers for IOError at 24:15",
	}

	assertSemaErrors(t, errors, expected)
}

func TestErrorHandlingValidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/errorhandling_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestDeferStatementValid(t *testing.T) {
	input := `
fn Cleanup() void {
	return
}

fn Test() void {
	let mut value := 1
	defer {
		value = 2
		Cleanup()
	}
	value = 3
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestDeferDelayedUseRejectsLaterMove(t *testing.T) {
	input := `
module main

type Handle struct {
	view: ref mut int,
}

fn Consume(handle: Handle) void {
}

fn Invalid() void {
	let mut value := 1
	let handle := Handle{ view: ref mut value }
	defer {
		Consume(<-handle)
	}
	let moved :<- handle
	discard moved
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"cannot move handle while it is required by defer at 17:16, previous declaration at 15:13",
	}
	assertSemaErrors(t, errors, expected)
}

func TestDeferDelayedUseRejectsLaterDiscard(t *testing.T) {
	input := `
fn Invalid() void {
	let value := 1
	defer {
		let observed := value
	}
	discard value
}
`

	errors := analyzeSource(t, input)
	expected := []string{
		"cannot discard value while it is required by defer at 7:10, previous declaration at 5:19",
	}
	assertSemaErrors(t, errors, expected)
}

func TestConditionalDeferDelayedUseRejectsLaterMove(t *testing.T) {
	input := `
module main

type Handle struct {
	view: ref mut int,
}

fn Consume(handle: Handle) void {
}

fn Invalid(condition: bool) void {
	let mut value := 1
	let handle := Handle{ view: ref mut value }
	if condition {
		defer {
			Consume(<-handle)
		}
	}
	let moved :<- handle
	discard moved
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"cannot move handle while it is required by defer at 19:16, previous declaration at 16:14",
	}
	assertSemaErrors(t, errors, expected)
}

func TestDeferLocalBindingDoesNotCaptureOuterBinding(t *testing.T) {
	input := `
module main

type Handle struct {
	view: ref mut int,
}

fn Consume(handle: Handle) void {
}

fn Valid() void {
	let mut outerValue := 1
	let handle := Handle{ view: ref mut outerValue }
	defer {
		let mut innerValue := 2
		let inner := Handle{ view: ref mut innerValue }
		Consume(<-inner)
	}
	let moved :<- handle
	discard moved
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestDeferInvalidControlFlow(t *testing.T) {
	input := `
fn Test() void {
	defer {
		break
	}
	defer {
		continue
	}
	defer {
		fallthrough
	}
	defer {
		defer {
		}
	}
}
`

	errors := analyzeSource(t, input)
	expected := []string{
		"break is not allowed inside defer at 4:3",
		"continue is not allowed inside defer at 7:3",
		"fallthrough is not allowed inside defer at 10:3",
		"defer is not allowed inside defer at 13:3",
	}
	assertSemaErrors(t, errors, expected)
}

func TestDeferRejectsReturn(t *testing.T) {
	input := `
fn Test() void {
	defer {
		return
	}
}
`

	expected := []string{
		"return is not allowed inside defer at 4:3",
	}
	assertSemaErrors(t, analyzeSource(t, input), expected)
}

func TestDeferRejectsTryPropagation(t *testing.T) {
	input := `
enum IOError error {
	Failed,
}

fn Close() Result[int, IOError] {
	return Ok(0)
}

fn Test() Result[int, IOError] {
	defer {
		try Close()
	}
	return Ok(1)
}
`

	errors := analyzeSource(t, input)
	expected := []string{
		"bodyless try cannot propagate from inside defer; add a local try handler at 12:3",
	}
	assertSemaErrors(t, errors, expected)
}

func TestDeferRejectsUnhandledResultExpression(t *testing.T) {
	input := `
enum CleanupError error {
	failed,
}

type Resource struct {
	id: int,
}

fn CloseResource(resource: Resource) Result[void, CleanupError] {
	return Ok()
}

fn Test(resource: Resource) void {
	defer {
		CloseResource(resource)
	}
}
`

	errors := analyzeSource(t, input)
	expected := []string{
		"unhandled Result inside defer; handle it or discard it explicitly at 16:3",
	}
	assertSemaErrors(t, errors, expected)
}

func TestDeferOutsideFunction(t *testing.T) {
	input := `
module main

defer {
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"defer is only valid inside functions at 4:1",
	}
	assertSemaErrors(t, errors, expected)
}

func TestDeferInsideLoopWarning(t *testing.T) {
	input := `
fn Test() void {
	for {
		defer {
		}
		break
	}
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)
	warnings := analyzer.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("wrong warning count. got=%d warnings=%v", len(warnings), warnings)
	}
	want := "defer inside loop registers once per execution and runs at function exit at 4:3"
	if warnings[0].Error() != want {
		t.Fatalf("wrong warning. got=%q want=%q", warnings[0].Error(), want)
	}
}

func TestEnumValidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/enum_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestGenericStructAndFunctionHeaders(t *testing.T) {
	// rules/declarations/struct.md section 4 requires generic specialization to
	// retain ordered, open-key field-tag metadata while substituting field types.
	input := `
module main

type Stack[T] struct {
	value: T ` + "`wire:\"value\" json:\"payload\"`" + `,
}

type Pair[A, B] struct {
	first: A,
	second: B,
}

fn Identity[T](value: T) T {
	return value
}

fn Use(value: Stack[int], pair: Pair[string, int]) void {
}

fn Read(value: Stack[int]) int {
	return value.value
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)

	stack := analyzer.types["Stack"]
	if len(stack.GenericParameters) != 1 || stack.GenericParameters[0] != "T" {
		t.Fatalf("wrong Stack generic params: %+v", stack.GenericParameters)
	}
	if len(stack.Fields) != 1 || stack.Fields[0].Type.Kind != GenericType || stack.Fields[0].Type.Name != "T" {
		t.Fatalf("wrong Stack field type: %+v", stack.Fields)
	}

	functions := analyzer.functions["Identity"]
	if len(functions) != 1 {
		t.Fatalf("wrong Identity function count: %d", len(functions))
	}
	if functions[0].Parameters[0].Type.Kind != GenericType || functions[0].ReturnType.Kind != GenericType {
		t.Fatalf("Identity did not preserve generic types: %+v", functions[0])
	}

	read := analyzer.functions["Read"][0]
	paramType := read.Parameters[0].Type
	if typeDisplayName(paramType) != "Stack[int]" {
		t.Fatalf("wrong instantiated Stack type: %+v display=%s", paramType, typeDisplayName(paramType))
	}
	if len(paramType.Fields) != 1 || paramType.Fields[0].Type.Name != "int" {
		t.Fatalf("Stack[int] field was not substituted: %+v", paramType.Fields)
	}
	if tags := paramType.Fields[0].Tags; len(tags) != 2 || tags[0].Key != "wire" || tags[0].Value != "value" || tags[1].Key != "json" || tags[1].Value != "payload" {
		t.Fatalf("Stack[int] field tags were not preserved: %+v", tags)
	}
}

func TestGenericHeaderErrors(t *testing.T) {
	input := `
module main

type Duplicate[T, T] struct {
	value: T,
}

type Box[T] struct {
	value: T,
}

fn BadArity(value: Box[int, string]) void {
}

fn NonGeneric(value: int[string]) void {
}

fn MissingArgs(value: Box) void {
}
`

	_, errors := analyzeSourceWithAnalyzerRaw(t, input)
	expected := []string{
		`duplicate generic parameter "T" at 4:19`,
		"Box requires 1 generic arguments, got 2 at 12:20",
		"int is not generic at 15:22",
		"Box requires 1 generic arguments, got 0 at 18:23",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericStructInstantiationsAreDistinct(t *testing.T) {
	input := `
module main

type Box[T] struct {
	value: T,
}

fn Bad(value: Box[int]) Box[string] {
	return value
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"function Bad must return Box[string], got Box[int] at 9:9",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericFunctionCallInference(t *testing.T) {
	input := `
module main

type Box[T] struct {
	value: T,
}

fn Identity[T](value: T) T {
	return value
}

fn Unbox[T](box: Box[T]) T {
	return box.value
}

fn UseIdentity() int {
	let value := Identity(10)
	return value
}

fn UseNested(box: Box[string]) string {
	return Unbox(box)
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestGenericFunctionInferenceErrors(t *testing.T) {
	input := `
module main

fn Same[T](left: T, right: T) T {
	return left
}

fn Test() int {
	return Same(1, "hello")
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"cannot infer generic arguments for Same at 9:9",
	}
	assertSemaErrors(t, errors, expected)
}

func TestExplicitGenericFunctionCall(t *testing.T) {
	input := `
module main

fn Identity[T](value: T) T {
	return value
}

fn UseInt() int {
	return Identity[int](10)
}

fn UseString() string {
	return Identity[string]("hello")
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestExplicitGenericFunctionCallErrors(t *testing.T) {
	input := `
module main

fn Identity[T](value: T) T {
	return value
}

fn NonGeneric(value: int) int {
	return value
}

fn WrongArgument() int {
	return Identity[int]("hello")
}

fn WrongArity() int {
	return Identity[int, string](10)
}

fn GenericArgumentsOnNonGenericFunction() int {
	return NonGeneric[int](10)
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"argument 1 to Identity must be int, got string at 13:23",
		"Identity requires 1 explicit generic arguments, got 2 at 17:9",
		"function NonGeneric is not generic at 21:9",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericInstanceCaching(t *testing.T) {
	input := `
module main

type Box[T] struct {
	value: T,
}

fn TakeBoxes(first: Box[int], second: Box[int], third: Box[string]) void {
}

fn Identity[T](value: T) T {
	return value
}

fn Use() void {
	let a := Identity(1)
	let b := Identity(2)
	let c := Identity("hello")
	let d := Identity[int](3)
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)

	if got := len(analyzer.genericTypeInstances); got != 2 {
		t.Fatalf("wrong generic type instance count. got=%d instances=%+v", got, analyzer.genericTypeInstances)
	}
	if got := len(analyzer.genericFuncInstances); got != 2 {
		t.Fatalf("wrong generic function instance count. got=%d instances=%+v", got, analyzer.genericFuncInstances)
	}
}

func TestGenericDirectRecursiveStorageErrors(t *testing.T) {
	input := `
module main

type Node[T] struct {
	next: Node[T],
}

fn Use(value: Node[int]) void {
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"recursive generic type Node[T] has infinite size at 5:2",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericArrayRecursiveStorageErrors(t *testing.T) {
	input := `
module main

type Node[T] struct {
	children: [2]Node[T],
}

fn Use(value: Node[int]) void {
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"recursive generic type Node[T] has infinite size at 5:2",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericChangingRecursiveInstantiationErrors(t *testing.T) {
	input := `
module main

type Infinite[T] struct {
	next: Infinite[[]T],
}

fn Use(value: Infinite[int]) void {
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"recursive generic instantiation does not converge for Infinite at 5:2",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericSliceRecursiveStorageAllowed(t *testing.T) {
	input := `
module main

type Node[T] struct {
	children: ref Node[T][],
}

fn Use(value: Node[int]) void {
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestGenericImplProperty(t *testing.T) {
	input := `
module main

type Box[T] struct {
	value: T,
}

impl Box[T] {
	property Value: T {
		get {
			return value
		}
	}
}

fn Use(box: Box[int]) int {
	return box.Value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestGenericImplErrors(t *testing.T) {
	input := `
module main

type Box[T] struct {
	value: T,
}

impl Box[U] {
}

impl Box {
}

impl Box[T] {
	fn Map[U](value: U) void {
		return
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"unknown generic parameter U in impl target Box at 8:10",
		"Box requires 1 generic arguments, got 0 at 11:6",
		"generic methods with additional type parameters are not supported yet at 15:5",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericExpectedResultInference(t *testing.T) {
	input := `
module main

enum IOError error {
	failed,
}

fn Fail[T]() Result[T, IOError] {
	return Err(IOError.failed)
}

fn UseLet() void {
	let value: Result[int, IOError] := Fail()
}

fn UseAssignment() void {
	let mut value: Result[string, IOError] := Err(IOError.failed)
	value = Fail()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestGenericExpectedResultInferenceDoesNotOverrideArguments(t *testing.T) {
	input := `
module main

enum IOError error {
	failed,
}

fn Wrap[T](value: T) Result[T, IOError] {
	return Ok(value)
}

fn Bad() void {
	let value: Result[string, IOError] := Wrap(1)
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"cannot initialize Result[string, IOError] with Result[int, IOError] at 13:40",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericConstraintDeclarationErrors(t *testing.T) {
	input := `
module main

type BadTypeConstraint[T: int] struct {
	value: T,
}

type UnknownTypeConstraint[T: MissingConstraint] struct {
	value: T,
}

fn BadFunctionConstraint[T: string](value: T) void {
	discard value
}

fn UnknownFunctionConstraint[T: MissingConstraint](value: T) void {
	discard value
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"generic constraint int is not an interface at 4:27",
		"unknown generic constraint MissingConstraint for T at 8:31",
		"generic constraint string is not an interface at 12:29",
		"unknown generic constraint MissingConstraint for T at 16:33",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericUnionTypeReferences(t *testing.T) {
	input := `
module main

type Maybe[T] union {
	Some(T)
	None
}

fn Empty[T]() Maybe[T] {
	return Maybe.None
}

fn Use() Maybe[int] {
	return Empty[int]()
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)

	maybe := analyzer.types["Maybe"]
	if maybe.Kind != UnionType || len(maybe.GenericParameters) != 1 {
		t.Fatalf("wrong Maybe type: %+v", maybe)
	}
	use := analyzer.functions["Use"][0]
	if typeDisplayName(use.ReturnType) != "Maybe[int]" {
		t.Fatalf("wrong Use return type: %+v display=%s", use.ReturnType, typeDisplayName(use.ReturnType))
	}
	if len(use.ReturnType.UnionVariants) != 2 || use.ReturnType.UnionVariants[0].Payload == nil || use.ReturnType.UnionVariants[0].Payload.Name != "int" {
		t.Fatalf("wrong instantiated union variants: %+v", use.ReturnType.UnionVariants)
	}
}

func TestGenericUnionPayloadVariantRequiresPayload(t *testing.T) {
	input := `
module main

type Maybe[T] union {
	Some(T)
	None
}

fn Bad() Maybe[int] {
	return Maybe.Some
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"union variant Maybe[int].Some requires payload at 10:15",
	}
	assertSemaErrors(t, errors, expected)
}

func TestRecursiveGenericUnionStorageIsRejected(t *testing.T) {
	input := `
module main

type Direct[T] union {
	Next(Direct[T]),
	End,
}

type Named[T] union {
	Next { value: Named[T] },
	End,
}

type ArrayWrapped[T] union {
	Next(ArrayWrapped[T][2]),
	End,
}

type Diverging[T] union {
	Next(Diverging[T[]]),
	End,
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"recursive generic type Direct[T] has infinite size at 5:2",
		"recursive generic type Named[T] has infinite size at 10:9",
		"recursive generic type ArrayWrapped[T] has infinite size at 15:2",
		"recursive generic instantiation does not converge for Diverging at 20:2",
	})
}

func TestGenericUnionPayloadConstructor(t *testing.T) {
	input := `
module main

type Maybe[T] union {
	Some(T)
	None
}

fn SomeInt() Maybe[int] {
	return Maybe.Some(10)
}

fn LetSome() void {
	let value: Maybe[string] := Maybe.Some("hello")
	let inferred := Maybe.Some(10)
	discard value
	discard inferred
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestBuiltinOptionShorthandConstructors(t *testing.T) {
	input := `
module main

enum IOError error {
	failed,
}

fn SomeError() Option[IOError] {
	return Some(IOError.failed)
}

fn NoError() Option[IOError] {
	return None
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestLinuxPlatformErrorFile(t *testing.T) {
	input, err := os.ReadFile("../../sec/platform/linux/error.sec")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestGenericUnionPayloadConstructorErrors(t *testing.T) {
	input := `
module main

type Maybe[T] union {
	Some(T)
	None
}

fn WrongPayload() Maybe[int] {
	return Maybe.Some("hello")
}

fn MissingPayload() Maybe[int] {
	return Maybe.Some()
}

fn ExtraPayload() Maybe[int] {
	return Maybe.None(1)
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"union variant Maybe[int].Some payload must be int, got string at 10:20",
		"union variant Maybe[int].Some expects 1 argument, got 0 at 14:14",
		"union variant Maybe[int].None expects 0 arguments, got 1 at 18:14",
	}
	assertSemaErrors(t, errors, expected)
}

func TestUnionPayloadConstructorChecksIntegerRepresentability(t *testing.T) {
	input := `
module main

type Small union {
	Value(uint8),
	Named {
		n: uint8,
	},
}

fn Test() void {
	let maximum := Small.Value(255)
	let overflow := Small.Value(256)
	let named := Small.Named { n: 300 }
	discard maximum
	discard overflow
	discard named
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"value 256 overflows uint8 at 13:30",
		"value 300 overflows uint8 at 14:32",
	})
}

func TestUnionValidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/union_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestUnionDefaultsAndOmittedDefaultablePayloadFields(t *testing.T) {
	input := `
module main

type State union {
	Idle default
	Running
}

type Value union {
	Number(int) default
	Text(string)
}

type Position union {
	Unknown
	Known {
		x: int,
		label: string,
	} default
}

fn Use() void {
	let mut state: State
	let mut value: Value
	let mut position: Position
	let explicit := Position.Known { x: 10 }
	discard state
	discard value
	discard position
	discard explicit
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)

	state := analyzer.types["State"]
	if state.UnionDefault != "Idle" {
		t.Fatalf("State default variant = %q, want Idle", state.UnionDefault)
	}
	if display, kind, ok := DefaultValueDisplay(state); !ok || kind != UnionDefault || display != "State.Idle" {
		t.Fatalf("State default = %q, %q, %v", display, kind, ok)
	}
	value := analyzer.types["Value"]
	if display, kind, ok := DefaultValueDisplay(value); !ok || kind != UnionDefault || display != "Value.Number(0)" {
		t.Fatalf("Value default = %q, %q, %v", display, kind, ok)
	}
	position := analyzer.types["Position"]
	if display, kind, ok := DefaultValueDisplay(position); !ok || kind != UnionDefault || display != `Position.Known { x: 0, label: "" }` {
		t.Fatalf("Position default = %q, %q, %v", display, kind, ok)
	}
}

func TestUnionRejectsMultipleAndNonDefaultableDefaults(t *testing.T) {
	input := `
module main

type Multiple union {
	First default
	Second default
}

type Invalid union {
	Borrowed(ref int) default
	None
}
`

	errors := analyzeSourceRaw(t, input)
	if len(errors) != 2 {
		t.Fatalf("wrong union default error count: got %d errors=%v", len(errors), errors)
	}
	if !strings.Contains(errors[0].Message, "may only mark one variant default") || !strings.Contains(errors[1].Message, "cannot be default-constructed") {
		t.Fatalf("wrong union default errors: %v", errors)
	}
}

func TestNonDefaultableMutableUnionUsesLegacyEmptyAssignmentState(t *testing.T) {
	valid := `
module main

type State union {
	Idle
	Running
}

fn Use() void {
	let mut state: State
	state = State.Idle
	discard state
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, valid), nil)

	invalid := `
module main

type State union {
	Idle
	Running
}

fn Consume(state: State) void {}

fn Use() void {
	let mut state: State
	Consume(state)
}
`
	errors := analyzeSourceRaw(t, invalid)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "is unassigned") {
		t.Fatalf("wrong empty union use diagnostics: %v", errors)
	}
}

func TestUnionInvalidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/union_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	if len(errors) == 0 {
		t.Fatal("expected union_invalid.sec to produce semantic errors")
	}
}

func TestGenericsValidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/generics_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestGenericsInvalidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/generics_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, supportedGenericsInvalidFixture(string(input)))
	expected := []string{
		`duplicate generic parameter "T" at 11:19`,
		"unknown generic parameter U in impl target Box at 105:10",
		"Box requires 1 generic arguments, got 0 at 108:6",
		"recursive generic type Node[T] has infinite size at 98:5",
		"recursive generic instantiation does not converge for Infinite at 102:5",
		"generic methods with additional type parameters are not supported yet at 112:8",
		"Box requires 1 generic arguments, got 0 at 61:35",
		"Box requires 1 generic arguments, got 2 at 64:35",
		"int is not generic at 67:44",
		"function DifferentInstantiations must return Box[string], got Box[int] at 71:12",
		"cannot infer generic arguments for Same at 76:12",
		"argument 1 to Identity must be int, got string at 82:26",
		"Identity requires 1 explicit generic arguments, got 2 at 86:12",
		"function NonGeneric is not generic at 94:12",
	}
	assertSemaErrors(t, errors, expected)
}

func TestImplValidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/impl_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestImplInvalidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/impl_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	if len(errors) == 0 {
		t.Fatal("expected impl_invalid.sec to produce semantic errors")
	}
}

func TestEventValidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/event_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestEventInvalidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/event_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	if len(errors) == 0 {
		t.Fatal("expected event_invalid.sec to produce semantic errors")
	}
}

func supportedGenericsInvalidFixture(input string) string {
	const marker = "// -----------------------------------------------------------------------------\n// Duplicate generic parameters\n// -----------------------------------------------------------------------------"
	idx := strings.LastIndex(input, marker)
	if idx < 0 {
		return input
	}
	return input[:idx]
}

func TestErrorHandlingMatchInvalidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/errorhandling_match_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))

	expected := []string{
		"non-exhaustive match for Result[Speed, IOError]: missing Err at 17:18",
		"non-exhaustive match for closed enum IOError; cover every declared underlying value class or add _ at 25:17",
		"match arms must produce compatible types, got int and string at 36:29",
		"match pattern must match Result[Speed, IOError], got IOError at 45:16",
		"catch-all pattern may not hide Err at 44:18",
		"unreachable enum match arm; underlying value is already covered by InvalidValue at 55:9",
		"unreachable match arm at 65:9",
	}

	assertSemaErrors(t, errors, expected)
}

func TestMatchRequiresAtLeastOneBranch(t *testing.T) {
	input := `
module main

enum IOError error {
	InvalidValue,
}

fn Test(error: IOError) void {
	match error {
	}
	return
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"match requires at least one branch at 9:2",
	}

	assertSemaErrors(t, errors, expected)
}

func TestMatchCatchAllMayNotHideResultErr(t *testing.T) {
	input := `
module main

enum IOError error {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(1)
}

fn Test() int {
	let value := match Read() {
		Ok(value) => value
		_ => 0
	}
	return value
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"catch-all pattern may not hide Err at 13:15",
	}

	assertSemaErrors(t, errors, expected)
}

func TestMatchGuardMustBeBool(t *testing.T) {
	input := `
module main

enum IOError error {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(1)
}

fn Test() int {
	let value := match Read() {
		Ok(value) where value => value
		Ok(value) => value
		Err(error) => 0
	}
	return value
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"match guard must be bool, got int at 14:19",
	}

	assertSemaErrors(t, errors, expected)
}

func TestGuardedMatchArmDoesNotExhaustPattern(t *testing.T) {
	input := `
module main

enum IOError error {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(1)
}

fn Test(flag: bool) int {
	let value := match Read() {
		Ok(value) where flag => value
		Err(error) => 0
	}
	return value
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"non-exhaustive match for Result[int, IOError]: missing Ok at 13:15",
	}

	assertSemaErrors(t, errors, expected)
}

func TestMatchDuplicateResultArms(t *testing.T) {
	input := `
module main

enum IOError error {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(1)
}

fn DuplicateOk() int {
	return match Read() {
		Ok(value) => value
		Ok(other) => other
		Err(error) => 0
	}
}

fn DuplicateErr() int {
	return match Read() {
		Ok(value) => value
		Err(error) => 0
		Err(other) => 1
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"duplicate match arm for Result[int, IOError].Ok at 15:3",
		"duplicate match arm for Result[int, IOError].Err at 24:3",
	}
	assertSemaErrors(t, errors, expected)
}

func TestMatchDiscardSuccessPayloadAndExplicitErrorDiscard(t *testing.T) {
	input := `
module main

enum IOError error {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(1)
}

fn Test() void {
	match Read() {
		Ok(_) => {
		}
		Err(error) => {
			discard error
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestMatchPatternBindingCannotShadowOuterVariable(t *testing.T) {
	input := `
module main

enum IOError error {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(1)
}

fn ShadowOk() int {
	let value: int := 10
	return match Read() {
		Ok(value) => value
		Err(error) => 0
	}
}

fn ShadowErr() int {
	let error: IOError := IOError.InvalidValue
	return match Read() {
		Ok(value) => value
		Err(error) => 0
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"variable \"value\" already declared at 15:3, previous declaration at 13:6",
		"variable \"error\" already declared at 24:3, previous declaration at 21:6",
	}
	assertSemaErrors(t, errors, expected)
}

func TestMatchStatementReturnArmsSatisfyFunctionReturn(t *testing.T) {
	input := `
module main

enum IOError error {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(1)
}

fn Test() int {
	match Read() {
		Ok(value) => return value
		Err(error) => return 0
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestExhaustiveMatchExpressionWithReturningBlockArmsIsNever(t *testing.T) {
	input := `
module main

fn Next(found: Option[int]) Option[int] {
	return match found {
		Some(index) => {
			return Some(index)
		}
		None => {
			return None()
		}
	}
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)

	functions := analyzer.functions["Next"]
	if len(functions) != 1 {
		t.Fatalf("Next functions = %#v, want one", functions)
	}
}

func TestMatchExpressionResolvesUnqualifiedEnumCases(t *testing.T) {
	input := `
module main

enum HttpKnownMethod {
	GET,
	POST,
}

impl HttpKnownMethod {
	property Name: string {
		get {
			return match self {
				GET => "GET"
				POST => "POST"
			}
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestMatchAnalysisRejectsNilRecoveredArmWithoutPanic(t *testing.T) {
	program := &ast.Program{Statements: []ast.Statement{
		&ast.ModuleStatement{
			Token: lexer.Token{Type: lexer.MODULE, Line: 1, Column: 1, Lexeme: "module"},
			Path:  "main",
		},
		&ast.EnumDeclaration{
			Token: lexer.Token{Line: 4, Column: 1, Lexeme: "enum"},
			Name:  &ast.Identifier{Token: lexer.Token{Line: 4, Column: 6, Lexeme: "Status"}, Value: "Status"},
			Values: []*ast.EnumValue{
				{Name: &ast.Identifier{Token: lexer.Token{Line: 5, Column: 2, Lexeme: "Ready"}, Value: "Ready"}},
			},
		},
		&ast.FunctionDeclaration{
			Token:      lexer.Token{Line: 8, Column: 1, Lexeme: "fn"},
			Name:       &ast.Identifier{Token: lexer.Token{Line: 8, Column: 4, Lexeme: "Test"}, Value: "Test"},
			ReturnType: &ast.TypeReference{Token: lexer.Token{Line: 8, Column: 25, Lexeme: "string"}, Name: "string"},
			Parameters: []*ast.Parameter{
				{Name: &ast.Identifier{Token: lexer.Token{Line: 8, Column: 9, Lexeme: "status"}, Value: "status"}, Type: &ast.TypeReference{Token: lexer.Token{Line: 8, Column: 17, Lexeme: "Status"}, Name: "Status"}},
			},
			Body: &ast.BlockStatement{Statements: []ast.Statement{
				&ast.ReturnStatement{
					Token: lexer.Token{Line: 9, Column: 2, Lexeme: "return"},
					Value: &ast.MatchExpression{
						Token:   lexer.Token{Line: 9, Column: 9, Lexeme: "match"},
						Subject: &ast.Identifier{Token: lexer.Token{Line: 9, Column: 15, Lexeme: "status"}, Value: "status"},
						Arms: []*ast.MatchArm{
							nil,
							{Token: lexer.Token{Line: 10, Column: 3, Lexeme: "Ready"}},
						},
					},
				},
			}},
		},
	}}

	errors := NewAnalyzer().Analyze(program)
	assertSemaErrors(t, errors, []string{
		"invalid match arm at 9:9",
		"match expression must produce a value at 9:9",
	})
}

func TestMatchStatementAssignmentsMergeAcrossExhaustiveArms(t *testing.T) {
	input := `
module main

enum Direction {
	North,
	East,
}

enum IOError error {
	InvalidValue,
}

fn AllEnumArmsAssign(direction: Direction) int {
	let mut result: int

	match direction {
		Direction.North => {
			result = 1
		}
		Direction.East => {
			result = 2
		}
		_ => {
			result = 0
		}
	}

	return result
}

fn ResultReturningArmAssigns(result: Result[int, IOError]) int {
	let mut value: int

	match result {
		Ok(number) => {
			value = number
		}
		Err(error) => {
			return 0
		}
	}

	return value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestMatchEnumNumericCoverageAndAliases(t *testing.T) {
	input := `
module main

enum Alias: bit[1] {
	Off = 0,
	Disabled = 0,
	On = 1,
}

fn Complete(value: Alias) int {
	return match value {
		Alias.Off => 0
		Alias.On => 1
	}
}

fn Duplicate(value: Alias) int {
	return match value {
		Alias.Off => 0
		Alias.Disabled => 1
		Alias.On => 2
	}
}

enum Open {
	First,
	Second,
}

fn OpenDomain(value: Open) int {
	return match value {
		Open.First => 0
		Open.Second => 1
	}
}
`
	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"unreachable enum match arm; underlying value is already covered by Off at 20:3",
	})
}

func TestMatchRuneSubjectRejectsCharacterLiteralPatterns(t *testing.T) {
	input := `
module main

fn Classify(character: rune) int {
	return match character {
		'-' => 1
		'\n' => 2
		_ => 0
	}
}
`

	p := parser.New(lexer.New(input))
	result := p.Parse()
	if !result.HasErrors || len(p.Errors()) != 2 {
		t.Fatalf("parser errors = %v", p.Errors())
	}
	for _, err := range p.Errors() {
		if !strings.Contains(err, "literal and range match patterns are not part of Sec 0.1; use switch") {
			t.Fatalf("wrong parser guidance: %s", err)
		}
	}
}

func TestMatchRejectsDirectBoolSubjectWithFocusedGuidance(t *testing.T) {
	input := `
module main

fn Choose(ready: bool) int {
	return match ready {
		_ => 1
	}
}

fn Act(ready: bool) void {
	match ready {
		_ => {}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"match subject cannot be bool; use if and else for boolean control flow at 5:15",
		"match subject cannot be bool; use if and else for boolean control flow at 11:8",
	})
}

// rules/control-flow/flowcontrol_match.md; correction20.md.
func TestMatchErrDiscardPatternIsAccepted(t *testing.T) {
	input := `
module main

enum IOError error {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(1)
}

fn Test() int {
	return match Read() {
		Err(_) => 0
		Ok(value) => value
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

// rules/errors/errorhandling.md and rules/control-flow/flowcontrol_match.md;
// correction20.md makes Err(_) an exhaustive try-handler catch-all.
func TestTryErrDiscardPatternIsAccepted(t *testing.T) {
	input := `
module main

enum IOError error {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(1)
}

fn Test() int {
	return try Read() {
		Err(_) => 0
	}
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)
	foundDiscard := false
	for _, plan := range analyzer.resolvedTryPlans {
		for _, handler := range plan.Handlers {
			if handler.PatternKind == TryHandlerErrCatchAll && handler.PayloadDiscard && handler.BindingName == "" {
				foundDiscard = true
			}
		}
	}
	if !foundDiscard {
		t.Fatal("missing explicit Err(_) payload-discard fact")
	}
}

func TestDiscardRequiresDefinedName(t *testing.T) {
	input := `
module main

fn Test() void {
	discard error
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"undefined variable error at 5:10",
	}

	assertSemaErrors(t, errors, expected)
}

func TestDiscardExpressionAndUseAfterDiscard(t *testing.T) {
	input := `
module main

fn Calculate() int {
	return 10
}

fn Test() void {
	let value := 42
	discard Calculate()
	discard value
	let again := value
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"value value was discarded here and is no longer available at 12:15, previous declaration at 11:10",
	}

	assertSemaErrors(t, errors, expected)
}

func TestStandaloneOrdinaryCallResultIsImplicitlyDiscarded(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

fn Calculate() int {
	return 10
}

fn Test() void {
	Calculate()
}
`)

	assertSemaErrors(t, errors, nil)
}

func TestStandaloneResultCallRequiresHandling(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

enum Failure error {
	failed,
}

fn TryCalculate() Result[int, Failure] {
	return Ok(10)
}

fn Test() void {
	TryCalculate()
}
`)

	expected := []string{
		"result of TryCalculate has type Result[int, Failure] and must be handled explicitly at 13:2",
	}
	assertSemaErrors(t, errors, expected)
	if len(errors) != 1 || errors[0].ID != diagnostics.UnhandledMustUseResult {
		t.Fatalf("wrong must-use diagnostic. got=%#v", errors)
	}
	if errors[0].Help != "use try, match, binding, return, or explicit discard" {
		t.Fatalf("wrong must-use help. got=%q", errors[0].Help)
	}
}

func TestMustUseAndDiscardabilityPropagateThroughWrappers(t *testing.T) {
	implicitErrors := analyzeSourceRaw(t, `
module main

fn Pass(value: Option[Task[int]]) Option[Task[int]] {
	return value
}

fn Test(value: Option[Task[int]]) void {
	Pass(<-value)
}
`)
	implicitExpected := []string{
		"result of Pass has type Option[Task[int]] and must be handled explicitly at 9:2",
	}
	assertSemaErrors(t, implicitErrors, implicitExpected)
	if implicitErrors[0].ID != diagnostics.UnhandledMustUseResult {
		t.Fatalf("wrong wrapper must-use diagnostic ID. got=%q", implicitErrors[0].ID)
	}

	discardErrors := analyzeSourceRaw(t, `
module main

fn Pass(value: Option[Task[int]]) Option[Task[int]] {
	return value
}

fn Test(value: Option[Task[int]]) void {
	discard Pass(<-value)
}
`)
	discardExpected := []string{
		"cannot discard Option[Task[int]] because it may contain an unresolved lifecycle handle at 9:10",
	}
	assertSemaErrors(t, discardErrors, discardExpected)
	if discardErrors[0].ID != diagnostics.NonDiscardableValue {
		t.Fatalf("wrong non-discardable diagnostic ID. got=%q", discardErrors[0].ID)
	}
}

func TestDiscardRejectsUnresolvedTaskHandle(t *testing.T) {
	input := `
module main

fn Test(task: Task[int]) void {
	discard task
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"cannot discard unresolved Task[int]; await, join or detach it explicitly at 5:10",
	}

	assertSemaErrors(t, errors, expected)
}

func TestDiscardRejectsSpawnResult(t *testing.T) {
	input := `
module main

fn Work() int {
	return 1
}

fn Test() void {
	discard spawn Work()
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"cannot discard spawn result because successful creation would abandon Task[int] at 9:2",
	}

	assertSemaErrors(t, errors, expected)
}

func TestDiscardedMutableBindingCanBeReinitialized(t *testing.T) {
	input := `
module main

fn Test() void {
	let mut value := 42
	discard value
	value = 10
}
`

	errors := analyzeSourceRaw(t, input)

	assertSemaErrors(t, errors, nil)
}

func TestIfConditionsAndBranchScopes(t *testing.T) {
	input := `
module main

fn Test(ready: bool, score: int) int {
	let mut result := 0
	if ready && score >= 10 {
		let branchOnly := 1
		result = branchOnly
	} else if !ready {
		result = 2
	} else {
		result = 3
	}
	return result
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestIfConditionMustBeBool(t *testing.T) {
	input := `
module main

enum Status {
	Active,
}

fn Count() int {
	return 10
}

fn Test(value: int, name: string, status: Status) void {
	if value {
	}
	if name {
	}
	if status {
	}
	if Count() {
	}
	return
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"if condition must be bool, got int at 13:5",
		"if condition must be bool, got string at 15:5",
		"if condition must be bool, got Status at 17:5",
		"if condition must be bool, got int at 19:5",
	}

	assertSemaErrors(t, errors, expected)
}

func TestIfBranchesCanSatisfyFunctionReturn(t *testing.T) {
	input := `
module main

fn Sign(value: int) int {
	if value < 0 {
		return -1
	} else {
		return 1
	}
}

fn Grade(score: int) int {
	if score >= 90 {
		return 1
	} else if score >= 80 {
		return 2
	} else {
		return 3
	}
}

fn Early(value: int) int {
	if value < 0 {
		return 0
	}
	return value
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestIfWithoutElseDoesNotSatisfyFunctionReturn(t *testing.T) {
	input := `
module main

fn Missing(value: int) int {
	if value < 0 {
		return 0
	}
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"function Missing must return int at 4:4",
	}

	assertSemaErrors(t, errors, expected)
}

func TestElseIfWithoutFinalElseDoesNotSatisfyFunctionReturn(t *testing.T) {
	input := `
module main

fn MissingReturnAfterElseIf(value: int) int {
	if value < 0 {
		return -1
	} else if value == 0 {
		return 0
	}
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"function MissingReturnAfterElseIf must return int at 4:4",
	}

	assertSemaErrors(t, errors, expected)
}

func TestIfRangeMembershipCondition(t *testing.T) {
	input := `
module main

type Percent int range 0..100

fn Test(score: int, percent: Percent) void {
	if score in 80..<100 {
	}
	if score in 1.. {
	}
	if score in ..100 {
	}
	if percent in Percent(1)..<Percent(100) {
	}
	return
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestIfRangeMembershipTypeMismatch(t *testing.T) {
	input := `
module main

type Percent int range 0..100

fn Test(percent: Percent) void {
	let lower := 1
	if percent in lower..100 {
	}
	return
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot test Percent in range of int at 8:16",
	}

	assertSemaErrors(t, errors, expected)
}

func TestInvalidNegatedRangeDoesNotCascade(t *testing.T) {
	input := `
module main

fn InvalidNegatedRange(score: string) void {
	if !(score in 0..100) {
	}
	return
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot test string in range of int at 5:16",
	}

	assertSemaErrors(t, errors, expected)
}

func TestIfBranchLocalDoesNotLeakToFollowingCall(t *testing.T) {
	input := `
module main

fn ScopeTest(value: bool) void {
	if value {
		let local: int := 10
	}

	println(local)
}

fn println(s: string) void {
	return
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"undefined variable local at 9:10",
	}

	assertSemaErrors(t, errors, expected)
}

func TestIfDefiniteAssignment(t *testing.T) {
	input := `
module main

fn AssignedInBothBranches(value: bool) int {
	let mut result: int

	if value {
		result = 10
	} else {
		result = 20
	}

	return result
}

fn MissingAssignment(value: bool) int {
	let mut result: int

	if value {
		result = 10
	}

	return result
}
`

	errors := analyzeSource(t, input)

	expected := []string{}

	assertSemaErrors(t, errors, expected)
}

func TestIfDefiniteAssignmentOnlyRequiresContinuingPaths(t *testing.T) {
	input := `
module main

fn AssignmentInAllContinuingPaths(value: bool) int {
	let mut result: int

	if value {
		result = 10
	} else {
		return 20
	}

	return result
}

fn AssignmentInElseOnlyWhenThenReturns(value: bool) int {
	let mut result: int

	if value {
		return 20
	} else {
		result = 10
	}

	return result
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestSwitchValidCasesAndDefiniteAssignment(t *testing.T) {
	input := `
module main

fn Select(value: int) int {
	let mut result: int

	switch value {
	case < 0:
		result = -1
	case 0, 1, 2..<10:
		result = 10
	default:
		result = 20
	}

	return result
}

fn Subjectless(value: int) int {
	switch {
	case value < 0:
		return -1
	case value == 0:
		return 0
	default:
		return 1
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestSwitchFallthroughIntoReturningCaseSatisfiesReturn(t *testing.T) {
	input := `
module main

fn FallthroughIntoReturningCase(value: int) int {
	switch value {
	case 1:
		fallthrough
	case 2:
		return 20
	default:
		return 30
	}
}

fn MultipleFallthrough(value: int) int {
	switch value {
	case 1:
		fallthrough
	case 2:
		fallthrough
	case 3:
		return 10
	default:
		return 20
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestSwitchSemanticErrors(t *testing.T) {
	input := `
module main

fn Invalid(value: int, name: string) void {
	switch value {
	case "one":
		return
	case 1:
		return
	case 1:
		return
	}

	switch {
	case 10:
		return
	}

	switch value {
	case 3:
		fallthrough
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"switch case must be compatible with subject type int, got string at 6:7",
		"duplicate switch case value 1 at 10:7",
		"subjectless switch case must be bool, got int at 15:7",
		"fallthrough is not allowed in the final switch case at 21:3",
	}

	assertSemaErrors(t, errors, expected)
}

func TestSwitchRejectsDuplicateStringConstants(t *testing.T) {
	input := `
module main

fn Select(command: string) void {
	switch command {
	case "start":
		return
	case "start":
		return
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		`duplicate switch case value "start" at 8:7`,
	}
	assertSemaErrors(t, errors, expected)
	if errors[0].ID != diagnostics.DuplicateSwitchCase {
		t.Fatalf("wrong diagnostic ID. got=%q want=%q", errors[0].ID, diagnostics.DuplicateSwitchCase)
	}
}

func TestSwitchWarnsAboutMissingEnumValuesWithoutDefault(t *testing.T) {
	input := `
module main

enum Direction {
	north,
	south,
	east,
}

fn Select(direction: Direction) void {
	switch direction {
	case Direction.north:
		return
	}
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)
	warnings := analyzer.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("wrong warning count. got=%d warnings=%v", len(warnings), warnings)
	}
	if warnings[0].Error() != "switch over Direction omits known values: south, east at 11:9" {
		t.Fatalf("wrong warning. got=%q", warnings[0].Error())
	}
	if warnings[0].ID != diagnostics.IncompleteEnumSwitch || warnings[0].Help == "" {
		t.Fatalf("wrong warning metadata: %+v", warnings[0])
	}
}

func TestEnumSwitchDefaultSuppressesCoverageWarning(t *testing.T) {
	input := `
module main

enum Direction { north, south }

fn Select(direction: Direction) void {
	switch direction {
	case Direction.north:
		return
	default:
		return
	}
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)
	if len(analyzer.Warnings()) != 0 {
		t.Fatalf("expected no warnings. got=%v", analyzer.Warnings())
	}
}

func TestSwitchCaseReportsUnreachableAfterReturn(t *testing.T) {
	input := `
module main

fn UnreachableAfterReturn(value: int) int {
	switch value {
	case 1:
		return 10
		let local: int := 20
	default:
		return 30
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"unreachable code at 8:3",
	}

	assertSemaErrors(t, errors, expected)
}

func TestSwitchDefaultPlacementErrorsAreSemaErrors(t *testing.T) {
	input := `
module main

fn MultipleDefault(value: int) void {
	switch value {
	case 1:
	default:
	default:
	}
}

fn DefaultNotFinal(value: int) void {
	switch value {
	default:
	case 1:
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"default must be the final switch clause at 8:2",
		"switch may contain only one default clause at 8:2",
		"default must be the final switch clause at 15:2",
	}

	assertSemaErrors(t, errors, expected)
}

func TestSwitchConstantCoverageErrors(t *testing.T) {
	input := `
module main

fn Invalid(value: int) void {
	switch value {
	case 0..100:
		return
	case 50:
		return
	case 40..120:
		return
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"switch case value 50 is already covered by previous case at 8:7",
		"switch case range overlaps previous case at 10:9",
	}

	assertSemaErrors(t, errors, expected)
}

func TestSwitchDescendingRangeIsNormalized(t *testing.T) {
	input := `
module main

fn Valid(value: int) void {
	switch value {
	case 10..0:
		return
	}
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)
	if len(analyzer.Warnings()) != 0 {
		t.Fatalf("expected no warnings. got=%v", analyzer.Warnings())
	}
}

func TestSwitchRelationalCoveredCasesAreUnreachable(t *testing.T) {
	input := `
module main

fn Invalid(value: int) void {
	switch value {
	case >= 0:
		return
	case > 10:
		return
	}

	switch value {
	case <= 100:
		return
	case <= 5:
		return
	}

	switch value {
	case 0..100:
		return
	case > 50:
		return
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"unreachable switch case; previous case already covers this condition at 8:7",
		"unreachable switch case; previous case already covers this condition at 15:7",
		"unreachable switch case; previous case already covers this condition at 22:7",
	}

	assertSemaErrors(t, errors, expected)
}

func TestSwitchPartialRelationalOverlapIsAllowed(t *testing.T) {
	input := `
module main

fn Valid(value: int) int {
	switch value {
	case <= -10:
		return -2
	case < 0:
		return -1
	case 0:
		return 0
	case < 10:
		return 1
	case >= 10:
		return 2
	default:
		return 3
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestSwitchAdjacentExclusiveRangesAreAllowed(t *testing.T) {
	input := `
module main

fn Valid(value: int) void {
	switch value {
	case 0..<10:
		return
	case 10..<20:
		return
	case < 0:
		return
	case >= 20:
		return
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestBoolSwitchCanBeExhaustiveWithoutDefault(t *testing.T) {
	input := `
module main

fn BoolReturn(value: bool) int {
	switch value {
	case true:
		return 1
	case false:
		return 0
	}
}

fn BoolAssignment(value: bool) int {
	let mut result: int

	switch value {
	case true:
		result = 1
	case false:
		result = 0
	}

	return result
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestBoolSwitchDuplicateCase(t *testing.T) {
	input := `
module main

fn Invalid(value: bool) void {
	switch value {
	case true:
		return
	case true:
		return
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"duplicate switch case value true at 8:7",
	}

	assertSemaErrors(t, errors, expected)
}

func TestAsmRequiresUnsafe(t *testing.T) {
	input := `
module main

fn Valid() void {
	unsafe {
		asm "nop"
	}
	return
}

fn Invalid() void {
	asm "nop"
	return
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"asm is only allowed inside unsafe at 12:2",
	}

	assertSemaErrors(t, errors, expected)
}

func TestOnlyUnsafeExternFunctionRequiresUnsafeCall(t *testing.T) {
	input := `
module main

extern "C" fn inspect(value: ref int) int32
extern "C" fn modify(value: ref mut int) int32
unsafe extern "C" fn write(fd: int32, buffer: RawPtr[byte], length: uint) int64

fn Bad(value: ref int, buffer: RawPtr[byte]) void {
	let inspected := inspect(value)
	let result := write(1, buffer, 4u)
}

fn Good(value: ref mut int, buffer: RawPtr[byte]) void {
	let modified := modify(value)
	unsafe {
		let result := write(1, buffer, 4u)
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"calling unsafe extern function write requires unsafe at 10:16",
	}

	assertSemaErrors(t, errors, expected)
}

func TestExternFunctionValidation(t *testing.T) {
	input := `
module main

extern "Rust" fn badABI(value: int32) int32
extern "C" fn badParam(value: string) int32
extern "C" fn badReturn() string
extern "C" fn badReferenceReturn() ref int
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"unknown extern ABI \"Rust\" at 4:1",
		"extern C parameter 1 value has non-ABI-compatible type string at 5:24",
		"extern C function badReturn has non-ABI-compatible return type string at 6:15",
		"extern C function badReferenceReturn has non-ABI-compatible return type ref int at 7:15",
	}

	assertSemaErrors(t, errors, expected)
}

func TestExternCAllowsRawPtrVoid(t *testing.T) {
	input := `
module main

extern "C" fn c_malloc(size: uint) RawPtr[void]
extern "C" fn c_free(ptr: RawPtr[void]) void

fn Use(size: uint) void {
	unsafe {
		let ptr := c_malloc(size)
		c_free(ptr)
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestExternLinkNamesMustBeUnique(t *testing.T) {
	input := `
module main

@link_name("foreign_add")
extern "C" fn add(left: int32, right: int32) int32

@link_name("foreign_add")
extern "C" fn addAlias(left: int32, right: int32) int32
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		`duplicate extern symbol "foreign_add" at 8:15, previous declaration at 5:15`,
	}
	assertSemaErrors(t, errors, expected)
}

func TestCompilerIntrinsicTypesAreRegistered(t *testing.T) {
	analyzer := NewAnalyzer()
	intrinsic := analyzer.IntrinsicTypes()

	for _, name := range []string{
		"any",
		"bool",
		"byte",
		"char",
		"rune",
		"int",
		"int8",
		"int16",
		"int32",
		"int64",
		"int128",
		"int256",
		"uint",
		"uint8",
		"uint16",
		"uint32",
		"uint64",
		"uint128",
		"uint256",
		"float",
		"float32",
		"float64",
		"decimal",
		"decimal128",
		"date",
		"time",
		"datetime",
		"duration",
		"string",
		"void",
		"RawPtr",
		"Option",
		"Iterator",
		"Result",
		"Vec",
		"Set",
		"Map",
		"list",
		"map",
		"set",
		"vector",
		"matrix",
		"tensor",
		"tensor_view",
		"Shape",
		"Strides",
		"TensorLayout",
		"MemorySpace",
		"Task",
		"Thread",
		"ThreadObserver",
		"ThreadLocal",
		"ThreadConfig",
		"ThreadContext",
		"ThreadID",
		"ThreadPriority",
		"ThreadStatus",
		"ThreadSpawnError",
		"ThreadStartError",
		"ThreadSchedulingError",
		"ThreadTerminationError",
		"ThreadContextError",
		"Mutex",
		"MutexGuard",
		"Atomic",
		"CompareExchangeResult",
		"Event",
		"EventStorage",
		"Subscription",
		"EventSubscribeResult",
		"Channel",
		"Sender",
		"Receiver",
		"MessageTicket",
		"ChannelSendResult",
		"ChannelTryReceiveResult",
		"ChannelRevokeResult",
		"MessageDisposition",
		"Arena",
		"AllocationError",
	} {
		typ, ok := intrinsic[name]
		if !ok {
			t.Fatalf("expected %s to be registered as an intrinsic type", name)
		}
		if !typ.Intrinsic {
			t.Fatalf("expected %s to be marked intrinsic: %+v", name, typ)
		}
	}

	rawPtr := intrinsic["RawPtr"]
	if rawPtr.Kind != RawPtrType || len(rawPtr.GenericParameters) != 1 || rawPtr.GenericParameters[0] != "T" {
		t.Fatalf("wrong RawPtr intrinsic metadata: %+v", rawPtr)
	}

	allocationError := intrinsic["AllocationError"]
	if allocationError.Kind != EnumType {
		t.Fatalf("AllocationError should be an intrinsic enum: %+v", allocationError)
	}
	for _, value := range []string{"OutOfMemory", "Unsupported", "InvalidSize", "InvalidAlignment"} {
		if _, ok := allocationError.EnumConsts[value]; !ok {
			t.Fatalf("AllocationError missing enum const %s: %+v", value, allocationError.EnumConsts)
		}
	}
}

func TestEventDeclarationPublishAndSubscribe(t *testing.T) {
	input := `
module main

type ButtonPressData struct {
	value: int,
}

type Button struct {
	ButtonPressed: Event[ButtonPressData],
	customStorage: EventStorage[ButtonPressData, 8],
}

impl Button {
	event CustomPressed using customStorage

	fn Press(data: ButtonPressData) void {
		ButtonPressed.Publish(data)
		CustomPressed.Publish(data)
	}
}

fn OnButtonPressed(data: ButtonPressData) void {
	discard data
}

fn Use(button: Button) void {
	let subscription := button.ButtonPressed.Subscribe(OnButtonPressed)
	discard subscription
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)

	button := analyzer.types["Button"]
	if len(button.Events) != 2 {
		t.Fatalf("wrong event count: %+v", button.Events)
	}
	if button.Events[0].Name != "ButtonPressed" || button.Events[0].Capacity != 4 {
		t.Fatalf("wrong short-form event: %+v", button.Events[0])
	}
	if button.Events[1].Name != "CustomPressed" || button.Events[1].Capacity != 8 || !button.Events[1].StorageBacked {
		t.Fatalf("wrong storage-backed event: %+v", button.Events[1])
	}
	subscription := analyzer.completionSymbols["subscription"].Type
	if typeDisplayName(subscription) != "EventSubscribeResult" {
		t.Fatalf("wrong Subscribe result type: %+v display=%s", subscription, typeDisplayName(subscription))
	}
}

func TestEventPublishRequiresOwningImpl(t *testing.T) {
	input := `
module main

type ButtonPressData struct {
	value: int,
}

type Button struct {
	ButtonPressed: Event[ButtonPressData],
}

fn Bad(button: Button, data: ButtonPressData) void {
	button.ButtonPressed.Publish(data)
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"event ButtonPressed may only be published by Button at 13:23",
	})
}

func TestEventSemanticErrors(t *testing.T) {
	input := `
module main

type ButtonPressData struct {
	value: int,
}

type BadCapacity struct {
	Bad: Event[ButtonPressData, 0],
}

type BadStorage struct {
	storage: EventStorage[ButtonPressData],
}

type Button struct {
	count: int,
}

impl Button {
	event Pressed using count
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"event capacity must be greater than zero at 9:7",
		"EventStorage requires explicit capacity at 13:11",
		"event Pressed storage field count must be EventStorage, got int at 21:22",
	})
}

func TestChannelConstructionAndCapabilities(t *testing.T) {
	input := `
module main

type Message struct {
	value: int,
}

fn Use() void {
	let channel := Channel[Message](32)
	let tx := channel.tx
	let rx := channel.rx
	let received := rx.Receive()
	let attempt := rx.TryReceive()
	discard tx
	discard rx
	discard received
	discard attempt
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)

	tx := analyzer.completionSymbols["tx"].Type
	if !isSenderType(tx) || typeDisplayName(tx.TypeArgs[0]) != "Message" {
		t.Fatalf("wrong tx type: %+v", tx)
	}
	rx := analyzer.completionSymbols["rx"].Type
	if !isReceiverType(rx) || typeDisplayName(rx.TypeArgs[0]) != "Message" {
		t.Fatalf("wrong rx type: %+v", rx)
	}
	received := analyzer.completionSymbols["received"].Type
	if typeDisplayName(received) != "Option[Message]" {
		t.Fatalf("wrong Receive result type: %+v display=%s", received, typeDisplayName(received))
	}
	attempt := analyzer.completionSymbols["attempt"].Type
	if typeDisplayName(attempt) != "ChannelTryReceiveResult[Message]" {
		t.Fatalf("wrong TryReceive result type: %+v display=%s", attempt, typeDisplayName(attempt))
	}
}

func TestChannelSendAndRevokeTypes(t *testing.T) {
	input := `
module main

type Message struct {
	value: int,
}

fn Use() void {
	let channel := Channel[Message](1)
	let tx := channel.tx
	let sent := tx.Send(Message{ value: 1 })
	let ticket := tx.SendRevocable(Message{ value: 2 })
	let revoke := tx.Revoke(ticket)
	discard sent
	discard revoke
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)

	sent := analyzer.completionSymbols["sent"].Type
	if typeDisplayName(sent) != "ChannelSendResult[Message]" {
		t.Fatalf("wrong Send result type: %+v display=%s", sent, typeDisplayName(sent))
	}
	ticket := analyzer.completionSymbols["ticket"].Type
	if typeDisplayName(ticket) != "MessageTicket[Message]" {
		t.Fatalf("wrong SendRevocable result type: %+v display=%s", ticket, typeDisplayName(ticket))
	}
	revoke := analyzer.completionSymbols["revoke"].Type
	if typeDisplayName(revoke) != "ChannelRevokeResult[Message]" {
		t.Fatalf("wrong Revoke result type: %+v display=%s", revoke, typeDisplayName(revoke))
	}
}

func TestChannelOperationResultsCanBeMatched(t *testing.T) {
	input := `
module main

type Message struct {
	value: int,
}

fn Use() void {
	let channel := Channel[Message](1)
	let tx := channel.tx
	let rx := channel.rx
	let outbound := Message{ value: 1 }

	match rx.Receive() {
		Some(message) => {
			discard message
		}
		None => {
		}
	}

	match tx.Send(outbound) {
		Sent => {
		}
		Closed(message) => {
			discard message
		}
	}

	match rx.TryReceive() {
		Received(message) => {
			discard message
		}
		Empty => {
		}
		Closed => {
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestChannelCapabilitiesAreMoveOnly(t *testing.T) {
	input := `
module main

type Message struct {
	value: int,
}

fn Use() void {
	let channel := Channel[Message](1)
	let tx := channel.tx
	let moved :<- tx
	let again := tx
	discard moved
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"use of moved value tx at 12:15, previous declaration at 11:16",
	}
	assertSemaErrors(t, errors, expected)
}

func TestChannelSendConsumesMoveOnlyMessage(t *testing.T) {
	input := `
module main

type Message struct {
	view: ref mut int,
}

fn Use() void {
	let channel := Channel[Message](1)
	let tx := channel.tx
	let mut value := 1
	let message := Message{ view: ref mut value }
	let sent := tx.Send(message)
	let again := message
	discard sent
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"use of moved value message at 14:15, previous declaration at 13:22",
	}
	assertSemaErrors(t, errors, expected)
}

func TestChannelSendRequiresMessageType(t *testing.T) {
	input := `
module main

type Message struct {
	value: int,
}

fn Use() void {
	let channel := Channel[Message](1)
	let tx := channel.tx
	let sent := tx.Send(1)
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"Sender.Send message must be Message, got int at 11:22",
	}
	assertSemaErrors(t, errors, expected)
}

func TestSelectValidChannelBranches(t *testing.T) {
	input := `
module main

type Message struct {
	value: int,
}

fn Use(task: Task[int]) void {
	let channel := Channel[Message](1)
	let tx := channel.tx
	let rx := channel.rx
	let outbound := Message{ value: 1 }

	select {
		message := rx.Receive() => {
			discard message
		}
		tx.Send(outbound) => {
		}
		result := await task => {
			discard result
		}
		after 10 => {
		}
		default => {
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestSelectDefaultPlacementErrors(t *testing.T) {
	input := `
module main

fn Use() void {
	select {
		default => {
		}
		after 1 => {
		}
		default => {
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"default branch must be last in select at 8:3",
		"select may contain only one default branch at 10:3",
		"timeout branch is unreachable because default executes immediately at 8:3",
	}
	assertSemaErrors(t, errors, expected)
}

func TestSelectRejectsOrdinaryCall(t *testing.T) {
	input := `
module main

fn Work() int {
	return 1
}

fn Use() void {
	select {
		value := Work() => {
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"operation is not selectable at 10:3",
	}
	assertSemaErrors(t, errors, expected)
}

func TestSelectRejectsDuplicateReceiverBranch(t *testing.T) {
	input := `
module main

fn Use(rx: Receiver[int]) void {
	select {
		first := rx.Receive() => {
			discard first
		}
		second := rx.TryReceive() => {
			discard second
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"receiver rx is used by more than one branch in the same select at 9:3, previous declaration at 6:3",
	}
	assertSemaErrors(t, errors, expected)
}

func TestSelectRejectsDuplicateSenderBranch(t *testing.T) {
	input := `
module main

fn Use(tx: Sender[int]) void {
	select {
		tx.Send(1) => {
		}
		tx.TrySend(2) => {
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"sender tx is used by more than one branch in the same select at 8:3, previous declaration at 6:3",
	}
	assertSemaErrors(t, errors, expected)
}

func TestSelectRejectsDuplicateTaskBranch(t *testing.T) {
	input := `
module main

fn Use(task: Task[int]) void {
	select {
		first := await task => {
			discard first
		}
		second := await task => {
			discard second
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"task task is used by more than one branch in the same select at 9:3, previous declaration at 6:3",
	}
	assertSemaErrors(t, errors, expected)
}

func TestSelectMergesMovedMessageState(t *testing.T) {
	input := `
module main

type Message struct {
	view: ref mut int,
}

fn Use() void {
	let channel := Channel[Message](1)
	let tx := channel.tx
	let mut value := 1
	let message := Message{ view: ref mut value }
	select {
		tx.Send(message) => {
		}
		default => {
		}
	}
	let again := message
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"use of moved value message at 19:15, previous declaration at 14:11",
	}
	assertSemaErrors(t, errors, expected)
}

func TestSelectRejectsMessageMovedByMultipleBranches(t *testing.T) {
	input := `
module main

type Message struct {
	view: ref mut int,
}

fn Use() void {
	let channel := Channel[Message](1)
	let tx := channel.tx
	let second := tx.Share()
	let mut value := 1
	let message := Message{ view: ref mut value }
	select {
		tx.Send(message) => {
		}
		second.Send(message) => {
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"message value message is moved by multiple select branches at 17:3, previous declaration at 15:3",
	}
	assertSemaErrors(t, errors, expected)
}

func TestSelectRejectsLiveMutexGuard(t *testing.T) {
	input := `
module main

type State struct {
	value: int,
}

fn Use(mutex: Mutex[State], rx: Receiver[int]) void {
	let guard := mutex.lock()
	select {
		value := rx.Receive() => {
			discard value
		}
	}
	discard guard
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"mutex guard guard remains active across select at 10:2, previous declaration at 9:6",
	}
	assertSemaErrors(t, errors, expected)
}

func TestSelectAllowsMovedMutexGuard(t *testing.T) {
	input := `
module main

type State struct {
	value: int,
}

fn Consume(guard: MutexGuard[State]) void {
}

fn Use(mutex: Mutex[State], rx: Receiver[int]) void {
	let guard := mutex.lock()
	Consume(<-guard)
	select {
		value := rx.Receive() => {
			discard value
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestRawPtrConversionRequiresUnsafe(t *testing.T) {
	input := `
module main

fn Bad(address: uint) RawPtr[byte] {
	return RawPtr[byte](address)
}

fn Good(address: uint) RawPtr[byte] {
	unsafe {
		return RawPtr[byte](address)
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"conversion involving RawPtr requires unsafe at 5:9",
	}

	assertSemaErrors(t, errors, expected)
}

func TestInlineAsmBlockCanReturnInt64(t *testing.T) {
	input := `
module main

fn _sysWrite(fd: int64, ptr: RawPtr[byte], len: uint) int64 {
	unsafe {
		asm {
			"syscall"
			inputs: rax(1), rdi(fd), rsi(ptr), rdx(len)
			outputs: rax
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestUnsafeFunctionAllowsAsmAndNamedOutput(t *testing.T) {
	input := `
module platform_linux_amd64

enum ErrorNumber int error { Native = 1 }

fn _decodeSyscallResult(result: int) Result[uint, ErrorNumber] {
	if result < 0 {
		return Err(ErrorNumber.Native)
	}

	return Ok(uint(result))
}

unsafe fn _rawSyscall3(number: uint, arg1: uint, arg2: uint, arg3: uint) int {
	asm {
		"syscall"

		inputs:
			rax(number)
			rdi(arg1)
			rsi(arg2)
			rdx(arg3)

		outputs:
			rax(result)

		clobbers:
			rcx
			r11
			memory
	}

	return result
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestStringPtrAndLenMembers(t *testing.T) {
	input := `
module main

fn _sysWrite(fd: int64, ptr: RawPtr[byte], len: uint) int64 {
	unsafe {
		asm {
			"syscall"
			inputs: rax(1), rdi(fd), rsi(ptr), rdx(len)
			outputs: rax
		}
	}
}

fn Println(s: string) void {
	unsafe {
		_sysWrite(1, s.ptr, s.len)

		let nl := "\n"
		_sysWrite(1, nl.ptr, 1)
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestPtrMemberRequiresUnsafe(t *testing.T) {
	input := `
module main

fn Invalid(value: int) RawPtr[int] {
	return value.ptr
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"member ptr requires unsafe at 5:15",
	}
	assertSemaErrors(t, errors, expected)
}

func TestUniversalPtrMemberTypes(t *testing.T) {
	input := `
module main

type Packet struct {
	value: int,
}

fn IntPointer(value: int) RawPtr[int] {
	unsafe {
		return value.ptr
	}
}

fn FieldPointer(packet: Packet) RawPtr[int] {
	unsafe {
		return packet.value.ptr
	}
}

fn StringPointer(value: string) RawPtr[byte] {
	unsafe {
		return value.ptr
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestRawPtrPointerArithmeticRequiresUnsafeAndTypes(t *testing.T) {
	input := `
module main

fn Bad(ptr: RawPtr[int], bytes: RawPtr[byte], other: RawPtr[byte], offset: uint) void {
    ptr.Offset(1)
    bytes.AddBytes(offset)
    ptr.AddBytes(1)
    ptr.Difference(other)
}

fn Good(ptr: RawPtr[int], other: RawPtr[int], bytes: RawPtr[byte]) int {
    unsafe {
        let moved := ptr.Offset(2)
        let byteMoved := bytes.AddBytes(4)
        discard moved
        discard byteMoved
        return ptr.Difference(other)
    }
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"RawPtr.Offset requires unsafe because it performs raw-pointer arithmetic at 5:9",
		"RawPtr.AddBytes requires unsafe because it performs raw-pointer arithmetic at 6:11",
		"RawPtr.AddBytes requires unsafe because it performs raw-pointer arithmetic at 7:9",
		"RawPtr.Difference requires unsafe because it performs raw-pointer arithmetic at 8:9",
	})
}

func TestRawPtrPointerArithmeticValidationInsideUnsafe(t *testing.T) {
	input := `
module main

fn Invalid(ptr: RawPtr[int], voidPtr: RawPtr[void], bytes: RawPtr[byte], other: RawPtr[byte], offset: uint) void {
    unsafe {
        voidPtr.Offset(1)
        bytes.AddBytes(offset)
        ptr.AddBytes(1)
        ptr.Difference(other)
    }
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"RawPtr.Offset requires a typed element pointer, got RawPtr[void] at 6:17",
		"RawPtr.AddBytes argument must be int, got uint at 7:24",
		"RawPtr.AddBytes requires RawPtr[byte], got RawPtr[int] at 8:13",
		"RawPtr.Difference argument must be RawPtr[int], got RawPtr[byte] at 9:24",
	})
}

func TestAggregateReferenceFieldHoldsBorrow(t *testing.T) {
	input := `
module main

type Handle struct {
	view: ref mut int,
}

fn Invalid() void {
	let mut value := 1
	let handle := Handle{ view: ref mut value }
	let copy := value
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"cannot read value while it is mutably borrowed at 11:14, previous declaration at 10:30",
	}
	assertSemaErrors(t, errors, expected)
}

func TestMovingAggregateReferenceHolderEndsBorrow(t *testing.T) {
	input := `
module main

type Handle struct {
	view: ref mut int,
}

fn Consume(handle: Handle) void {
}

fn Valid() void {
	let mut value := 1
	let handle := Handle{ view: ref mut value }
	Consume(<-handle)
	let copy := value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestCannotMoveBorrowedMoveOnlyValue(t *testing.T) {
	input := `
module main

type Handle struct {
	view: ref mut int,
}

fn Consume(handle: Handle) void {
}

fn Invalid() void {
	let mut value := 1
	let handle := Handle{ view: ref mut value }
	let borrow := ref handle
	Consume(<-handle)
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"cannot move handle while it is borrowed at 15:12, previous declaration at 14:16",
	}
	assertSemaErrors(t, errors, expected)
}

func TestIfBranchBorrowHolderMergesAfterBranch(t *testing.T) {
	input := `
module main

fn Invalid(condition: bool) void {
	let mut value := 1
	let other := 0
	let mut view: ref int := ref other
	if condition {
		view = ref value
	}
	value = 2
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"cannot assign to value while it is shared borrowed at 11:2, previous declaration at 9:10",
	}
	assertSemaErrors(t, errors, expected)
}

func TestIfBranchMutableBorrowHolderMergesAfterBranch(t *testing.T) {
	input := `
module main

fn Invalid(condition: bool) void {
	let mut value := 1
	let mut other := 0
	let mut view: ref mut int := ref mut other
	if condition {
		view = ref mut value
	}
	let copy := value
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"cannot read value while it is mutably borrowed at 11:14, previous declaration at 9:10",
	}
	assertSemaErrors(t, errors, expected)
}

func TestIfBranchLocalBorrowEndsBeforeMerge(t *testing.T) {
	input := `
module main

fn Valid(condition: bool) void {
	let mut value := 1
	if condition {
		let view := ref value
		discard view
	}
	value = 2
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestIfBranchesCanEndEarlierBorrow(t *testing.T) {
	input := `
module main

fn Valid(condition: bool) void {
	let mut value := 1
	let mut other := 2
	let mut view := ref value
	if condition {
		view = ref other
	} else {
		view = ref other
	}
	value = 3
	discard view
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestCallSpreadRejectsMoveOnlyArrayElements(t *testing.T) {
	input := `
module main

type Resource struct {
	view: ref mut int,
}

fn Consume(first: Resource, second: Resource) void {
}

fn Invalid() void {
	let mut firstValue := 1
	let mut secondValue := 2
	let resources: Resource[2] := [
		Resource{ view: ref mut firstValue },
		Resource{ view: ref mut secondValue },
	]
	Consume(resources...)
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"cannot spread Resource[2] into function arguments; indexed expansion would require unsupported consuming element reads at 18:19",
	}
	assertSemaErrors(t, errors, expected)
}

func TestCallSpreadRuntimeLengthDiagnosticRetainsReferenceType(t *testing.T) {
	input := `
module main

fn Use(first: int, second: int) void {
}

fn Invalid(values: ref int[]) void {
	Use(values...)
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"cannot spread ref int[] into fixed-arity call; expansion count is not known at compile time at 8:12",
	}
	assertSemaErrors(t, errors, expected)
}

func TestArrayLiteralSpreadRejectsMoveOnlyArrayElements(t *testing.T) {
	input := `
module main

type Resource struct {
	view: ref mut int,
}

fn Invalid() void {
	let mut firstValue := 1
	let mut secondValue := 2
	let resources: Resource[2] := [
		Resource{ view: ref mut firstValue },
		Resource{ view: ref mut secondValue },
	]
	let copy := [resources...]
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"cannot spread Resource[2] into fixed-array literal; indexed expansion would require unsupported consuming element reads at 15:24",
	}
	assertSemaErrors(t, errors, expected)
}

func TestStructSpreadRejectsMoveOnlySource(t *testing.T) {
	input := `
module main

type Resource struct {
	view: ref mut int,
}

fn Invalid() void {
	let mut value := 1
	let resource := Resource{ view: ref mut value }
	let copy := Resource {
		resource...
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"cannot spread Resource into Resource; Resource is not implicitly copyable at 12:11",
		"field \"view\" in struct Resource has no default value and must be initialized at 11:14",
	}
	assertSemaErrors(t, errors, expected)
}

// rules/declarations/spread.md; correction14.md requires consuming parameters
// to consume prepared element copies rather than the fixed-array source.
func TestCallSpreadIntoConsumingParametersKeepsArrayAvailable(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main
fn Take(-> first: int, -> second: int) void {}
fn Valid() int {
	let values: int[2] := [1, 2]
	Take(values...)
	return values[0]
}
`)
	assertSemaErrors(t, errors, nil)
}

func TestStructSpreadAllowsCopyableSource(t *testing.T) {
	input := `
module main

type User struct {
	name: string,
	age: int,
}

fn Valid() void {
	let original := User{ name: "Anna", age: 30 }
	let updated := User {
		original...,
		name: "Maria",
	}
	discard updated
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestInvalidStructSpreadDoesNotSuppressOmittedFieldDefaults(t *testing.T) {
	input := `
module main

type Source struct {
	name: string,
}

type Target struct {
	name: string,
	view: ref int,
}

fn Invalid(source: Source) Target {
	return Target {
		source...,
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"cannot spread Source into Target; spread source must have type Target at 15:9",
		`field "view" in struct Target has no default value and must be initialized at 14:9`,
	}
	assertSemaErrors(t, errors, expected)
}

func TestArrayLiteralSpreadLengthUsesExactCompactAccounting(t *testing.T) {
	input := `
module main

fn Invalid(first: int[9223372036854775807], second: int[1]) void {
	let values := [first..., second...]
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestResultTypeArgumentCountErrors(t *testing.T) {
	input := `
module main

enum IOError error {
	InvalidValue,
}

fn MissingResultArgument() Result[int] {
	return Ok(1)
}

fn TooManyResultArguments() Result[int, IOError, string] {
	return Ok(1)
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"Result requires exactly 2 type arguments, got 1 at 8:28",
		"Result requires exactly 2 type arguments, got 3 at 12:29",
	}

	assertSemaErrors(t, errors, expected)
}

func TestForRangeLoopIsValid(t *testing.T) {
	input := `
module main

fn Test() void {
	for i in 0..<10 {
		let x := i
	}

	return
}

fn DecimalRange() void {
	let start: decimal := 0.001
	let end: decimal := 0.002
	let increment: decimal := 0.00001

	for value in start..<end step increment {
		let copy: decimal := value
	}
}

fn FloatRange() void {
	let start: float := 0.0
	let end: float := 1.0
	let increment: float := 0.1

	for value in start..<end step increment {
		let copy: float := value
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestForArrayAndSliceLoopBindings(t *testing.T) {
	input := `
module main

fn ArrayLoop(values: int[3]) void {
	for value in values {
		let copy: int := value
	}
}

fn SliceLoop(values: ref int[]) void {
	for value in values {
		let copy: int := value
	}
}

fn SliceIndexValueLoop(values: ref int[]) void {
	for index, value in values {
		let i: int := index
		let copy: int := value
	}
}

fn StringIndexValueLoop(values: string) void {
	for index, value in values {
		let i: int := index
		let r: rune := value
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestForCompilerKnownCollectionLoopBindings(t *testing.T) {
	input := `
module main

fn VecLoop(values: Vec[int]) void {
	for value in values {
		let copy: int := value
	}
}

fn VecIndexValueLoop(values: Vec[int]) void {
	for index, value in values {
		let i: int := index
		let copy: int := value
	}
}

fn RefVecLoop(values: ref Vec[int]) void {
	for value in values {
		let copy: int := value
	}
}

fn VectorLoop(values: vector[int, 3]) void {
	for value in values {
		let copy: int := value
	}
}

fn RefVectorIndexValueLoop(values: ref vector[float64, 3]) void {
	for index, value in values {
		let i: int := index
		let copy: float64 := value
	}
}

fn SetLoop(values: Set[string]) void {
	for value in values {
		let copy: string := value
	}
}

fn MapLoop(values: Map[string, int]) void {
	for key, value in values {
		let copyKey: string := key
		let copyValue: int := value
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestForCompilerKnownIteratorUsesExplicitGenericConformance(t *testing.T) {
	input := `
module main

type Counter struct {
	current: int,
}

impl Counter implements Iterator[int] {
	fn Next() Option[int] {
		self.current += 1
		return Some(self.current)
	}
}

fn MakeCounter() Counter {
	return Counter { current: 0 }
}

fn FromMutableStorage() void {
	let mut counter := Counter { current: 0 }
	for value in counter {
		let typed: int := value
	}
}

fn FromOwnedTemporary() void {
	for value in MakeCounter() {
		let typed: int := value
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestForCompilerKnownIteratorRejectsImmutableStorageAndDuckTyping(t *testing.T) {
	input := `
module main

type Explicit struct {}
impl Explicit implements Iterator[int] {
	fn Next() Option[int] { return None() }
}

type Accidental struct {}
impl Accidental {
	fn Next() Option[int] { return None() }
}

fn Test() void {
	let immutable := Explicit {}
	for value in immutable {}

	let mut accidental := Accidental {}
	for value in accidental {}
}
`

	errors := analyzeSourceRaw(t, input)
	if len(errors) != 2 {
		t.Fatalf("wrong iterator diagnostics: %v", errors)
	}
	if !strings.Contains(errors[0].Message, "iteration requires a mutable iterator source") {
		t.Fatalf("missing immutable iterator diagnostic: %v", errors)
	}
	if !strings.Contains(errors[1].Message, "type Accidental is not iterable") {
		t.Fatalf("naming convention unexpectedly enabled iteration: %v", errors)
	}
}

func TestCompilerKnownIteratorConformanceSubstitutesElementType(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type Correct struct {}
impl Correct implements Iterator[string] {
	fn Next() Option[string] { return None() }
}

type Wrong struct {}
impl Wrong implements Iterator[string] {
	fn Next() Option[int] { return None() }
}
`)

	if len(errors) != 1 || !strings.Contains(errors[0].Message, "type Wrong method Next does not match interface Iterator") {
		t.Fatalf("generic Iterator conformance was not substituted: %v", errors)
	}
}

func TestForCompilerKnownCollectionLoopBindingErrors(t *testing.T) {
	input := `
module main

fn SetIndexValueLoop(values: Set[string]) void {
	for index, value in values {
	}
}

fn MapSingleBinding(values: Map[string, int]) void {
	for entry in values {
	}
}

fn MapTooManyBindings(values: Map[string, int]) void {
	for key, value, extra in values {
	}
}

fn VectorTooManyBindings(values: vector[int, 3]) void {
	for index, value, extra in values {
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"set iteration supports one loop binding, got 2 at 5:6",
		"map iteration requires key and value bindings, got 1 at 10:6",
		"map iteration requires key and value bindings, got 3 at 15:6",
		"sequential iteration supports one or two loop bindings, got 3 at 20:6",
	}
	assertSemaErrors(t, errors, expected)
}

func TestBareSequenceTypesAreOwnedArrays(t *testing.T) {
	input := `
module main

type Packet struct {
	payload: byte[],
}

type Lexer struct {
	input: rune[],
	file: string,
}

fn Use(values: int[]) int {
	return values[0]
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	packet := analyzer.types["Packet"]
	if len(packet.Fields) != 1 || packet.Fields[0].Type.Kind != ArrayType || packet.Fields[0].Type.ArrayShape != ArrayShapeDynamic || packet.Fields[0].Type.ArrayLengthDecimal != "" {
		t.Fatalf("Packet.payload should be owned byte[] array, got %+v", packet.Fields)
	}
	lexer := analyzer.types["Lexer"]
	if len(lexer.Fields) != 2 || lexer.Fields[0].Type.Kind != ArrayType || lexer.Fields[0].Type.ArrayShape != ArrayShapeDynamic || lexer.Fields[0].Type.ArrayLengthDecimal != "" {
		t.Fatalf("Lexer.input should be owned rune[] array, got %+v", lexer.Fields)
	}
}

func TestForTooManySequentialBindings(t *testing.T) {
	input := `
module main

fn Test(values: ref int[]) void {
	for a, b, c in values {
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"sequential iteration supports one or two loop bindings, got 3 at 5:6",
	}

	assertSemaErrors(t, errors, expected)
}

func TestWhileConditionMustBeBool(t *testing.T) {
	input := `
module main

fn Test() void {
	while 1 {
	}
	return
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"while condition must be bool, got int at 5:8",
	}

	assertSemaErrors(t, errors, expected)
}

func TestWhileBodyAssignmentsDoNotLeak(t *testing.T) {
	input := `
module main

fn Test(running: bool) int {
	let mut result: int

	while running {
		result = 1
	}

	return result
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{}

	assertSemaErrors(t, errors, expected)
}

func TestWhileTrueWithoutBreakSatisfiesReturn(t *testing.T) {
	input := `
module main

fn Test() int {
	while true {
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

// rules/control-flow/flowcontrol_for.md; correction21.md requires a nested
// loop's break to leave only that nested loop.
func TestNestedWhileBreakDoesNotExitInfiniteOuterWhile(t *testing.T) {
	input := `
module main

fn Test() int {
	while true {
		while true {
			break
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

// rules/control-flow/flowcontrol_while.md; correction21.md excludes a break in
// a statically unreachable branch from the outer loop's exit fact.
func TestUnreachableBreakDoesNotExitInfiniteWhile(t *testing.T) {
	input := `
module main

fn Test() int {
	while true {
		if false {
			break
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestWhileTrueWithBreakRequiresReturnAfterLoop(t *testing.T) {
	input := `
module main

fn Test() int {
	while true {
		break
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"function Test must return int at 4:4",
	}

	assertSemaErrors(t, errors, expected)
}

func TestWhileTrueBreakCarriesDefiniteAssignment(t *testing.T) {
	input := `
module main

fn Test() int {
	let mut result: int

	while true {
		result = 10
		break
	}

	return result
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestWhileTrueBreakWithoutAssignmentDoesNotSatisfyDefiniteAssignment(t *testing.T) {
	input := `
module main

fn Test() int {
	let mut result: int

	while true {
		break
	}

	return result
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{}

	assertSemaErrors(t, errors, expected)
}

func TestUnreachableStatementsInsideWhileBody(t *testing.T) {
	input := `
module main

fn UnreachableAfterBreak() void {
	while true {
		break
		let value: int := 10
	}
}

fn UnreachableAfterContinue() void {
	while true {
		continue
		let value: int := 10
	}
}

fn UnreachableAfterReturn() int {
	while true {
		return 10
		let value: int := 20
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"unreachable code at 7:3",
		"unreachable code at 14:3",
		"unreachable code at 21:3",
	}

	assertSemaErrors(t, errors, expected)
}

func TestComparisonChainingIsRejected(t *testing.T) {
	input := `
module main

fn InvalidComparisonChain(value: int) void {
	while 0 <= value < 100 {
		break
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"comparison chaining is not supported at 5:19",
	}

	assertSemaErrors(t, errors, expected)
}

func TestForLoopVariableIsImmutableAndScoped(t *testing.T) {
	input := `
module main

fn Test() void {
	for i in 0..<10 {
		i = 1
	}

	let x := i
	return
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"cannot assign to immutable variable i at 6:3",
		"undefined variable i at 9:11",
	}

	assertSemaErrors(t, errors, expected)
}

func TestForOpenEndedRangeIsRejected(t *testing.T) {
	input := `
module main

fn Test() void {
	for i in 0.. {
	}
	return
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"range used in for loop must be finite at 5:12",
	}

	assertSemaErrors(t, errors, expected)
}

func TestForRangeBoundsMustHaveSameType(t *testing.T) {
	input := `
module main

fn InvalidStringBounds(end: string) void {
	for i in 0..<end {
	}
}

fn InvalidMixedBounds(start: int, end: uint) void {
	for i in start..<end {
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"cannot create range with bounds int and string at 5:12",
		"cannot create range with bounds int and uint at 10:16",
	}

	assertSemaErrors(t, errors, expected)
}

func TestForRangeStep(t *testing.T) {
	input := `
module main

fn ValidStep() void {
	for i in 0..<10 step 2 {
		let copy: int := i
	}
}

fn InvalidZeroStep() void {
	for i in 0..<10 step 0 {
	}
}

fn InvalidNegativeStep() void {
	for i in 0..<10 step -1 {
	}
}

fn InvalidStepOnSlice(values: ref int[]) void {
	for value in values step 2 {
	}
}

fn InvalidDecimalZeroStep() void {
	for value in 0.0..<1.0 step 0.0 {
	}
}

fn InvalidDecimalNegativeStep() void {
	for value in 0.0..<1.0 step -0.1 {
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"for range step must not be zero at 11:23",
		"for ascending range step must be positive at 16:23",
		"for step is only valid for range iteration at 21:27",
		"for range step must not be zero at 26:30",
		"for ascending range step must be positive at 31:30",
	}

	assertSemaErrors(t, errors, expected)
}

func TestBreakAndContinueRequireLoop(t *testing.T) {
	input := `
module main

fn Test() void {
	break
	continue
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"break is only valid inside a loop at 5:2",
		"continue is only valid inside a loop at 6:2",
	}

	assertSemaErrors(t, errors, expected)
}

func TestInfiniteForWithoutBreakSatisfiesReturn(t *testing.T) {
	input := `
module main

fn Test() int {
	for {
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

// rules/control-flow/flowcontrol_for.md; correction21.md requires target-aware
// break classification for nested infinite for loops.
func TestNestedForBreakDoesNotExitInfiniteOuterFor(t *testing.T) {
	input := `
module main

fn Test() int {
	for {
		for {
			break
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

// rules/control-flow/flowcontrol_for.md; correction21.md excludes a break in a
// statically unreachable branch from the current loop's continuation paths.
func TestUnreachableBreakDoesNotExitInfiniteFor(t *testing.T) {
	input := `
module main

fn Test() int {
	for {
		if false {
			break
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestInfiniteForWithContinueMakesFollowingReturnUnreachable(t *testing.T) {
	input := `
module main

fn Test() int {
	let mut value: int

	for {
		value = 10
		continue
	}

	return value
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"unreachable code at 12:2",
	}

	assertSemaErrors(t, errors, expected)
}

func TestLambdaFunctionValueCall(t *testing.T) {
	input := `
module main

fn Test() int {
	let double := fn(value: int) int {
		return value * 2
	}

	return double(10)
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestOwnedLambdaParameterIsMutable(t *testing.T) {
	input := `
module main

fn Test() int {
	let normalize := fn(value: int) int {
		value = 0
		return value
	}
	return normalize(10)
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestTypedLambdaVariableAndFunctionValueCall(t *testing.T) {
	input := `
module main

fn Test() bool {
	let positive: fn(int) bool := fn(value: int) bool {
		return value > 0
	}

	return positive(10)
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestLambdaArgumentToFunctionValueParameter(t *testing.T) {
	input := `
module main

fn Apply(value: int, callback: fn(int) int) int {
	return callback(value)
}

fn Test() int {
	return Apply(
		10,
		fn(value: int) int {
			return value * 2
		},
	)
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestLambdaInvalidReturnType(t *testing.T) {
	input := `
module main

fn Test() void {
	let operation := fn() int {
		return true
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"lambda must return int, got bool at 6:10",
	}

	assertSemaErrors(t, errors, expected)
}

func TestLambdaFunctionAssignmentMismatch(t *testing.T) {
	input := `
module main

fn Test() void {
	let operation: fn(int) bool := fn(value: string) bool {
		return value != ""
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"cannot initialize fn(int) bool with fn(string) bool at 5:33",
	}

	assertSemaErrors(t, errors, expected)
}

func TestLambdaImplicitCaptureIsRejected(t *testing.T) {
	input := `
module main

fn Test(factor: int) int {
	let multiply := fn(value: int) int {
		return value * factor
	}

	return multiply(10)
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"lambda cannot access outer variable factor without explicit capture at 6:18",
	}

	assertSemaErrors(t, errors, expected)
}

func TestNamedFunctionValueIsAssignable(t *testing.T) {
	input := `
module main

fn IsPositive(value: int) bool {
	return value > 0
}

fn Test() bool {
	let predicate := IsPositive
	return predicate(10)
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

// rules/declarations/lambda-functions.md; correction11.md distinguishes ordinary
// module lookup from closure capture candidates.
func TestLambdaModuleBindingLookupIsNotCapture(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main
let answer: int := 42
fn Read() int {
	let reader := fn() int { return answer }
	return reader()
}
`)
	assertSemaErrors(t, errors, nil)

	errors = analyzeSourceRaw(t, `
module main
let answer: int := 42
fn Read() int {
	let reader := capture(answer) fn() int { return answer }
	return reader()
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "cannot capture non-local declaration answer") {
		t.Fatalf("explicit module capture errors = %v", errors)
	}
}

// rules/declarations/static.md; correction15.md removes instance receiver
// authority from static methods and rejects static free functions.
func TestStaticFunctionDeclarationContextAndReceiver(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main
type Counter struct { value: int, }
impl Counter {
	static fn Invalid() int { return self.value }
}
static fn Free() void {}
`)
	if len(errors) != 2 || !errorsContainMessage(errors, "undefined variable self") || !errorsContainMessage(errors, "static fn is only valid inside impl") {
		t.Fatalf("static function errors = %v", errors)
	}
}

func errorsContainMessage(errors []Error, fragment string) bool {
	for _, err := range errors {
		if strings.Contains(err.Message, fragment) {
			return true
		}
	}
	return false
}

// rules/control-flow/discard.md and rules/memory/copy_move.md require an
// explicit move into a non-terminal aggregate temporary, then commit it before
// the aggregate itself is discarded.
func TestDiscardAggregateTemporaryCommitsMoves(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main
@noCopy
type Session struct { id: int, }
type Holder struct { session: Session, }
fn Use(value: ref Session) void {}
fn Test() void {
	let session := Session { id: 1 }
	discard Holder { session: <-session }
	Use(ref session)
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "use of moved value session") {
		t.Fatalf("aggregate discard errors = %v", errors)
	}
}

// rules/foundations/operators.md, logical operators; correction22.md keeps
// proven short-circuit RHS ownership effects out of the continuing path.
func TestLogicalShortCircuitDoesNotCommitImpossibleMove(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main
@noCopy
type Token struct { id: int, }
fn Consume(-> token: Token) bool { return true }
fn Use(value: ref Token) void {}
fn Test() void {
	let first := Token { id: 1 }
	let second := Token { id: 2 }
	let a := false && Consume(<-first)
	let b := true || Consume(<-second)
	Use(ref first)
	Use(ref second)
}
`)
	assertSemaErrors(t, errors, nil)
}

func TestOverloadedNamedFunctionValueUsesExplicitTargetType(t *testing.T) {
	input := `
module main

fn Convert(value: int) string {
	return "int"
}

fn Convert(value: string) int {
	return 1
}

fn Test() string {
	let converter: fn(int) string := Convert
	return converter(10)
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func analyzeSource(t *testing.T, input string) []Error {
	t.Helper()

	_, errors := analyzeSourceWithAnalyzer(t, input)
	return errors
}

func analyzeSourceWithAnalyzer(t *testing.T, input string) (*Analyzer, []Error) {
	t.Helper()
	return analyzeSourceWithAnalyzerRaw(t, ensureModuleForTest(input))
}

func analyzeSourceRaw(t *testing.T, input string) []Error {
	t.Helper()

	_, errors := analyzeSourceWithAnalyzerRaw(t, input)
	return errors
}

func analyzeSourceWithAnalyzerRaw(t *testing.T, input string) (*Analyzer, []Error) {
	t.Helper()

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	analyzer := NewAnalyzer()
	return analyzer, analyzer.Analyze(program)
}

func ensureModuleForTest(input string) string {
	if strings.Contains(input, "module ") {
		return input
	}
	return input + "\nmodule test\n"
}

func parseExpressionSource(t *testing.T, input string) ast.Expression {
	t.Helper()

	l := lexer.New("let value := " + input)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	if len(program.Statements) != 1 {
		t.Fatalf("wrong statement count. got=%d want=1", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("statement is not LetStatement. got=%T", program.Statements[0])
	}

	return stmt.Value
}

func TestBasicTypesWideIntegerRangesAndDecimal128(t *testing.T) {
	input := `
module main

fn Test() void {
	let int128Max: int128 := 170141183460469231731687303715884105727
	let int128Overflow: int128 := 170141183460469231731687303715884105728
	let uint256Max: uint256 := 115792089237316195423570985008687907853269984665640564039457584007913129639935
	let uint256Overflow: uint256 := 115792089237316195423570985008687907853269984665640564039457584007913129639936
	let d128: decimal128 := 12345678901234567890.12345678901234
	let d: decimal := d128
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"value 170141183460469231731687303715884105728 overflows int128 at 6:32",
		"value 115792089237316195423570985008687907853269984665640564039457584007913129639936 overflows uint256 at 8:34",
		"cannot initialize decimal with decimal128 at 10:20",
	}

	assertSemaErrors(t, errors, expected)
}

func TestExplicitDecimalToIntegerConversions(t *testing.T) {
	input := `
module main

fn Test(value: decimal, exact: decimal128) int128 {
	let small: int := int(value)
	let wide: int128 := int128(exact)
	return int128(small) + wide
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestBasicTypesBitwiseAndCharRules(t *testing.T) {
	input := `
module main

fn Test(left: int128, right: int128, b: bool) int128 {
	let ok: int128 := (left & right) | (left ^ right)
	let bad := b & b
	let invalidChar: char := 'AB'
	return ~ok
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"operator & requires integer operands at 6:15",
		"character literal must contain exactly one character at 7:27",
	}

	assertSemaErrors(t, errors, expected)
}

func TestNumericLiteralsUseIntegerOperandContext(t *testing.T) {
	valid := `
module main

fn Mask(crc: uint32) uint32 {
	if (crc & 1) != 0 {
		return crc ^ 0xFFFFFFFFu
	}
	return 1u | crc
}
`
	analyzer, errors := analyzeSourceWithAnalyzer(t, valid)
	assertSemaErrors(t, errors, nil)
	function := analyzer.functions["Mask"][0]
	if function.ReturnType.Name != "uint32" {
		t.Fatalf("Mask return type = %s, want uint32", typeDisplayName(function.ReturnType))
	}

	invalid := `
module main

fn Invalid(value: uint8, runtimeMask: uint) void {
	let overflow := value & 256
	let typedMismatch := value & runtimeMask
}
`
	errors = analyzeSourceRaw(t, invalid)
	assertSemaErrors(t, errors, []string{
		"value 256 overflows uint8 at 5:26",
		"cannot apply operator & to uint8 and uint at 6:29",
	})
}

func TestOrderedComparisonsRequireOrderableCompatibleOperands(t *testing.T) {
	valid := `
module main

fn Test(number: int, decimal: float64, character: char, codepoint: rune, text: string) void {
	let numeric := number < decimal
	let characters := character <= 'z'
	let codepoints := codepoint > 'a'
	let strings := text >= "prefix"
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, valid), nil)

	invalid := `
module main

enum State {
	ready,
}

type Point struct {
	x: int,
}

type Choice union {
	value(int),
}

fn Test(flag: bool, state: State, point: Point, choice: Choice, values: int[2]) void {
	let bools := flag < flag
	let enums := state <= state
	let structs := point > point
	let unions := choice >= choice
	let arrays := values < values
}
`
	errors := analyzeSourceRaw(t, invalid)
	if len(errors) != 5 {
		t.Fatalf("wrong sema error count. got=%d want=5 errors=%v", len(errors), errors)
	}
	for index, err := range errors {
		if err.ID != diagnostics.OperatorNonOrderable {
			t.Fatalf("error %d ID = %q, want %q", index, err.ID, diagnostics.OperatorNonOrderable)
		}
		if err.Help == "" {
			t.Fatalf("error %d is missing help", index)
		}
	}
}

func TestCompileTimeShiftValidation(t *testing.T) {
	input := `
module main

type Small int8

fn Test(dynamic: int) void {
	let negative := Small(1) << -1
	let equalWidth := Small(1) << 8
	let tooLarge := Small(1) >> 9
	let overflow := Small(64) << 1
	let validMinimum := Small(-1) << 7
	let unsignedTruncation := uint8(255) << 7
	let dynamicCount := Small(1) << dynamic
}

`
	errors := analyzeSourceRaw(t, input)
	if len(errors) != 4 {
		t.Fatalf("wrong sema error count. got=%d want=4 errors=%v", len(errors), errors)
	}
	wantIDs := []string{
		diagnostics.OperatorInvalidShiftCount,
		diagnostics.OperatorInvalidShiftCount,
		diagnostics.OperatorInvalidShiftCount,
		diagnostics.OperatorShiftOverflow,
	}
	for index, err := range errors {
		if err.ID != wantIDs[index] {
			t.Fatalf("error %d ID = %q, want %q (%v)", index, err.ID, wantIDs[index], err)
		}
		if err.Help == "" {
			t.Fatalf("error %d is missing help", index)
		}
	}
}

func TestCompileTimeCheckedIntegerFailuresDoNotReachSemanticIR(t *testing.T) {
	input := `
module main

fn Invalid() void {
    let add := int8(127) + int8(1)
    let subtract := uint8(0) - uint8(1)
    let multiply := int8(64) * int8(2)
    let divideZero := int32(1) / int32(0)
    let remainderZero := uint64(1) % uint64(0)
    let divideOverflow := int8(-128) / int8(-1)
    let remainderOverflow := int8(-128) % int8(-1)
    let negateOverflow := -int8(-128)
}

fn Valid() void {
    let add := int8(126) + int8(1)
    let divide := int32(8) / int32(2)
    let remainder := uint64(9) % uint64(4)
    let negate := -int8(127)
}
`
	errors := analyzeSourceRaw(t, input)
	want := []string{
		diagnostics.OperatorIntegerOverflow,
		diagnostics.OperatorIntegerOverflow,
		diagnostics.OperatorIntegerOverflow,
		diagnostics.OperatorDivisionByZero,
		diagnostics.OperatorRemainderByZero,
		diagnostics.OperatorIntegerOverflow,
		diagnostics.OperatorIntegerOverflow,
		diagnostics.OperatorIntegerOverflow,
	}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v, want %d", errors, len(want))
	}
	for index, id := range want {
		if errors[index].ID != id || errors[index].Help == "" {
			t.Errorf("error %d = %#v, want ID %s with help", index, errors[index], id)
		}
	}
}

func TestEqualityRequiresComparableCompatibleOperands(t *testing.T) {
	valid := `
module main

enum State {
	ready,
}

type Percent int range 0..100

type Point struct {
	x: int,
	label: string,
}

type Choice union {
	none,
	point(Point),
}

fn Test(
	flag: bool,
	state: State,
	left: Point,
	right: Point,
	choice: Choice,
	values: Point[2],
	percent: Percent,
	first: RawPtr[int],
	second: RawPtr[int],
	firstRef: ref int,
	secondRef: ref int,
) void {
	let bools := flag == false
	let enums := state != State.ready
	let structs := left == right
	let unions := choice != choice
	let arrays := values == values
	let shapedLiteral := percent == 50
	let pointers := first != second
	let references := firstRef == secondRef
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, valid), nil)

	invalid := `
module main

type Point struct {
	x: int,
}

type OtherPoint struct {
	x: int,
}

type SliceView struct {
	values: ref int[],
}

type MaybeSlice union {
	none,
	view(ref int[]),
}

fn Test(
	point: Point,
	other: OtherPoint,
	leftSlice: ref int[],
	rightSlice: ref int[],
	view: SliceView,
	choice: MaybeSlice,
	task: Task[int],
) void {
	let differentStructs := point == other
	let slices := leftSlice == rightSlice
	let structWithSlice := view == view
	let unionWithSlice := choice != choice
	let opaqueResource := task == task
}
`
	errors := analyzeSourceRaw(t, invalid)
	if len(errors) != 5 {
		t.Fatalf("wrong sema error count. got=%d want=5 errors=%v", len(errors), errors)
	}
	for index, err := range errors {
		if err.ID != diagnostics.OperatorNonComparable {
			t.Fatalf("error %d ID = %q, want %q", index, err.ID, diagnostics.OperatorNonComparable)
		}
		if err.Help == "" {
			t.Fatalf("error %d is missing help", index)
		}
	}
}

func TestStringConcatenationAcceptsCanonicalTextOperandMatrix(t *testing.T) {
	valid := `
module main

fn Test(text: string, character: char, codepoint: rune) void {
	let stringString: string := text + " suffix"
	let stringChar: string := text + character
	let charString: string := character + text
	let stringRune: string := text + codepoint
	let runeString: string := codepoint + text
	let charChar: string := character + 'x'
	let runeRune: string := codepoint + 10r
	let charRune: string := character + codepoint
	let runeChar: string := codepoint + character
	let folded: string := "SEC " + "language" + " compiler"

	let mut appended: string := stringString
	appended += character
	appended += codepoint
	appended += text
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, valid), nil)
}

func TestStringConcatenationRejectsHiddenConversions(t *testing.T) {
	invalid := `
module main

type Label string

enum State {
	ready,
}

type Point struct {
	x: int,
}

type Choice union {
	value(int),
}

fn Test(
	number: int,
	real: float64,
	decimalValue: decimal,
	flag: bool,
	state: State,
	point: Point,
	choice: Choice,
	array: int[2],
	view: ref int[],
	values: list[int],
	label: Label,
) void {
	let integerOperand := "value " + number
	let floatOperand := "value " + real
	let decimalOperand := "value " + decimalValue
	let boolOperand := "value " + flag
	let enumOperand := "value " + state
	let structOperand := "value " + point
	let unionOperand := "value " + choice
	let arrayOperand := "value " + array
	let sliceOperand := "value " + view
	let collectionOperand := "value " + values
	let nominalOperand := "value " + label

	let mut output: string := "value "
	output += number
}
`
	errors := analyzeSourceRaw(t, invalid)
	if len(errors) != 12 {
		t.Fatalf("wrong sema error count. got=%d want=12 errors=%v", len(errors), errors)
	}
	for index, err := range errors {
		if err.ID != diagnostics.OperatorInvalidConcatOperand {
			t.Fatalf("error %d ID = %q, want %q", index, err.ID, diagnostics.OperatorInvalidConcatOperand)
		}
		if err.Help == "" {
			t.Fatalf("error %d is missing help", index)
		}
		if !strings.Contains(err.Message, "string, char, and rune") {
			t.Fatalf("error %d does not name accepted operand categories: %v", index, err)
		}
	}
}

func TestArrayAndSliceMembership(t *testing.T) {
	input := `
module main

type Percent int range 0..100

fn Test(
	number: int,
	fixed: int[3],
	fixedRef: ref int[3],
	view: ref int[],
	writable: ref mut int[],
	runes: rune[2],
	percents: Percent[2],
	rows: int[2][2],
) bool {
	let literalArray := 2 in [1, 2, 3]
	let fixedArray := number in fixed
	let referencedArray := number in fixedRef
	let sharedSlice := number in view
	let mutableSlice := number in writable
	let runeLiteral := 'a' in runes
	let shapedLiteral := 50 in percents
	let nestedArray := [1, 2] in rows

	return literalArray && fixedArray && referencedArray && sharedSlice &&
		mutableSlice && runeLiteral && shapedLiteral && nestedArray
}
`

	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)
}

func TestInvalidArrayAndSliceMembershipHasStructuredDiagnostics(t *testing.T) {
	input := `
module main

type ViewHolder struct {
	values: ref int[],
}

fn Test(
	text: string,
	numbers: int[2],
	view: ref int[],
	holder: ViewHolder,
	holders: ViewHolder[1],
	dynamic: int[],
	listValues: list[int],
	mapValues: map[int, string],
	setValues: set[int],
) void {
	let arrayMismatch := text in numbers
	let sliceMismatch := text in view
	let nonComparableElement := holder in holders
	let ownedDynamicArray := 1 in dynamic
	let unsupportedList := 1 in listValues
	let unsupportedMap := 1 in mapValues
	let unsupportedSet := 1 in setValues
	let unsupportedString := "x" in text
}
`

	errors := analyzeSourceRaw(t, input)
	if len(errors) != 8 {
		t.Fatalf("wrong sema error count. got=%d want=8 errors=%v", len(errors), errors)
	}
	for index, err := range errors {
		if err.ID != diagnostics.OperatorInvalidMembership {
			t.Fatalf("error %d ID = %q, want %q (%v)", index, err.ID, diagnostics.OperatorInvalidMembership, err)
		}
		if err.Help == "" {
			t.Fatalf("error %d is missing help", index)
		}
	}
	if !strings.Contains(errors[2].Message, "element type ViewHolder is not equality-comparable") {
		t.Fatalf("non-comparable element diagnostic is not specific: %v", errors[2])
	}
}

func TestImplSelfMethodCallRetainsReturnType(t *testing.T) {
	input := `
module main

type Counter struct {
	value: int,
}

impl Counter {
	fn advance() int {
		return self.value + 1
	}

	fn Next() int {
		return self.advance()
	}
}
`

	assertSemaErrors(t, analyzeSource(t, input), nil)
}

func TestContextualMatrixMultiplyTypesAndShapes(t *testing.T) {
	valid := `
module main

fn MatrixProduct(left: matrix[float32, 4, 3], right: matrix[float32, 3, 2]) matrix[float32, 4, 2] {
	return left x right
}

fn MatrixVectorProduct(left: matrix[int, 4, 3], right: vector[int, 3]) vector[int, 4] {
	return left x right
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, valid), nil)

	invalid := `
module main

fn BadInner(left: matrix[int, 4, 3], right: matrix[int, 2, 5]) matrix[int, 4, 5] {
	return left x right
}

fn BadElement(left: matrix[int, 4, 3], right: vector[float32, 3]) vector[int, 4] {
	return left x right
}

fn BadLeft(left: int, right: matrix[int, 2, 2]) matrix[int, 2, 2] {
	return left x right
}
`
	errors := analyzeSourceRaw(t, invalid)
	assertSemaErrors(t, errors, []string{
		"matrix multiplication inner dimensions differ: 3 and 2 at 5:14",
		"cannot apply operator * to int and float32 at 9:14",
		"left operand of x must be matrix, got int at 13:14",
	})
}

func TestStaticShapedExtentProductUsesTargetUint(t *testing.T) {
	valid := `
module main

fn Empty(value: tensor[int, 4294967296, 4294967296, 0]) void {
	discard value
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, valid), nil)

	invalid := `
module main

fn TooLarge(value: matrix[int, 4294967296, 4294967296]) void {
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, invalid), []string{
		"matrix static element count 18446744073709551616 overflows target uint at 4:20",
	})
}

func TestEnumInitializerRepetitionUsesCurrentIota(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main

enum X int {
	A = iota,
	B,
	C,
	D = 10,
	E,
	F = iota + 7,
	G,
}
`)
	if len(errors) != 0 {
		t.Fatalf("enum initializer repetition produced errors: %v", errors)
	}
	want := map[string]string{"A": "0", "B": "1", "C": "2", "D": "10", "E": "10", "F": "12", "G": "13"}
	for name, expected := range want {
		got := analyzer.Types()["X"].EnumConsts[name].Value
		if got == nil || got.String() != expected {
			t.Fatalf("X.%s = %v, want %s", name, got, expected)
		}
	}
}

func TestClosedAndOpenEnumDomains(t *testing.T) {
	valid := `module main

enum Closed int {
	FIRST = 1,
	ALIAS = 1,
	SECOND = 2,
}

enum Open bit[2] {
	ZERO = 0,
	ONE = 1,
	TWO = 2,
}

fn ClosedMatch(value: Closed) int {
	return match value {
		Closed.FIRST => 1
		Closed.SECOND => 2
	}
}

fn OpenValue() Open {
	return Open(3)
}

fn CheckedClosed(raw: int) Result[Closed, EnumValueError] {
	let value := try Closed(raw)
	return Ok(value)
}

fn CheckedOpen(raw: int) Result[Open, EnumValueError] {
	let value := try Open(raw)
	return Ok(value)
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, valid), nil)

	invalidConversion := analyzeSourceRaw(t, `module main

enum Closed int {
	FIRST = 1,
	SECOND = 2,
}

fn Invalid() Closed {
	return Closed(9)
}
`)
	if len(invalidConversion) != 1 || !strings.Contains(invalidConversion[0].Message, "not a declared value of closed enum Closed") {
		t.Fatalf("invalid closed enum conversion errors = %v", invalidConversion)
	}

	openMatch := analyzeSourceRaw(t, `module main

enum Open bit[2] {
	ZERO = 0,
	ONE = 1,
	TWO = 2,
}

fn Invalid(value: Open) int {
	return match value {
		Open.ZERO => 0
		Open.ONE => 1
		Open.TWO => 2
	}
}
`)
	if len(openMatch) != 1 || !strings.Contains(openMatch[0].Message, "open bit-backed enum Open") {
		t.Fatalf("open enum match errors = %v", openMatch)
	}
}

// rules/declarations/enums.md; correction7.md requires enum narrowing to use
// ordinary checked integer conversion semantics.
func TestEnumToIntegerNarrowingIsChecked(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main
enum Wide int { Zero = 0, Big = 300, }
enum Open bit[16] { Zero = 0, }
fn Known() uint8 { return uint8(Wide.Zero) }
fn Narrow(value: Wide) Result[uint8, ArithmeticError] { return Ok(try uint8(value)) }
fn NarrowOpen(value: Open) Result[uint8, ArithmeticError] { return Ok(try uint8(value)) }
`)
	assertSemaErrors(t, errors, nil)

	errors = analyzeSourceRaw(t, `
module main
enum Wide int { Big = 300, }
fn Invalid() uint8 { return uint8(Wide.Big) }
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "overflows uint8") {
		t.Fatalf("known enum narrowing errors = %v", errors)
	}
}

// rules/declarations/interfaces.md and rules/declarations/functions.md;
// correction10.md preserves distinct interface overload requirements.
func TestInterfaceMethodOverloadSets(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main
interface IntFormatter { fn Format(value: int) string }
interface StringFormatter { fn Format(value: string) string }
interface Formatter implements IntFormatter, StringFormatter {
	fn Format(value: bool) string
}
`)
	assertSemaErrors(t, errors, nil)
	if got := len(analyzer.types["Formatter"].InterfaceMethods); got != 3 {
		t.Fatalf("Formatter overload count = %d, want 3", got)
	}

	errors = analyzeSourceRaw(t, `
module main
interface Bad {
	fn Read(value: int) string
	fn Read(value: int) bool
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "incompatible callable contract") {
		t.Fatalf("interface overload conflict errors = %v", errors)
	}
}

func assertSemaErrors(t *testing.T, errors []Error, expected []string) {
	t.Helper()

	if len(errors) != len(expected) {
		t.Fatalf("wrong sema error count. got=%d want=%d errors=%v", len(errors), len(expected), errors)
	}

	for i, expectedError := range expected {
		if errors[i].Error() != expectedError {
			t.Fatalf("wrong sema error %d. got=%q want=%q", i, errors[i].Error(), expectedError)
		}
	}
}
