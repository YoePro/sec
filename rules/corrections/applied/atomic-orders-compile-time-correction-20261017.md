# Correction — Compile-time atomic memory orders

- **Status:** Applied
- **Created:** 2026-10-17
- **Applied:** 2026-10-17
- **Authority:** Explicit language-owner decision on 2026-10-17: ordinary atomic operations must require compile-time-known explicit order arguments.
- **Owning rulebook:** `rules/concurrency/atomics.md`, revision 2.1

## 1. Decision

Every explicit `MemoryOrder` argument of an ordinary `Atomic[T]` operation or
`atomic.Fence` is a `SemanticCompileTimeRequiredContext`, named
`AtomicMemoryOrderArgument`, with expected type `MemoryOrder`.

Apply ordinary typed semantic CTE. Enum variants and compile-time-established
immutable bindings qualify. Runtime-dependent parameters, mutable state and
volatile reads do not. An immutable binding is insufficient without a
compile-time-established value. A missing CTE execution implementation is
reported as an implementation limitation, rather than an invalid enum value.

The compiler establishes the value before validating the existing operation
matrix and selected target capability. It must not select the order at runtime
or substitute another order.

## 2. Integration

The decision is integrated into `atomics.md` §24(4–6) and §45(5).
The operation/order matrix, omitted `SeqCst` defaults, mandatory explicit fence
order and independent CompareExchange success/failure orders are unchanged.
In particular, a valid failure order need not be weaker than the success order.
The existing IPC compile-time order requirement remains in force.
