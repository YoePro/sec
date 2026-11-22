// Package testexec defines the compiler/tooling boundary to test execution.
// It defines no target transport, source timeout, or implicit host fallback.
package testexec

import (
	"context"
	"time"

	"sec/internal/layout"
	"sec/internal/sema"
)

// Artifact identifies a separately built selected-test image. PlanID and
// Variant are compiler-resolved identities, never inferred by the provider.
// Tests use Sema identities; generated symbol names are not test identities.
// Rules: rules/tooling/testing.md — §§6,26.4,33.1,34.
type Artifact struct {
	Path    string
	PlanID  string
	Variant string
	Plan    layout.ResolvedScalarPlan
	Tests   []Test
}

// Test retains host-side metadata without prescribing the target's compact ID
// encoding or embedding source paths in firmware.
// Rules: rules/tooling/testing.md — §§31.2,36.2.
type Test struct {
	Identity sema.TestIdentity
	Source   sema.TestSourceLocation
}

// Capabilities are probed for the exact selected artifact, not the compiler
// host. Deployment/start/collection belong inside Execute. Optional controls
// require Controller and are separate from source-language testing operations.
// Rules: rules/tooling/testing.md — §§34.2,35.1–35.5.
type Capabilities struct {
	Execute   bool
	Reset     bool
	Interrupt bool
	Debug     bool
}

// Provider implements a host runner, simulator, emulator, probe, farm or other
// launcher. Probe returning Execute=false is execution-unavailable. Execute
// owns deployment, launch, result collection and its reporting resources; it
// must honor context cancellation and return after cleanup. It may never
// recompile the image for the host or silently consume a tested peripheral.
// Returned report storage transfers to the caller and must not be mutated by
// the provider afterward. Providers use Fault.Active only when attribution is
// known, and report their recovery requirement without silently retrying.
// No event wire encoding or debugger protocol is imposed by this interface.
// Rules: rules/tooling/testing.md — §§34–38.
type Provider interface {
	Probe(context.Context, Artifact) (Capabilities, error)
	Execute(context.Context, Request) (Report, error)
}

// Request binds a run to its exact artifact and optional runner-owned timeout.
// Zero timeout adds no deadline. FailFast affects scheduling, not observed
// outcomes; already reported failures must remain failures.
// Rules: rules/tooling/testing.md — §§29,34.4,37.4.
type Request struct {
	RunID    string
	Artifact Artifact
	Timeout  time.Duration
	FailFast bool
}

type Action string

const (
	Reset     Action = "reset"
	Interrupt Action = "interrupt"
	Debug     Action = "debug"
)

// Controller is optional provider control. The request identifies the exact
// run and artifact; the provider owns session and transport details. Recovery
// after a reset/fault is reported separately, never converted to Skip.
// Rules: rules/tooling/testing.md — §§35.1–35.2,37.3–37.6.
type Controller interface {
	Control(context.Context, Request, Action) error
}

type Outcome string

const (
	Passed  Outcome = "passed"
	Failed  Outcome = "failed"
	Skipped Outcome = "skipped"
)

// Failure is logical assertion/error data, not a transport frame or a runtime
// reflection requirement. Expected/Actual are provider-rendered diagnostics.
// Rules: rules/tooling/testing.md — §§31.3–31.5,36.3.
type Failure struct {
	Source          sema.TestSourceLocation
	Kind            string
	Message         string
	Expected        string
	Actual          string
	UnexpectedError string
}

// Invocation reports a completed top-level test or hierarchical subtest.
// Infrastructure states are deliberately absent from Outcome. A provider must
// report outcomes only after the harness has completed ordinary cleanup.
// Rules: rules/tooling/testing.md — §§3.3,6,19–20,31.
type Invocation struct {
	Identity sema.TestIdentity
	Outcome  Outcome
	Message  string
	Failures []Failure
}

type FaultKind string

const (
	ProcessorFault FaultKind = "processor-fault"
	Panic          FaultKind = "unhandled-panic"
	Crash          FaultKind = "process-crash"
	TargetReset    FaultKind = "target-reset"
	ProviderLost   FaultKind = "provider-lost"
	Timeout        FaultKind = "timeout"
	Cancelled      FaultKind = "cancelled"
	Infrastructure FaultKind = "infrastructure"
	InvalidReport  FaultKind = "invalid-report"
)

type Recovery string

const (
	RecoveryUnknown Recovery = "unknown"
	Continue        Recovery = "continue"
	Restart         Recovery = "restart"
	Redeploy        Recovery = "redeploy"
	CannotContinue  Recovery = "cannot-continue"
)

// Fault distinguishes execution failure from an ordinary failed expectation.
// Active is nil when attribution is unknown, rather than guessed from the next
// selected test. Recovery is provider-owned and never silently retries a run.
// Rules: rules/tooling/testing.md — §37.
type Fault struct {
	Kind     FaultKind
	Active   *sema.TestIdentity
	Message  string
	Recovery Recovery
}

// Report preserves completed invocations and faults independently. NotRun
// explicitly accounts for selected tests omitted by fail-fast or infrastructure
// failure; an absent test result can never be fabricated as Passed or Skipped.
// Rules: rules/tooling/testing.md — §§29–31,35–37.
type Report struct {
	RunID       string
	PlanID      string
	Invocations []Invocation
	Faults      []Fault
	NotRun      []sema.TestIdentity
}

type Status string

const (
	Completed       Status = "completed"
	Unavailable     Status = "execution-unavailable"
	ExecutionFailed Status = "execution-failed"
)

// Execution keeps completed outcomes apart from provider failures. A completed
// execution may contain Failed tests. On provider/protocol failure the original
// Report remains available as evidence, but is not a validated completed run.
// Rules: rules/tooling/testing.md — §§3.3,30–31,35.3,37.
type Execution struct {
	Status Status
	Report Report
	Err    error
}
