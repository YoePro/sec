package main

import (
	"os"
	"strings"
	"testing"
)

// TestThreadSpawnResultLSP displays the canonical raw creation layer and its
// extracted owner, with the same creation/lifecycle errors as the compiler.
// Rules: rules/concurrency/threads.md — §§2,19,34;
// rules/tooling/lsp.md — Diagnostics, Hover, One semantic source of truth.
func TestThreadSpawnResultLSP(t *testing.T) {
	uri := "file:///thread-spawn-result/main.sec"
	for _, mode := range []string{"valid", "invalid"} {
		source, err := os.ReadFile("../../testdata/sema/thread_spawn_result_" + mode + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, d := range analyze(uri, string(source)) {
			if d.Severity == 1 {
				count++
			}
		}
		want := 0
		if mode == "invalid" {
			want = 7
		}
		if count != want {
			t.Fatal(mode, count)
		}
		if mode != "valid" {
			continue
		}
		text := string(source)
		for _, item := range []struct{ marker, name, detail string }{{"match creation", "creation", "Result[Thread[int], ThreadSpawnError]"}, {"join worker", "worker", "Thread[int]"}} {
			offset := strings.LastIndex(text, item.marker) + strings.Index(item.marker, item.name) + 1
			hover, ok := hoverForSource(uri, text, offsetPosition(text, offset))
			if !ok || !strings.Contains(hover.Contents.Value, item.detail) {
				t.Fatal(item, hover, ok)
			}
		}
	}
}
