# BUG-000034 — Module-internal helpers unreachable from a trusted-core test file

- **Status:** fixed (2026-11-22)
- **Reported:** 2026-11-20 (found while writing `sec/core/types_test.sec`)
- **Area:** Sema / source loading — visibility of `_name` declarations in
  `*_test.sec` files of the trusted core module
- **Rulebooks:** `rules/projects/modules.md` § 6 (`_name` is module-internal);
  `rules/tooling/testing.md` § 4 (a `*_test.sec` file participates in the
  same module)

## Observation

`sec check sec/core` with this in `sec/core/types_test.sec`:

```sec
module core

fn _expectDecoded(result: Result[Utf8DecodedRune, StringError], scalar: uint, width: uint) bool {
    ...
}

test "..." {
    testing.Expect(_expectDecoded(DecodeUtf8(ref bytes, 0), 0x41, 1))
}
```

reported `function _expectDecoded is not accessible from module core` at
each call, although caller and declaration are in the same file and module.

## Contrast

- The same pattern in an ordinary module (`module demo`, helper and test in
  `demo_test.sec`) checks cleanly, also when that module is named `core`
  outside the trusted core path.
- A copy of `sec/core` outside the repository is not loaded as trusted core
  and reports hundreds of unrelated `_UnicodeHasProperty is not accessible`
  errors, so it cannot isolate the case.

The initial suspicion was a loader module-identity mismatch. The isolated
reproduction instead identified a missing Sema statement-token projection;
see the resolution below.

## Workaround

`sec/core/types_test.sec` uses public-spelled test-only helpers
(`typesTestDecodes`, `typesTestInvalidUtf8At`).

## Resolution (2026-11-22)

`statementToken` did not recognize `ast.TestDeclaration`. The common semantic
module visitor consequently cleared `currentSourceFile` for test invocations.
The test still had the correct module name, but no source file with which to
prove core authority or same-file private visibility. The same omission also
rejected ordinary same-file `__` helpers, beyond the original core observation.

The helper now projects the test declaration token, retaining all existing
visibility checks. It was extracted into `internal/sema/statement_tokens.go`
from the oversized analyzer, with explicit rulebook references. Its old
traceability exemption was removed through the traceability updater.

Regression fixtures live in `testdata/core/test_visibility/`. Before the fix,
`TestTestSourceVisibility` rejected all four legitimate core helper calls;
after the fix, all three analysis depths and both 32/64-bit plans pass. The
negative cases continue to reject cross-file `__` access and untrusted callers
spelling their module `core`. The CLI regression uses an isolated real core
loader tree and verifies listing, unavailable execution and private access
rejection. The LSP regression verifies diagnostics and source navigation for
same-file helpers in an unsaved test buffer.

Tests: `internal/sema/test_source_visibility_test.go`,
`cmd/compiler/test_source_visibility_test.go`,
`cmd/lsp/test_source_visibility_test.go`.

This resolves frontend access; it does not implement test execution. The CLI
continues to distinguish listed tests from execution-unavailable tests until
the static harness and test-body lowering are implemented. The public-spelled
helpers in the original library test may remain; changing their names is no
longer necessary to avoid this defect.
