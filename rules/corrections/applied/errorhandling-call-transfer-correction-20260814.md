# Correction: functions and error-handling ownership

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** error-handling rulebook

When a function call contains fallible argument evaluation, call ownership transfer is not partially committed before call entry.

Example:

```sec
try Use(
    resource,
    LoadConfiguration(),
) {
    Err(error) => {
        Handle(error)
    }
}
```

Arguments evaluate left-to-right.

If `LoadConfiguration()` fails before `Use` is entered:

- the outer call does not consume an earlier owned caller binding merely because it was an earlier direct argument;
- earlier temporary values created solely for the outer call are cleaned by the caller;
- bodyless/local `try` handling proceeds with ownership state corresponding to an outer call that never entered the callee.

Effects and ownership transfers performed while evaluating an earlier argument expression itself are not rolled back.

This rule applies to ordinary move-only by-value parameters and explicit `->` consuming parameters.
