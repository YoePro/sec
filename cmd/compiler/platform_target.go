package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"sec/internal/layout"
	platformtarget "sec/internal/platform/target"
)

const platformTargetUsage = "usage: sec platform target list [--json]\n       sec platform target info [--target <os-arch>] [--json]"

// platformTargetSummary is one registered target in `sec platform target list`.
type platformTargetSummary struct {
	Target  string `json:"target"`
	Triple  string `json:"triple"`
	Profile string `json:"profile"`
	Status  string `json:"status"`
}

// platformTargetInfo is the immutable, resolved plan of one target as
// `sec platform target info` reports it: identity, data model, layout facts,
// reference and epoch policy, capability tables, compiler stage support and
// the semantic fingerprints. Nothing is derived from the compiler host.
type platformTargetInfo struct {
	Target           string                        `json:"target"`
	Triple           string                        `json:"triple"`
	ABI              string                        `json:"abi"`
	CABI             string                        `json:"c_abi,omitempty"`
	Profile          string                        `json:"profile"`
	ProfileFamily    string                        `json:"profile_family,omitempty"`
	Status           string                        `json:"status"`
	PointerWidthBits uint16                        `json:"pointer_width_bits"`
	Endianness       string                        `json:"endianness"`
	DataLayout       string                        `json:"data_layout,omitempty"`
	ScalarAlignments map[string]uint64             `json:"scalar_alignments"`
	AtomicAlignments map[string]uint64             `json:"atomic_alignments"`
	EpochBits        uint16                        `json:"logical_epoch_bits"`
	EpochExhaustion  string                        `json:"arena_epoch_exhaustion"`
	AddressDomains   int                           `json:"address_domains"`
	Allocation       layout.AllocationCapabilities `json:"allocation"`
	AtomicFacts      int                           `json:"atomic_capability_facts"`
	IPCFacts         int                           `json:"ipc_capability_facts"`
	MutexFacts       int                           `json:"mutex_capability_facts"`
	PhysicalThreads  bool                          `json:"physical_thread_capability"`
	Stages           map[string]bool               `json:"compiler_stages"`
	Fingerprints     layout.PlanFingerprints       `json:"fingerprints"`
}

// runPlatformTarget lists registered targets or inspects one target's
// frozen CompilationPlan. It reads canonical registry and plan facts only:
// it resolves no project configuration, consults no host capability and
// never mutates the plan.
// Rules: rules/tooling/cli.md — § 10; rules/compiler/compiler.md — § 64(3);
// rules/platform/platform_model.md — §§ 14, 21, 23.
func runPlatformTarget(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "list" && args[0] != "info" {
		fmt.Fprintln(stderr, platformTargetUsage)
		return 1
	}
	command, rest := args[0], args[1:]
	asJSON := false
	selected := hostCompilerTarget()
	for i := 0; i < len(rest); i++ {
		switch {
		case rest[i] == "--json":
			asJSON = true
		case rest[i] == "--target" && command == "info" && i+1 < len(rest):
			parsed, ok := parseCompilerTarget(rest[i+1])
			if !ok {
				fmt.Fprintf(stderr, "sec platform target info: unknown target %q\n", rest[i+1])
				return 1
			}
			selected = parsed
			i++
		default:
			fmt.Fprintf(stderr, "sec platform target %s: unexpected argument %q\n%s\n", command, rest[i], platformTargetUsage)
			return 1
		}
	}
	if command == "list" {
		var targets []platformTargetSummary
		for _, definition := range platformtarget.Definitions() {
			targets = append(targets, platformTargetSummary{Target: definition.OS + "-" + definition.Arch, Triple: definition.LLVMTriple, Profile: definition.Profile, Status: string(definition.Status)})
		}
		sort.Slice(targets, func(i, j int) bool { return targets[i].Target < targets[j].Target })
		if asJSON {
			return writePlatformJSON(stdout, stderr, targets)
		}
		for _, target := range targets {
			fmt.Fprintf(stdout, "%-22s %-34s %-14s %s\n", target.Target, target.Triple, target.Profile, target.Status)
		}
		return 0
	}
	definition, ok := platformtarget.Find(selected)
	if !ok {
		fmt.Fprintf(stderr, "sec platform target info: no registered target %s\n", selected.String())
		return 1
	}
	info, err := platformTargetInfoOf(definition)
	if err != nil {
		fmt.Fprintf(stderr, "sec platform target info: %s has no valid resolved plan: %v\n", selected.String(), err)
		return 1
	}
	if asJSON {
		return writePlatformJSON(stdout, stderr, info)
	}
	writePlatformTargetInfo(stdout, info)
	return 0
}

// platformTargetInfoOf projects the frozen plan of one registry definition.
// Rules: rules/compiler/compiler.md — § 64(3); rules/memory/layout.md — §§ 47.2, 47.6.
func platformTargetInfoOf(definition platformtarget.Definition) (platformTargetInfo, error) {
	plan, err := definition.ScalarPlan()
	if err != nil {
		return platformTargetInfo{}, err
	}
	info := platformTargetInfo{
		Target: definition.OS + "-" + definition.Arch, Triple: plan.LLVMTriple, ABI: plan.ABI, CABI: plan.CABI.Name,
		Profile: plan.Profile, Status: string(definition.Status), PointerWidthBits: plan.PointerWidthBits,
		Endianness: string(plan.Endianness), ScalarAlignments: alignmentMap(plan.Alignments), AtomicAlignments: alignmentMap(plan.AtomicAlignments),
		AddressDomains: len(plan.AddressDomains), Allocation: plan.AllocationCapabilities(),
		AtomicFacts: len(plan.Atomics.Facts()), IPCFacts: len(plan.IPC.Facts()), MutexFacts: len(plan.Mutex.Facts()),
		PhysicalThreads: plan.PhysicalThreads.Defined(), Stages: map[string]bool{}, Fingerprints: plan.Fingerprints(),
	}
	if family, err := definition.ProfileFamily(); err == nil {
		info.ProfileFamily = family
	}
	if layoutText, ok := definition.DataLayout(); ok {
		info.DataLayout = layoutText
	}
	if policy, err := plan.References.ResolveArenaEpochPolicy(); err == nil {
		info.EpochBits, info.EpochExhaustion = policy.LogicalEpochBits, string(policy.Exhaustion)
	} else {
		info.EpochBits, info.EpochExhaustion = plan.References.EpochBits(), "unresolved: "+err.Error()
	}
	support := definition.CompilerSupport()
	for _, stage := range []platformtarget.PipelineStage{platformtarget.StageSemanticValidation, platformtarget.StageAnalysis, platformtarget.StageSecMLIRLowering,
		platformtarget.StageObjectEmission, platformtarget.StageLinking, platformtarget.StageExecution} {
		info.Stages[string(stage)] = support.Supports(stage)
	}
	return info, nil
}

// alignmentMap spells typed alignment facts by representation; nil stays empty.
func alignmentMap(facts layout.ScalarAlignments) map[string]uint64 {
	out := map[string]uint64{}
	for representation, alignment := range facts {
		out[representation.String()] = alignment
	}
	return out
}

func sortedAlignments(facts map[string]uint64) string {
	if len(facts) == 0 {
		return "unresolved"
	}
	keys := make([]string, 0, len(facts))
	for key := range facts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, key := range keys {
		parts[i] = fmt.Sprintf("%s:%d", key, facts[key])
	}
	return strings.Join(parts, " ")
}

func writePlatformTargetInfo(out io.Writer, info platformTargetInfo) {
	line := func(label, format string, args ...any) {
		fmt.Fprintf(out, "%-18s %s\n", label, fmt.Sprintf(format, args...))
	}
	line("target", "%s (%s)", info.Target, info.Triple)
	line("status", "%s", info.Status)
	line("abi", "%s, C ABI %s", info.ABI, info.CABI)
	line("profile", "%s (family %s)", info.Profile, info.ProfileFamily)
	line("pointer", "%d bits, %s-endian", info.PointerWidthBits, info.Endianness)
	if info.DataLayout != "" {
		line("data layout", "%s", info.DataLayout)
	}
	line("scalar align", "%s", sortedAlignments(info.ScalarAlignments))
	line("atomic align", "%s", sortedAlignments(info.AtomicAlignments))
	line("arena epoch", "%d-bit logical, %s on exhaustion", info.EpochBits, info.EpochExhaustion)
	line("allocation", "dynamic %s/%s, Arena context %s/%s", info.Allocation.DynamicAllocation.Availability, info.Allocation.DynamicAllocation.Activation,
		info.Allocation.FixedOrSuppliedArenaContext.Availability, info.Allocation.FixedOrSuppliedArenaContext.Activation)
	line("capabilities", "atomics %d, ipc %d, mutex %d, physical threads %t", info.AtomicFacts, info.IPCFacts, info.MutexFacts, info.PhysicalThreads)
	stages := make([]string, 0, len(info.Stages))
	for stage, supported := range info.Stages {
		if supported {
			stages = append(stages, stage)
		}
	}
	sort.Strings(stages)
	line("compiler stages", "%s", strings.Join(stages, ", "))
	line("fingerprint", "plan %s", info.Fingerprints.Plan)
}

func writePlatformJSON(stdout, stderr io.Writer, value any) int {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "sec platform target: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, string(encoded))
	return 0
}
