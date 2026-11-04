# BUG-000023 — Polymorphic generic recursion makes `sec check` hang

## Status

Fixed, 2026-11-04.

## Report

A generic function that calls itself with a strictly growing type argument
never finishes semantic analysis. `sec check` neither reports a diagnostic nor
terminates. The governance note on `compiler.monomorphization` points to
unbounded recursion in `typeDisplayName` during template analysis, before
Semantic IR is reached.

The program is valid at template level. The specialization chain
`Grow[int]`, `Grow[Box[int]]`, `Grow[Box[Box[int]]]`, … has no finite closure,
so a concrete compilation must report proven non-convergence or a compiler
resource limit (monomorphization.md termination model). An endless analysis
violates that, and on this machine it also consumes unbounded memory.

## Reproduction

```sec
module main

type Box[T] struct {
    Value: T,
}

fn Grow[T](value: T, n: int) int {
    if n <= 0 {
        return 0
    }
    return Grow(Box[T] { Value: value }, n - 1)
}

fn main() int {
    return Grow(1, 3)
}
```

```sh
timeout 60 bin/sec check grow.sec   # killed by timeout (exit 143), no output
```

Always run the reproduction under `timeout`, because the analysis keeps
allocating while it recurses.

## Expected and actual behavior

- **Expected:** `sec check` terminates. The template itself type-checks, and
  demanding the concrete chain from `main` reports a canonical generic
  instantiation diagnostic for non-converging specialization (distinct
  instantiation keys that grow without bound). The diagnostic carries the
  instantiation trace and never presents a resource threshold as proof of
  non-convergence.
- **Actual:** no diagnostic and no termination. Reproduced 2026-11-04 with the
  rebuilt `bin/sec`, and first recorded 2026-10-31 against HEAD 1eba0fd.

## Scope

- Area: generic template analysis and specialization demand (Sema)
- Rulebooks and corrections: `rules/declarations/generics.md`;
  `rules/compiler/monomorphization.md` (instantiation graph termination,
  non-convergence versus resource limits)
- Governance integration: `compiler.monomorphization` (remaining item
  "terminate polymorphic recursion in Sema"; also items on canonical
  InstantiationKey non-convergence detection and resource thresholds) in
  `governance/generics.yaml`
- Affected phase: semantic analysis (before Semantic IR)

## Resolution

Cause: Sema records a call inside a generic template that forwards template
parameters as a forwarded instantiation. `linkGenericSpecializations` (and the
other passes that follow forwarded chains) composed `T = Box[T]` with the
current substitution and linked the new specialization again and again,
spending time in `typeDisplayName` on ever deeper types.

Fix: `recordGenericInstantiation` now checks a forwarded call to the same
template. If some parameter `P` is replaced by a type that properly contains
`P`, each specialization demands a strictly larger canonical key, which is
proven non-convergence (monomorphization.md § 18.5). Sema reports the new
registered diagnostic S1198 `generics.non-converging-specialization` at the
call and does not record the instantiation, so no pass follows the chain.
Unchanged or permuted arguments and finite mutual recursion are not affected.
General detection over canonical instantiation keys, such as growing mutual
recursion, and a documented resource limit remain separate governance items.

Regression: `TestGenericSelfRecursionWithGrowingArgumentIsRejected` in
`internal/sema/generic_nonconvergence_test.go` expects exactly one S1198 for
`Grow` and accepts `Same`, `Swap` and `First`/`Second` in the same program.
The reproduction now ends at once with exit 3 and the S1198 diagnostic.
`go test ./internal/sema ./internal/diagnostics ./internal/ir/semantic
./internal/lowering/secmlir ./cmd/compiler` passes.

