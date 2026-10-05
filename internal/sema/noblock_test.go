package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

const noBlockSource = `module main

type Message struct {
	value: int,
}

@noBlock
fn Poll(tx: Sender[Message], rx: Receiver[Message]) void {
	discard tx.TrySend(Message{ value: 1 })
	discard rx.TryReceive()
	select {
		message := rx.Receive() => {
			discard message
		}
		default => {
		}
	}
}

@noBlock
fn Receive(rx: Receiver[Message]) void {
	discard rx.Receive()
}

@noBlock
fn Await(work: Task[int]) void {
	discard await work
}

@noBlock
fn Wait(rx: Receiver[Message]) void {
	select {
		message := rx.Receive() => {
			discard message
		}
	}
}

fn Helper() void {
	let channel := Channel[Message](1)
	let tx := channel.tx
	discard tx.Send(Message{ value: 2 })
}

@noBlock
fn Indirect() void {
	Helper()
}

extern "C" fn native_wait() int32

@noBlock
extern "C" fn native_poll() int32

@noBlock
fn Foreign() int32 {
	return native_wait()
}

@noBlock
fn TrustedForeign() int32 {
	return native_poll()
}

@noBlock
fn Callback(action: fn() void) void {
	action()
}

@noBlock
fn Locks(state: Mutex[int]) void {
	discard state.tryLock()
	discard state.lock()
}
`

// @noBlock is verified transitively over synchronous calls: represented
// waiting operations (Receive, Send, await, select without default), extern
// calls without a trusted @noBlock contract, and calls with unknown targets
// violate it, while TrySend, TryReceive, a select with default, and a
// trusted extern do not.
//
// Rules:
//   - rules/foundations/attributes.md — "@noBlock", "Blocking operations", "What @noBlock does not automatically forbid", "@noBlock verification"
//   - rules/concurrency/blocking.md — "Potentially blocking operations", "Nonblocking operations", "Blocking effects", "FFI"
func TestNoBlockGuaranteeIsVerifiedTransitively(t *testing.T) {
	errors := analyzeSourceRaw(t, noBlockSource)
	violations := map[string]string{}
	for _, err := range errors {
		if err.ID != "S1116" {
			t.Errorf("unexpected error: %s", err.Message)
			continue
		}
		name := strings.Fields(strings.TrimPrefix(err.Message, "function "))[0]
		violations[name] = err.Message
		if err.Help == "" {
			t.Errorf("%s has no help", name)
		}
	}
	for name, want := range map[string]string{
		"Receive":  "reachable blocking operation Receiver.Receive via Receive",
		"Await":    "reachable blocking operation await via Await",
		"Wait":     "reachable blocking operation select without default via Wait",
		"Indirect": "reachable blocking operation Sender.Send via Indirect -> Helper",
		"Foreign":  "reachable foreign call with unknown blocking behavior via Foreign",
		"Callback": "reachable call with unknown blocking behavior via Callback",
		"Locks":    "reachable blocking operation Mutex.lock via Locks",
	} {
		if !strings.Contains(violations[name], want) {
			t.Errorf("%s violation = %q, want %q", name, violations[name], want)
		}
	}
	for _, name := range []string{"Poll", "TrustedForeign"} {
		if message, found := violations[name]; found {
			t.Errorf("%s must satisfy @noBlock: %s", name, message)
		}
	}
}

// The implication graph derives effective guarantees without synthesizing
// attributes: @interrupt implies @isr, and @isr and @interruptSafe imply
// @noPanic, @noAlloc, and @noBlock. The verifiers check effective
// guarantees and name the implication in the diagnostic.
//
// Rules:
//   - rules/foundations/attributes.md — "Attribute implications", "Redundant attributes", "ISR verification", "Interrupt-safe guarantee"
func TestAttributeImplicationGraph(t *testing.T) {
	attributes := func(names ...string) []*ast.Attribute {
		result := []*ast.Attribute{}
		for _, name := range names {
			result = append(result, &ast.Attribute{Token: lexer.Token{Lexeme: "@"}, Name: &ast.Identifier{Value: name}})
		}
		return result
	}
	describe := func(guarantees []EffectiveGuarantee) string {
		parts := []string{}
		for _, guarantee := range guarantees {
			part := string(guarantee.Guarantee)
			if !guarantee.Explicit() {
				part += "<" + strings.Join(guarantee.ImpliedBy, "<")
			}
			parts = append(parts, part)
		}
		return strings.Join(parts, ",")
	}
	for names, want := range map[string]string{
		"interrupt":        "isr<interrupt,noPanic<interrupt<isr,noAlloc<interrupt<isr,noBlock<interrupt<isr",
		"interruptSafe":    "interruptSafe,noPanic<interruptSafe,noAlloc<interruptSafe,noBlock<interruptSafe",
		"noPanic,isr":      "noPanic,isr,noAlloc<isr,noBlock<isr",
		"noAlloc":          "noAlloc",
		"interrupt,isr":    "isr,noPanic<isr,noAlloc<isr,noBlock<isr",
		"noCopy,link_name": "",
	} {
		if got := describe(EffectiveGuarantees(attributes(strings.Split(names, ",")...))); got != want {
			t.Errorf("%s effective guarantees = %s, want %s", names, got, want)
		}
	}

	errors := analyzeSourceRaw(t, `module main

type Message struct {
	value: int,
}

@interruptSafe
fn Status(rx: Receiver[Message], values: int[4], index: int) int {
	discard rx.Receive()
	return values[index]
}

@interrupt(vector: 3)
fn Handler(rx: Receiver[Message]) void {
	discard rx.Receive()
}
`)
	messages := []string{}
	for _, err := range errors {
		messages = append(messages, err.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, want := range []string{
		"function Status does not satisfy @noPanic (implied by @interruptSafe)",
		"function Status does not satisfy @noBlock (implied by @interruptSafe)",
		"function Handler does not satisfy @noBlock (implied by @interrupt via @isr)",
		"attribute @interruptSafe is not implemented yet",
		"attribute @interrupt is not implemented yet",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in\n%s", want, joined)
		}
	}
}
