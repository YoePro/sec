package testexec

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

type fakeProvider struct {
	probe   func(context.Context, Artifact) (Capabilities, error)
	execute func(context.Context, Request) (Report, error)
}

// Probe supplies test-controlled capability evidence for the exact image.
// Rules: rules/tooling/testing.md — §35.
func (p fakeProvider) Probe(ctx context.Context, artifact Artifact) (Capabilities, error) {
	if p.probe != nil {
		return p.probe(ctx, artifact)
	}
	return Capabilities{Execute: true}, nil
}

// Execute is a deliberately injected provider, not execution of Sec source.
// Rules: rules/tooling/testing.md — §§35–37.
func (p fakeProvider) Execute(ctx context.Context, request Request) (Report, error) {
	return p.execute(ctx, request)
}

type controlledProvider struct {
	fakeProvider
	control func(context.Context, Request, Action) error
}

// Control captures optional run controls without choosing a transport.
// Rules: rules/tooling/testing.md — §§35,37.
func (p controlledProvider) Control(ctx context.Context, request Request, action Action) error {
	return p.control(ctx, request, action)
}

// providerRequest consumes real compiler-owned test metadata on a selected
// non-host target, rather than discovering tests through a second parser.
// Rules: rules/tooling/testing.md — §§6,26.4,27.1,34.2.
func providerRequest(t *testing.T, width uint16) Request {
	t.Helper()
	path := "../../testdata/testing/provider/selection_test.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), path))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	plan := layout.ResolvedScalarPlan{TargetOS: "baremetal", TargetArch: "arm", Profile: "runtime-free", PointerWidthBits: width, Endianness: layout.LittleEndian}
	a := sema.NewAnalyzerWithScalarPlan(plan)
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	request := Request{RunID: "run-1", Artifact: Artifact{Path: "selected-tests.bin", PlanID: "selected-plan", Variant: "board-debug", Plan: plan}}
	for _, metadata := range a.ResolvedTests() {
		request.Artifact.Tests = append(request.Artifact.Tests, Test{Identity: metadata.Identity, Source: metadata.Source})
	}
	if len(request.Artifact.Tests) != 2 {
		t.Fatal(request)
	}
	return request
}

// passingReport models collected completed target invocations for selected tests.
// Rules: rules/tooling/testing.md — §§31,36.
func passingReport(request Request) Report {
	report := Report{RunID: request.RunID, PlanID: request.Artifact.PlanID}
	for _, test := range request.Artifact.Tests {
		report.Invocations = append(report.Invocations, Invocation{Identity: test.Identity, Outcome: Passed})
	}
	return report
}

// TestProviderCompletedOutcomes distinguishes execution completion from pass,
// fail and skip, including legitimate fail-fast omissions and child attribution.
// Rules: rules/tooling/testing.md — §§3.3,19–20,29–31.
func TestProviderCompletedOutcomes(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, outcome := range []Outcome{Passed, Failed, Skipped} {
			request := providerRequest(t, width)
			provider := fakeProvider{execute: func(_ context.Context, got Request) (Report, error) {
				if got.Artifact.Plan.PointerWidthBits != width || got.Artifact.Variant != "board-debug" || got.Artifact.Plan.TargetOS != "baremetal" {
					t.Fatal("host substitution", got)
				}
				report := passingReport(got)
				report.Invocations[0].Outcome = outcome
				if outcome == Failed {
					report.Invocations[0].Failures = []Failure{{Kind: "expect-equal", Expected: "1", Actual: "2", Source: got.Artifact.Tests[0].Source}}
				}
				return report, nil
			}}
			result := Run(context.Background(), provider, request)
			if result.Status != Completed || result.Err != nil || result.Report.Invocations[0].Outcome != outcome {
				t.Fatal(result)
			}
		}
	}
	request := providerRequest(t, 32)
	request.FailFast = true
	provider := fakeProvider{execute: func(_ context.Context, got Request) (Report, error) {
		report := passingReport(got)
		report.Invocations[0].Outcome = Failed
		report.Invocations = report.Invocations[:1]
		report.NotRun = []sema.TestIdentity{got.Artifact.Tests[1].Identity}
		child := got.Artifact.Tests[0].Identity
		child.Path = append(slices.Clone(child.Path), "child")
		report.Invocations = append(report.Invocations, Invocation{Identity: child, Outcome: Failed})
		return report, nil
	}}
	if result := Run(context.Background(), provider, request); result.Status != Completed || len(result.Report.NotRun) != 1 {
		t.Fatal(result)
	}
}

// TestProviderFaultsPreserveAttribution keeps hardware/process faults, reset,
// timeout and provider loss separate from completed test outcomes and preserves
// the provider's recovery choice without automatic retry.
// Rules: rules/tooling/testing.md — §37.
func TestProviderFaultsPreserveAttribution(t *testing.T) {
	request := providerRequest(t, 32)
	for _, kind := range []FaultKind{ProcessorFault, Panic, Crash, TargetReset, ProviderLost, Timeout} {
		for _, recovery := range []Recovery{RecoveryUnknown, Continue, Restart, Redeploy, CannotContinue} {
			provider := fakeProvider{execute: func(_ context.Context, got Request) (Report, error) {
				report := passingReport(got)
				report.Invocations = report.Invocations[:1]
				active := got.Artifact.Tests[1].Identity
				report.Faults = []Fault{{Kind: kind, Active: &active, Recovery: recovery}}
				return report, nil
			}}
			result := Run(context.Background(), provider, request)
			if result.Status != ExecutionFailed || result.Err != nil || len(result.Report.Invocations) != 1 || result.Report.Faults[0].Kind != kind || result.Report.Faults[0].Recovery != recovery || result.Report.Faults[0].Active.Path[0] != "second case" {
				t.Fatal(result)
			}
		}
	}
	provider := fakeProvider{execute: func(_ context.Context, got Request) (Report, error) {
		return Report{RunID: got.RunID, PlanID: got.Artifact.PlanID, Faults: []Fault{{Kind: TargetReset, Recovery: RecoveryUnknown}}}, nil
	}}
	if result := Run(context.Background(), provider, request); result.Status != ExecutionFailed || result.Report.Faults[0].Active != nil {
		t.Fatal(result)
	}
}

// TestProviderRejectsInvalidReports prevents a transport/provider bug from
// certifying missing/foreign outcomes or weakening monotonic failure semantics.
// Rules: rules/tooling/testing.md — §§6,19–20,30–31,37.
func TestProviderRejectsInvalidReports(t *testing.T) {
	mutations := map[string]func(*Report){
		"missing":            func(r *Report) { r.Invocations = r.Invocations[:1] },
		"duplicate":          func(r *Report) { r.Invocations = append(r.Invocations, r.Invocations[0]) },
		"foreign":            func(r *Report) { r.Invocations[0].Identity.Module = "other" },
		"outcome":            func(r *Report) { r.Invocations[0].Outcome = "crashed" },
		"pass with failures": func(r *Report) { r.Invocations[0].Failures = []Failure{{Message: "observed failure"}} },
		"skip with failures": func(r *Report) {
			r.Invocations[0].Outcome = Skipped
			r.Invocations[0].Failures = []Failure{{Message: "observed failure"}}
		},
		"run":  func(r *Report) { r.RunID = "other-run" },
		"plan": func(r *Report) { r.PlanID = "other-plan" },
		"unexplained omission": func(r *Report) {
			r.NotRun = []sema.TestIdentity{r.Invocations[1].Identity}
			r.Invocations = r.Invocations[:1]
		},
		"duplicate omission": func(r *Report) { r.NotRun = []sema.TestIdentity{r.Invocations[0].Identity} },
		"unknown fault":      func(r *Report) { r.Faults = []Fault{{Kind: "mystery", Recovery: RecoveryUnknown}} },
		"unknown recovery":   func(r *Report) { r.Faults = []Fault{{Kind: TargetReset, Recovery: "retry-and-pass"}} },
		"foreign fault": func(r *Report) {
			r.Faults = []Fault{{Kind: Crash, Recovery: CannotContinue, Active: &sema.TestIdentity{Module: "other", Path: []string{"x"}}}}
		},
		"failed child": func(r *Report) {
			child := r.Invocations[0].Identity
			child.Path = append(slices.Clone(child.Path), "child")
			r.Invocations = append(r.Invocations, Invocation{Identity: child, Outcome: Failed})
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			request := providerRequest(t, 32)
			provider := fakeProvider{execute: func(_ context.Context, got Request) (Report, error) {
				r := passingReport(got)
				mutate(&r)
				return r, nil
			}}
			result := Run(context.Background(), provider, request)
			if result.Status != ExecutionFailed || result.Err == nil || result.Report.Faults[len(result.Report.Faults)-1].Kind != InvalidReport {
				t.Fatal(result)
			}
		})
	}
}

// TestProviderUnavailableAndCancellation verifies no host fallback, bounded
// provider cooperation, partial evidence retention and non-Skipped timeouts.
// Rules: rules/tooling/testing.md — §§30,34.2,35.3,37.
func TestProviderUnavailableAndCancellation(t *testing.T) {
	request := providerRequest(t, 32)
	if result := Run(context.Background(), nil, request); result.Status != Unavailable || !errors.Is(result.Err, ErrUnavailable) {
		t.Fatal(result)
	}
	for _, probeErr := range []error{nil, ErrUnavailable, errors.New("broken launcher")} {
		calls := 0
		provider := fakeProvider{probe: func(context.Context, Artifact) (Capabilities, error) { return Capabilities{}, probeErr }, execute: func(context.Context, Request) (Report, error) { calls++; return Report{}, nil }}
		result := Run(context.Background(), provider, request)
		want := Unavailable
		if probeErr != nil && !errors.Is(probeErr, ErrUnavailable) {
			want = ExecutionFailed
		}
		if result.Status != want || calls != 0 {
			t.Fatal(result, calls)
		}
	}
	provider := fakeProvider{execute: func(ctx context.Context, got Request) (Report, error) {
		report := passingReport(got)
		report.Invocations = report.Invocations[:1]
		<-ctx.Done()
		return report, ctx.Err()
	}}
	request.Timeout = time.Millisecond
	result := Run(context.Background(), provider, request)
	if result.Status != ExecutionFailed || !errors.Is(result.Err, context.DeadlineExceeded) || result.Report.Faults[0].Kind != Timeout || len(result.Report.Invocations) != 1 {
		t.Fatal(result)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result := Run(ctx, nil, request); result.Status != ExecutionFailed || result.Report.Faults[0].Kind != Cancelled {
		t.Fatal(result)
	}
}

// TestProviderRequestValidation fails before probing for malformed compiler
// requests rather than guessing a host target or allowing ambiguous identities.
// Rules: rules/tooling/testing.md — §§6,26.4,34.2.
func TestProviderRequestValidation(t *testing.T) {
	mutations := map[string]func(*Request){
		"run":       func(r *Request) { r.RunID = "" },
		"artifact":  func(r *Request) { r.Artifact.Path = "" },
		"plan":      func(r *Request) { r.Artifact.PlanID = "" },
		"target":    func(r *Request) { r.Artifact.Plan.TargetOS = "" },
		"width":     func(r *Request) { r.Artifact.Plan.PointerWidthBits = 0 },
		"endian":    func(r *Request) { r.Artifact.Plan.Endianness = "" },
		"timeout":   func(r *Request) { r.Timeout = -1 },
		"empty":     func(r *Request) { r.Artifact.Tests = nil },
		"module":    func(r *Request) { r.Artifact.Tests[0].Identity.Module = "" },
		"path":      func(r *Request) { r.Artifact.Tests[0].Identity.Path = []string{""} },
		"subtest":   func(r *Request) { r.Artifact.Tests[0].Identity.Path = []string{"first / case", "dynamic child"} },
		"duplicate": func(r *Request) { r.Artifact.Tests = append(r.Artifact.Tests, r.Artifact.Tests[0]) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			request := providerRequest(t, 32)
			mutate(&request)
			calls := 0
			provider := fakeProvider{probe: func(context.Context, Artifact) (Capabilities, error) { calls++; return Capabilities{}, nil }}
			result := Run(context.Background(), provider, request)
			if result.Status != ExecutionFailed || result.Err == nil || calls != 0 {
				t.Fatal(result, calls)
			}
		})
	}
}

// TestProviderControlsAndSnapshots verifies advertised optional controls and
// isolated capability/execution requests without transport or global sessions.
// Rules: rules/tooling/testing.md — §§26.4,35,37.6.
func TestProviderControlsAndSnapshots(t *testing.T) {
	request := providerRequest(t, 32)
	request.Artifact.Plan.AddressDomains = []layout.AddressDomain{{Identity: "memory", AddressBits: 32, UnitBits: 8, Linear: true}}
	calls := 0
	provider := controlledProvider{
		fakeProvider: fakeProvider{
			probe: func(_ context.Context, artifact Artifact) (Capabilities, error) {
				artifact.Tests[0].Identity.Path[0] = "corrupted"
				artifact.Plan.AddressDomains[0].Identity = "corrupted"
				return Capabilities{Execute: true, Reset: true, Interrupt: true, Debug: true}, nil
			},
			execute: func(_ context.Context, got Request) (Report, error) {
				if got.Artifact.Tests[0].Identity.Path[0] != "first / case" || got.Artifact.Plan.AddressDomains[0].Identity != "memory" {
					t.Fatal(got)
				}
				return passingReport(got), nil
			},
		},
		control: func(_ context.Context, got Request, action Action) error {
			calls++
			if got.RunID != request.RunID || got.Artifact.PlanID != request.Artifact.PlanID || got.Artifact.Tests[0].Identity.Path[0] != "first / case" {
				t.Fatal(got)
			}
			got.Artifact.Tests[0].Identity.Path[0] = "control mutation"
			return nil
		},
	}
	if result := Run(context.Background(), provider, request); result.Status != Completed {
		t.Fatal(result)
	}
	for _, action := range []Action{Reset, Interrupt, Debug} {
		if err := Control(context.Background(), provider, request, action); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 3 || request.Artifact.Tests[0].Identity.Path[0] != "first / case" || request.Artifact.Plan.AddressDomains[0].Identity != "memory" {
		t.Fatal(calls, request)
	}
	if err := Control(context.Background(), provider, request, "unknown"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if err := Control(context.Background(), provider.fakeProvider, request, Reset); err == nil {
		t.Fatal("advertised control without Controller accepted")
	}
	provider.probe = func(context.Context, Artifact) (Capabilities, error) { return Capabilities{Execute: true}, nil }
	if err := Control(context.Background(), provider, request, Debug); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Control(ctx, provider, request, Reset); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

// TestProviderPartialErrorCannotBecomeUnavailable preserves observed failures
// even when the launcher subsequently reports an unavailable/lost provider.
// Rules: rules/tooling/testing.md — §§30–31,35.3,37.
func TestProviderPartialErrorCannotBecomeUnavailable(t *testing.T) {
	request := providerRequest(t, 32)
	provider := fakeProvider{execute: func(_ context.Context, got Request) (Report, error) {
		report := passingReport(got)
		report.Invocations = report.Invocations[:1]
		report.Invocations[0].Outcome = Failed
		return report, ErrUnavailable
	}}
	result := Run(context.Background(), provider, request)
	if result.Status != ExecutionFailed || result.Report.Invocations[0].Outcome != Failed || len(result.Report.Faults) != 1 {
		t.Fatal(result)
	}
}
