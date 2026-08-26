# Correction: remove whole-self consuming interface receivers

- **Status:** Normative correction
- **Created:** 2026-08-26
- **Last updated:** 2026-08-26
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/declarations/interfaces.md`

---

## Correction

Synchronize interface receiver contracts with `rules/memory/ownership.md`
revision 2.0.

### Remove consuming instance receiver mode

Ordinary interface instance methods must not declare a receiver contract that
consumes the complete implementing value. Remove `-> fn` as an instance-receiver
method kind.

Interface methods may require the receiver access modes supported by the normal
shared/mutable receiver model, and static functions remain unaffected.
Consuming parameters inside an interface method signature remain valid:

```sec
interface Sink {
    fn Write(-> buffer: Buffer) Result[void, IOError]
}
```

A reusable source passed to such a consuming parameter must use the canonical
call-site move marker:

```sec
sink.Write(<-buffer)
```

The prohibition concerns whole-`self` consumption by ordinary method dispatch;
it does not prohibit consuming ordinary parameters or owned members when the
receiver has sufficient authority.

### Lifetime termination

Whole-instance destruction is represented by the lifecycle/destruction model
and `free`, not by a consuming interface method receiver.

## Cross-reference

`rules/memory/ownership.md` revision 2.0 owns whole-self and call-site
consumption semantics.
