# Correction: closures in Semantic IR

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** Semantic IR rulebook

Semantic IR must preserve closure semantics before representation-specific lowering.

A closure representation must retain or make unambiguously derivable:

```text
lambda source identity
concrete callable signature
callable capability: shared / mutable / consuming
capture list
capture mode per entry
concrete capture type
environment copy/move classification
escape classification
environment lifetime
closure construction
closure call
closure consumption
environment destruction
```

IR must not force every callable into a universal `{code_ptr, env_ptr}` representation before ownership, escape, region, ABI, and optimization decisions are complete.

Escaping environments may require owned dynamic storage, but the physical strategy is compiler-controlled.

Any allocation/resource effect introduced by the selected environment strategy must remain visible to relevant compiler analyses.

Native closure representation is not automatically a foreign callback ABI.
