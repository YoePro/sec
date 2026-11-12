package sema

import (
	"fmt"
	"sort"
	"strings"

	"sec/internal/layout"
	"sec/internal/lexer"
)

// programDetachSite is an accepted detach statement and its callable.
type programDetachSite struct {
	Callable CallableID
	Token    lexer.Token
}

func (a *Analyzer) noteDetachSite(token lexer.Token) {
	if a.summaryPass || token.Line == 0 {
		return
	}
	a.detachSites = append(a.detachSites, programDetachSite{Callable: a.currentCallable, Token: token})
}

// SetProgramTargetKind selects the Target kind before analysis, so that a
// library Target never acquires a program-entry root (initialization.md § 26).
func (a *Analyzer) SetProgramTargetKind(kind ProgramTargetKind) {
	a.recordConfiguration(func(b *Analyzer) { b.SetProgramTargetKind(kind) })
	a.programTargetKind = kind
}

// RuntimeFacility is a compiler-known Sec runtime facility a program may
// require (initialization.md §§ 11–12). Facilities are typed requirements,
// never opaque backend commands.
type RuntimeFacility string

const (
	RuntimeAllocator         RuntimeFacility = "AllocatorRuntime"
	RuntimeThreadSupport     RuntimeFacility = "PlatformThreadSupport"
	RuntimeTaskScheduler     RuntimeFacility = "TaskScheduler"
	RuntimeProcessSupport    RuntimeFacility = "ProcessSupport"
	RuntimeDetachedExecution RuntimeFacility = "DetachedExecutionManager"
)

// runtimeFacilityPrerequisites is the canonical prerequisite relation of
// runtime facilities (§ 15).
var runtimeFacilityPrerequisites = map[RuntimeFacility][]RuntimeFacility{
	RuntimeTaskScheduler:     {RuntimeAllocator},
	RuntimeDetachedExecution: {},
}

// RuntimeRequirement records why the reachable program needs a facility:
// the first source site in canonical order that requires it.
type RuntimeRequirement struct {
	Facility RuntimeFacility
	Reason   string
	Source   lexer.Token
	// Capability is the profile's resolution of the facility: "supported",
	// "disabled" or "unresolved" when the active TargetProfile does not yet
	// resolve it (MD-034).
	Capability string
}

// StartupActionKind categorizes typed startup actions (§ 14).
type StartupActionKind string

const (
	StartupEstablishStaticStorage    StartupActionKind = "EstablishStaticStorage"
	StartupInitializeRuntimeFacility StartupActionKind = "InitializeRuntimeFacility"
	StartupTransferToEntry           StartupActionKind = "TransferToEntry"
)

// StartupAction is one typed startup requirement with its prerequisites,
// identified by a stable plan-derived name.
type StartupAction struct {
	ID       string
	Kind     StartupActionKind
	Facility RuntimeFacility
	Requires []string
	// Fallible marks an action that can fail before entry, making the
	// failure a ProgramStartupFailure (§§ 17, 18). Static storage is
	// established physically from the artifact image and transfer to entry
	// cannot fail; runtime facility initialization can.
	Fallible bool
}

// StaticStorageRequirement is the physical establishment of one static's
// storage: its compile-time initial value, already resolved by semantic CTE,
// or zero initialization. Startup never runs code to compute it (§§ 6, 7).
type StaticStorageRequirement struct {
	Name     string
	Module   string
	Mutable  bool
	Initial  string
	ZeroInit bool
	// Destruction is set when the static's type is not trivially
	// destructible, so its end of lifetime is a shutdown requirement
	// (§§ 9, 33, 37).
	Destruction bool
	Type        string
}

// ProgramEntryFact is the resolved source entry of an executable Target.
type ProgramEntryFact struct {
	Kind     ProgramTargetKind
	Module   string
	Callable CallableID
	Token    lexer.Token
	// Status is the termination meaning of entry return (§§ 23–25).
	Status string
}

// ProgramInitializationPlan is the canonical startup plan of one executable
// concrete CompilationPlan (§ 8): entry, static storage establishment,
// execution-environment and runtime requirements, typed startup actions in
// deterministic topological order, and the termination policy requirement.
type ProgramInitializationPlan struct {
	Profile string
	// PlanFingerprint is the composite semantic fingerprint of the
	// CompilationPlan the plans were built for (platform_model.md § 23).
	PlanFingerprint                  string
	Entry                            ProgramEntryFact
	StaticStorage                    []StaticStorageRequirement
	ExecutionEnvironmentRequirements []string
	Runtime                          []RuntimeRequirement
	StartupActions                   []StartupAction
	TerminationPolicy                string
}

// ShutdownAction is one dependency-ordered shutdown step (§§ 31–36).
type ShutdownAction struct {
	ID       string
	Phase    string
	Facility RuntimeFacility
	// After lists actions that must complete first; a facility is torn down
	// only after every action that depends on it.
	After []string
}

// ProgramShutdownPlan is the canonical normal-termination plan (§ 9),
// derived from teardown dependencies rather than reversing startup.
type ProgramShutdownPlan struct {
	DetachedExecutionShutdown bool
	// StaticDestructionRequirements name the statics whose destruction
	// shutdown must account for; StaticDestructionCapability is the
	// TargetProfile's deterministic static destruction resolution
	// ("unresolved" until profiles resolve it, MD-034).
	StaticDestructionRequirements []string
	StaticDestructionCapability   string
	// StaticDestructionOrder is the deterministic destruction order from
	// dependency and use relations (static.md § 20); StaticDestructionCycle
	// is set instead when those relations are cyclic (§ 34).
	StaticDestructionOrder          []string
	StaticDestructionCycle          []string
	RuntimeTeardown                 []ShutdownAction
	ExecutionEnvironmentTermination string
	// Terminations distinguish normal termination, which follows this plan,
	// from forced termination, whose cleanup only the owning panic and
	// target policy defines (§ 35).
	Terminations []TerminationPath
}

// TerminationKind separates the two ways a program ends (§ 35).
type TerminationKind string

const (
	NormalTermination TerminationKind = "NormalTermination"
	ForcedTermination TerminationKind = "ForcedTermination"
)

// TerminationPath records what ends the program on one path, which cleanup
// is guaranteed, and where control goes.
type TerminationPath struct {
	Kind     TerminationKind
	Trigger  string
	Cleanup  []string
	Endpoint string
}

// ProgramPlans builds the initialization and shutdown plans of an executable
// Target from the validated semantic program and the analyzer's resolved
// profile (initialization.md §§ 8–16, 26–27, 31–39). A library Target has no
// plans. A test Target's entry is the compiler-generated harness. The
// returned errors are the entry-contract diagnostics; a program with entry
// errors gets no plans.
// Rules: rules/compiler/initialization.md — §§ 6–16, 20–27, 31–39, 42–43.
func (a *Analyzer) ProgramPlans(kind ProgramTargetKind, entryModule string, anchor lexer.Token) (*ProgramInitializationPlan, *ProgramShutdownPlan, []Error) {
	if a == nil || kind == ProgramTargetLibrary || kind == "" {
		return nil, nil, nil
	}
	entry := ProgramEntryFact{Kind: kind, Module: entryModule}
	switch kind {
	case ProgramTargetCommand:
		entry.Status = "logical status from main's int result (0 success, nonzero failure); physical encoding belongs to the ExecutionEnvironment"
	case ProgramTargetFirmware:
		entry.Status = "main returns into the TargetProfile's defined firmware termination policy; no implicit re-entry, reset or halt"
	case ProgramTargetTest:
		entry.Status = "compiler-generated test harness entry"
	}
	if kind != ProgramTargetTest {
		if errors := a.ValidateProgramEntry(kind, entryModule, anchor); len(errors) > 0 {
			return nil, nil, errors
		}
		for _, function := range a.functions["main"] {
			if function.Module == entryModule && function.ImplTarget == "" {
				entry.Callable, entry.Token = callableID(function), function.Token
			}
		}
	}
	initialization := &ProgramInitializationPlan{Profile: a.targetProfile, Entry: entry, PlanFingerprint: a.planFingerprints.Plan}
	initialization.StaticStorage = a.programStaticStorage()
	initialization.Runtime = a.programRuntimeRequirements(entry)
	// Each requirement records the profile's resolution of its facility. A
	// disabled heap capability is already rejected by the allocation policy
	// analysis of the reachable graph, so a valid program never reaches a
	// plan with a disabled required facility; other facilities stay
	// unresolved until the TargetProfile resolves them (MD-034).
	capabilities := a.AllocationCapabilities()
	for index, requirement := range initialization.Runtime {
		initialization.Runtime[index].Capability = runtimeCapability(requirement.Facility, capabilities)
	}
	if len(initialization.StaticStorage) > 0 {
		initialization.ExecutionEnvironmentRequirements = append(initialization.ExecutionEnvironmentRequirements, "physical static storage establishment from the artifact image")
	}
	initialization.StartupActions = programStartupActions(initialization)
	switch kind {
	case ProgramTargetFirmware:
		initialization.TerminationPolicy = "TargetProfile-defined firmware return"
	default:
		initialization.TerminationPolicy = "return logical status to the execution environment"
	}
	shutdown := programShutdownPlan(initialization)
	shutdown.StaticDestructionOrder, shutdown.StaticDestructionCycle = a.staticDestructionOrder()
	// The plans are verified before any lowering consumes them; a failure
	// rejects the concrete CompilationPlan (§§ 38, 39).
	if errors := VerifyProgramPlans(initialization, shutdown); len(errors) > 0 {
		return nil, nil, errors
	}
	return initialization, shutdown, nil
}

// runtimeCapability resolves a facility against the profile. Only heap
// allocation has a resolved profile capability today; other facilities stay
// unresolved rather than invented (target_profiles.md § 6; MD-034).
func runtimeCapability(facility RuntimeFacility, capabilities layout.AllocationCapabilities) string {
	if facility != RuntimeAllocator || !capabilities.ProfileKnown {
		return "unresolved"
	}
	if capabilities.DynamicAllocation.Activation == layout.AllocationDisabled {
		return "disabled"
	}
	if capabilities.DynamicAllocation.Availability == layout.AllocationSupported && capabilities.DynamicAllocation.Activation == layout.AllocationEnabled {
		return "supported"
	}
	return "unresolved"
}

// programStaticStorage lists the module and function-local statics whose
// storage the execution environment establishes, from their compile-time
// values, in canonical identity order.
func (a *Analyzer) programStaticStorage() []StaticStorageRequirement {
	statics := a.ResolvedStaticDeclarations()
	out := make([]StaticStorageRequirement, 0, len(statics))
	for _, static := range statics {
		requirement := StaticStorageRequirement{Name: static.Name, Module: static.Module, Mutable: static.Mutable,
			Destruction: !TriviallyDestructible(static.Type), Type: typeDisplayName(static.Type)}
		switch {
		case static.Initializer.Integer != nil:
			requirement.Initial = static.Initializer.Integer.String()
		case static.Initializer.Bool != nil:
			requirement.Initial = fmt.Sprint(*static.Initializer.Bool)
		case static.Declaration == nil || static.Declaration.Value == nil:
			requirement.ZeroInit = true
		default:
			requirement.Initial = "compile-time value"
		}
		if static.Function != "" {
			requirement.Name = static.Function + "::" + static.Name
		}
		out = append(out, requirement)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Module != out[j].Module {
			return out[i].Module < out[j].Module
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// programRuntimeRequirements derives facilities from the semantic program
// reachable from the entry (§ 12): allocation, spawned tasks, threads and
// processes, and detached execution. A program needing none of them has no
// runtime requirement (§ 11).
func (a *Analyzer) programRuntimeRequirements(entry ProgramEntryFact) []RuntimeRequirement {
	graph := a.callGraph
	if graph == nil {
		return nil
	}
	reachable := map[CallableID]bool{}
	starts := []CallableID{}
	if entry.Kind == ProgramTargetTest {
		// The generated harness enters every validated test body.
		for _, node := range graph.Nodes() {
			if node.Kind == CallableBodyTest {
				starts = append(starts, node.ID)
			}
		}
	} else {
		for _, root := range graph.Roots() {
			if root.Kind == CallRootProgramEntry && root.Node == entry.Callable {
				starts = append(starts, root.Node)
			}
		}
	}
	for _, start := range starts {
		for id := range graph.reachableIDsFrom(start) {
			reachable[id] = true
		}
	}
	found := map[RuntimeFacility]RuntimeRequirement{}
	note := func(facility RuntimeFacility, reason string, source lexer.Token) {
		if existing, ok := found[facility]; ok && !tokenBefore(source, existing.Source) {
			return
		}
		found[facility] = RuntimeRequirement{Facility: facility, Reason: reason, Source: source}
	}
	ids := make([]CallableID, 0, len(reachable))
	for id := range reachable {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		node, _ := graph.Node(id)
		if fact := a.AllocationFact(id); fact.Knowledge == AllocationMayAllocate && len(fact.AllocationPath) <= 1 {
			note(RuntimeAllocator, "dynamic allocation in "+node.Name, node.Declaration)
		}
		for _, site := range graph.Outgoing(id) {
			switch site.Execution {
			case CallExecutionSpawnTask:
				note(RuntimeTaskScheduler, "a task spawned in "+node.Name, site.Source)
			case CallExecutionSpawnThread:
				note(RuntimeThreadSupport, "a thread started in "+node.Name, site.Source)
			case CallExecutionSpawnProcess:
				note(RuntimeProcessSupport, "a process launched in "+node.Name, site.Source)
			}
		}
	}
	for _, site := range a.detachSites {
		if reachable[site.Callable] {
			note(RuntimeDetachedExecution, "detached execution", site.Token)
		}
	}
	out := make([]RuntimeRequirement, 0, len(found))
	for _, requirement := range found {
		out = append(out, requirement)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Facility < out[j].Facility })
	return out
}

// programStartupActions builds the startup dependency graph and orders it
// topologically with a canonical name tie-breaker (§§ 13–16): static
// storage first, then required facilities after their prerequisites, then
// the transfer to entry after everything.
func programStartupActions(plan *ProgramInitializationPlan) []StartupAction {
	actions := map[string]StartupAction{}
	all := []string{}
	if len(plan.StaticStorage) > 0 {
		actions["establish-static-storage"] = StartupAction{ID: "establish-static-storage", Kind: StartupEstablishStaticStorage}
		all = append(all, "establish-static-storage")
	}
	required := map[RuntimeFacility]bool{}
	for _, requirement := range plan.Runtime {
		required[requirement.Facility] = true
	}
	for _, requirement := range plan.Runtime {
		id := "initialize-" + string(requirement.Facility)
		action := StartupAction{ID: id, Kind: StartupInitializeRuntimeFacility, Facility: requirement.Facility, Fallible: true}
		if _, static := actions["establish-static-storage"]; static {
			action.Requires = append(action.Requires, "establish-static-storage")
		}
		for _, prerequisite := range runtimeFacilityPrerequisites[requirement.Facility] {
			if required[prerequisite] {
				action.Requires = append(action.Requires, "initialize-"+string(prerequisite))
			}
		}
		actions[id] = action
		all = append(all, id)
	}
	actions["transfer-to-entry"] = StartupAction{ID: "transfer-to-entry", Kind: StartupTransferToEntry, Requires: append([]string(nil), all...)}
	return topologicalStartup(actions)
}

// topologicalStartup orders actions so prerequisites come first, choosing the
// lexicographically smallest ready action at each step.
func topologicalStartup(actions map[string]StartupAction) []StartupAction {
	done := map[string]bool{}
	out := []StartupAction{}
	for len(out) < len(actions) {
		ready := []string{}
		for id, action := range actions {
			if done[id] {
				continue
			}
			satisfied := true
			for _, prerequisite := range action.Requires {
				if !done[prerequisite] {
					satisfied = false
				}
			}
			if satisfied {
				ready = append(ready, id)
			}
		}
		if len(ready) == 0 {
			// A cycle cannot arise from the canonical prerequisite relation;
			// stop rather than invent an order.
			return out
		}
		sort.Strings(ready)
		action := actions[ready[0]]
		action.Requires = append([]string(nil), action.Requires...)
		sort.Strings(action.Requires)
		out = append(out, action)
		done[ready[0]] = true
	}
	return out
}

// programShutdownPlan derives normal shutdown from teardown dependencies:
// detached execution shuts down first, then each facility is torn down only
// after every facility that depends on it (§§ 31–36).
func programShutdownPlan(initialization *ProgramInitializationPlan) *ProgramShutdownPlan {
	plan := &ProgramShutdownPlan{ExecutionEnvironmentTermination: initialization.TerminationPolicy}
	for _, static := range initialization.StaticStorage {
		if static.Destruction {
			plan.StaticDestructionRequirements = append(plan.StaticDestructionRequirements, static.Module+"."+static.Name+": "+static.Type)
		}
	}
	if len(plan.StaticDestructionRequirements) > 0 {
		plan.StaticDestructionCapability = "unresolved"
	}
	dependents := map[RuntimeFacility][]RuntimeFacility{}
	required := map[RuntimeFacility]bool{}
	for _, requirement := range initialization.Runtime {
		required[requirement.Facility] = true
	}
	for _, requirement := range initialization.Runtime {
		for _, prerequisite := range runtimeFacilityPrerequisites[requirement.Facility] {
			if required[prerequisite] {
				dependents[prerequisite] = append(dependents[prerequisite], requirement.Facility)
			}
		}
	}
	actions := map[string]StartupAction{}
	for _, requirement := range initialization.Runtime {
		id := "teardown-" + string(requirement.Facility)
		after := []string{}
		for _, dependent := range dependents[requirement.Facility] {
			after = append(after, "teardown-"+string(dependent))
		}
		if requirement.Facility == RuntimeDetachedExecution {
			plan.DetachedExecutionShutdown = true
		} else if required[RuntimeDetachedExecution] {
			after = append(after, "teardown-"+string(RuntimeDetachedExecution))
		}
		actions[id] = StartupAction{ID: id, Facility: requirement.Facility, Requires: after}
	}
	defer func() { plan.Terminations = programTerminations(initialization, plan) }()
	for _, action := range topologicalStartup(actions) {
		phase := "runtime-teardown"
		if action.Facility == RuntimeDetachedExecution {
			phase = "detached-execution-shutdown"
		}
		plan.RuntimeTeardown = append(plan.RuntimeTeardown, ShutdownAction{ID: action.ID, Phase: phase, Facility: action.Facility, After: action.Requires})
	}
	return plan
}

// String renders a plan deterministically for tooling and tests.
func (plan *ProgramInitializationPlan) String() string {
	if plan == nil {
		return "no initialization plan"
	}
	lines := []string{fmt.Sprintf("entry %s %s (%s)", plan.Entry.Kind, plan.Entry.Module, plan.Entry.Status)}
	for _, static := range plan.StaticStorage {
		value := static.Initial
		if static.ZeroInit {
			value = "zero"
		}
		line := fmt.Sprintf("static %s.%s mutable=%t init=%s", static.Module, static.Name, static.Mutable, value)
		if static.Destruction {
			line += " destruction=" + static.Type
		}
		lines = append(lines, line)
	}
	if len(plan.Runtime) == 0 {
		lines = append(lines, "runtime none")
	}
	for _, requirement := range plan.Runtime {
		lines = append(lines, fmt.Sprintf("runtime %s (%s, %s)", requirement.Facility, requirement.Reason, requirement.Capability))
	}
	for _, action := range plan.StartupActions {
		lines = append(lines, fmt.Sprintf("startup %s %s requires [%s]", action.ID, action.Kind, strings.Join(action.Requires, ", ")))
	}
	lines = append(lines, "termination "+plan.TerminationPolicy)
	return strings.Join(lines, "\n")
}

// programTerminations derives the normal and forced termination paths
// (initialization.md §§ 29, 31, 35; panic.md §§ 8, 11(6), 23). Normal
// termination runs the phases of the shutdown plan the Target has, after the
// entry's ordinary cleanup. Forced termination by root panic follows the
// resolved panic policy, which on every current target is the immediate
// trap of MD-101: it guarantees none of the normal cleanup, and the plan
// never claims it does.
func programTerminations(initialization *ProgramInitializationPlan, shutdown *ProgramShutdownPlan) []TerminationPath {
	normal := TerminationPath{Kind: NormalTermination, Endpoint: initialization.TerminationPolicy}
	switch initialization.Entry.Kind {
	case ProgramTargetFirmware:
		normal.Trigger = "main returns"
	case ProgramTargetTest:
		normal.Trigger = "the test harness completes"
	default:
		normal.Trigger = "main returns its status"
	}
	if initialization.Entry.Kind != ProgramTargetTest {
		normal.Cleanup = append(normal.Cleanup, "entry cleanup: main's defer and destruction complete as ordinary function cleanup")
	}
	if shutdown.DetachedExecutionShutdown {
		normal.Cleanup = append(normal.Cleanup, "detached-execution shutdown")
	}
	if len(shutdown.StaticDestructionRequirements) > 0 {
		normal.Cleanup = append(normal.Cleanup, "static destruction ("+shutdown.StaticDestructionCapability+")")
	}
	for _, action := range shutdown.RuntimeTeardown {
		if action.Phase == "runtime-teardown" {
			normal.Cleanup = append(normal.Cleanup, action.ID)
		}
	}
	forced := TerminationPath{Kind: ForcedTermination, Trigger: "root panic"}
	if initialization.Entry.Kind == ProgramTargetFirmware {
		forced.Endpoint = "the TargetProfile's non-returning panic endpoint (immediate trap, MD-101)"
	} else {
		forced.Endpoint = "the process terminates without normal continuation (immediate trap, MD-101)"
	}
	return []TerminationPath{normal, forced}
}
