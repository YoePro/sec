# Correction: function calls and variadic packs in Semantic IR

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** Semantic IR rulebook

Semantic IR for functions and calls must preserve:

- concrete callable identity;
- ordered parameters;
- by-value, shared-borrow, mutable-borrow, and forced-consuming parameter modes;
- native variadic element type and position;
- explicit return type;
- call argument source order;
- ownership-transfer commit point;
- static versus instance method;
- inferred concrete receiver capability.

## Call evaluation

Argument expressions are semantically ordered left-to-right.

Ownership transfer to callee-owned parameters is committed only after every argument has evaluated successfully and the call is ready to enter the callee.

IR must retain enough structure to clean earlier temporaries and preserve caller ownership if a later argument evaluation fails.

## Native variadic pack

A native variadic pack is not an ordinary array/slice node merely because one backend representation could use contiguous storage.

IR must preserve:

- call-lifetime-only semantics;
- read-only pack structure;
- non-escaping restriction;
- no `Ptr` or contiguity guarantee;
- no individual element move-out;
- per-element by-value copy/move semantics.

Lowering may select a target-efficient representation but must not introduce semantic heap allocation solely to materialize the pack.

Native variadic IR must remain distinct from foreign/C varargs.
