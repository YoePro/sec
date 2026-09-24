package main

import (
	"strings"
	"testing"
)

// Result try hover consumes the exact carrier and propagation facts recorded
// by Sema, both at the try keyword and at the protected callable.
//
// Rules:
//   - rules/tooling/lsp.md — "Hover", try and protected-operand hover
//   - rules/errors/errorhandling.md — § 12.1 and § 37.10
func TestTryHoverShowsResolvedResultPropagationAtKeywordAndOperand(t *testing.T) {
	source := `module main

enum ReadError error {
    Failed,
}

fn Read() Result[int, ReadError] {
    return Err(ReadError.Failed)
}

fn Use() Result[int, error] {
    let value := try Read()
    return Ok(value)
}
`
	tryStart := strings.Index(source, "try Read()")
	for name, offset := range map[string]int{
		"keyword": tryStart + 1,
		"operand": tryStart + len("try "),
	} {
		t.Run(name, func(t *testing.T) {
			hover, ok := hoverForSource("", source, offsetPosition(source, offset))
			if !ok || !strings.Contains(hover.Contents.Value, "Protected carrier: `Result[int, ReadError]`") ||
				!strings.Contains(hover.Contents.Value, "Success value: `int`") ||
				!strings.Contains(hover.Contents.Value, "Failure handling: `propagated`") ||
				!strings.Contains(hover.Contents.Value, "Propagated error: `ReadError`") ||
				!strings.Contains(hover.Contents.Value, "Propagation target: `Result[int, error]`") ||
				!strings.Contains(hover.Contents.Value, "Err consumed by: `enclosing function return`") {
				t.Fatalf("try propagation hover = %+v, %v", hover, ok)
			}
		})
	}
}

// Local try hover presents the handler plan selected by Sema and does not
// mistake it for bodyless propagation.
//
// Rules:
//   - rules/tooling/lsp.md — "Hover", try and protected-operand hover
//   - rules/errors/errorhandling.md — §§ 15–16 and § 37.10
func TestTryHoverDistinguishesResolvedLocalHandling(t *testing.T) {
	source := `module main

enum ReadError error {
    Failed,
}

fn Read() Result[int, ReadError] {
    return Err(ReadError.Failed)
}

fn Use() int {
    return try Read() {
        Err(_) => 0
    }
}
`
	offset := strings.LastIndex(source, "Read()")
	hover, ok := hoverForSource("", source, offsetPosition(source, offset))
	if !ok || !strings.Contains(hover.Contents.Value, "Protected carrier: `Result[int, ReadError]`") ||
		!strings.Contains(hover.Contents.Value, "Failure handling: `local try handlers`") ||
		!strings.Contains(hover.Contents.Value, "Error channel: `ReadError`") ||
		!strings.Contains(hover.Contents.Value, "Err consumed by: `local handler`") ||
		!strings.Contains(hover.Contents.Value, "Handler coverage: `exhaustive`") ||
		!strings.Contains(hover.Contents.Value, "Resolved handlers: `1`") ||
		strings.Contains(hover.Contents.Value, "Propagation target") {
		t.Fatalf("local try hover = %+v, %v", hover, ok)
	}
}
