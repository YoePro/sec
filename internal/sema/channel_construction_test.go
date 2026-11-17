package sema

import (
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Channel construction is fallible only for allocation: both the capacity
// form and the ChannelOptions form yield Result[Channel[T], AllocationError]
// (no ChannelCreateError), and invalid compile-time-known options are
// compile-time diagnostics: MaxSenders must be written in the literal at the
// construction, compile-time-known and at least 1; Capacity may be runtime.
//
// Rules:
//   - rules/concurrency/channels.md — § 6(2)–(6), § 19(1)–(4), § 20
func TestChannelConstructionIsFallibleAndValidatesOptions(t *testing.T) {
	analyze := func(body string) []Error {
		t.Helper()
		source := "module main\n\nfn F(capacity: uint, n: uint, options: ChannelOptions) Result[void, AllocationError] {\n" + body + "\n    return Ok()\n}\n"
		parsed := parser.New(lexer.NewWithFile(source, "channel.sec")).Parse()
		if parsed.HasErrors {
			t.Fatal(parsed.Diagnostics)
		}
		return NewAnalyzer().Analyze(parsed.Program)
	}
	for _, valid := range []string{
		"    let channel := try Channel[int](4)\n    let tx :<- channel.Tx\n    discard tx",
		"    let channel := try Channel[int](ChannelOptions { Capacity: capacity, MaxSenders: 8 })\n    discard channel",
		"    let result: Result[Channel[int], AllocationError] := Channel[int](4)\n    discard result",
	} {
		if errors := analyze(valid); len(errors) != 0 {
			t.Errorf("%q: %v", valid, errors)
		}
	}
	for body, want := range map[string]string{
		"    let channel := try Channel[int](ChannelOptions { Capacity: 4, MaxSenders: 0 })\n    discard channel": "ChannelOptions.MaxSenders must be at least 1, got 0",
		"    let channel := try Channel[int](ChannelOptions { Capacity: 4 })\n    discard channel":                "ChannelOptions.MaxSenders must be at least 1",
		"    let channel := try Channel[int](ChannelOptions { Capacity: 4, MaxSenders: n })\n    discard channel": "MaxSenders must be compile-time-known",
		"    let channel := try Channel[int](options)\n    discard channel":                                       "must be compile-time-known at channel construction",
		"    let channel := Channel[int](4)\n    let tx :<- channel.Tx\n    discard tx":                           "unknown member Tx on Result[Channel[int], AllocationError]",
	} {
		errors := analyze(body)
		found := false
		for _, err := range errors {
			if strings.Contains(err.Message, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%q: want %q, got %v", body, want, errors)
		}
	}
}

// Sender.Share returns Option[Sender[T]]: Some is a new sender capability,
// None means MaxSenders is reached, and the original sender is not consumed
// either way, so it stays usable after Share.
//
// Rules:
//   - rules/concurrency/channels.md — § 16(3), § 24(2)–(5)
func TestSenderShareReturnsOption(t *testing.T) {
	analyze := func(body string) []Error {
		t.Helper()
		source := "module main\n\nfn F() Result[void, AllocationError] {\n    let channel := try Channel[int](4)\n    let tx :<- channel.Tx\n" + body + "\n    return Ok()\n}\n"
		parsed := parser.New(lexer.NewWithFile(source, "share.sec")).Parse()
		if parsed.HasErrors {
			t.Fatal(parsed.Diagnostics)
		}
		return NewAnalyzer().Analyze(parsed.Program)
	}
	valid := "    match tx.Share() {\n        Some(second) => {\n            discard second\n        }\n        None => {\n        }\n    }\n    let typed: Option[Sender[int]] := tx.Share()\n    discard typed\n    discard tx"
	if errors := analyze(valid); len(errors) != 0 {
		t.Fatalf("valid Share use rejected: %v", errors)
	}
	errors := analyze("    let second := tx.Share()\n    discard second.Send(1)\n    discard tx")
	found := false
	for _, err := range errors {
		if strings.Contains(err.Message, "second.Send") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Send on an unwrapped Share result was accepted or misreported: %v", errors)
	}
}
