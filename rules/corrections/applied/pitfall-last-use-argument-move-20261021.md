# Final-use large-argument move advisory

- Status: Applied 2026-10-21
- Authority: explicit language-owner request and clarification, 2026-10-21
- Targets: `rules/analysis/pitfall_analysis.md`, `rules/memory/copy_move.md`

Pitfall analysis gains `pitfall.ownership.last-use-argument-copy`: selected
ordinary value copies of large aggregates may recommend `<-name` when complete
binding-use and ownership/dependency facts support the final argument use.
The selected initializer supplies the same contract for `new`. Cost estimates
are advisory, not layout or language semantics. Later arguments, error handlers,
borrowed aliases, loops, captures and deferred uses must prevent unsupported
last-use claims; incomplete evidence stays silent.

`copy_move.md` §§8.1(4–5) explicitly permit caller-chosen move into an ordinary
owning value parameter, independently of whether the type is copyable. The
parameter's default remains copy; `->` still makes transfer mandatory. This
clarifies existing explicit-argument handling and introduces no implicit move.
Move availability, borrowing and call-entry commit rules are unchanged.

The suggested edit is manual. No automatic rewrite, source-order change,
new pointer representation or alternate compilation path is authorized.
Implementation status and evidence belong to `sema.pitfall-analysis` in
`governance/analysis.yaml`; no existing blocked decision is declared resolved.
