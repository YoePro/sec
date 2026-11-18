package main

import (
	"os"
	"testing"
)

// TestMD131AssignmentLSP preserves A2011's default Information level and keeps
// the noPanic failure mandatory; retired S1069 is never emitted.
// Rules: rules/types/contracts.md §12(2–3); diagnostics.md §24(4k); panic.md §21.
func TestMD131AssignmentLSP(t *testing.T) {
	for _, mode := range []string{"valid", "invalid"} {
		source, err := os.ReadFile("../../testdata/sema/md131_assignment_" + mode + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		advisory, failures := 0, 0
		for _, d := range analyze("file:///md131-assignment/main.sec", string(source)) {
			switch d.Code {
			case "A2011":
				advisory++
				if d.Severity != 3 {
					t.Fatal(d)
				}
			case "S1092":
				failures++
				if d.Severity != 1 {
					t.Fatal(d)
				}
			default:
				t.Fatal(d)
			}
		}
		want := 0
		if mode == "invalid" {
			want = 1
		}
		if advisory != 1 || failures != want {
			t.Fatal(mode, advisory, failures)
		}
	}
}
