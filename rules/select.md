# Select

## Purpose

`select` waits on multiple concurrent operations and executes exactly one branch.

It is the concurrency equivalent of choosing among operations that may become
ready at different times.

`select` is not a replacement for:

- `if`
- `switch`
- `match`

The distinction is:

```text
if
    Selects from boolean control flow.

switch
    Selects from values or conditions.

match
    Selects from patterns and variants.

select
    Selects from concurrent operations that become ready.
```

---

## Core syntax

```sec
select {
    value := operationA => {
        HandleA(value)
    }

    value := operationB => {
        HandleB(value)
    }
}
```

`select` waits until at least one branch can commit.

Exactly one branch commits during one execution of the `select` statement.

After the selected branch completes, control continues after the `select`.

---

## Source-order priority

Branches are evaluated for selection in source order.

When more than one branch is ready at the same time, the first ready branch in
source order is selected.

Example:

```sec
select {
    critical := criticalRx.Receive() => {
        HandleCritical(critical)
    }

    normal := normalRx.Receive() => {
        HandleNormal(normal)
    }
}
```

If both channels have a deliverable message, the `critical` branch is selected.

Source order is therefore an explicit priority mechanism.

The compiler and runtime must not introduce hidden fairness that overrides source
order.

---

## Starvation

A later branch may starve when an earlier branch remains continuously ready.

Example:

```sec
while true {
    select {
        critical := criticalRx.Receive() => {
            HandleCritical(critical)
        }

        normal := normalRx.Receive() => {
            HandleNormal(normal)
        }
    }
}
```

If `criticalRx` is always ready, the normal branch may never execute.

This behavior is intentional and visible in source order.

Programs requiring independent progress should:

- reorder branches;
- use separate tasks;
- use a dispatcher;
- use another channel arrangement;
- explicitly define another scheduling policy in a future abstraction.

Version 0.1 does not provide implicit round-robin branch selection.

---

## Readiness and commit

Every selectable operation has two conceptual phases:

```text
readiness
commit
```

Readiness determines whether the operation could proceed.

Commit performs the operation and transfers any ownership.

Only the selected branch commits.

Non-selected branches must not change observable state.

---

## Exactly one branch

A `select` execution chooses at most one branch.

If several branches are ready:

1. the first ready branch in source order is selected;
2. only that branch commits;
3. its body executes;
4. the `select` ends;
5. control continues after the statement.

Other ready branches remain uncommitted.

They may be selected by a later `select`.

---

## Channel receive

A channel receive may be used as a branch:

```sec
select {
    message := rx.Receive() => {
        Process(message)
    }
}
```

The branch is ready when the receive operation can complete.

This includes:

- a deliverable message is available;
- the send side is closed and the channel is drained.

The bound value has the normal return type of the receive operation.

For:

```sec
Receiver[T].Receive() -> Option[T]
```

the branch may receive:

```sec
Some(message)
```

or:

```sec
None
```

when the channel is permanently closed and drained.

---

## Non-selected receive

A non-selected receive branch must not:

- remove a message;
- advance the queue head;
- reclaim a tombstone as part of committed receive;
- consume `Receiver[T]`;
- alter channel state.

The channel may perform safe internal readiness inspection.

Observable receive effects occur only during commit.

---

## Channel send

A channel send may be used as a branch:

```sec
select {
    tx.Send(message) => {
        Sent()
    }
}
```

The branch is ready when the send can commit.

For a buffered channel, this may mean:

- capacity is available;
- the receive side is closed and the operation can return its closed outcome.

For a rendezvous channel, the branch is ready when a receiver can accept the
message or the receive side is closed.

---

## Non-selected send

A non-selected send branch must not:

- transfer ownership of the message;
- enqueue the message;
- perform rendezvous handoff;
- destroy the message;
- create a ticket;
- alter sender or channel state.

The current task retains ownership of every message belonging to a non-selected
send branch.

This rule is essential for move-only values.

---

## Revocable send

A revocable send may participate in select:

```sec
select {
    ticket := tx.SendRevocable(message) => {
        Store(ticket)
    }

    after 20ms => {
        HandleTimeout(message)
    }
}
```

If the send branch is selected:

- the send commits;
- ownership transfers;
- the ticket is created;
- the branch receives the ticket.

If another branch is selected:

- the message remains owned by the current task;
- no ticket exists.

---

## Await branches

A task await may participate in select:

```sec
select {
    result := await first => {
        HandleFirst(result)
    }

    result := await second => {
        HandleSecond(result)
    }
}
```

A branch is ready when the task outcome can be resolved without further waiting.

Only the selected await consumes its task handle.

Non-selected await branches must not:

- consume the task;
- take its result;
- alter task ownership;
- cancel the task;
- detach the task.

---

## Join branches

A join operation may participate in select when join is a selectable operation.

```sec
select {
    join first => {
        HandleFirstCompletion()
    }

    join second => {
        HandleSecondCompletion()
    }
}
```

Only the selected join commits its completion synchronization.

The non-selected task handle remains unchanged.

---

## Process and I/O readiness

The `select` model may later support other readiness-based operations, including:

- process completion;
- IPC receive;
- socket receive;
- listener accept;
- timer completion;
- device-event readiness;
- future channel types.

An operation may participate only when semantic analysis recognizes it as
selectable.

Ordinary function calls are not selectable merely because they may block.

---

## Timeout branch

A timeout branch uses:

```sec
after duration => {
    HandleTimeout()
}
```

Example:

```sec
select {
    message := rx.Receive() => {
        Process(message)
    }

    after 100ms => {
        HandleTimeout()
    }
}
```

The timeout begins when execution enters the `select`.

The branch becomes ready after the duration has elapsed.

If another earlier branch is ready at the same selection point, source-order
priority still applies.

---

## Timeout duration

The `after` expression must have a valid duration type.

Example:

```sec
after 100ms
```

The exact duration literal rules are defined by the units and time rules.

`after` uses a monotonic time source.

Wall-clock adjustments must not cause the timeout to run backward or repeat.

A target without a valid timer source must reject timeout branches.

---

## Zero duration

A zero-duration timeout is immediately ready:

```sec
after 0ms => {
    Continue()
}
```

It behaves as an explicitly ordered immediate branch.

Earlier ready branches still win.

---

## Default branch

A `default` branch is optional.

```sec
select {
    message := rx.Receive() => {
        Process(message)
    }

    default => {
        DoOtherWork()
    }
}
```

`default` executes only when no preceding selectable branch is ready.

If `default` exists, the `select` does not wait.

---

## Default placement

`default` must be the final branch.

Invalid:

```sec
select {
    default => {
    }

    message := rx.Receive() => {
    }
}
```

Expected diagnostic:

```text
default branch must be last in select
```

Only one `default` branch is permitted.

---

## Default and timeout

A `select` may not contain both an immediately available `default` branch and a
positive timeout branch when the timeout can never be selected.

Example:

```sec
select {
    message := rx.Receive() => {
    }

    after 100ms => {
    }

    default => {
    }
}
```

The `default` branch always wins when no earlier branch is ready, so the timeout
branch is unreachable.

The compiler should report this as statically unreachable concurrent control
flow.

A zero-duration `after` branch and `default` must likewise follow source-order
reachability rules.

---

## Cancellation

A waiting `select` is a cancellation point.

If cancellation is requested while no branch has committed, the current task may
exit through normal cancellation control flow.

No branch commits merely because cancellation occurred.

Non-selected values remain owned by the current task and are destroyed through
normal cancellation cleanup.

---

## Explicit cancellation branch

A program may include an explicit cancellation-related branch only through a
recognized selectable cancellation operation.

Conceptually:

```sec
select {
    message := rx.Receive() => {
        Process(message)
    }

    task.cancelRequested => {
        cancel
    }
}
```

The exact readiness semantics of `task.cancelRequested` must be compiler-defined.

An explicit branch is useful when local cleanup or alternate control flow is
required.

Without such a branch, cancellation still affects the waiting `select` through
normal task cancellation semantics.

---

## Branch binding

A branch may bind the selected operation result:

```sec
select {
    message := rx.Receive() => {
        Use(message)
    }
}
```

The bound value exists only inside the branch body.

Its type is the ordinary result type of the selected operation.

Branch bindings follow ordinary:

- mutability;
- move;
- destructuring;
- pattern;
- shadowing;
- visibility rules.

---

## Branch without binding

An operation returning `void` or an ignored result may omit binding:

```sec
select {
    tx.Send(message) => {
        Sent()
    }

    after 20ms => {
        TimedOut()
    }
}
```

An operation result that must not be ignored remains subject to normal unused
result and explicit discard rules.

---

## Scope

Each branch body has its own lexical scope.

Example:

```sec
select {
    message := rx.Receive() => {
        let decoded := Decode(message)
        Process(decoded)
    }

    default => {
        let decoded := CachedValue()
        Process(decoded)
    }
}
```

Branch-local declarations do not escape automatically.

Values moved into the selected branch remain moved after the `select`.

Values associated only with non-selected operations remain available after the
`select`, subject to control-flow analysis.

---

## Ownership merge after select

The compiler must merge ownership state across all possible selected branches.

Example:

```sec
let message := CreateMessage()

select {
    tx.Send(message) => {
    }

    default => {
        Use(message)
    }
}
```

After the `select`, ownership depends on the selected branch.

The compiler must reject later use unless all reachable branches leave the value
available.

Example:

```sec
Use(message)
```

is invalid when one branch may have moved it.

The diagnostic should identify the branch that consumes the value.

---

## Definite initialization

A variable assigned in select branches is definitely initialized after the
select only when every reachable branch assigns it.

Example:

```sec
let result: int

select {
    value := first.Receive() => {
        result = value
    }

    value := second.Receive() => {
        result = value
    }
}
```

Without `default`, cancellation or non-local control flow may still affect
definite assignment according to ordinary rules.

The compiler must analyze every reachable branch.

---

## Select in loops

`select` may appear inside loops:

```sec
while true {
    select {
        critical := criticalRx.Receive() => {
            HandleCritical(critical)
        }

        normal := normalRx.Receive() => {
            HandleNormal(normal)
        }

        after 1s => {
            Maintain()
        }
    }
}
```

Every loop iteration performs a new readiness evaluation.

No unselected operation from the previous iteration remains implicitly reserved.

---

## Select outside loops

A select executes once unless ordinary control flow repeats it.

Example:

```sec
select {
    message := rx.Receive() => {
        Process(message)
    }

    after 100ms => {
        HandleTimeout()
    }
}

Continue()
```

After one branch completes, execution reaches `Continue()`.

---

## Branch control flow

Branch bodies may use ordinary control flow:

- `return`
- `break`
- `continue`
- error propagation
- `cancel`
- nested `select`
- nested blocks

A `break` or `continue` must target a valid surrounding loop according to normal
rules.

---

## Nested select

A branch may contain another `select`.

```sec
select {
    request := requests.Receive() => {
        select {
            reply := replies.Receive() => {
                Handle(reply)
            }

            after 100ms => {
                HandleReplyTimeout()
            }
        }
    }

    after 1s => {
        HandleIdle()
    }
}
```

Nested selection must not inherit readiness reservations from the outer select.

---

## Mutex guards

A live `MutexGuard[T]` must not cross a blocking select.

Invalid:

```sec
let mut state := State.lock()

select {
    message := rx.Receive() => {
        state.value = message.value
    }
}
```

The select may suspend while the guard remains active.

Expected diagnostic:

```text
mutex guard state remains active across select
```

A select containing only statically immediate branches may still be rejected
conservatively in version 0.1.

End the guard scope before select:

```sec
{
    let mut state := State.lock()
    Prepare(state)
}

select {
    message := rx.Receive() => {
        Process(message)
    }
}
```

---

## Borrows

Values referenced by selectable operations must remain valid until one branch
commits or the select exits through cancellation.

The compiler must track:

- shared borrows;
- mutable borrows;
- message borrows;
- receiver borrows;
- task-handle borrows;
- timeout expressions.

Conflicting simultaneous branch preparations are invalid even though only one
branch commits when readiness inspection itself requires incompatible access.

---

## Same resource in several branches

Using the same exclusive resource in multiple branches requires special
validation.

Example:

```sec
select {
    first := rx.Receive() => {
        HandleFirst(first)
    }

    second := rx.Receive() => {
        HandleSecond(second)
    }
}
```

Both branches refer to the same move-only receiver.

This is normally invalid because one `Receiver[T]` cannot represent two
independent receive candidates in the same select.

Expected diagnostic:

```text
receiver rx is used by more than one branch in the same select
```

The same rule applies to conflicting use of one task handle or one move-only send
value.

---

## Branch readiness side effects

Readiness checks must not execute arbitrary user side effects.

The selectable operation must expose compiler-known readiness semantics.

The following is invalid unless `Prepare()` is proven pure and part of a valid
selectable expression:

```sec
select {
    value := rx.Receive(Prepare()) => {
    }
}
```

Arguments needed by a selectable operation are evaluated according to
select-specific preparation rules.

They must not be repeatedly re-evaluated while waiting.

---

## Preparation

Expressions required to construct selectable branch operands are evaluated once
when entering the select, unless the operation is a compiler-defined deferred
operand.

Preparation must not commit the operation.

Prepared move-only values remain owned by the select context until:

- their branch commits;
- another branch commits and they return to ordinary local ownership;
- cancellation cleanup destroys them.

The compiler must represent preparation ownership explicitly.

---

## Closed operations

An operation that can complete immediately with a closed outcome is ready.

Example:

```sec
select {
    message := rx.Receive() => {
        match message {
            Some(value) => Process(value)
            None => HandleClosed()
        }
    }

    after 1s => {
        HandleTimeout()
    }
}
```

If RX is already closed and drained, the receive branch is ready and wins over a
later timeout.

---

## Failed or completed tasks

An await branch is ready when the task has reached any terminal outcome:

- completed;
- cancelled;
- failed.

The selected branch receives the normal await outcome.

A failed task is not skipped in favor of a later branch merely because its
outcome is undesirable.

Readiness is based on completion, not success.

---

## Selection determinism

Given the same set of ready branches at one selection point, source order
determines the selected branch.

The exact time at which concurrent operations become ready may still depend on:

- task scheduling;
- hardware;
- interrupts;
- I/O;
- process behavior;
- target runtime.

Sec guarantees deterministic branch priority, not deterministic external
readiness timing.

---

## Memory synchronization

The selected operation establishes its normal synchronization edge.

Examples:

```text
selected receive
    Acquires message publication.

selected send
    Publishes the committed message.

selected await
    Acquires task completion.

selected join
    Acquires task completion.

selected timeout
    Provides timer completion only.
```

Non-selected branches establish no operation synchronization edge.

The `select` mechanism itself must safely coordinate readiness and commit.

Detailed ordering is defined in `concurrency_memory_model.txt`.

---

## Target profiles

A target may implement select through:

- scheduler wait sets;
- channel registration;
- event loops;
- OS polling primitives;
- RTOS wait sets;
- static embedded wait tables;
- compiler-generated cooperative state machines.

The target must preserve:

- source-order priority;
- exactly-one commit;
- ownership rollback for non-selected branches;
- cancellation;
- timeout semantics;
- synchronization.

A target that cannot support a branch combination must reject the program.

---

## Allocation

A select should not require hidden heap allocation when all branch metadata can
be represented statically or on the task frame.

Embedded profiles may require:

- compile-time-known branch count;
- fixed wait-set storage;
- supported selectable operation combinations;
- statically bounded timer registration.

Dynamic branch construction is not part of version 0.1.

---

## Semantic analysis

The compiler must validate:

- at least one branch exists;
- each non-default branch is selectable;
- at most one `default` exists;
- `default` is last;
- timeout duration is valid;
- source-order reachability;
- exactly-one commit semantics;
- non-selected ownership preservation;
- branch binding types;
- move-only values are not duplicated;
- one exclusive capability is not used by several branches;
- live mutex guards do not cross select;
- borrows remain valid;
- target profile supports the branch set;
- branch result and post-select ownership merge are valid.

## Current implementation status

Implemented:

- `select` and `after` are reserved lexer keywords and are exposed by LSP
  keyword completion.
- `select { ... }` statement syntax is parsed into AST branches.
- Parsed branch forms currently include:
  `binding := operation => { ... }`, `operation => { ... }`,
  `after duration => { ... }` and `default => { ... }`.
- Semantic analysis validates that a select has at least one branch.
- Semantic analysis validates duplicate `default` branches.
- Semantic analysis validates that `default` is the last branch.
- Semantic analysis reports timeout branches after `default` as unreachable.
- Semantic analysis recognizes selectable operands for channel receive,
  channel try-receive, channel send, channel try-send, channel revocable-send
  and `await task`.
- Ordinary function calls used as select operands are rejected as non-selectable.
- Select branch result bindings are scoped to the branch body.
- Select branch bodies are analyzed as independent branches.
- Initial post-select assignment, move-state, local-reference-container and
  arena-generation state is merged across possible selected branches.
- Initial duplicate exclusive resource validation rejects using the same
  receiver/sender/task root in more than one select branch with
  resource-kind-specific diagnostics.
- Initial select preparation validation rejects moving the same move-only
  message value through multiple send branches.
- Semantic analysis rejects live `MutexGuard[T]` values crossing a `select`.
- The LSP formatter indents `select` blocks and select branch bodies.
- Channel operation result types now exist in semantic analysis and can be used
  as the typed basis for future selectable receive, send, revocable-send and
  revoke operands.
- Channel send/revoke operations currently apply their normal non-select
  ownership effects inside the selected branch state. This gives conservative
  post-select ownership merging, but is not yet the full readiness/commit model
  needed for runtime lowering.

Not implemented yet:

- Full two-phase readiness/commit ownership preservation for non-selected
  branches.
- Complete same-resource validation across branches beyond the initial
  root-symbol checks.
- Semantic IR select operations and backend/runtime lowering.

---

## Semantic IR

Semantic IR must represent select explicitly.

At minimum:

```text
SelectCreate
SelectPrepareBranch
SelectRegisterBranch
SelectReady
SelectChoose
SelectCommit
SelectCancel
SelectTimeout
SelectDefault
SelectMerge
```

Each branch must record:

- source-order index;
- operation kind;
- operand ownership;
- readiness function;
- commit operation;
- result type;
- synchronization effect;
- cancellation behavior;
- timeout metadata;
- source location.

The backend must not lower select as a sequence of destructive polling calls.

---

## Diagnostics

Examples:

```text
select requires at least one branch
```

```text
operation is not selectable
```

```text
default branch must be last in select
```

```text
select may contain only one default branch
```

```text
timeout branch is unreachable because default executes immediately
```

```text
receiver rx is used by more than one branch in the same select
```

```text
task worker is consumed by more than one await branch
```

```text
message value is moved by multiple select branches
```

```text
mutex guard state remains active across select
```

```text
target profile does not support select with process wait and channel receive
```

```text
value message may have been moved by selected branch
```

---

## Restrictions

`select` must not:

- commit more than one branch;
- introduce hidden fairness;
- ignore source-order priority;
- consume non-selected receives;
- move non-selected send values;
- consume non-selected tasks;
- execute arbitrary readiness side effects;
- hold mutex guards across suspension;
- duplicate move-only capabilities;
- hide dynamic allocation on restricted profiles;
- be treated as `switch`;
- be lowered as destructive polling.

---

## Future extensions

Possible future additions include:

```text
explicit fairness policies
weighted selection
dynamic wait sets
named selection policies
process and socket integration
select observers
compile-time branch groups
priority annotations independent of source order
```

These are not required for version 0.1.

---

## Related rules

```text
channels.txt
tasks.txt
spawn.txt
await.txt
concurrency.txt
mutex.txt
atomics.txt
concurrency_memory_model.txt
processes.txt
ipc.txt
```
