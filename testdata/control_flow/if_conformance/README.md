# If-statement conformance matrix

Owner: `frontend.if-statements-v2` in `governance/control_flow.yaml`.
Rules: `rules/control-flow/flowcontrol_if.md` revision 2.0, with MD-006 state
tests and the MD-024 explicit unsafe raw-null contract.

This matrix connects the owning checklist to executable regressions. Fixtures
ending in `_invalid.sec` must be rejected. The native fixture exits with the
first failed expectation's number, or zero when every check passes.

| Contract | Regression evidence |
| --- | --- |
| Boolean conditions and mandatory branch braces | Parser `TestParseIfElseIfElseStatement`, `TestParseInvalidIfFormsReportOneError`; native `TestIfConformanceNative` |
| No numeric/string/collection/enum/struct/Option/Result/reference/raw-pointer truthiness | `truthiness_invalid.sec`: `TestIfConformanceConditionsAndAssignment`, `TestIfConformanceTruthinessCLI`, `TestIfConformanceTruthinessLSP` |
| Left-to-right `&&`/`||`, skipped RHS and later else-if conditions, exactly one selected body | `native.sec`: all eight input combinations for the chain and four grouped/negated cases in `TestIfConformanceNative`; Sema `TestLogicalShortCircuitOwnershipAcrossContexts` and `TestFoldedLogicalShortCircuitDoesNotCommitRuntimeStateOrEffects`; IR `TestLogicalOperatorsBuildShortCircuitFlow` |
| Ordinary comparison, equality and range membership | Parser `TestParseIfRangeMembershipCondition`; IR `TestRangeMembershipBuildsBoundComparisons`, `TestBooleanEqualityBuildsEagerTypedFlow`; native trace comparisons |
| Try success must be bool; true/false success, error propagation, local recovery and negation | Sema `TestIfTryConditionsRequireBooleanSuccess`; IR `TestIfTryBooleanConditionsLowerOnce`; CLI `TestIfTryConditionDiagnosticsCLI`, `TestIfTryConditionsRunNatively`; LSP `TestIfTryConditionsLSP` |
| No assignment/declaration headers or structural payload binding | `headers_invalid.sec`: parser `TestIfConformanceHeaders` and `TestConditionPatternBindingReportsFocusedDiagnostics` |
| No comparison chains | `comparison_invalid.sec`: Sema `TestIfConformanceConditionsAndAssignment`; parser retains the expression for semantic rejection |
| Ordinary non-binding state tests and the sole positive Option binding form | Parser `TestConditionStateTestsParse`; Sema `TestUnionStateTestsTypeAndRefineAssignment`; IR `TestOptionIfBindingEvaluatesAndProjectsOnce`; native `TestNativeTrackBuildsAndRuns/state_tests`; formatter `TestFormatOptionIfBinding` |
| Raw null remains explicit-unsafe-only | `TestIfRawNullUnsafeBoundary`, `TestIfRawNullUnsafeBoundaryCLI`, `TestIfRawNullUnsafeBoundaryLSP` |
| Every continuing path assigns; terminated paths do not require assignment | `assignment.sec` and `assignment_invalid.sec`: `TestIfConformanceConditionsAndAssignment`. Uses a possibly-empty union, so scalar default initialization cannot hide a missing assignment |
| Branch locals do not leak | `assignment_invalid.sec`: `TestIfConformanceConditionsAndAssignment`; existing `TestIfBranchLocalDoesNotLeakToFollowingCall` |
| Return validation, implicit no-branch path and continuation | Sema `TestResolvedIfFlowRetainsPathExecutionAndTermination`, `TestIfBranchesCanSatisfyFunctionReturn`; IR `TestIfWithoutElseJoinsOnce`; native `Join(true)` and `Join(false)` |
| Proven-unreachable diagnostics; mutable-derived values remain runtime conditions | `TestConstantIfBranchesReportCanonicalUnreachableStatement`, `TestFoldedBooleanConditionsReportCanonicalUnreachableStatement`, `TestConditionsOverValuesDerivedFromMutableBindingsStayRuntime` |
| Canonical else-if layout, required grouping and opt-in outer-parenthesis correction | `TestIfConformanceFormatting` at widths 2/4/6 with LF/CRLF input; `TestControlConditionCorrectionNeedsOneEnclosingPair`; repeated formatting must be identical |
| Registered bool-condition identity, help and source ranges through Sema, CLI and unsaved LSP snapshots | `S1246 control-flow.if-condition-not-bool` in the conformance and try tests; invalid conditions block check/sema/emit-ir/emit-mlir/emit-llvm with no emitted artifact |

The native tests use the one maintained compilation track and require the
configured `sec-mlir-opt` and `clang`. They do not substitute an older backend.

## Type-owned lowering boundaries

This matrix verifies the `if` contract, not native representation of every
possible condition operand or branch value. For example,
`TestIfTryConditionsUnsupportedResultBoundary` requires a named
`[unsupported-lowering]` for an unrepresented string Result payload and
preserves existing output. Non-binding empty-state/ownership-sensitive state
tests retain their named boundaries. Their representation work remains with
the owning union/Option/Result, string, ownership and lowering integrations;
the regression matrix does not waive or complete that work.

Focused reproduction from the repository root:

```sh
SEC_MLIR_BIN="$PWD/build/sec-mlir/bin" go test ./internal/parser ./internal/sema ./internal/ir/semantic ./internal/formatter ./internal/diagnostics ./cmd/compiler ./cmd/lsp -run 'If|ConditionPatternBinding|ConditionStateTests|ComparisonChaining|Logical|FoldedBoolean|UnionState|ControlCondition|KnownDiagnostic|RegistryIsValid' -count=1 -timeout=180s
SEC_MLIR_BIN="$PWD/build/sec-mlir/bin" go test ./cmd/compiler -run '^TestNativeTrackBuildsAndRuns$/^state_tests$' -count=1 -timeout=120s
go test ./internal/sema ./internal/ir/semantic -run 'RangeMembershipBuildsBoundComparisons|BooleanEqualityBuildsEagerTypedFlow|ConditionsOverValuesDerivedFromMutableBindingsStayRuntime' -count=1 -timeout=120s
```
