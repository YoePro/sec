# Escape Analysis — Revision 2.0 Synchronization

- **Status:** Normative synchronization correction
- **Created:** 2026-10-07
- **Target:** `rules/analysis/escape_analysis.md`
- **Target revision after application:** 2.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `main-reviewed-2026-10-07`
- **Implementation governance:** `governance/analysis.yaml` (`sema.escape-analysis`)
- **Purpose:** Synchronize the existing escape-analysis model with current Sec declaration syntax, borrow-call semantics, diagnostics governance, and split implementation governance.

---

## 1. Revision scope

This is a synchronization revision.

The existing escape-analysis semantic model remains normative.

Revision 2.0 does **not** redesign:

- escape subjects;
- escape modes;
- escape destinations;
- canonical sources and sinks;
- Place provenance;
- disjoint projections;
- view/backing-storage dependencies;
- aggregate/container propagation;
- current provenance versus historical escape;
- interprocedural summaries;
- recursive fixed points;
- retention contracts;
- closure environment escape;
- task/thread transfer;
- FFI retention;
- separate-compilation summaries;
- no-silent-promotion behavior;
- Semantic IR placement;
- diagnostic evidence requirements.

The revision corrects stale source examples and governance ownership and makes
the boundary to current borrow and ownership-transfer semantics explicit.

---

## 2. Replace the document header

Replace the current opening with:

```md
# Escape Analysis

- **Status:** Normative
- **Created:** Legacy rulebook; exact original creation date not established
- **Last updated:** 2026-10-07
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/analysis/escape_analysis.md`
- **Replaces:** Earlier unversioned revision at the same canonical path
- **Repository baseline reviewed:** `main-reviewed-2026-10-07`
- **Implementation governance:** `governance/analysis.yaml` (`sema.escape-analysis`)
- **Related rulebooks:** `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/copy_move.md`, `rules/memory/reference_model.md`, `rules/memory/storage.md`, `rules/memory/layout.md`, `rules/memory/destruction.md`, `rules/declarations/functions.md`, `rules/declarations/lambda-functions.md`, `rules/analysis/closure_analysis.md`, `rules/analysis/parameter_usage_analysis.md`, `rules/analysis/call_graph.md`, `rules/analysis/effect_analysis.md`, `rules/analysis/stack_analysis.md`, `rules/platform/ffi.md`, `rules/tooling/diagnostics.md`, `rules/tooling/lsp.md`, `rules/compiler/semantic_ir.md`, `rules/compiler/incremental_compilation.md`

---

## Status and authority

This is the normative compiler-analysis rulebook for Sec 0.1 escape analysis.

This rulebook defines the semantic purpose, analysis domain, dataflow model,
interprocedural summaries, conservative behavior, consumers, diagnostic
requirements, and completion criteria of Sec escape analysis.

Mutable implementation status belongs to `sema.escape-analysis` in
`governance/analysis.yaml`.

Escape analysis consumes canonical ownership, borrowing, storage, reference,
callable, and FFI facts. It does not redefine their source syntax or legality.
```

---

## 3. Preserve the Sec 0.1 declaration example

In `## BorrowEscape`, keep the existing Sec 0.1 declaration syntax:

```sec
fn Broken() ref int {
    int value := 10
    return ref value
}
```

`int value := 10` remains valid Sec 0.1 syntax and must not be rewritten merely
because a later Sec version is expected to remove this declaration form.

The semantic example remains:

- `value` has Automatic local storage;
- `return ref value` exposes a reference whose origin is that local Place;
- escape analysis identifies the origin and destination;
- lifetime/storage validation rejects the result because the origin cannot
  satisfy caller lifetime.

Revision 2.0 of this rulebook targets Sec 0.1 and therefore preserves valid 0.1
source syntax. Any 0.2 declaration-syntax migration belongs to the rulebooks
that define Sec 0.2.

---

## 4. Correct call-site borrow syntax

In `# Function and lexical boundaries`, replace old call-site borrow forms with
current parameter-driven borrowing.

Use:

```sec
fn Inspect(value: ref Value) void {
    ...
}

Inspect(value)
```

instead of:

```sec
Inspect(ref value)
```

Use:

```sec
fn Register(value: ref Value) void {
    ...
}

Register(value)
```

instead of:

```sec
Register(ref value)
```

The surrounding semantics remain:

```text
Inspect
    proven NoRetention
    call-bounded shared borrow
    no escape beyond the call

Register
    retaining contract
    the borrowed dependency may escape beyond call return
```

Sec 0.1 borrowed parameters create the required call-bounded borrow from a
compatible caller Place without a separate `ref` or `ref mut` marker at the
call site.

Escape analysis therefore consumes the resolved parameter borrow mode and
retention contract.

It must not infer borrow mode from obsolete call-site syntax.

---

## 5. Add an explicit call-boundary semantic-classification rule

Immediately after `# Call retention contracts`, add:

```md
# Call-boundary semantic classification

Escape analysis consumes the resolved call contract.

For a borrowed parameter:

```sec
fn Inspect(value: ref Buffer) void {
    ...
}

Inspect(buffer)
```

the call establishes borrowed authority without transferring ownership.

For a mutable borrowed parameter:

```sec
fn Modify(value: ref mut Buffer) void {
    ...
}

Modify(buffer)
```

the call establishes temporary exclusive borrowed authority without transferring
ownership.

For an ordinary by-value parameter, copy or ownership transfer follows the
concrete type's canonical copy/move classification.

For a reusable move-only caller Place, transfer is explicit:

```sec
Process(<-resource)
```

For an explicit consuming parameter:

```sec
fn Transform(-> data: BigArray) BigArray {
    ...
}

Transform(<-data)
```

the resolved call contract is ownership transfer even when the concrete type
would otherwise be copyable.

Fresh temporaries may satisfy consuming transfer without a redundant `<-`
marker.

Escape analysis must record the resolved semantic relation:

```text
borrow
mutable borrow
copy
ownership transfer
retention
```

rather than using source-token presence as the escape model.

The escape summary must remain valid after source-level syntax has been lowered
to Semantic IR.
```

This section introduces no new ownership or borrowing semantics.

---

## 6. Clarify return transfer

In `# Core rule`, `## OwnedValue`, and `# Returned owned values`, retain examples
using:

```sec
return value
```

Add:

```text
A non-reference return is already an ownership/result boundary under the
canonical copy/move rules.
```

Therefore:

```sec
return value
```

may transfer a move-only owned local to the caller.

The equivalent explicit documentation form:

```sec
return <-value
```

is permitted but not required.

Escape analysis must classify the resolved return as `ValueTransfer` regardless
of whether the optional return marker was written.

The source local's Automatic storage does not thereby become caller storage.
The returned value may be lowered through caller result storage, move elision,
SSA forwarding, or another semantics-preserving mechanism.

This preserves the existing distinction between:

```text
owned-value escape
physical source-storage escape
```

---

## 7. Keep escape legality separate from ownership legality

Retain the existing rule that escape analysis consumes canonical copy/move
classification.

Add:

```text
Escape analysis must never decide that a value is movable merely because a flow
would be easier to represent as ValueTransfer.

Likewise, it must not reinterpret an ordinary by-value call as a borrow or
reinterpret a borrow as ownership transfer to avoid an escape/lifetime failure.

When ownership or borrow legality rejects the operation, escape facts may still
be retained for diagnostics, but escape analysis must not repair the operation
through hidden copy, heap promotion, storage substitution, or contract
weakening.
```

---

## 8. Synchronize diagnostics ownership

In `# Diagnostic categories`, retain the conceptual category names:

```text
escape.reference-outlives-origin
escape.view-outlives-backing
escape.retained-borrow
escape.static-store-of-short-lived-reference
escape.thread-local-store-of-short-lived-reference
escape.foreign-retention
escape.task-transfer
escape.thread-transfer
escape.closure-capture
escape.unknown-retention
escape.analysis-precision-exhausted
```

but replace the old central-governance wording with:

```text
These strings are stable escape-analysis categories unless and until a
registered diagnostic definition explicitly uses one as its symbolic diagnostic
name.

Every emitted primary compiler diagnostic must use a stable registered
diagnostic ID governed by `rules/tooling/diagnostics.md`.

Escape-analysis categories are not automatically diagnostic IDs.

Diagnostic severity, configurability, notes, help, related locations, structured
fixes, CLI transport, JSON transport, and LSP diagnostic identity are owned by
`rules/tooling/diagnostics.md`.

An existing published diagnostic ID must never be repurposed merely because an
escape category appears similar.
```

---

## 9. Clarify diagnostic evidence transport

In `# Diagnostic quality`, retain the existing evidence list and add:

```text
When the escape depends on an earlier borrow, transfer, capture, storage
placement, or retaining call, that source location should be transported as a
related location when available.

Escape diagnostics should expose semantic facts such as:

origin Place
origin storage domain
escape sink
escape mode
retention contract
backing-storage dependency
cause path

rather than forcing the renderer or LSP to reconstruct them from English text.

The analysis must not emit a misleading concrete lifetime claim when the actual
reason for rejection is conservative UnknownEscape or UnknownRetention.
```

---

## 10. Synchronize parameter-usage integration

In `# Parameter-usage analysis consumer`, retain the consumer relation and add:

```text
Revision 2.0 parameter-usage analysis consumes semantic ownership-transfer facts
rather than source move-token presence.

Escape summaries therefore expose semantic facts such as:

NoEscape
NoRetention
Returned
Retained
OwnershipTransferred

and must not encode `caller wrote <-` as the inter-analysis contract.

Source syntax remains available separately for diagnostics and code actions when
needed.
```

---

## 11. Synchronize closure-analysis references

Where the rulebook refers to:

```text
closure_analysis.md
```

use the canonical path:

```text
rules/analysis/closure_analysis.md
```

The semantic ownership boundary remains:

```text
closure analysis
    owns capture mode and callable-flow precision

escape analysis
    owns movement/retention of the resulting dependencies
```

No closure semantics are changed by this revision.

---

## 12. Synchronize FFI references

Where the rulebook refers to FFI retention contracts, use:

```text
rules/platform/ffi.md
```

Retain the existing rule that foreign retention must be explicit or
conservative.

Add:

```text
Escape analysis consumes the canonical resolved FFI retention/ownership
contract.

It must not infer foreign retention from parameter names, pointer type alone,
foreign symbol name, library name, or calling convention alone.

Unknown retention remains conservative.
```

---

## 13. Synchronize CompilationPlan and incremental-summary boundary

Add to the separate-compilation/incremental sections:

```text
Escape analysis runs for the resolved CompilationPlan.

Project, Target, Variant, and project-configuration precedence are owned by
`rules/projects/projects.md`.

Escape analysis must not invent a second project-configuration syntax.

A persisted escape summary may be reused only when its schema, semantic
dependencies, and CompilationPlan-sensitive compatibility requirements remain
valid under the incremental-compilation rules.

Missing or incompatible metadata falls back conservatively rather than being
accepted as proof of non-escape.
```

---

## 14. Replace the governance section

Replace the existing `# Governance` section with:

```md
# Governance

Normative escape-analysis behavior remains in this rulebook.

Mutable implementation status belongs to:

```text
governance/analysis.yaml
```

under:

```text
sema.escape-analysis
```

That integration should track granular implementation capabilities including at
least:

```text
escape subject classification
escape destination classification
escape mode classification
canonical Place provenance
disjoint projection precision
view/backing-storage dependencies
aggregate/container propagation
current versus historical provenance
direct-call summaries
recursive summary fixed points
callable-target integration
closure-environment integration
borrow retention
ownership-transfer escape
return transfer
task/thread transfer
FFI retention
static/TLS storage escape
separate-compilation summaries
incremental invalidation
diagnostic evidence
LSP integration
```

Implementation state owned by another subsystem remains in that subsystem's
governance entry.

In particular:

```text
ownership and copy/move legality
    owning memory/declaration governance integration

borrowing legality
    owning memory governance integration

diagnostic registry and transport
    governance/errors_diagnostics.yaml

closure capture/callable-flow implementation
    sema.closure-analysis

parameter-demand implementation
    sema.parameter-usage-analysis

call graph implementation
    analysis.call-graph or its current canonical integration

FFI contract implementation
    owning FFI governance integration

LSP presentation
    governance/tooling_lsp.yaml

incremental summary compatibility/invalidation
    owning incremental-compilation governance integration
```

`sema.escape-analysis` may depend on those integrations but must not duplicate
their mutable implementation state.
```

---

## 15. Update completion/governance wording

Replace the final/root-ledger wording with:

```text
Mutable implementation status is governed by `sema.escape-analysis` in
`governance/analysis.yaml` rather than by this normative rulebook.
```

Replace remaining normative references to root `implementation-status.yaml` as
the escape-analysis owner with the split governance owner above.

---

## 16. Required regression tests added by revision 2.0

Add:

```text
shared borrowed parameter called without explicit call-site ref
mutable borrowed parameter called without explicit call-site ref mut
borrowed non-retaining call remains call-local
borrowed retaining call produces retention escape
reusable move-only caller Place transfer uses canonical <-
fresh temporary consumption remains marker-free
explicit -> consuming parameter yields ownership-transfer escape
return value yields ValueTransfer without requiring <-
return <-value yields equivalent escape facts when legal
owned return does not imply physical source-storage escape
escape summary stores semantic ownership transfer rather than source-token presence
escape category identity remains distinct from registered diagnostic ID
unknown retention diagnostic reports uncertainty rather than invented concrete lifetime
```

---

## 17. Governance synchronization required

No new governance file or integration is created.

Update the existing:

```text
sema.escape-analysis
```

entry in:

```text
governance/analysis.yaml
```

to reference revision 2.0 and include explicit rule coverage for:

```text
current borrow-call syntax
semantic ownership-transfer integration
intrinsic return transfer
diagnostic-governance boundary
CompilationPlan-sensitive summary reuse
```

Preserve existing truthful `implemented`, `partial`, `unimplemented`, and
`identified_bugs` entries unless repository verification justifies changing
them.

---

## 18. Revision note

Revision 2.0 does not redesign Sec escape analysis.

It synchronizes the existing model with later locked Sec rules:

1. valid Sec 0.1 declaration syntax, including `int value := 10`, remains valid
   in this revision;
2. borrowed parameters create call-bounded borrows without explicit `ref` /
   `ref mut` call-site markers;
3. reusable ownership transfer uses `<-` where the copy/move rules require it;
4. fresh temporary consumption remains marker-free;
5. return is intrinsically an ownership/result transfer boundary;
6. escape summaries represent semantic dependency flow rather than syntax-token
   presence;
7. diagnostic categories remain distinct from registered compiler diagnostic
   identity;
8. mutable implementation status belongs to `sema.escape-analysis` in
   `governance/analysis.yaml`.
