# BUG-000034 — Module-internal helpers unreachable from a trusted-core test file

- **Status:** open (cause not isolated)
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

The likely cause is that the trusted-core loader gives test files a module
identity different from the loader-proven core sources.

## Workaround

`sec/core/types_test.sec` uses public-spelled test-only helpers
(`typesTestDecodes`, `typesTestInvalidUtf8At`).
