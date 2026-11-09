package sema

import (
	"strings"
	"testing"
)

// Channel endpoints reached through `ref`/`ref mut` use the same members as
// owned ones, also as select operations; only Close consumes the endpoint
// and needs an owned one. TrySend returns ChannelTrySendResult[T] with
// Sent, WouldBlock(T) and Closed(T) (channels.md §§ 9–10).
func TestChannelCallsThroughReferences(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

fn Refs(rx: ref mut Receiver[int], tx: ref mut Sender[int]) int {
    let received := rx.Receive()
    let polled := rx.TryReceive()
    let sent := tx.Send(1)
    select {
        value := rx.Receive() => {
            discard value
        }
        tx.Send(2) => {
        }
    }
    match tx.TrySend(3) {
        Sent => {
            return 0
        }
        WouldBlock(value) => {
            return value
        }
        Closed(value) => {
            return -value
        }
    }
}
`)
	for _, err := range errors {
		if !strings.Contains(err.Message, "must be handled") && !strings.Contains(err.Message, "discard") {
			t.Fatalf("unexpected diagnostic: %v", errors)
		}
	}
	closeErrors := analyzeSourceRaw(t, "module main\n\nfn F(rx: ref mut Receiver[int]) void {\n    rx.Close()\n}\n")
	if len(closeErrors) != 1 || !strings.Contains(closeErrors[0].Message, "Receiver.Close requires an owned Receiver[int], got a reference") {
		t.Fatalf("Close through a reference: %v", closeErrors)
	}
}
