# Parameter Usage Analysis — Revision 2.0 Synchronization

- **Status:** Normative synchronization correction
- **Created:** 2026-10-04
- **Target:** `rules/analysis/parameter_usage_analysis.md`
- **Target revision after application:** 2.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `main-reviewed-2026-10-04`
- **Implementation governance:** `governance/analysis.yaml` (`sema.parameter-usage-analysis`)
- **Purpose:** Synchronize the existing parameter-demand model with the current Sec ownership-transfer syntax, diagnostic governance, and split implementation-governance structure.

---

## 1. Revision scope

This is a synchronization revision.

The existing multidimensional `ParameterDemand` model remains normative.

Revision 2.0 does **not** redesign:

- access demand;
- mutation demand;
- ownership demand;
- lifetime/retention demand;
- identity demand;
- shape demand;
- storage demand;
- representation demand;
- dimension-specific unknown state;
- direct-call and callable propagation;
- recursive fixed points;
- FFI contract boundaries;
- recommendation policy;
- Interactive / Standard / Deep analysis budgets;
- separate-compilation summaries;
- LSP consumption of analysis results.

The revision corrects stale source syntax and governance ownership and makes the
interaction with the current copy/move rulebook explicit.

---

## 2. Replace the document header

Replace the current opening:

```md
# Parameter Usage Analysis
## Status

Normative compiler-analysis rulebook for Sec 0.1.

This rulebook defines the semantic purpose, demand model, transfer rules,
interprocedural propagation, FFI contract boundaries, narrowing-candidate model,
recommendation policy, tooling integration, summary requirements, diagnostics,
and completion criteria for Sec parameter usage analysis.

Mutable implementation status does not belong in this rulebook. It is governed
by the repository-level `implementation-status.yaml` ledger.
```

with:

```md
# Parameter Usage Analysis

- **Status:** Normative
- **Created:** 2026-08-08
- **Last updated:** 2026-10-04
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/analysis/parameter_usage_analysis.md`
- **Replaces:** Earlier unversioned revision at the same canonical path
- **Repository baseline reviewed:** `main-reviewed-2026-10-04`
- **Implementation governance:** `governance/analysis.yaml` (`sema.parameter-usage-analysis`)
- **Related rulebooks:** `rules/declarations/functions.md`, `rules/memory/copy_move.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/reference_model.md`, `rules/memory/storage.md`, `rules/memory/layout.md`, `rules/analysis/escape_analysis.md`, `rules/analysis/closure_analysis.md`, `rules/analysis/call_graph.md`, `rules/analysis/effect_analysis.md`, `rules/platform/ffi.md`, `rules/tooling/diagnostics.md`, `rules/tooling/lsp.md`, `rules/compiler/incremental_compilation.md`

---

## Status and authority

This is the normative compiler-analysis rulebook for Sec 0.1 parameter usage
analysis.

This rulebook defines the semantic purpose, demand model, transfer rules,
interprocedural propagation, FFI contract boundaries, narrowing-candidate model,
recommendation policy, tooling integration, summary requirements, diagnostics,
and completion criteria for Sec parameter usage analysis.

Mutable implementation status belongs to `sema.parameter-usage-analysis` in
`governance/analysis.yaml`.

Source-level parameter ownership syntax and copy/move legality remain owned by
`rules/declarations/functions.md` and `rules/memory/copy_move.md`.
```

---

## 3. Add an explicit ownership-syntax boundary

Immediately after the existing `# Normative role` section, add:

```md
# Ownership-transfer syntax boundary

Parameter usage analysis consumes resolved ownership-transfer facts.

It does not infer consumption from obsolete source spellings or from type
copyability alone.

The canonical Sec 0.1 source rules are:

```sec
fn Consume(-> value: Buffer) void {
    ...
}

Consume(<-buffer)
```

A consuming parameter is declared with `->`.

Passing an existing reusable source Place to a consuming parameter requires
`<-` at the call site.

A fresh temporary may be passed directly:

```sec
Consume(CreateBuffer())
```

The call-site marker is not required for a fresh temporary because there is no
reusable caller binding whose availability must be explicitly surrendered.

A normal by-value parameter is not consuming merely because its type is
move-only.

Parameter usage analysis therefore derives consumption demand from the resolved
semantic operation and callable contract, not from spelling heuristics or from
the physical ABI.

Return is a separate ownership boundary.

For a non-reference return:

```sec
return value
```

ownership of an owned local may transfer to the caller without a mandatory
`<-` marker.

The optional explicit form:

```sec
return <-value
```

has the same transfer meaning when legal.

Parameter-demand analysis must therefore record the semantic return transfer,
not test for presence of a call-site move marker.
```

This section introduces no new ownership semantics. It incorporates the current
canonical copy/move contract into this analysis rulebook.

---

## 4. Correct `ConsumptionRequired` example

Replace:

```sec
fn Enqueue(job: Job) void {
    queue.Add(move job)
}
```

with:

```sec
fn Enqueue(-> job: Job) void {
    queue.Add(<-job)
}
```

Add immediately after the example:

```text
The parameter itself is consuming.

Inside the function, `job` is an owned named Place. Forwarding that existing
Place into another consuming parameter therefore uses `<-` at that call site.

The analysis records `ConsumptionRequired` from the resolved ownership transfer,
not from the textual presence of `<-` alone.
```

This last sentence matters because a fresh temporary may produce the same
semantic consumption without a marker.

---

## 5. Correct the `Move use` section

Replace the existing example:

```sec
Consume(move value)
```

with:

```sec
fn Consume(-> item: Item) void {
    ...
}

Consume(<-value)
```

Then replace the first explanatory paragraph after the example with:

```text
The resolved call transfers ownership of the existing named Place `value` into
the consuming parameter.

That use contributes:

```text
Ownership = ConsumptionRequired
```

A borrow-only candidate cannot satisfy this demand.
```

Add:

```text
The syntax `<-` is evidence of a source-level transfer request at this call
site, but the analysis must consume the resolved semantic ownership operation.

For example:

```sec
Consume(CreateItem())
```

also contributes consumption demand even though no `<-` marker is required for
the fresh temporary.
```

---

## 6. Correct the control-flow join example

Replace:

```sec
if store {
    Save(move value)
} else {
    Inspect(value)
}
```

with:

```sec
if store {
    Save(<-value)
} else {
    Inspect(value)
}
```

Add before or immediately after this example:

```text
This example assumes `Save` has a consuming parameter contract:

```sec
fn Save(-> value: Value) void
```

The consuming branch therefore requires the combined parameter demand to retain
consumption capability.
```

The existing control-flow-join semantics remain unchanged.

---

## 7. Clarify direct-call propagation

In `# Calls propagate demand`, retain the existing interprocedural model but add:

```text
Propagation uses the resolved callee parameter contract.

For a non-consuming ordinary by-value or borrowed parameter, the caller demand
is instantiated according to that contract.

For a consuming `->` parameter, caller demand includes the required ownership
transfer.

When the actual argument is an existing reusable Place, valid source syntax
contains `<-` at the call site.

When the actual argument is a fresh temporary, the same consuming contract may
be satisfied without `<-`.

The demand summary therefore stores semantic transfer requirements and must not
store "move marker present" as the ownership model.
```

---

## 8. Clarify forwarding

In `# Forwarding`, after:

```text
If `Inner` consumes ownership, `Outer` must retain the stronger demand.
```

add:

```text
If the source parameter of `Outer` is itself available as an owned named Place,
forwarding it into an explicitly consuming `Inner` parameter uses the canonical
call-site transfer marker:

```sec
fn Inner(-> value: Value) void {
    ...
}

fn Outer(-> value: Value) void {
    Inner(<-value)
}
```

This example describes source syntax only.

The propagated `ParameterDemand` remains expressed as semantic ownership demand,
not as syntax tokens.
```

---

## 9. Clarify return-boundary demand

In `# Returning a parameter by value`, retain:

```sec
return value
```

and replace the paragraph beginning:

```text
may require ownership, copy, or move capability according to ordinary Sec value
semantics.
```

with:

```text
The resolved return operation may require ownership, copy, or transfer
capability according to ordinary Sec value semantics.

A non-reference return is intrinsically a result-value ownership boundary.
Therefore `return value` may transfer an owned local even though `<-` is not
mandatory at the return boundary.

The analysis must use the resolved return operation rather than treating absence
of `<-` as evidence that ownership is not transferred.
```

Optionally retain this explicit equivalent example:

```sec
return <-value
```

with the note that the marker is permitted but not required at the return
boundary.

---

## 10. Keep copy demand separate from source copy mechanics

The existing `# Current copy is not proof of required copy` and `# Copy use`
sections remain normative.

Add this clarification:

```text
Likewise, ordinary by-value parameter syntax is not evidence of
`ConsumptionRequired`.

A by-value parameter may create source-level copy semantics when its argument is
an existing reusable Place and the value is copyable.

Parameter usage analysis asks what the implementation semantically requires,
not merely what the current declaration causes at its boundary.
```

This preserves the existing distinction between:

```text
DeclaredCapability
RequiredCapability
```

and synchronizes it with the revision-2 copy/move model.

---

## 11. Synchronize diagnostic ownership

In `# Diagnostics and advisories`, replace:

```text
Exact stable diagnostic IDs are assigned under the compiler-wide diagnostic
governance.
```

with:

```text
User-facing diagnostic identity, severity, configurability, structured
occurrences, help, fixes, CLI transport, and LSP transport are governed by
`rules/tooling/diagnostics.md`.

Parameter-usage analysis may define stable internal advisory categories such as:

```text
parameter.unused-capability
parameter.unnecessary-by-value
parameter.unnecessary-ownership
parameter.unnecessary-mutable-access
parameter.unnecessary-fixed-extent
parameter.owning-sequence-where-view-suffices
parameter.fixed-array-where-view-suffices
parameter.narrowing-blocked
parameter.ffi-wrapper-overconstrained
```

Those category strings are not automatically compiler diagnostic IDs.

Every emitted primary diagnostic must use a registered stable diagnostic ID.

Published IDs must not be repurposed merely because a parameter-usage category
would fit them approximately.
```

---

## 12. Synchronize recommendation fixes with ownership syntax

In the recommendation/corrective-action sections, add:

```text
A source edit proposed by parameter usage analysis must use canonical current
ownership syntax.

In particular:

- changing a parameter to consuming form uses `->`;
- transferring an existing reusable Place at a call site uses `<-`;
- a code action must not insert `<-` merely because a value is move-only;
- a code action must not remove `<-` when the resolved callee parameter is
  consuming and the source is an existing reusable Place;
- a fresh temporary passed to a consuming parameter must not receive synthetic
  move boilerplate;
- return rewrites must respect that `return value` already transfers an owned
  result where ordinary return semantics permit it.

A code action changing a function parameter contract must update affected call
sites only when the compiler has proven that the complete edit set remains
semantically valid.
```

---

## 13. Synchronize project-policy ownership

Where the rulebook discusses project analysis configuration or recommendation
policy, add:

```text
Project-file syntax and configuration precedence are owned by
`rules/projects/projects.md`.

Parameter usage analysis consumes the resolved project/CompilationPlan analysis
policy and does not define a competing manifest syntax.
```

---

## 14. Replace the governance section

Replace the existing `# Governance` section with:

```md
# Governance

This rulebook contains normative parameter-usage analysis behavior and
completion requirements.

Mutable implementation progress belongs to:

```text
governance/analysis.yaml
```

under:

```text
sema.parameter-usage-analysis
```

That integration should track granular capabilities including at least:

```text
local demand
control-flow demand
direct-call propagation
consuming-parameter transfer demand
return-boundary transfer demand
recursive demand
array/view narrowing
large-value narrowing
FFI contract demand
callable-contract propagation
summary persistence
Deep whole-program analysis
diagnostic integration
LSP integration
```

Implementation status owned by another subsystem must remain in that subsystem's
governance entry.

In particular:

```text
copy/move frontend semantics
    declarations/memory ownership governance owner

diagnostic registry and transport
    governance/errors_diagnostics.yaml

FFI contracts
    FFI governance owner

LSP presentation
    governance/tooling_lsp.yaml

incremental summary invalidation
    incremental-compilation governance owner
```

`sema.parameter-usage-analysis` may record dependencies on those integrations
but must not duplicate their mutable implementation state.
```

---

## 15. Update the final normative summary

Replace:

```text
Mutable implementation progress is governed by `implementation-status.yaml`.
```

with:

```text
Mutable implementation progress is governed by
`sema.parameter-usage-analysis` in `governance/analysis.yaml`.
```

---

## 16. Required regression tests added by revision 2.0

Add these tests to the rulebook's required test inventory:

```text
consuming parameter declared with ->
existing reusable Place passed with <-
fresh temporary passed to consuming parameter without <-
ordinary by-value parameter not inferred as consuming from move-only type alone
return value recognized as transfer without mandatory <-
optional return <-value produces equivalent ownership demand when legal
consuming forwarding propagates ConsumptionRequired
branch join with one consuming path retains ConsumptionRequired
parameter-demand summaries encode semantic consumption rather than syntax-token presence
code action never inserts synthetic <- before a fresh temporary
code action does not remove required <- from an existing reusable Place
diagnostic/advisory categories remain distinct from stable compiler diagnostic IDs
```

---

## 17. Governance synchronization required

No new governance file is created.

Update the existing `sema.parameter-usage-analysis` integration in:

```text
governance/analysis.yaml
```

to identify revision 2.0 and to include explicit rule coverage for:

```text
ownership-transfer syntax boundary
consuming-parameter demand
return-boundary transfer demand
semantic-not-token-based transfer summaries
diagnostic-governance boundary
current ownership-aware code actions
```

Preserve all existing truthful `implemented`, `partial`, `unimplemented`, and
`identified_bugs` entries unless repository verification justifies changing
them.

---

## 18. Revision note

Revision 2.0 does not alter the multidimensional parameter-demand design.

It synchronizes the rulebook with later locked Sec decisions:

1. consuming parameters use `->`;
2. an existing reusable caller Place uses `<-` when passed to a consuming
   parameter;
3. fresh temporaries need no synthetic transfer marker;
4. return is intrinsically a result-value transfer boundary and does not require
   `<-`;
5. analysis summaries represent semantic ownership demand rather than syntax
   tokens;
6. diagnostics are governed by the canonical diagnostics rulebook;
7. mutable implementation status belongs to
   `sema.parameter-usage-analysis` in `governance/analysis.yaml`.
