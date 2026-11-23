# BUG-000050 — Unsafe method parsing and interface safety are inconsistent

## Status

Fixed, 2026-11-23.

## Reproduction and owning rules

An ordinary `unsafe fn Read() int` inside an impl became an invalid member,
while an attributed unsafe method used the general attribute parser and was
retained correctly. The new source-backed conformance test initially produced
invalid-member and missing-method errors instead of safety incompatibility.

Canonical method compatibility also ignored Function.Unsafe. An attributed
unsafe method could therefore witness a safe interface requirement and erase
caller obligations through interface/generic use. A temporary Go overlay
restoring that comparison reproduces accepted incompatible conformance.

`rules/memory/unsafe.md` §§6–7,39–41 requires unsafe methods and preservation
of caller obligations. `rules/declarations/interfaces.md` §§6,9,12 owns the
conformance result and diagnostic. Governance owner: `frontend.unsafe`.

## Resolution and verification

Parse ordinary unsafe impl methods through the same declaration parser as the
attributed form. Retain their marker, body and documentation. The grammar's
ImplMember list now includes the method form already required by unsafe.md §6;
foreign declarations and runtime unsafe statements remain invalid impl members.

The shared conformance comparator rejects an unsafe method for a safe
requirement and reports InterfaceMemberIncompatible with both declaration
locations and a focused caller-obligation explanation. Safe implementations
remain admissible for a requirement carrying unsafe obligations.

Tests cover direct/inherited/generic conformance, plain/attributed methods,
ordinary/generic/overloaded functions, unsafe extern calls, explicit declaration
bodies, nested argument/receiver authorization, safe wrappers and unsafe function
value rejection. CLI gates emit no artifact for invalid source; LSP uses the
same diagnostics. Broader caller-obligation proof and future unsafe interface
source forms are outside this fix and remain separately tracked.
