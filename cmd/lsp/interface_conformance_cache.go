package main

import "sync"

type interfaceConformanceFlight struct {
	done    chan struct{}
	summary []interfaceConformanceSummary
}

var interfaceConformanceCache = struct {
	sync.Mutex
	key     string
	summary []interfaceConformanceSummary
	flights map[string]*interfaceConformanceFlight
}{}

// cachedInterfaceConformance coalesces overlapping resolves of one immutable
// module snapshot. Waiters share the completed projection while distinct source
// keys may compute independently on the existing bounded worker pool. A panic
// publishes no partial report and is not retained as a successful cache entry.
// Rules: rules/tooling/lsp.md — "Interface conformance code lens",
// "Responsiveness model", "Document snapshots".
func cachedInterfaceConformance(key string, compute func() []interfaceConformanceSummary) (summary []interfaceConformanceSummary) {
	interfaceConformanceCache.Lock()
	if interfaceConformanceCache.key == key && key != "" {
		summary = interfaceConformanceCache.summary
		interfaceConformanceCache.Unlock()
		return summary
	}
	if flight := interfaceConformanceCache.flights[key]; flight != nil {
		interfaceConformanceCache.Unlock()
		<-flight.done
		return flight.summary
	}
	flight := &interfaceConformanceFlight{done: make(chan struct{})}
	if interfaceConformanceCache.flights == nil {
		interfaceConformanceCache.flights = map[string]*interfaceConformanceFlight{}
	}
	interfaceConformanceCache.flights[key] = flight
	interfaceConformanceCache.Unlock()
	defer func() {
		failed := recover() != nil
		if failed {
			summary = []interfaceConformanceSummary{}
		}
		interfaceConformanceCache.Lock()
		if !failed {
			interfaceConformanceCache.key = key
			interfaceConformanceCache.summary = summary
		}
		flight.summary = summary
		delete(interfaceConformanceCache.flights, key)
		close(flight.done)
		interfaceConformanceCache.Unlock()
	}()
	return compute()
}
