# Sec StdLib — os/dirs

- **Status:** Draft scaffold — no API is normative yet
- **Created:** 2026-10-10
- **Updated:** 2026-10-10
- **Revision:** 0.1 (scaffold)
- **Sec version:** 0.1
- **Canonical path:** `stdlib/os/dirs/std-os-dirs.md`
- **Replaces:** None

## 1. Purpose

Standard OS-defined locations used by applications.

## 2. Scope

Home, configuration, cache, data, and temporary directory locations.

## 3. Exclusions and ownership boundaries

Filesystem I/O, path parsing, directory creation and traversal (stdlib/fs).

`sec/platform` owns native OS bindings, system-call/ABI details and
platform-specific implementations. This module owns its public Sec-facing API.
The governing `rules/library/stdlib.md` must be synchronized to the finished
module rulebook, not treated as the source of new API decisions.

## 4. API specification status

This file reserves the module and its scope. It is **not** a complete API
rulebook: no public declarations are defined by this revision. Before marking
it normative, document every public Sec type, error and member completely;
include exact Sec declarations, structured LSP documentation comments,
ownership/move behavior, allocation, blocking, cancellation, target support,
error translation and tests. Do not use invented constructors or partial APIs.

## 5. Platform integration

Use compiler-resolved target/profile capabilities and `sec/platform` bindings.
Do not make public API names depend on the native operating-system syscall.
State unsupported-target and runtime-failure behavior explicitly.

## 6. Required synchronization

When the API is completed, synchronize `rules/library/stdlib.md`, other owning
module rulebooks and their implementation-governance fragments. Do not edit
those files automatically from this scaffold.
