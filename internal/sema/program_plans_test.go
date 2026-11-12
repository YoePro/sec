package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func analyzePlanSource(t *testing.T, source string, configure func(*Analyzer)) *Analyzer {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, "main.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	analyzer := NewAnalyzer()
	if configure != nil {
		configure(analyzer)
	}
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("sema errors: %v", errors)
	}
	return analyzer
}

// A command program whose reachable code needs no runtime facility gets a
// runtime-free plan: statics are established physically from their
// compile-time values, lifecycle init and importing never become startup
// actions, and startup is static storage then transfer to entry.
// Rules: rules/compiler/initialization.md — §§ 3, 4, 6–8, 11, 13–16, 21, 23.
func TestProgramInitializationPlanRuntimeFree(t *testing.T) {
	source := `module main

type Widget struct {
    value: int,
}

impl Widget {
    init(value: int) {
        self.value = value
    }
}

fn Limit() int {
    return 40 + 2
}

let MaximumConnections: int := Limit()
let mut Ready: bool

fn main() int {
    return MaximumConnections - 42
}
`
	analyzer := analyzePlanSource(t, source, nil)
	initialization, shutdown, errors := analyzer.ProgramPlans(ProgramTargetCommand, "main", lexer.Token{})
	if len(errors) != 0 || initialization == nil || shutdown == nil {
		t.Fatalf("plans = %v %v %v", initialization, shutdown, errors)
	}
	text := initialization.String()
	for _, want := range []string{
		"entry command main (logical status from main's int result",
		"static main.MaximumConnections mutable=false init=42",
		"static main.Ready mutable=true init=false",
		"runtime none",
		"startup establish-static-storage EstablishStaticStorage requires []",
		"startup transfer-to-entry TransferToEntry requires [establish-static-storage]",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("plan lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Widget") || strings.Contains(text, "init(") {
		t.Errorf("lifecycle init became a startup action:\n%s", text)
	}
	if len(shutdown.RuntimeTeardown) != 0 || shutdown.DetachedExecutionShutdown {
		t.Errorf("runtime-free shutdown = %+v", shutdown)
	}
	if initialization.String() != analyzePlanSourcePlan(t, source) {
		t.Error("plan is not deterministic across analyses")
	}
}

func analyzePlanSourcePlan(t *testing.T, source string) string {
	plan, _, _ := analyzePlanSource(t, source, nil).ProgramPlans(ProgramTargetCommand, "main", lexer.Token{})
	return plan.String()
}

// Runtime requirements come from code reachable from the entry: allocation,
// spawned tasks and threads and detached execution, each with its source;
// startup orders facilities after their prerequisites and shutdown tears a
// facility down only after its dependents, detached execution first.
// Unreachable code contributes nothing.
// Rules: rules/compiler/initialization.md — §§ 12, 14–16, 30–36.
func TestProgramInitializationPlanRuntimeRequirements(t *testing.T) {
	source := `module main

fn Work() void {
}

fn Alloc() Result[void, AllocationError] {
    let mem := try Arena.WithCapacity(64u)
    return Ok()
}

fn Unused() void {
    let worker := spawn thread Work()
    detach worker
}

fn main() int {
    let job := try spawn task Work() { Err(_) => { return 1 } }
    await job
    let worker := spawn thread Work()
    detach worker
    match Alloc() {
        Ok(_) => return 0
        Err(error) => return 1
    }
}
`
	analyzer := analyzePlanSource(t, source, nil)
	initialization, shutdown, errors := analyzer.ProgramPlans(ProgramTargetCommand, "main", lexer.Token{})
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	text := initialization.String()
	for _, want := range []string{
		"runtime AllocatorRuntime (dynamic allocation in Alloc, supported)",
		"runtime DetachedExecutionManager (detached execution, unresolved)",
		"runtime PlatformThreadSupport (a thread started in main, unresolved)",
		"runtime TaskScheduler (a task spawned in main, unresolved)",
		"startup initialize-TaskScheduler InitializeRuntimeFacility requires [initialize-AllocatorRuntime]",
		"startup transfer-to-entry TransferToEntry requires [initialize-AllocatorRuntime, initialize-DetachedExecutionManager, initialize-PlatformThreadSupport, initialize-TaskScheduler]",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("plan lacks %q:\n%s", want, text)
		}
	}
	order := []string{}
	for _, action := range initialization.StartupActions {
		order = append(order, action.ID)
	}
	if strings.Index(strings.Join(order, ","), "initialize-AllocatorRuntime") > strings.Index(strings.Join(order, ","), "initialize-TaskScheduler") {
		t.Errorf("startup order %v starts the scheduler before its allocator", order)
	}
	teardown := []string{}
	for _, action := range shutdown.RuntimeTeardown {
		teardown = append(teardown, action.ID)
	}
	if !shutdown.DetachedExecutionShutdown || teardown[0] != "teardown-DetachedExecutionManager" ||
		strings.Index(strings.Join(teardown, ","), "teardown-TaskScheduler") > strings.Index(strings.Join(teardown, ","), "teardown-AllocatorRuntime") {
		t.Errorf("shutdown order %v", teardown)
	}
}

// A library Target has no program entry, plan or program-entry root, even
// when it declares `main`.
// Rules: rules/compiler/initialization.md — §§ 26, 39.
func TestProgramPlansLibraryAndDisabledAllocation(t *testing.T) {
	source := `module main

fn Alloc() Result[void, AllocationError] {
    let mem := try Arena.WithCapacity(64u)
    return Ok()
}

fn main() int {
    match Alloc() {
        Ok(_) => return 0
        Err(error) => return 1
    }
}
`
	library := analyzePlanSource(t, source, func(a *Analyzer) { a.SetProgramTargetKind(ProgramTargetLibrary) })
	if initialization, shutdown, errors := library.ProgramPlans(ProgramTargetLibrary, "main", lexer.Token{}); initialization != nil || shutdown != nil || len(errors) != 0 {
		t.Fatalf("library plans = %v %v %v", initialization, shutdown, errors)
	}
	for _, root := range library.CallGraph().Roots() {
		if root.Kind == CallRootProgramEntry {
			t.Fatalf("library Target acquired a program-entry root: %+v", root)
		}
	}

	// A heap requirement under a profile that disables allocation never
	// reaches planning: the reachable-graph allocation policy rejects the
	// program first.
	p := parser.New(lexer.NewWithFile(source, "main.sec"))
	program := p.ParseProgram()
	noalloc := NewAnalyzer()
	noalloc.SetAllocationProfile("noalloc")
	if errors := noalloc.Analyze(program); len(errors) == 0 {
		t.Fatal("noalloc profile accepted reachable allocation")
	}
}

// Statics whose type is not trivially destructible are recorded as static
// lifetime facts and shutdown destruction requirements, with the profile's
// deterministic static destruction capability left unresolved (MD-034).
// Rules: rules/compiler/initialization.md — §§ 9, 33, 37.
func TestProgramShutdownPlanRecordsStaticDestruction(t *testing.T) {
	analyzer := analyzePlanSource(t, `module main

type Res struct {
    id: int,
}

impl Res {
    free {
    }
}

let Handle: Res := Res { id: 1 }
let Count: int := 3

fn main() int {
    return Handle.id - 1
}
`, nil)
	initialization, shutdown, errors := analyzer.ProgramPlans(ProgramTargetCommand, "main", lexer.Token{})
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	if !strings.Contains(initialization.String(), "static main.Handle mutable=false init=compile-time value destruction=Res") ||
		strings.Contains(initialization.String(), "static main.Count mutable=false init=3 destruction") {
		t.Errorf("static lifetime facts:\n%s", initialization)
	}
	if len(shutdown.StaticDestructionRequirements) != 1 || shutdown.StaticDestructionRequirements[0] != "main.Handle: Res" || shutdown.StaticDestructionCapability != "unresolved" {
		t.Errorf("static destruction = %v (%s)", shutdown.StaticDestructionRequirements, shutdown.StaticDestructionCapability)
	}
}

// A test Target's plan uses the compiler-generated harness entry and derives
// runtime requirements from the test entries.
// Rules: rules/compiler/initialization.md — §§ 20, 27.
func TestProgramInitializationPlanTestHarness(t *testing.T) {
	p := parser.New(lexer.NewWithFile(`module main

fn Work() void {
}

test "threads are joined or detached" {
    let worker := spawn thread Work()
    detach worker
}
`, "plans_test.sec"))
	program := p.ParseProgram()
	analyzer := NewAnalyzer()
	analyzer.SetProgramTargetKind(ProgramTargetTest)
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	plan, _, errors := analyzer.ProgramPlans(ProgramTargetTest, "", lexer.Token{})
	if len(errors) != 0 || plan == nil {
		t.Fatal(plan, errors)
	}
	text := plan.String()
	for _, want := range []string{"entry test  (compiler-generated test harness entry)", "runtime PlatformThreadSupport (a thread started in", "runtime DetachedExecutionManager"} {
		if !strings.Contains(text, want) {
			t.Errorf("harness plan lacks %q:\n%s", want, text)
		}
	}
}

// Planning depends on the concrete CompilationPlan, never on the compiler
// host: the same source yields a different static value outcome on a 32-bit
// and a 64-bit target and a different allocator capability per profile.
// Rules: rules/compiler/initialization.md — §§ 10, 43, 45.7.
func TestProgramInitializationPlanFollowsCompilationPlan(t *testing.T) {
	source := `module main

fn Large() uint {
    let mut value: uint := 1u
    for i in 0..39 {
        value = value * 2u
    }
    return value
}

fn Alloc() Result[void, AllocationError] {
    let mem := try Arena.WithCapacity(64u)
    return Ok()
}

let Big: uint := Large()

fn main() int {
    match Alloc() {
        Ok(_) => return 0
        Err(error) => return 1
    }
}
`
	analyze := func(width uint16, profile string) (*ProgramInitializationPlan, []Error) {
		p := parser.New(lexer.NewWithFile(source, "main.sec"))
		program := p.ParseProgram()
		analyzer := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "linux", TargetArch: "test", PointerWidthBits: width, Endianness: layout.LittleEndian, Profile: profile})
		if errors := analyzer.Analyze(program); len(errors) != 0 {
			return nil, errors
		}
		plan, _, errors := analyzer.ProgramPlans(ProgramTargetCommand, "main", lexer.Token{})
		return plan, errors
	}
	wide, errors := analyze(64, "hosted")
	if len(errors) != 0 || !strings.Contains(wide.String(), "static main.Big mutable=false init=1099511627776") ||
		!strings.Contains(wide.String(), "runtime AllocatorRuntime (dynamic allocation in Alloc, supported)") {
		t.Fatalf("64-bit hosted plan: %v\n%v", errors, wide)
	}
	if _, errors := analyze(32, "hosted"); len(errors) != 1 || errors[0].ID != diagnostics.CompileTimeEvaluationFailed || !strings.Contains(errors[0].Message, "does not fit uint") {
		t.Fatalf("32-bit target must evaluate with its own uint: %v", errors)
	}
	unknown, errors := analyze(64, "")
	if len(errors) != 0 || !strings.Contains(unknown.String(), "runtime AllocatorRuntime (dynamic allocation in Alloc, unresolved)") {
		t.Fatalf("unresolved profile plan: %v\n%v", errors, unknown)
	}
}

// The plan is identical regardless of declaration order and across repeated
// analyses (map iteration), so source traversal and import order never
// choose startup or shutdown order.
// Rules: rules/compiler/initialization.md — §§ 16, 33, 43, 45.5.
func TestProgramPlansAreDeterministic(t *testing.T) {
	parts := []string{
		"fn Work() void {\n}\n",
		"fn Alloc() Result[void, AllocationError] {\n    let mem := try Arena.WithCapacity(64u)\n    return Ok()\n}\n",
		"let Zeta: int := 26\n",
		"let Alpha: int := 1\n",
		"let mut Middle: bool\n",
		"fn main() int {\n    let worker := spawn thread Work()\n    detach worker\n    let job := try spawn task Work() { Err(_) => { return 1 } }\n    await job\n    match Alloc() {\n        Ok(_) => return 0\n        Err(error) => return 1\n    }\n}\n",
	}
	render := func(order []int) string {
		var source strings.Builder
		source.WriteString("module main\n\n")
		for _, index := range order {
			source.WriteString(parts[index] + "\n")
		}
		initialization, shutdown, errors := analyzePlanSource(t, source.String(), nil).ProgramPlans(ProgramTargetCommand, "main", lexer.Token{})
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		teardown := []string{}
		for _, action := range shutdown.RuntimeTeardown {
			teardown = append(teardown, action.ID)
		}
		return initialization.String() + "\n" + strings.Join(teardown, ",")
	}
	want := render([]int{0, 1, 2, 3, 4, 5})
	for attempt := 0; attempt < 10; attempt++ {
		for _, order := range [][]int{{5, 4, 3, 2, 1, 0}, {3, 5, 0, 4, 2, 1}, {2, 0, 5, 1, 3, 4}} {
			if got := render(order); got != want {
				t.Fatalf("order %v changed the plan:\n%s\nvs\n%s", order, got, want)
			}
		}
	}
}
