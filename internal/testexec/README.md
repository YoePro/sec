# Test execution-provider boundary

`Provider` is the compiler/tooling boundary after a selected test image has
been built. It is not a Sec interpreter, a native launcher implementation or
a substitute for harness generation.

The compiler supplies `Request` with the image path, compilation-plan identity,
variant, resolved scalar/target facts and selected top-level identities from
Sema. `Test.Source` retains host-side source metadata; this does not require
embedding names or paths in an embedded image. Generated function symbols are
never used as test identities.

`Run(ctx, provider, request)` probes the exact artifact, then executes it only
when the provider declares support. Native, simulator, emulator, probe and farm
providers can implement the same interface. The provider owns deployment,
launch, collection and transport resources. There is no implicit host rebuild,
UART reservation, default source timeout or automatic recovery/retry.

Callers must examine `Execution.Status` before publishing test outcomes:

- `Completed`: the report passed validation. Inspect each invocation's outcome;
  a completed execution can contain ordinary failed tests.
- `Unavailable`: the image may be buildable, but no provider can execute it.
- `ExecutionFailed`: execution or reporting failed. The report remains evidence,
  including any partial observations; it is not a validated completed run.

Compilation failures occur before this boundary and retain their existing CLI
classification. Missing test results never become passed or skipped tests.
Fail-fast omissions must be explicit and accompanied by an observed failure.

Faults distinguish processor faults, panic, process crash, target reset,
provider loss, timeout and interruption from ordinary expectation failures.
Only known active-test attribution is supplied. The provider reports whether
continuation, restart or redeployment is possible; the coordinator never
silently retries an observed failure.

`Request.Timeout` is optional runner policy. Providers must cooperate with the
context and return after cleanup; the coordinator does not leave a launcher
running in a detached goroutine. Host-side code can use optional `Control`
actions only when `Probe` advertises the capability and `Controller` exists.
There is no prescribed debugger or result-event wire protocol.

The package tests use injected providers and real compiler-owned metadata from
`testdata/testing/provider/selection_test.sec`. They do not execute Sec source.
The static harness, native provider and CLI execution wiring remain separate
implementation work in `governance/testing.yaml`.
