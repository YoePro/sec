# Correction: borrowed closure captures

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** borrowing rulebook

Closures may explicitly capture borrows.

Shared:

```sec
capture(ref value) fn() int {
    return value
}
```

Mutable/exclusive:

```sec
capture(ref mut value) fn() void {
    value += 1
}
```

The closure may not outlive the borrowed value.

A mutable borrowed capture reserves exclusive authority for the lifetime of the closure's capture.

The outer owner may not perform conflicting access while that borrow remains live.

A closure containing `ref mut` capture is move-only and has at least mutable callable capability.

No new explicit lifetime syntax is introduced; normal borrow/lifetime analysis proves validity.
