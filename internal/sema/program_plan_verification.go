package sema

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// ProgramFailureKind separates a failure before source entry begins from an
// ordinary Sec execution failure after it (initialization.md § 17).
type ProgramFailureKind string

const (
	ProgramStartupFailure   ProgramFailureKind = "ProgramStartupFailure"
	ProgramExecutionFailure ProgramFailureKind = "ProgramExecutionFailure"
)

// ProgramFailureClass classifies a failure by whether source entry had
// begun: before it the failure is a ProgramStartupFailure, after it an
// ordinary execution failure governed by the owning panic/error rules.
// Rules: rules/compiler/initialization.md — § 17.
func ProgramFailureClass(entryBegun bool) ProgramFailureKind {
	if entryBegun {
		return ProgramExecutionFailure
	}
	return ProgramStartupFailure
}

// StartupFailureCleanup is the canonical response to one failed fallible
// startup action: source entry has not begun, no entry-scope cleanup runs,
// and only the actions that reached their established state are cleaned up,
// each after every established dependent (§§ 17–19).
type StartupFailureCleanup struct {
	Kind              ProgramFailureKind
	Failed            string
	Established       []string
	NotAttempted      []string
	EntryBegun        bool
	EntryScopeCleanup bool
	// Cleanup lists rollback actions in dependency order; After names the
	// rollbacks of established dependents that must complete first.
	Cleanup []ShutdownAction
}

// StartupFailureCleanup derives the rollback for a failure of the fallible
// startup action failed. Startup runs the plan's actions in order, so the
// actions before failed are established and the later ones never ran.
// Static storage is established physically by the execution environment
// from the artifact image and has no Sec rollback action; runtime
// facilities do.
// Rules: rules/compiler/initialization.md — §§ 17, 18, 19.
func (plan *ProgramInitializationPlan) StartupFailureCleanup(failed string) (StartupFailureCleanup, error) {
	result := StartupFailureCleanup{Kind: ProgramStartupFailure, Failed: failed}
	if plan == nil {
		return result, fmt.Errorf("no initialization plan")
	}
	position := -1
	for index, action := range plan.StartupActions {
		if action.ID == failed {
			position = index
		}
	}
	if position < 0 {
		return result, fmt.Errorf("startup action %q is not in the plan", failed)
	}
	if !plan.StartupActions[position].Fallible {
		return result, fmt.Errorf("startup action %q cannot fail", failed)
	}
	established := map[string]bool{}
	for _, action := range plan.StartupActions[:position] {
		result.Established = append(result.Established, action.ID)
		established[action.ID] = true
	}
	for _, action := range plan.StartupActions[position+1:] {
		result.NotAttempted = append(result.NotAttempted, action.ID)
	}
	rollbacks := map[string]StartupAction{}
	for _, action := range plan.StartupActions[:position] {
		if action.Kind != StartupInitializeRuntimeFacility {
			continue
		}
		id := "rollback-" + string(action.Facility)
		rollbacks[id] = StartupAction{ID: id, Facility: action.Facility}
	}
	// An established resource stays alive until every established resource
	// requiring it has been removed (§ 19).
	for _, action := range plan.StartupActions[:position] {
		if action.Kind != StartupInitializeRuntimeFacility {
			continue
		}
		for _, prerequisite := range action.Requires {
			if !established[prerequisite] || !strings.HasPrefix(prerequisite, "initialize-") {
				continue
			}
			id := "rollback-" + strings.TrimPrefix(prerequisite, "initialize-")
			rollback := rollbacks[id]
			rollback.Requires = append(rollback.Requires, "rollback-"+string(action.Facility))
			rollbacks[id] = rollback
		}
	}
	ordered := topologicalStartup(rollbacks)
	if len(ordered) != len(rollbacks) {
		return result, fmt.Errorf("startup rollback dependencies are cyclic")
	}
	for _, action := range ordered {
		result.Cleanup = append(result.Cleanup, ShutdownAction{ID: action.ID, Phase: "startup-rollback", Facility: action.Facility, After: action.Requires})
	}
	return result, nil
}

// VerifyProgramPlans checks resolved plans before lowering consumes them
// (§ 39): every required runtime facility is available on the
// TargetProfile, startup and shutdown dependencies are acyclic and respected
// by the plan order, demanded deterministic static destruction is
// supported, and the Target has a supported normal termination policy.
// Facilities whose profile capability is still unresolved (MD-034) are not
// rejected; only a profile that resolves them as unavailable is.
// Rules: rules/compiler/initialization.md — §§ 34, 38, 39, 42.
func VerifyProgramPlans(initialization *ProgramInitializationPlan, shutdown *ProgramShutdownPlan) []Error {
	if initialization == nil {
		return nil
	}
	anchor := initialization.Entry.Token
	errors := []Error{}
	for _, requirement := range initialization.Runtime {
		if requirement.Capability != "disabled" && requirement.Capability != "unsupported" {
			continue
		}
		at := requirement.Source
		if at.File == "" {
			at = anchor
		}
		errors = append(errors, entryError(at, lexer.Token{}, diagnostics.ProgramRuntimeUnsupported,
			"Select a TargetProfile that provides this facility, or remove the operation that requires it.",
			"this target cannot provide the %s required by %s (TargetProfile %s: %s)",
			runtimeFacilityDisplay(requirement.Facility), requirement.Reason, profileDisplay(initialization.Profile), requirement.Capability))
	}
	startup := map[string][]string{}
	for _, action := range initialization.StartupActions {
		startup[action.ID] = action.Requires
	}
	if cycle := dependencyCycle(startup); len(cycle) > 0 {
		errors = append(errors, entryError(anchor, lexer.Token{}, diagnostics.ProgramStartupCycle,
			"Startup prerequisites must form an acyclic graph; this is a fault in the plan's prerequisite relation.",
			"startup dependency cycle: %s", strings.Join(cycle, " -> ")))
	} else if missing := orderViolation(initialization.StartupActions); missing != "" {
		errors = append(errors, entryError(anchor, lexer.Token{}, diagnostics.ProgramStartupCycle,
			"Startup prerequisites must be established before the actions that require them.",
			"startup plan is not satisfiable: %s", missing))
	}
	if shutdown != nil {
		teardown := map[string][]string{}
		actions := make([]StartupAction, 0, len(shutdown.RuntimeTeardown))
		for _, action := range shutdown.RuntimeTeardown {
			teardown[action.ID] = action.After
			actions = append(actions, StartupAction{ID: action.ID, Requires: action.After})
		}
		if cycle := dependencyCycle(teardown); len(cycle) > 0 {
			errors = append(errors, entryError(anchor, lexer.Token{}, diagnostics.ProgramShutdownCycle,
				"No canonical ownership or destruction rule orders these facilities; shutdown never picks an arbitrary order.",
				"shutdown dependency cycle: %s", strings.Join(cycle, " -> ")))
		} else if missing := orderViolation(actions); missing != "" {
			errors = append(errors, entryError(anchor, lexer.Token{}, diagnostics.ProgramShutdownCycle,
				"A facility is torn down only after every cleanup that depends on it.",
				"shutdown plan is not satisfiable: %s", missing))
		}
		if len(shutdown.StaticDestructionCycle) > 0 {
			errors = append(errors, entryError(anchor, lexer.Token{}, diagnostics.ProgramShutdownCycle,
				"Each static's destruction uses another static that must outlive it; no canonical rule orders this cycle, so shutdown never picks an order.",
				"static destruction dependency cycle: %s", strings.Join(shutdown.StaticDestructionCycle, " -> ")))
		}
		if len(shutdown.StaticDestructionRequirements) > 0 && shutdown.StaticDestructionCapability == "unsupported" {
			errors = append(errors, entryError(anchor, lexer.Token{}, diagnostics.ProgramStaticDestructionUnsupported,
				"Give these statics trivially destructible types, or select a TargetProfile with deterministic static destruction.",
				"TargetProfile %s does not support the deterministic static destruction required by %s",
				profileDisplay(initialization.Profile), strings.Join(shutdown.StaticDestructionRequirements, ", ")))
		}
	}
	if initialization.TerminationPolicy == "" || initialization.TerminationPolicy == "unsupported" {
		errors = append(errors, entryError(anchor, lexer.Token{}, diagnostics.ProgramTerminationUnsupported,
			"Select a TargetProfile that defines how this Target terminates normally.",
			"%s target has no supported normal termination policy on TargetProfile %s", initialization.Entry.Kind, profileDisplay(initialization.Profile)))
	}
	return errors
}

// dependencyCycle returns a deterministic representative cycle of a
// dependency graph, rotated to start at its smallest member, or nil.
func dependencyCycle(graph map[string][]string) []string {
	ids := make([]string, 0, len(graph))
	for id := range graph {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	state := map[string]int{}
	stack := []string{}
	var found []string
	var visit func(string) bool
	visit = func(id string) bool {
		state[id] = 1
		stack = append(stack, id)
		next := append([]string(nil), graph[id]...)
		sort.Strings(next)
		for _, dependency := range next {
			if _, known := graph[dependency]; !known {
				continue
			}
			switch state[dependency] {
			case 1:
				start := 0
				for index, member := range stack {
					if member == dependency {
						start = index
					}
				}
				found = append([]string(nil), stack[start:]...)
				return true
			case 0:
				if visit(dependency) {
					return true
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = 2
		return false
	}
	for _, id := range ids {
		if state[id] == 0 && visit(id) {
			break
		}
	}
	if len(found) == 0 {
		return nil
	}
	smallest := 0
	for index, member := range found {
		if member < found[smallest] {
			smallest = index
		}
	}
	rotated := append(append([]string(nil), found[smallest:]...), found[:smallest]...)
	return append(rotated, rotated[0])
}

// orderViolation reports the first action whose prerequisite is unknown or
// ordered after it.
func orderViolation(actions []StartupAction) string {
	position := map[string]int{}
	for index, action := range actions {
		position[action.ID] = index
	}
	for index, action := range actions {
		for _, prerequisite := range action.Requires {
			at, known := position[prerequisite]
			switch {
			case !known:
				return fmt.Sprintf("%s requires unknown action %s", action.ID, prerequisite)
			case at > index:
				return fmt.Sprintf("%s is ordered before its prerequisite %s", action.ID, prerequisite)
			}
		}
	}
	return ""
}

func runtimeFacilityDisplay(facility RuntimeFacility) string {
	switch facility {
	case RuntimeAllocator:
		return "allocator runtime"
	case RuntimeThreadSupport:
		return "platform thread support"
	case RuntimeTaskScheduler:
		return "task scheduler"
	case RuntimeProcessSupport:
		return "process support"
	case RuntimeDetachedExecution:
		return "detached-execution manager"
	}
	return string(facility)
}

func profileDisplay(profile string) string {
	if profile == "" {
		return "(default)"
	}
	return profile
}

// ProgramLinkRequirements are the startup and shutdown facilities the
// resolved plans require from linking: what must exist, not how objects,
// symbols and sections are assembled, which linking.md owns (§ 41).
type ProgramLinkRequirements struct {
	EntryAdapter       string
	StartupComponents  []string
	RuntimeComponents  []RuntimeFacility
	ShutdownComponents []string
	TerminationAdapter string
	Sections           []string
}

// ProgramLinkingRequirements derives the linking-facing requirements of the
// resolved plans: the entry adapter bridging the execution-environment entry
// to source main (§ 28), the runtime components and their startup and
// teardown actions, the termination adapter, and the static-data sections
// the physical storage establishment needs.
// Rules: rules/compiler/initialization.md — §§ 7, 28, 41.
func ProgramLinkingRequirements(initialization *ProgramInitializationPlan, shutdown *ProgramShutdownPlan) ProgramLinkRequirements {
	var out ProgramLinkRequirements
	if initialization == nil {
		return out
	}
	switch initialization.Entry.Kind {
	case ProgramTargetCommand:
		out.EntryAdapter = "command entry adapter: execution-environment entry -> Sec main() int"
	case ProgramTargetFirmware:
		out.EntryAdapter = "firmware entry adapter: TargetProfile entry -> Sec main() void"
	case ProgramTargetTest:
		out.EntryAdapter = "test entry adapter: execution-environment entry -> generated test harness"
	}
	for _, action := range initialization.StartupActions {
		if action.Kind == StartupInitializeRuntimeFacility {
			out.StartupComponents = append(out.StartupComponents, action.ID)
		}
	}
	for _, requirement := range initialization.Runtime {
		out.RuntimeComponents = append(out.RuntimeComponents, requirement.Facility)
	}
	initialized, zeroed := false, false
	for _, static := range initialization.StaticStorage {
		if static.ZeroInit {
			zeroed = true
		} else {
			initialized = true
		}
	}
	if initialized {
		out.Sections = append(out.Sections, "initialized static data")
	}
	if zeroed {
		out.Sections = append(out.Sections, "zero-initialized static data")
	}
	if shutdown != nil {
		if len(shutdown.StaticDestructionRequirements) > 0 {
			out.ShutdownComponents = append(out.ShutdownComponents, "static-destruction")
		}
		for _, action := range shutdown.RuntimeTeardown {
			out.ShutdownComponents = append(out.ShutdownComponents, action.ID)
		}
	}
	out.TerminationAdapter = initialization.TerminationPolicy
	return out
}

// programPlanSchema versions the initialization-plan contract so a schema
// change invalidates every dependent artifact (§ 44).
const programPlanSchema = "sec.program-plans/1"

// ProgramPlanFingerprint is the incremental identity of the complete
// initialization/shutdown contract (§ 44): Target kind, entry module and
// entry identity, static storage, runtime requirements with their
// TargetProfile capabilities, detached-execution and static-destruction
// requirements, startup and shutdown dependency graphs, the generated
// adapter requirements, termination policy, TargetProfile and plan schema.
// Source positions and requirement reasons are diagnostics, not contract,
// so an edit that preserves the contract preserves the fingerprint.
// Rules: rules/compiler/initialization.md — §§ 43, 44.
func ProgramPlanFingerprint(initialization *ProgramInitializationPlan, shutdown *ProgramShutdownPlan) string {
	if initialization == nil {
		return ""
	}
	lines := []string{
		"schema " + programPlanSchema,
		"profile " + initialization.Profile,
		"compilation-plan " + initialization.PlanFingerprint,
		fmt.Sprintf("entry %s %s %s", initialization.Entry.Kind, initialization.Entry.Module, initialization.Entry.Callable),
	}
	for _, static := range initialization.StaticStorage {
		lines = append(lines, fmt.Sprintf("static %s.%s %s mutable=%t zero=%t init=%s destruction=%t",
			static.Module, static.Name, static.Type, static.Mutable, static.ZeroInit, static.Initial, static.Destruction))
	}
	for _, requirement := range initialization.Runtime {
		lines = append(lines, fmt.Sprintf("runtime %s %s", requirement.Facility, requirement.Capability))
	}
	for _, action := range initialization.StartupActions {
		lines = append(lines, fmt.Sprintf("startup %s %s fallible=%t [%s]", action.ID, action.Kind, action.Fallible, strings.Join(action.Requires, ",")))
	}
	if shutdown != nil {
		lines = append(lines, fmt.Sprintf("detached-shutdown %t", shutdown.DetachedExecutionShutdown),
			fmt.Sprintf("static-destruction %s [%s] order [%s]", shutdown.StaticDestructionCapability, strings.Join(shutdown.StaticDestructionRequirements, ","), strings.Join(shutdown.StaticDestructionOrder, ",")))
		for _, action := range shutdown.RuntimeTeardown {
			lines = append(lines, fmt.Sprintf("shutdown %s %s [%s]", action.ID, action.Phase, strings.Join(action.After, ",")))
		}
		lines = append(lines, "environment-termination "+shutdown.ExecutionEnvironmentTermination)
	}
	link := ProgramLinkingRequirements(initialization, shutdown)
	lines = append(lines, "entry-adapter "+link.EntryAdapter, "sections "+strings.Join(link.Sections, ","),
		"termination "+initialization.TerminationPolicy)
	digest := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(digest[:])
}
