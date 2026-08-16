# Destruction Correction: Defer Cleanup Ordering

- **Status:** Applied normative correction
- **Created:** 2026-08-16
- **Applied:** 2026-08-16
- **Last updated:** 2026-08-16
- **Sec language version:** 0.1
- **Source:** `rules/control-flow/defer.md` revision 2.0
- **Target:** `rules/memory/destruction.txt`
- **Repository baseline:** `56be75d`

## Correction

Replace any unresolved or alternative ordering between deferred cleanup and automatic destruction with the following normative rule:

> Deferred cleanup and ordinary automatic destruction participate in one common LIFO cleanup order according to when their cleanup obligations are registered.

Example:

```sec
fn Example() void {
    let first := OpenFirst()

    defer {
        Log("leaving")
    }

    let second := OpenSecond()
}
```

Registration order:

```text
destroy first
defer Log
destroy second
```

Exit order:

```text
destroy second
run defer Log
destroy first
```

This does not require every local to live until invocation exit.

Ordinary destruction still occurs at the earliest legal lifetime end.

A registered defer may extend the lifetime of a value it uses until the deferred cleanup executes.

The destruction/ownership analysis must preserve this ordering on all ordinary cleanup edges.

## Applied changes

The canonical destruction rule now uses one normative reverse-registration
cleanup order for returns, propagated errors, deferred operations, and automatic
destruction. The obsolete alternative ordering was removed, and lifetime
extension is explicitly limited to values used by registered deferred cleanup.
