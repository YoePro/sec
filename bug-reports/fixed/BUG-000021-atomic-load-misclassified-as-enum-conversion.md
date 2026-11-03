# BUG-000021 — Atomic Load hover misclassified as enum conversion

## Status

Fixed, 2026-11-03.

## Report and reproduction

Claude reported `TestAtomicOrderMatrixLSP` failing after enum-conversion hover
was introduced. Reproduced with:

```sh
go test ./cmd/lsp -run '^TestAtomicOrderMatrixLSP$' -count=1
```

Hovering `value.Load(MemoryOrder.Relaxed)` displayed `MemoryOrder → int` and
an infallible-conversion explanation instead of the atomic Load signature and
its valid explicit memory orders.

## Cause and fix

The presentation query inferred a conversion from an enum operand and integer
result when no ordinary selected-call fact existed. Compiler-known atomic
methods use their own operation facts, so absence of an ordinary call target
was not evidence of a cast.

Sema now retains accepted explicit enum-conversion call identities at the
actual conversion producer. `EnumConversionInfoOf` requires that identity before
projecting the existing resolved source/result types and exactness proof.
The identity set resets with the other per-analysis facts. Intrinsic call
shapes and callee spellings do not establish conversion identity. No conversion
acceptance, numeric proof or atomic semantics changed.

## Ownership and validation

- Owner: `frontend.enums`, `governance/declarations.yaml`.
- Rules: `rules/declarations/enums.md` §12;
  `rules/concurrency/atomics.md` §§24,43;
  `rules/tooling/lsp.md`, Hover and One semantic source of truth.
- `TestEnumConversionToolingRejectsIntrinsicCalls` rejects all calls in the
  atomic-order fixture as enum casts, including seven Load
  calls with explicit orders, bindings and the default order.
- `TestAtomicOrderMatrixLSP` again verifies Load, Store and CompareExchange
  hover documentation, and the existing positive enum-conversion tooling
  fixtures retain the twelve accepted conversions.

Verified on 2026-11-03: full Sema (16.905s), targeted enum/atomic regression
suites with the race detector (Sema 1.327s, LSP 20.233s), and vet.
