# Correction: closure escape analysis

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** escape-analysis rulebook

Escape analysis must classify closures and environments.

At minimum distinguish:

```text
non-capturing callable
non-escaping owned closure
escaping owned closure
shared-borrow closure
mutable-borrow closure
consuming closure
```

Owned escaping closures are valid Sec 0.1 values when their environment can be given sufficient lifetime.

Borrowed closures may escape only while the borrow remains valid.

Storage selection is compiler-internal and may use:

- elimination/SSA;
- stack or lexical region;
- longer-lived region;
- static storage where valid;
- owned dynamic storage where required.

The analysis must reject any closure whose selected lifetime would outlive a borrowed capture.

The language does not guarantee heap allocation for escaping closures.
