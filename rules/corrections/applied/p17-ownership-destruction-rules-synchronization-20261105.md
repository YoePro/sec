# Correction: P17 ownership and destruction source-rule synchronization

- **Status:** Applied 2026-11-05: proposals B, C and E were merged on
  2026-11-03; proposals A and D are **superseded by MD-125** (owner decision
  2026-11-05, option B), whose own insertions are applied
- **Created:** 2026-11-03
- **Document revision:** 1
- **Sec version:** 0.1
- **Work item:** P17-03
- **Source authority:** `rules/mlir/normative-sync/sec_ownership_destruction_sync_v1.md`
  §§ 1–18; `rules/mlir/semantic-ir/sec_semantic_ir_ownership_destruction_v1.md`
  §§ 1–33
- **Targets:** `rules/memory/destruction.md`, `rules/control-flow/defer.md`,
  `rules/compiler/semantic_ir.md`

## Audit of the normative synchronization

Every section was compared with its canonical owner. "Present" means the
owner already states the rule, possibly in other words, and the newer
canonical text is kept.

| Sync section | Canonical owner and result |
|---|---|
| 1 unified cleanup order canonical | destruction § 6.2(1)–(5), defer § 7: present. The sync calls the model "recommended"; the owner books already make it normative. No change |
| 2 local registration | destruction §§ 6.1(1), 6.1(3), 9.1: present |
| 3 replacement creates a new incarnation | destruction § 13.2(2) and semantic_ir § 40(2) destroy the old value first. **Missing:** the new incarnation registers at the replacement point. See proposals A and D |
| 4 reinitialization after move | destruction §§ 6.1(2), 13.1(1)–(3); semantic_ir § 40(3)–(4): present |
| 5 defer registration | defer §§ 2, 5, 12, 13: present |
| 6 defer capture semantics | defer § 9 (read at execution, no capture-by-value): present. semantic_ir § 44(2) still says "captured values / capture modes". See proposal C |
| 7 lifetime of defer-referenced bindings | defer §§ 8, 10, 11; destruction § 6.3(3): present |
| 8 defer inside loops | defer § 13 counts registrations but does not say which binding instance each one reads. **Missing.** See proposal B |
| 9 function exit sequence | defer §§ 14–15; destruction §§ 19.1–19.3: present |
| 10 lexical scope exit | destruction § 19.4; defer § 5: present |
| 11 panic non-unwinding | destruction § 24.3; defer § 20: present |
| 12 discard uses destruction | discard §§ 3, 8, 10; destruction § 12; semantic_ir § 39: present |
| 13 reference/slice discard | discard §§ 19–21: present |
| 14 infallible semantic copy | copy_move § 3.3(1), § 4.5(2), § 4.6(1); ownership § 12.1(2): present |
| 15 move source state | copy_move §§ 2(3), 6.4(1); destruction § 9.1: present |
| 16 partial move scope | ownership § 18.5 rejects unsupported sub-Places conservatively. The array-element limit is P17 implementation scope, not a source rule. No change |
| 17 custom free restriction | destruction § 15; MD-025, which P17-04 already reconciled. No change |
| 18 Semantic IR cleanup requirement | destruction § 30(1), (5), (6); semantic_ir §§ 37(5), 42(3), 43(1): present |

## Audit of the Semantic IR amendment

| Amendment sections | Canonical owner and result |
|---|---|
| 1 ownership explicit | semantic_ir § 32(3), § 39(6): present |
| 2 transfer actions | The concepts are in semantic_ir §§ 34–36, 45, but the list of canonical action names is not. **Missing.** See proposal E |
| 3–5 owned state, Place state, partial subobjects | semantic_ir §§ 33, 37 hold the newer five-state lattice (with `ConditionallyAvailable` and separate reasons). It is kept in place of the amendment's reduced list. Present |
| 6–9 copy, semantic copy, move, move from place | semantic_ir §§ 34–35; copy_move § 3.3: present |
| 10–12 initialization, replacement, transactional replacement | semantic_ir § 40; destruction §§ 13.2(3)–(4); ownership § 12.1(2). The registration point is missing, as for sync § 3. See proposal D |
| 13–17 destruction plans, struct, array, variant, custom free | destruction §§ 7, 15, 16, 17; semantic_ir § 42: present |
| 18–19 discard | semantic_ir § 39; discard: present |
| 20–23 cleanup registration and order | semantic_ir §§ 43, 44(3); destruction § 6: present |
| 24–26 defer registration, captures, body | defer §§ 13, 15–17; semantic_ir § 44. Covered by proposals B and C |
| 27 panic boundary | destruction § 24.3; semantic_ir § 43(4): present |
| 28–30 branch ownership, calls, return | semantic_ir §§ 36, 67(1); destruction § 19.1: present |
| 31 verification | destruction § 34; semantic_ir §§ 96–97: present |
| 32 no runtime ownership table | destruction § 11.2 (bookkeeping only when needed): present |
| 33 deterministic printer | semantic_ir § 108 and the dialect v13 printer: present. Physical spelling is not a source rule |

## Proposed insertions

Each proposal restates authority that P17 already supplies. On 2026-11-03 the
owner merged B (defer.md § 13), C (semantic_ir.md §§ 44(2), 44(2a)) and E
(semantic_ir.md § 35(6)). A and D were left to MD-125, which the owner decided
on 2026-11-05 in the opposite direction (option B); they are **superseded** and
were never applied. The applied MD-125 text is destruction.md §§ 6.1(4)–(5),
13.2(5)–(8), 13.3(5), semantic_ir.md § 40(7)–(10) and the defer.md § 9
clarification (`rules/corrections/applied/MD-125-replacement-cleanup-registration-point-applied-20261105.md`).

**A. (superseded by MD-125) destruction.md § 13.2(5).** The new value of a replacement registers its
destruction responsibility at the replacement point. Cleanup registered between
the original initialization and the replacement therefore runs after the new
value is destroyed.

**B. defer.md § 13, after the five-registrations example.** When the loop body
declares a new binding on each iteration, each registration reads the binding
instance of its own iteration. Registrations are not merged by source name. That
instance stays valid until its registration runs (§ 8).

**C. semantic_ir.md § 44(2).** Replace "captured values / capture modes" with
"captured binding/Place identities, read when the defer executes (defer § 9);
capture never copies the current value".

**D. (superseded by MD-125) semantic_ir.md § 40(7).** A replacement registers the new owned incarnation
at the replacement point, as in destruction § 13.2(5).

**E. semantic_ir.md § 35(6).** The canonical transfer actions are `construct`,
`copy-trivial`, `copy-semantic-infallible`, `move`, `borrow-shared`,
`borrow-mutable` and `non-consuming`. Fallible clone-like operations are
ordinary calls, not copy actions.

## Open point for the owner (MD-125) — decided 2026-11-05

**Decision:** option B. A replacement keeps the Place's existing cleanup
position, so in the example below the defer runs first and reads the new value,
and the new value is destroyed afterwards. The program is **valid**. The record
below is kept as the historical finding. The compiler still stops this case at
the named lowering boundary until the P17 cleanup producers implement inherited
registration (governance `lowering.sec-mlir-package17`, `frontend.destruction`).


Proposal A combined with defer.md § 10 has a consequence:

```sec
let mut handle := Make(1)
defer {
    Log(handle.id)
}
handle = Make(2)
```

The new `handle` registers after the defer. It is therefore destroyed before the
defer reads it. Under defer.md § 10 the program is invalid. The compiler
currently stops it as a named lowering boundary ("P17 replacement of an owned
value a registered defer reads"), not as a Sema diagnostic. The owner decides
whether this becomes a source error or whether the replacement keeps the old
registration's position.

## Implementation finding during the audit

defer.md § 8 requires that an inner-scope value read by a registered defer is
destroyed after that defer runs. Before this audit, Semantic IR destroyed the
value at the end of its scope (`cleanup.run_scope`), while the defer read it at
function exit, and no verifier rejected this. The following now fail closed:

- Semantic IR names the boundary "P17 defer in a loop reading an owned value of
  the loop body" (`defer_producer.go`; boundary row `defer_reads_loop_owner.sec`).
  On 2026-11-03 this was the boundary for every inner scope.
- `--sec-verify-cleanups` rejects a `run_scope` that would destroy an owner that
  an active defer reads (lit `@defer_reads_inner_owner`).

For proposal B, Semantic IR also stops a defer in a loop that reads mutable
storage declared in the same loop. That storage is a single slot, so every
registration would read the last iteration's value ("P17 defer reading mutable
storage declared in its loop"; boundary row `defer_reads_loop_storage.sec`).
An immutable binding or a range `for` binding is captured as a value root, so
each registration already reads its own instance.

Since 2026-11-04 the extended lifetime lowers outside loops: such an owner
registers in the function scope and is destroyed after the defer at function
exit. In a loop it needs one instance per registration and stays a named
boundary.
