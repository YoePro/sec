# Correction: interface receiver capability and ownership

- **Status:** Applied 2026-08-13
- **Created:** 2026-08-13
- **Last updated:** 2026-08-13
- **Document revision:** 1
- **Sec language version:** 0.1

Synchronize ownership and borrowing rules with interface receiver contracts.

## Receiver capability

Interface declarations use:

```sec
fn Read() Data
mut fn Update() void
-> fn Consume() ResultData
```

Interpretation:

- `fn`: callable through shared, mutable, or owned receiver;
- `mut fn`: callable through mutable or owned receiver, not shared;
- `-> fn`: callable only when the caller owns the receiver.

`ref mut` grants exclusive mutable access but does not transfer ownership. Therefore a consuming interface method cannot be called through `ref mut`.

After a successful consuming call through an owned receiver, the receiver is consumed and cannot be used again.

Concrete methods do not spell these modifiers. Sema infers receiver usage from the method body and validates it against the interface contract.
