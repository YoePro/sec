package testexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"sec/internal/sema"
)

var ErrUnavailable = errors.New("test execution provider is unavailable")

// Run validates the selected image and separates execution completion from
// ordinary test outcomes. A provider must cooperate with context cancellation;
// this coordinator does not spawn an unbounded background launcher or retry.
// Rules: rules/tooling/testing.md — §§26.4,29–31,34–37.
func Run(ctx context.Context, provider Provider, request Request) Execution {
	if err := validateRequest(request); err != nil {
		return executionError(Report{}, Infrastructure, err)
	}
	if ctx == nil {
		return executionError(Report{}, Infrastructure, errors.New("execution requires a context"))
	}
	request = cloneRequest(request)
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}
	if err := ctx.Err(); err != nil {
		return contextFailure(Report{}, err)
	}
	if provider == nil {
		return Execution{Status: Unavailable, Err: ErrUnavailable}
	}
	capabilities, err := provider.Probe(ctx, cloneRequest(request).Artifact)
	if ctx.Err() != nil {
		return contextFailure(Report{}, ctx.Err())
	}
	if err != nil {
		return providerError(Report{}, err)
	}
	if !capabilities.Execute {
		return Execution{Status: Unavailable, Err: ErrUnavailable}
	}
	report, err := provider.Execute(ctx, cloneRequest(request))
	if ctx.Err() != nil {
		return contextFailure(report, ctx.Err())
	}
	if err != nil {
		return providerError(report, err)
	}
	if err := validateReport(request, report); err != nil {
		return executionError(report, InvalidReport, err)
	}
	if len(report.Faults) != 0 {
		return Execution{Status: ExecutionFailed, Report: report}
	}
	return Execution{Status: Completed, Report: report}
}

// Control probes optional controls against the same selected artifact. Unsupported
// actions never become success merely because a provider implements Controller.
// Rules: rules/tooling/testing.md — §§35.1–35.2,37.6.
func Control(ctx context.Context, provider Provider, request Request, action Action) error {
	if err := validateRequest(request); err != nil {
		return err
	}
	if ctx == nil {
		return errors.New("control requires a context")
	}
	if provider == nil {
		return ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	request = cloneRequest(request)
	capabilities, err := provider.Probe(ctx, cloneRequest(request).Artifact)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	supported := action == Reset && capabilities.Reset || action == Interrupt && capabilities.Interrupt || action == Debug && capabilities.Debug
	if !supported {
		return fmt.Errorf("%w: control %q", ErrUnavailable, action)
	}
	controller, ok := provider.(Controller)
	if !ok {
		return errors.New("provider advertises controls without implementing Controller")
	}
	err = controller.Control(ctx, request, action)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// validateRequest requires a compiler-resolved plan and canonical selected
// top-level identities. No architecture spelling supplies missing target facts.
// Rules: rules/tooling/testing.md — §§6,26.4,34.2–34.4.
func validateRequest(request Request) error {
	if request.RunID == "" || request.Artifact.Path == "" || request.Artifact.PlanID == "" {
		return errors.New("execution requires run identity, test artifact and compilation-plan identity")
	}
	if request.Timeout < 0 {
		return errors.New("execution timeout cannot be negative")
	}
	if err := request.Artifact.Plan.Validate(); err != nil {
		return err
	}
	if len(request.Artifact.Tests) == 0 {
		return errors.New("execution requires selected tests")
	}
	seen := map[string]bool{}
	for _, test := range request.Artifact.Tests {
		if test.Identity.Module == "" || len(test.Identity.Path) != 1 || test.Identity.Path[0] == "" {
			return errors.New("artifact requires canonical top-level test identities")
		}
		key := identityKey(test.Identity)
		if seen[key] {
			return fmt.Errorf("duplicate selected test %s", key)
		}
		seen[key] = true
	}
	return nil
}

// validateReport rejects incomplete or foreign results, unknown states and
// failed assertions disguised as Passed/Skipped. Subtests retain their selected
// root, with no reconstruction from generated symbols or display strings.
// Rules: rules/tooling/testing.md — §§6,19–20,29–31,37.
func validateReport(request Request, report Report) error {
	if report.RunID != request.RunID || report.PlanID != request.Artifact.PlanID {
		return errors.New("provider report belongs to a different run or compilation plan")
	}
	selected := map[string]bool{}
	completed := map[string]Outcome{}
	accounted := map[string]bool{}
	failed := false
	for _, test := range request.Artifact.Tests {
		selected[identityKey(test.Identity)] = true
	}
	rooted := func(identity sema.TestIdentity) bool {
		if identity.Module == "" || len(identity.Path) == 0 {
			return false
		}
		for _, part := range identity.Path {
			if part == "" {
				return false
			}
		}
		return selected[identityKey(sema.TestIdentity{Module: identity.Module, Path: identity.Path[:1]})]
	}
	for _, result := range report.Invocations {
		key := identityKey(result.Identity)
		if !rooted(result.Identity) {
			return fmt.Errorf("unselected invocation %s", key)
		}
		if _, duplicate := completed[key]; duplicate {
			return fmt.Errorf("duplicate invocation %s", key)
		}
		switch result.Outcome {
		case Passed, Skipped:
			if len(result.Failures) != 0 {
				return fmt.Errorf("failed assertions cannot produce %s", result.Outcome)
			}
		case Failed:
			failed = true
		default:
			return fmt.Errorf("unknown test outcome %q", result.Outcome)
		}
		completed[key] = result.Outcome
		if len(result.Identity.Path) == 1 {
			accounted[key] = true
		}
	}
	for _, result := range report.Invocations {
		if result.Outcome != Failed {
			continue
		}
		for length := 1; length < len(result.Identity.Path); length++ {
			parent := sema.TestIdentity{Module: result.Identity.Module, Path: result.Identity.Path[:length]}
			if outcome, exists := completed[identityKey(parent)]; exists && outcome != Failed {
				return errors.New("failed subtest cannot have a passed or skipped parent")
			}
		}
	}
	for _, fault := range report.Faults {
		switch fault.Kind {
		case ProcessorFault, Panic, Crash, TargetReset, ProviderLost, Timeout, Cancelled, Infrastructure, InvalidReport:
		default:
			return fmt.Errorf("unknown execution fault %q", fault.Kind)
		}
		switch fault.Recovery {
		case RecoveryUnknown, Continue, Restart, Redeploy, CannotContinue:
		default:
			return fmt.Errorf("unknown recovery policy %q", fault.Recovery)
		}
		if fault.Active != nil && !rooted(*fault.Active) {
			return errors.New("fault attributed to an unselected test")
		}
	}
	for _, identity := range report.NotRun {
		key := identityKey(identity)
		if !selected[key] || accounted[key] {
			return fmt.Errorf("invalid unexecuted selection %s", key)
		}
		accounted[key] = true
	}
	if len(report.NotRun) != 0 && len(report.Faults) == 0 && !(request.FailFast && failed) {
		return errors.New("unexecuted tests require a fault or fail-fast failure")
	}
	if len(report.Faults) == 0 && len(accounted) != len(selected) {
		return errors.New("provider omitted selected test results")
	}
	return nil
}

// identityKey uses an unambiguous structured identity, preserving arbitrary
// source test-name punctuation instead of joining path components.
// Rules: rules/tooling/testing.md — §6.
func identityKey(identity sema.TestIdentity) string {
	data, _ := json.Marshal(identity)
	return string(data)
}

// cloneRequest detaches caller-owned source identities and mutable plan slices
// separately for capability probing and execution.
// Rules: rules/tooling/testing.md — §§26.4,34.2,39.1.
func cloneRequest(request Request) Request {
	request.Artifact.Plan.AddressDomains = slices.Clone(request.Artifact.Plan.AddressDomains)
	request.Artifact.Tests = slices.Clone(request.Artifact.Tests)
	for i := range request.Artifact.Tests {
		request.Artifact.Tests[i].Identity.Path = slices.Clone(request.Artifact.Tests[i].Identity.Path)
	}
	return request
}

// executionError retains the provider report as evidence, but never publishes
// it as a validated completed execution after an infrastructure/protocol fault.
// Rules: rules/tooling/testing.md — §§30–31,37.
func executionError(report Report, kind FaultKind, err error) Execution {
	report.Faults = append(slices.Clone(report.Faults), Fault{Kind: kind, Message: err.Error(), Recovery: CannotContinue})
	return Execution{Status: ExecutionFailed, Report: report, Err: err}
}

// providerError preserves explicit unavailability separately from a launcher
// failure, including partial reports from a lost target/provider.
// Rules: rules/tooling/testing.md — §§3.3,35.3,37.1.
func providerError(report Report, err error) Execution {
	if errors.Is(err, ErrUnavailable) && len(report.Invocations) == 0 && len(report.Faults) == 0 && len(report.NotRun) == 0 {
		return Execution{Status: Unavailable, Err: err}
	}
	return executionError(report, Infrastructure, err)
}

// contextFailure classifies runner deadlines and interruption as execution
// failures, never as ordinary Failed/Skipped invocations.
// Rules: rules/tooling/testing.md — §§37.1,37.4–37.6.
func contextFailure(report Report, err error) Execution {
	kind := Cancelled
	if errors.Is(err, context.DeadlineExceeded) {
		kind = Timeout
	}
	return executionError(report, kind, err)
}
