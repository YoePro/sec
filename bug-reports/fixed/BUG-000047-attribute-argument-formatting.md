# BUG-000047 — Attribute arguments bypass canonical attachment layout

## Status

Fixed, 2026-11-23.

## Reproduction and expected behavior

A declaration such as `@when(true) @noPanic unsafe fn F() void {}` keeps
argument-bearing attributes on the declaration line. Init/free, accessor and
lambda-binding attributes also bypassed the general CST attachment roles.
A temporary Go overlay restoring the original CST role producer reproduces
the failure in TestGeneralAttributeFormatting.

`rules/tooling/formatter.md` §16(14–17) requires one attribute per line,
source order, ordinary call layout for long arguments and immediate attachment.
Owner: `frontend.attributes-v2`.

## Resolution and evidence

The parser retains real argument delimiters. A focused CST role producer
covers supported declaration and callable-body attachments, including modifier
prefixes. Complete argument groups use the shared comma-list layout and width
handling. Malformed lists receive no attribute layout roles; existing recovery
regions and intervening comments remain intact. Formatter tests cover 2/4/6-space
indentation, argument width, source order and idempotence. CLI and LSP use the
same golden output, and a second editor request produces no edit.
