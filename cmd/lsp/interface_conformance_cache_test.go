package main

import (
	"sync"
	"sync/atomic"
	"testing"
)

// Many pending lenses share one computation, with immutable publication and no
// cached successful projection of a failed computation.
// Rules: rules/tooling/lsp.md — "Interface conformance code lens", "Document snapshots".
func TestInterfaceConformanceCacheCoalescesAndRecovers(t *testing.T) {
	const key = "coalescing-test-snapshot"
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	compute := func() []interfaceConformanceSummary {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return []interfaceConformanceSummary{{name: "Native"}}
	}
	var ready, done sync.WaitGroup
	ready.Add(8)
	done.Add(8)
	for i := 0; i < 8; i++ {
		go func() {
			defer done.Done()
			ready.Done()
			summary := cachedInterfaceConformance(key, compute)
			if len(summary) != 1 || summary[0].name != "Native" {
				t.Errorf("summary=%+v", summary)
			}
		}()
	}
	<-started
	ready.Wait()
	close(release)
	done.Wait()
	if calls.Load() != 1 {
		t.Fatalf("computations=%d", calls.Load())
	}
	failed := cachedInterfaceConformance("panic-test-snapshot", func() []interfaceConformanceSummary { panic("failed projection") })
	if len(failed) != 0 {
		t.Fatal(failed)
	}
	recovered := cachedInterfaceConformance("panic-test-snapshot", func() []interfaceConformanceSummary { return []interfaceConformanceSummary{{name: "Recovered"}} })
	if len(recovered) != 1 || recovered[0].name != "Recovered" {
		t.Fatal(recovered)
	}
}
