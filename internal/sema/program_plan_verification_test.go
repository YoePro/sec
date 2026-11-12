package sema

import (
	"reflect"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

const planRuntimeSource = `module main

fn Work() void {
}

fn Alloc() Result[void, AllocationError] {
    let mem := try Arena.WithCapacity(64u)
    return Ok()
}

let Limit: int := 7
let mut Ready: bool

fn main() int {
    let worker := spawn thread Work()
    detach worker
    let job := try spawn task Work() { Err(_) => { return 1 } }
    await job
    match Alloc() {
        Ok(_) => return 0
        Err(error) => return 1
    }
}
`

func resolvedPlans(t *testing.T, source string) (*ProgramInitializationPlan, *ProgramShutdownPlan) {
	t.Helper()
	initialization, shutdown, errors := analyzePlanSource(t, source, nil).ProgramPlans(ProgramTargetCommand, "main", lexer.Token{})
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	return initialization, shutdown
}

// A failed fallible startup action is a ProgramStartupFailure: entry has
// not begun, no entry-scope cleanup runs, actions after the failure never
// ran, and only established runtime facilities are rolled back, each after
// every established facility that requires it.
// Rules: rules/compiler/initialization.md — §§ 17, 18, 19.
func TestProgramStartupFailureRollsBackOnlyEstablished(t *testing.T) {
	initialization, _ := resolvedPlans(t, planRuntimeSource)
	fallible := []string{}
	for _, action := range initialization.StartupActions {
		if action.Fallible {
			fallible = append(fallible, action.ID)
		}
	}
	want := []string{"initialize-AllocatorRuntime", "initialize-DetachedExecutionManager", "initialize-PlatformThreadSupport", "initialize-TaskScheduler"}
	if !reflect.DeepEqual(fallible, want) {
		t.Fatalf("fallible actions = %v\n%s", fallible, initialization)
	}
	failure, err := initialization.StartupFailureCleanup("initialize-PlatformThreadSupport")
	if err != nil {
		t.Fatal(err)
	}
	if failure.Kind != ProgramStartupFailure || failure.EntryBegun || failure.EntryScopeCleanup {
		t.Errorf("failure classification = %+v", failure)
	}
	if !reflect.DeepEqual(failure.Established, []string{"establish-static-storage", "initialize-AllocatorRuntime", "initialize-DetachedExecutionManager"}) ||
		!reflect.DeepEqual(failure.NotAttempted, []string{"initialize-TaskScheduler", "transfer-to-entry"}) {
		t.Errorf("establishment state = %v / %v", failure.Established, failure.NotAttempted)
	}
	rolled := []string{}
	for _, action := range failure.Cleanup {
		rolled = append(rolled, action.ID)
	}
	if !reflect.DeepEqual(rolled, []string{"rollback-AllocatorRuntime", "rollback-DetachedExecutionManager"}) {
		t.Errorf("rollback = %v", rolled)
	}
	if _, err := initialization.StartupFailureCleanup("transfer-to-entry"); err == nil {
		t.Error("transfer to entry cannot fail before entry")
	}
	if _, err := initialization.StartupFailureCleanup("establish-static-storage"); err == nil {
		t.Error("static storage is established from the image, not by a fallible action")
	}
	if ProgramFailureClass(false) != ProgramStartupFailure || ProgramFailureClass(true) != ProgramExecutionFailure {
		t.Error("failure class must follow whether entry began")
	}

	// A dependent established after its prerequisite is rolled back first.
	layered := &ProgramInitializationPlan{StartupActions: []StartupAction{
		{ID: "initialize-AllocatorRuntime", Kind: StartupInitializeRuntimeFacility, Facility: RuntimeAllocator, Fallible: true},
		{ID: "initialize-TaskScheduler", Kind: StartupInitializeRuntimeFacility, Facility: RuntimeTaskScheduler, Requires: []string{"initialize-AllocatorRuntime"}, Fallible: true},
		{ID: "initialize-ZFacility", Kind: StartupInitializeRuntimeFacility, Facility: "ZFacility", Fallible: true},
	}}
	failure, err = layered.StartupFailureCleanup("initialize-ZFacility")
	if err != nil {
		t.Fatal(err)
	}
	if len(failure.Cleanup) != 2 || failure.Cleanup[0].ID != "rollback-TaskScheduler" || failure.Cleanup[1].ID != "rollback-AllocatorRuntime" ||
		!reflect.DeepEqual(failure.Cleanup[1].After, []string{"rollback-TaskScheduler"}) {
		t.Errorf("dependency-ordered rollback = %+v", failure.Cleanup)
	}
}

// Plan verification accepts resolved plans and rejects, with distinct
// diagnostics, an unavailable runtime facility, startup and shutdown
// cycles shown as one deterministic representative cycle, unsupported
// static destruction and a missing termination policy.
// Rules: rules/compiler/initialization.md — §§ 34, 39, 42.
func TestVerifyProgramPlans(t *testing.T) {
	initialization, shutdown := resolvedPlans(t, planRuntimeSource)
	if errors := VerifyProgramPlans(initialization, shutdown); len(errors) != 0 {
		t.Fatalf("resolved plans rejected: %v", errors)
	}
	ids := func(errors []Error) []string {
		out := []string{}
		for _, err := range errors {
			out = append(out, err.ID)
		}
		return out
	}
	source := lexer.Token{File: "main.sec", Line: 14, Column: 18}
	unavailable := &ProgramInitializationPlan{Profile: "tiny", TerminationPolicy: "return",
		Runtime: []RuntimeRequirement{{Facility: RuntimeTaskScheduler, Reason: "a task spawned in main", Source: source, Capability: "unsupported"}}}
	errors := VerifyProgramPlans(unavailable, nil)
	if len(errors) != 1 || errors[0].ID != diagnostics.ProgramRuntimeUnsupported || errors[0].Line != 14 ||
		!strings.Contains(errors[0].Message, "this target cannot provide the task scheduler required by a task spawned in main") {
		t.Errorf("unsupported runtime = %v", errors)
	}

	cyclic := &ProgramInitializationPlan{TerminationPolicy: "return", StartupActions: []StartupAction{
		{ID: "initialize-B", Requires: []string{"initialize-A"}},
		{ID: "initialize-A", Requires: []string{"initialize-B"}},
	}}
	errors = VerifyProgramPlans(cyclic, nil)
	if len(errors) != 1 || errors[0].ID != diagnostics.ProgramStartupCycle ||
		!strings.Contains(errors[0].Message, "initialize-A -> initialize-B -> initialize-A") {
		t.Errorf("startup cycle = %v", errors)
	}
	misordered := &ProgramInitializationPlan{TerminationPolicy: "return", StartupActions: []StartupAction{
		{ID: "transfer-to-entry", Requires: []string{"initialize-A"}},
		{ID: "initialize-A"},
	}}
	if got := ids(VerifyProgramPlans(misordered, nil)); !reflect.DeepEqual(got, []string{diagnostics.ProgramStartupCycle}) {
		t.Errorf("misordered startup = %v", got)
	}

	teardown := &ProgramShutdownPlan{RuntimeTeardown: []ShutdownAction{
		{ID: "teardown-Y", After: []string{"teardown-X"}},
		{ID: "teardown-X", After: []string{"teardown-Y"}},
	}}
	errors = VerifyProgramPlans(&ProgramInitializationPlan{TerminationPolicy: "return"}, teardown)
	if len(errors) != 1 || errors[0].ID != diagnostics.ProgramShutdownCycle ||
		!strings.Contains(errors[0].Message, "teardown-X -> teardown-Y -> teardown-X") {
		t.Errorf("shutdown cycle = %v", errors)
	}

	destruction := &ProgramShutdownPlan{StaticDestructionRequirements: []string{"main.Handle: Res"}, StaticDestructionCapability: "unsupported"}
	if got := ids(VerifyProgramPlans(&ProgramInitializationPlan{TerminationPolicy: "return"}, destruction)); !reflect.DeepEqual(got, []string{diagnostics.ProgramStaticDestructionUnsupported}) {
		t.Errorf("static destruction = %v", got)
	}
	destruction.StaticDestructionCapability = "unresolved"
	if got := VerifyProgramPlans(&ProgramInitializationPlan{TerminationPolicy: "return"}, destruction); len(got) != 0 {
		t.Errorf("an unresolved profile capability is not a rejection: %v", got)
	}
	if got := ids(VerifyProgramPlans(&ProgramInitializationPlan{Entry: ProgramEntryFact{Kind: ProgramTargetFirmware}}, nil)); !reflect.DeepEqual(got, []string{diagnostics.ProgramTerminationUnsupported}) {
		t.Errorf("termination policy = %v", got)
	}
}

// The plans expose what linking must provide: the entry adapter, runtime
// components with their startup and teardown actions, static-data sections
// and the termination adapter.
// Rules: rules/compiler/initialization.md — §§ 7, 28, 41.
func TestProgramLinkingRequirements(t *testing.T) {
	initialization, shutdown := resolvedPlans(t, planRuntimeSource)
	link := ProgramLinkingRequirements(initialization, shutdown)
	if link.EntryAdapter != "command entry adapter: execution-environment entry -> Sec main() int" ||
		!reflect.DeepEqual(link.Sections, []string{"initialized static data"}) ||
		len(link.RuntimeComponents) != 4 || len(link.StartupComponents) != 4 || len(link.ShutdownComponents) != 4 ||
		link.TerminationAdapter != "return logical status to the execution environment" {
		t.Errorf("link requirements = %+v", link)
	}
	free, freeShutdown := resolvedPlans(t, "module main\n\nfn main() int {\n    return 0\n}\n")
	link = ProgramLinkingRequirements(free, freeShutdown)
	if len(link.RuntimeComponents) != 0 || len(link.Sections) != 0 || len(link.ShutdownComponents) != 0 || link.EntryAdapter == "" {
		t.Errorf("runtime-free link requirements = %+v", link)
	}
}

// The fingerprint covers the complete initialization/shutdown contract: an
// edit that preserves it (another body, moved declarations) keeps the
// fingerprint, while a changed static, runtime requirement, Target kind or
// profile changes it.
// Rules: rules/compiler/initialization.md — §§ 43, 44.
func TestProgramPlanFingerprintTracksTheContract(t *testing.T) {
	fingerprint := func(source string, kind ProgramTargetKind, configure func(*Analyzer)) string {
		initialization, shutdown, errors := analyzePlanSource(t, source, configure).ProgramPlans(kind, "main", lexer.Token{})
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		return ProgramPlanFingerprint(initialization, shutdown)
	}
	base := fingerprint(planRuntimeSource, ProgramTargetCommand, nil)
	if len(base) != 64 {
		t.Fatalf("fingerprint = %q", base)
	}
	preserving := strings.Replace(planRuntimeSource, "Ok(_) => return 0", "Ok(_) => return 2 - 2", 1)
	preserving = strings.Replace(preserving, "fn Work() void {\n}\n", "\n\nfn Work() void {\n    let unused := 1\n}\n", 1)
	if got := fingerprint(preserving, ProgramTargetCommand, nil); got != base {
		t.Error("a contract-preserving edit changed the fingerprint")
	}
	for name, source := range map[string]string{
		"static value": strings.Replace(planRuntimeSource, "Limit: int := 7", "Limit: int := 8", 1),
		"new static":   strings.Replace(planRuntimeSource, "let mut Ready: bool", "let mut Ready: bool\nlet mut Other: int", 1),
		"no detach":    strings.Replace(planRuntimeSource, "    detach worker\n", "    join worker\n", 1),
	} {
		if got := fingerprint(source, ProgramTargetCommand, nil); got == base {
			t.Errorf("%s did not change the fingerprint", name)
		}
	}
	if got := fingerprint(planRuntimeSource, ProgramTargetTest, nil); got == base {
		t.Error("the Target kind did not change the fingerprint")
	}
}

// Entry cannot complete while it owns unresolved structured child execution:
// a task or thread spawned in main must be awaited, joined or legally
// detached before main's scope ends. Legal detach transfers the child to the
// program's detached-execution manager, which the plans then require and
// shut down first.
// Rules: rules/compiler/initialization.md — §§ 29, 30, 31.
func TestEntryCannotCompleteOwningChildExecution(t *testing.T) {
	prefix := "module main\n\nfn Work() void {\n}\n\nfn main() int {\n"
	for name, body := range map[string]string{
		"task":   "    let job := try spawn task Work() { Err(_) => { return 1 } }\n    return 0\n}\n",
		"thread": "    let worker := spawn thread Work()\n    return 0\n}\n",
	} {
		errors := analyzeSourceRaw(t, prefix+body)
		if len(errors) != 1 || !strings.Contains(errors[0].Message, "is unresolved at scope exit") {
			t.Errorf("%s: errors = %v", name, errors)
		}
	}
	initialization, shutdown := resolvedPlans(t, prefix+"    let job := try spawn task Work() { Err(_) => { return 1 } }\n    await job\n    let worker := spawn thread Work()\n    detach worker\n    return 0\n}\n")
	detached := false
	for _, requirement := range initialization.Runtime {
		detached = detached || requirement.Facility == RuntimeDetachedExecution
	}
	if !detached || !shutdown.DetachedExecutionShutdown || len(shutdown.RuntimeTeardown) == 0 || shutdown.RuntimeTeardown[0].Facility != RuntimeDetachedExecution {
		t.Errorf("detached execution shutdown = %+v / %+v", initialization.Runtime, shutdown.RuntimeTeardown)
	}
}

// Normal termination follows the shutdown plan after the entry's ordinary
// cleanup; forced termination by root panic follows the resolved panic
// policy (an immediate trap, MD-101) and never claims normal cleanup.
// Rules: rules/compiler/initialization.md — §§ 29, 31, 35; rules/errors/panic.md — §§ 8, 11(6), 23.
func TestProgramTerminationPaths(t *testing.T) {
	initialization, shutdown := resolvedPlans(t, planRuntimeSource)
	if len(shutdown.Terminations) != 2 {
		t.Fatalf("terminations = %+v", shutdown.Terminations)
	}
	normal, forced := shutdown.Terminations[0], shutdown.Terminations[1]
	if normal.Kind != NormalTermination || normal.Trigger != "main returns its status" || normal.Endpoint != initialization.TerminationPolicy ||
		len(normal.Cleanup) < 3 || !strings.HasPrefix(normal.Cleanup[0], "entry cleanup") || normal.Cleanup[1] != "detached-execution shutdown" {
		t.Errorf("normal termination = %+v", normal)
	}
	if forced.Kind != ForcedTermination || forced.Trigger != "root panic" || len(forced.Cleanup) != 0 || !strings.Contains(forced.Endpoint, "MD-101") {
		t.Errorf("forced termination = %+v", forced)
	}
	analyzer := analyzePlanSource(t, "module main\n\nfn main() void {\n}\n", nil)
	_, firmware, errors := analyzer.ProgramPlans(ProgramTargetFirmware, "main", lexer.Token{})
	if len(errors) != 0 || firmware.Terminations[0].Trigger != "main returns" || !strings.Contains(firmware.Terminations[1].Endpoint, "non-returning panic endpoint") {
		t.Errorf("firmware terminations = %+v (%v)", firmware, errors)
	}
}

// Static destruction follows dependency and use relations, not source order:
// a static whose free uses another static is destroyed first, independent
// statics use canonical identity order, and a cycle is rejected (S1231)
// instead of picking an order.
// Rules: rules/declarations/static.md — § 20; rules/compiler/initialization.md — §§ 33, 34.
func TestStaticDestructionOrderFollowsUse(t *testing.T) {
	source := `module main

type Log struct {
    id: int,
}

impl Log {
    free {
    }
}

type Session struct {
    id: int,
}

impl Session {
    free {
        Flush()
    }
}

fn Flush() void {
    let entries := Journal.id
}

type Other struct {
    id: int,
}

impl Other {
    free {
    }
}

let Active: Session := Session { id: 1 }
let Zed: Other := Other { id: 3 }
let Journal: Log := Log { id: 2 }

fn main() int {
    return 0
}
`
	_, shutdown := resolvedPlans(t, source)
	if strings.Join(shutdown.StaticDestructionOrder, ",") != "main.Active,main.Journal,main.Zed" {
		t.Errorf("order = %v (requirements %v)", shutdown.StaticDestructionOrder, shutdown.StaticDestructionRequirements)
	}
	cyclic := strings.Replace(source, "impl Log {\n    free {\n    }", "impl Log {\n    free {\n        let back := Active.id\n    }", 1)
	analyzer := analyzePlanSource(t, cyclic, nil)
	_, _, errors := analyzer.ProgramPlans(ProgramTargetCommand, "main", lexer.Token{})
	if len(errors) != 1 || errors[0].ID != diagnostics.ProgramShutdownCycle ||
		!strings.Contains(errors[0].Message, "static destruction dependency cycle: main.Active -> main.Journal -> main.Active") {
		t.Errorf("errors = %v", errors)
	}
}
