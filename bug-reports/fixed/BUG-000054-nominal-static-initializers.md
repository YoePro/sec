# BUG-000054 — Nominal static initializers rejected by the dialect verifier

## Status

Fixed 2026-11-27.

## Report and reproduction

Claude reported that `TestStaticDefaultsEmitGlobals` failed with configured
MLIR tools. Reproduced before the fix with:

```sh
SEC_MLIR_BIN="$PWD/build/sec-mlir/bin" go test ./internal/lowering/secmlir -run '^TestStaticDefaultsEmitGlobals$' -count=1
```

The source fixture `testdata/semantic_ir/static_defaults/values.sec` declares
`Positive` as a named integer constrained to `2..9`. Its static default is
correctly resolved to `2`. The emitted `sec.static.global` retains
`!sec.named<"main::Positive", !sec.int>`, but the dialect verifier rejected it
with `static storage supports builtin integer and bool types`. The same
problem affected named leaves inside aggregate initializers.

## Expected behavior and scope

Validate the initializer against the selected base representation while
retaining nominal identity in the global and its uses. This follows
`rules/compiler/semantic_ir.md` §§7(4)-(5),28 and
`rules/declarations/static.md` §§15-16.

Owner: `frontend.static-declarations-members`. Affected phase: Sec MLIR
operation verification. This does not complete general static-storage,
generic-specialization or native representation requirements.

## Resolution

Both root and recursive component verification reuse the existing identity
unwrapper for validation only. They do not rewrite the global's declared type.
Source-backed 32/64-bit emission passes the existing place, reference, borrow
and scalar-lowering verifier chain. The dialect regression covers named and
distinct scalars, chained identities and named array elements, and rejects
malformed integer/bool values and oversized float bits. The source regression
also checks that the nominal wrapper remains in emitted MLIR.
