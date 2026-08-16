# Defer

- **Status:** Normative
- **Created:** 2026-08-16
- **Last updated:** 2026-08-16
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/control-flow/defer.md`
- **Replaces:** `rules/control-flow/defer.txt`
- **Repository baseline reviewed:** `56be75d`

## 1. Purpose

`defer` registers cleanup work that runs when the nearest enclosing executable invocation boundary exits.

It is intended for deterministic cleanup that must run on every ordinary exit path from that invocation.

`defer` does not replace ownership, automatic destruction, `free`, or explicit error handling.

It composes with them.

## 2. Syntax

Sec 0.1 uses block-form defer:

```sec
defer {
    statements
}
```

Shorthand forms such as:

```sec
defer Close()
```

are not part of Sec 0.1.

A deferred block is registered when execution reaches the `defer` statement.

Merely parsing or entering the surrounding lexical scope does not register it.

## 3. Invocation scope

A defer belongs to the nearest enclosing executable invocation boundary.

In Sec 0.1, defer is permitted inside:

- ordinary functions;
- instance methods;
- static functions/methods;
- lambdas;
- property getters;
- property setters;
- `init`.

A defer inside a lambda belongs to that lambda invocation, not to the function that created the lambda.

Example:

```sec
fn MakeWorker() fn() void {
    return fn() void {
        defer {
            Log("worker finished")
        }

        Work()
    }
}
```

The deferred block runs when the lambda invocation exits.

## 4. `free`

`defer` is not permitted inside `free` in Sec 0.1.

Invalid:

```sec
free {
    defer {
        ReleaseSecondaryResource()
    }
}
```

`free` is already lifecycle cleanup.

Locals created inside `free` still follow their ordinary automatic destruction rules.

## 5. Not block-scoped

A defer does not run merely because execution leaves the lexical block in which it was registered.

Example:

```sec
fn Example(condition: bool) void {
    if condition {
        defer {
            Log("leaving Example")
        }

        Work()
    }

    ContinueWork()
}
```

When the defer is registered, it runs when `Example` exits, not when the `if` block ends.

If `condition` is false, the defer is never registered.

## 6. Registration order

Each time execution reaches a `defer` statement, one deferred cleanup entry is registered.

Multiple deferred entries run in reverse registration order.

```sec
defer {
    Log("first")
}

defer {
    Log("second")
}
```

Exit order:

```text
second
first
```

This is deterministic language semantics.

## 7. Unified cleanup order

Deferred cleanup and ordinary automatic destruction participate in one common LIFO cleanup order according to when their cleanup obligations are registered.

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

The relevant registration order is:

```text
destroy first
defer Log
destroy second
```

The exit order is therefore:

```text
destroy second
run defer Log
destroy first
```

This ordering is normative.

`defer` is not a separate "run all defers first" phase.

## 8. Earliest legal destruction still applies

The unified cleanup model does not force every local value to live until function exit.

Ordinary locals are still destroyed at the earliest legal lifetime end according to ownership, borrowing, scope, and analysis rules.

A deferred block may extend the lifetime of a value it uses.

Example:

```sec
fn Example() void {
    if Ready() {
        let resource := Open()

        defer {
            Use(resource)
        }
    }

    ContinueWork()
}
```

Because the registered defer uses `resource`, `resource` must remain valid until that defer executes.

Its destruction therefore occurs after the defer.

A local value not needed by any later cleanup may still be destroyed earlier at its normal lifetime end.

## 9. Values are read when the defer executes

The defer block is not evaluated when it is registered.

Its statements execute later.

Ordinary variable reads observe the value that exists when the deferred block executes, subject to normal ownership and borrowing rules.

Example:

```sec
fn Example() void {
    let mut value: int := 1

    defer {
        Print(value)
    }

    value = 2
}
```

The deferred block observes `2`.

Sec 0.1 does not define a special capture-by-value syntax for defer.

## 10. Ownership and pending deferred uses

Registering a defer creates real future uses of the values referenced by the deferred block.

Ownership and borrowing analysis must account for those uses.

A value cannot be moved, destroyed, or invalidated in a way that would make a registered defer invalid.

Example:

```sec
fn Example() void {
    let resource := Open()

    defer {
        Use(resource)
    }

    Consume(-> resource)
}
```

This is invalid when the move would leave the registered defer without a valid resource.

The diagnostic should identify the pending deferred use.

## 11. Borrowing

Borrows required by a deferred block remain subject to ordinary borrow rules.

A deferred future use may extend the required lifetime of a binding or prevent an incompatible move/mutation.

The compiler must not treat a defer body as an opaque callback that escapes arbitrarily.

Its lifetime is known: the enclosing invocation boundary.

## 12. Defer in conditional control flow

A defer is registered only on paths that execute the statement.

```sec
if connected {
    defer {
        Disconnect()
    }
}
```

If `connected` is false, no defer is registered.

If it is true and execution reaches the defer, the cleanup is registered for the enclosing invocation.

## 13. Defer in loops

A defer inside a loop is registered each time execution reaches the statement.

```sec
for item in items {
    defer {
        Finish(item)
    }
}
```

If the defer statement executes five times, five deferred entries are registered.

They run in reverse registration order when the enclosing invocation exits.

A loop does not create a defer-execution boundary.

Analysis may diagnose or warn about patterns that accumulate an unexpectedly large number of deferred cleanups, according to the configured analysis policy.

## 14. Ordinary return

On an ordinary `return`:

1. the return expression is evaluated;
2. the function's return result is established according to ordinary ownership/move rules;
3. required cleanup runs in the canonical cleanup order;
4. control returns to the caller.

A deferred block therefore cannot retroactively replace the already-established return expression.

Example:

```sec
fn Example() int {
    let mut value: int := 1

    defer {
        value = 2
    }

    return value
}
```

The return expression is evaluated before cleanup begins.

The function returns `1`.

The later deferred mutation does not rewrite the already-established return value.

## 15. Error propagation and fallible exits

When an enclosing callable propagates an error, the error result is established before deferred cleanup begins.

Deferred cleanup then runs according to the canonical cleanup order.

A defer may not replace the propagated error with a different propagated error.

A fallible operation inside a defer must be handled within the deferred block according to the ordinary Sec error-handling rules.

A `try` inside a defer may not propagate out of the deferred block into the already-exiting enclosing invocation.

Example:

```sec
defer {
    match CloseResource() {
        Ok => {
        }

        Err(error) => {
            LogCloseError(error)
        }
    }
}
```

The exact handling form may use any ordinary Sec construct valid for the operation.

`defer` does not introduce a separate mandatory `discard` keyword requirement.

The normal unused-result and must-use rules apply.

## 16. `return` is invalid inside defer

A deferred block may not contain a `return` that targets the enclosing invocation.

Invalid:

```sec
defer {
    return
}
```

A deferred block performs cleanup; it does not replace or restart enclosing return control flow.

A function or lambda called from a deferred block may of course return normally from its own invocation.

## 17. Loop and switch control transfer is invalid inside defer

A deferred block may not use:

```text
break
continue
fallthrough
```

to target control flow outside the deferred block.

Invalid:

```sec
defer {
    break
}
```

Invalid:

```sec
defer {
    continue
}
```

Invalid:

```sec
defer {
    fallthrough
}
```

Deferred cleanup cannot resume, redirect, or skip the enclosing function's already-established exit path.

## 18. Nested defer

A deferred block may not register another defer in Sec 0.1.

Invalid:

```sec
defer {
    defer {
        CleanupAgain()
    }
}
```

The cleanup schedule for the enclosing invocation is fixed by defer statements reached during ordinary execution, not by new defer registration while cleanup is already running.

## 19. Normal completion of a deferred block

A deferred block completes by reaching the end of its block.

After it completes, cleanup continues with the next pending cleanup entry.

The block does not produce a value.

## 20. Panic and abnormal termination

`defer` does not independently define panic or unwind semantics.

Whether deferred cleanup runs during a panic or other abnormal termination is determined by the canonical panic, destruction, and unwind rules.

When those rules require cleanup to run, deferred entries participate in the same canonical cleanup order.

No special unwind semantics are introduced by this rulebook.

## 21. `init`

`defer` is permitted inside `init`.

This is useful for temporary construction resources.

Example:

```sec
init(path: string) IOError {
    let temporary := try OpenTemporary(path)

    defer {
        CloseTemporary(temporary)
    }

    BuildState(temporary)
}
```

The defer belongs to the `init` invocation.

Ownership analysis must reject any later move/transfer that conflicts with the pending deferred use.

If ownership is intentionally transferred into the constructed object, the program must not leave a conflicting deferred cleanup registered against the transferred resource.

## 22. Property accessors

A property getter or setter may register defer cleanup.

The defer belongs to that accessor invocation.

Example:

```sec
property Value int {
    get {
        defer {
            FinishRead()
        }

        return ReadValue()
    }
}
```

Normal accessor return, error, ownership, and cleanup rules apply.

## 23. Lambdas

A lambda may register defer cleanup.

```sec
let work := fn() void {
    defer {
        FinishWork()
    }

    RunWork()
}
```

The defer runs when that lambda invocation exits.

Calling the lambda multiple times creates independent defer registration state for each invocation.

## 24. Evaluation and side effects

Statements inside a deferred block execute when cleanup reaches that deferred entry.

Side effects therefore occur at cleanup time, not registration time.

The language does not reorder defer registration across observable source operations.

The compiler may optimize the implementation only when observable cleanup order remains unchanged.

## 25. Interaction with automatic destruction

A deferred cleanup may call explicit foreign/resource cleanup functions.

If an owning Sec wrapper also has automatic destruction, the program must not cause the same resource to be released twice.

Safe wrapper APIs and ownership analysis should normally make double release impossible.

When explicit deferred cleanup consumes or invalidates an owning value, the subsequent automatic destruction obligation must be resolved by ordinary move/lifecycle semantics rather than by silently performing a second release.

## 26. Definite assignment

Values read by a deferred block must be definitely initialized on every path that reaches and registers that defer.

Invalid:

```sec
let value: int

defer {
    Print(value)
}
```

when `value` is not definitely assigned before the defer is registered.

A later assignment does not make registration valid when the deferred future use cannot be proven safe under the ordinary definite-assignment rules.

## 27. Scope and name resolution

Names inside a defer body follow ordinary lexical name resolution.

The defer may refer to bindings visible at the declaration site.

Bindings declared later are not visible merely because the deferred block executes later.

Invalid:

```sec
defer {
    Use(resource)
}

let resource := Open()
```

Execution time does not change lexical scope.

## 28. Parser requirements

The parser must:

- parse block-form `defer`;
- require a block body;
- preserve source locations for the defer keyword and body;
- represent defer as a dedicated control-flow/cleanup statement;
- allow defer only in syntactic contexts that can later be validated against an enclosing executable invocation;
- parse nested statements normally so Sema can diagnose forbidden control transfer or nested defer.

The parser does not determine final lifetime or cleanup order.

## 29. Sema and flow-analysis requirements

Sema/analysis must:

- require an allowed enclosing invocation boundary;
- reject defer inside `free`;
- register each reached defer semantically as invocation-scoped cleanup;
- preserve reverse registration order;
- integrate defer with ordinary destruction in the canonical cleanup order;
- model future reads, writes, borrows, moves, and resource uses from the defer body;
- extend lifetimes where required by pending deferred uses;
- reject moves or invalidation that conflict with registered defer uses;
- validate definite assignment at defer registration;
- reject `return` targeting the enclosing invocation from a defer body;
- reject `break`, `continue`, and `fallthrough` escaping a defer body;
- reject nested defer;
- reject error propagation out of a defer body;
- preserve the already-established return/error result while cleanup executes;
- provide cleanup edges for every ordinary exit path.

## 30. Semantic IR requirements

Semantic IR must represent defer as explicit cleanup semantics rather than lowering it immediately to an arbitrary source-level callback.

The IR must preserve enough information to determine:

- registration point;
- enclosing invocation boundary;
- source-order cleanup registration;
- dependencies on local values;
- cleanup ordering relative to automatic destruction;
- ordinary return/error cleanup edges;
- ownership and borrow effects;
- source location for diagnostics.

Lowering may build explicit cleanup blocks or an equivalent representation.

The chosen representation must preserve normative execution order.

## 31. Required diagnostics

Diagnostics should be specific.

Examples:

```text
defer is not allowed inside free
```

```text
return is not allowed inside a deferred cleanup block
```

```text
break cannot target a loop from inside a deferred cleanup block
```

```text
continue cannot target a loop from inside a deferred cleanup block
```

```text
fallthrough is not allowed inside a deferred cleanup block
```

```text
nested defer is not part of Sec 0.1
```

```text
cannot move resource; it is used by a registered defer
```

```text
deferred use of value requires it to remain valid until function exit
```

```text
value is not definitely initialized when defer is registered
```

```text
error propagation cannot leave a deferred cleanup block
```

Diagnostics should point both to the conflicting operation and to the defer registration/use when practical.

## 32. Best practice

Use `defer` for cleanup that naturally belongs to the lifetime of the current invocation.

Prefer automatic ownership/destruction when a resource already has a correct owning Sec type.

Use defer when cleanup requires:

- a specific operation rather than ordinary destruction;
- coordination with a foreign API;
- temporary construction cleanup;
- an explicit side effect at invocation exit.

Keep deferred blocks short and cleanup-focused.

Avoid accumulating large numbers of defers in long-running loops when immediate per-iteration cleanup is more appropriate.

Do not use defer to simulate general control flow.

## 33. Cross-rulebook ownership

This rulebook owns:

- defer syntax;
- defer registration timing;
- invocation-scoped execution;
- LIFO order among deferred entries;
- integration of defer into the unified cleanup order;
- forbidden control transfer from deferred blocks;
- allowed executable contexts;
- defer prohibition inside `free`.

Other rulebooks own:

- ordinary ownership and move semantics;
- borrowing and lifetime analysis;
- automatic destruction and `free`;
- return-value ownership transfer;
- `Result`, `try`, and error propagation;
- panic/unwind semantics;
- loop `break` and `continue`;
- switch `fallthrough`;
- unused-result/must-use/discard policy;
- target/backend cleanup lowering details.
