# BUG-000033 — `new T(args)` rejected inside a static method of `T`

- **Status:** fixed 2026-11-20
- **Area:** Sema — static receiver isolation
- **Code:** `internal/sema/static_receiver.go` (`staticReceiverlessCandidates`)
- **Rulebooks:** `rules/declarations/static.md` § 9;
  `sec/stdlib/net/http/std-net-http.md` § 14.2 (static request factories)

## Reproduction

```sec
type Point struct { X: int, Y: int }

impl Point {
    init(x: int, y: int) { self.X = x; self.Y = y }
    static fn Origin() Point { return new Point(0, 0) }
}
```

Reported S1195 "method @init:Point requires an instance receiver in static
method Point.Origin". A parameterless `new T()` was not affected.

## Cause and fix

The static-context filter removed every impl function that uses a receiver,
including constructors. A constructor reached through `new` creates its own
receiver, so it now stays a candidate (`@init:` prefix).

## Validation

- Regression case `StaticFactoryPoint` in
  `testdata/sema/static_receiver/receiver_valid.sec`
  (`TestStaticReceiverIsolation`).
- `go test ./internal/sema/...` and `go test ./cmd/compiler -run StaticReceiver`
  passed 2026-11-20.
- `sec/stdlib/net/http/request.sec` factories (`Request.Get` etc.) check cleanly.
