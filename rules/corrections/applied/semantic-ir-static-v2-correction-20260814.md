# Correction: static declarations in Semantic IR

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-13
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** Semantic IR rulebook

Semantic IR must preserve static semantics explicitly.

It must represent or make unambiguously derivable:

```text
StaticStorage
StaticLoad
StaticStore
StaticMethod
StaticProperty
StaticInitialize
StaticDestroy
```

Required metadata includes, where applicable:

- owner module or type;
- concrete value type;
- mutability;
- visibility;
- initialization dependency;
- generic specialization;
- concurrency requirements;
- source location.

`StaticInitialize` represents compile-time-resolved static initialization semantics.

It must not imply or require a hidden runtime startup function.

Static members must never be lowered as instance fields.
